// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The files the Upgrading part reads at the previous tag and at HEAD
// (12 12.6, REQ-1175): the generated error table, which is the union of
// the per-package errors.go declarations (04 4.1); the format versions;
// and the embed directive of the product kits, whose directories hold
// the files the kit digest covers (06 6.2).
const (
	errorsPage   = "docs/reference/errors.md"
	versionsFile = "internal/spec/versions.go"
	kitsEmbed    = "kits/embed.go"
	kitsDir      = "kits"
)

// Upgrading returns the Upgrading part of the section of a tag: the
// error ids and exit codes added or changed, the format versions
// changed and the kits whose files changed, so whose projects need a
// recreate, between prev, the previous tag, and HEAD. With no previous
// tag it says so in one line.
func Upgrading(ctx context.Context, root, prev string) (string, error) {
	const head = "### Upgrading\n\n"
	if prev == "" {
		return head + "There is no previous tag: this is the first release, so nothing is compared.", nil
	}
	before, err := readState(ctx, root, prev)
	if err != nil {
		return "", err
	}
	after, err := readState(ctx, root, "HEAD")
	if err != nil {
		return "", err
	}
	var lines []string
	if added := after.errors.added(before.errors); len(added) != 0 {
		lines = append(lines, "- Error ids added: "+strings.Join(added, ", ")+".")
	}
	if changed := after.errors.changed(before.errors); len(changed) != 0 {
		lines = append(lines, "- Exit codes changed: "+strings.Join(changed, ", ")+".")
	}
	if changed := versionsChanged(before.versions, after.versions); len(changed) != 0 {
		lines = append(lines, "- Format versions changed: "+strings.Join(changed, ", ")+".")
	}
	if kits := kitsChanged(before.kits, after.kits); len(kits) != 0 {
		lines = append(lines, "- Kits whose projects need a recreate: "+strings.Join(kits, ", ")+".")
	}
	if len(lines) == 0 {
		return head + "Nothing to do: no error id or exit code was added or changed, no format version changed and no kit changed since " + prev + ".", nil
	}
	return head + strings.Join(lines, "\n"), nil
}

// state is what the three sources hold at one commit.
type state struct {
	errors   errorTable
	versions map[string]int
	kits     map[string]string
}

// readState reads the three sources at commit; a source the commit does
// not hold is empty.
func readState(ctx context.Context, root, commit string) (state, error) {
	var s state
	page, _, err := show(ctx, root, commit, errorsPage)
	if err != nil {
		return s, err
	}
	s.errors = parseErrors(page)
	src, _, err := show(ctx, root, commit, versionsFile)
	if err != nil {
		return s, err
	}
	if s.versions, err = parseVersions(src); err != nil {
		return s, fmt.Errorf("%s at %s: %w", versionsFile, commit, err)
	}
	embed, _, err := show(ctx, root, commit, kitsEmbed)
	if err != nil {
		return s, err
	}
	patterns, err := embedPatterns(embed)
	if err != nil {
		return s, fmt.Errorf("%s at %s: %w", kitsEmbed, commit, err)
	}
	if s.kits, err = kitTrees(ctx, root, commit, patterns); err != nil {
		return s, err
	}
	return s, nil
}

// errorRow is one row of the error table: the slug and the exit code
// of an id.
type errorRow struct {
	slug string
	exit string
}

// errorTable is the error table by id.
type errorTable map[string]errorRow

// errorsRow matches a row of the generated error table: the id, the
// slug and the exit code, the first three cells.
var errorsRow = regexp.MustCompile("(?m)^\\| `(RJ-[0-9]+)` \\| `([^`]+)` \\| ([0-9]+) \\|")

// parseErrors reads the rows of the error table from the page.
func parseErrors(page []byte) errorTable {
	t := errorTable{}
	for _, m := range errorsRow.FindAllSubmatch(page, -1) {
		t[string(m[1])] = errorRow{slug: string(m[2]), exit: string(m[3])}
	}
	return t
}

// added lists the ids of t that before lacks, as "`id` slug (exit n)",
// in id order. An id that before holds and t lacks is not listed:
// REQ-1175 asks for what was added or changed, and an id is never
// reused (04 4.1).
func (t errorTable) added(before errorTable) []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(t)) {
		if _, ok := before[id]; !ok {
			out = append(out, fmt.Sprintf("`%s` %s (exit %s)", id, t[id].slug, t[id].exit))
		}
	}
	return out
}

// changed lists the ids of t whose exit code differs from before's, as
// "`id` slug (exit m to n)", in id order.
func (t errorTable) changed(before errorTable) []string {
	var out []string
	for _, id := range slices.Sorted(maps.Keys(t)) {
		if old, ok := before[id]; ok && old.exit != t[id].exit {
			out = append(out, fmt.Sprintf("`%s` %s (exit %s to %s)", id, t[id].slug, old.exit, t[id].exit))
		}
	}
	return out
}

// parseVersions reads the integer constants whose name ends in
// "Version" from the Go source src: the n of each "<name>/v<n>" field
// (03, opening). An empty src is no constant.
func parseVersions(src []byte) (map[string]int, error) {
	versions := map[string]int{}
	if len(src) == 0 {
		return versions, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), versionsFile, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if !strings.HasSuffix(name.Name, "Version") || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}
				n, err := strconv.Atoi(lit.Value)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", name.Name, err)
				}
				versions[name.Name] = n
			}
		}
	}
	return versions, nil
}

// versionsChanged lists the constants whose value differs, as
// "<name> <m> to <n>", in name order.
func versionsChanged(before, after map[string]int) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if old, ok := before[name]; ok && old != after[name] {
			out = append(out, fmt.Sprintf("%s %d to %d", name, old, after[name]))
		}
	}
	return out
}

// embedDirective matches the go:embed directive of kits/embed.go, whose
// patterns are the kit directories, or the descriptor of a kit whose
// directory holds a testdata/.
var embedDirective = regexp.MustCompile(`(?m)^//go:embed (.+)$`)

// embedPatterns returns the patterns of every directive, each relative
// to the kits directory; a missing file is no pattern. A pattern this
// reader does not follow, one with the all: prefix, a quote or a glob
// character, is an error, so a kit never drops out of the comparison
// unseen.
func embedPatterns(src []byte) ([]string, error) {
	var patterns []string
	for _, m := range embedDirective.FindAllSubmatch(src, -1) {
		for pattern := range strings.FieldsSeq(string(m[1])) {
			if strings.ContainsAny(pattern, "*?[:'\"`") {
				return nil, fmt.Errorf("go:embed pattern %q: this reader follows a plain directory or file name only", pattern)
			}
			patterns = append(patterns, pattern)
		}
	}
	return patterns, nil
}

// kitTrees returns, for each kit the patterns name (its first path
// segment), the sorted entries of its embedded files at commit, "mode
// blob path" per line, which is what the kit digest covers (03 3.12:
// path, mode and content), as git ls-tree lists them. A directory
// pattern embeds its tree without the names that begin with "." or
// "_", as go:embed does.
func kitTrees(ctx context.Context, root, commit string, patterns []string) (map[string]string, error) {
	kits := map[string]string{}
	if len(patterns) == 0 {
		return kits, nil
	}
	out, err := git(ctx, root, "ls-tree", "-r", "--full-tree", commit, "--", kitsDir)
	if err != nil {
		return nil, err
	}
	entries := map[string][]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("git ls-tree: unexpected line %q", line)
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree: unexpected line %q", line)
		}
		rel := strings.TrimPrefix(path, kitsDir+"/")
		for _, pattern := range patterns {
			if !embeds(pattern, rel) {
				continue
			}
			kit, _, _ := strings.Cut(pattern, "/")
			entries[kit] = append(entries[kit], fields[0]+" "+fields[2]+" "+rel)
		}
	}
	for kit, lines := range entries {
		slices.Sort(lines)
		kits[kit] = strings.Join(lines, "\n")
	}
	return kits, nil
}

// embeds reports whether the go:embed pattern, a file or a directory
// below kits/, embeds the file rel: the file itself, or a file below
// the directory with no segment that begins with "." or "_".
func embeds(pattern, rel string) bool {
	if rel == pattern {
		return true
	}
	below, ok := strings.CutPrefix(rel, pattern+"/")
	if !ok {
		return false
	}
	for seg := range strings.SplitSeq(below, "/") {
		if strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_") {
			return false
		}
	}
	return true
}

// kitsChanged lists the kits whose tree differs or that are new, in
// name order; a kit that before holds and after lacks has no project to
// recreate, so it is not listed.
func kitsChanged(before, after map[string]string) []string {
	var out []string
	for _, kit := range slices.Sorted(maps.Keys(after)) {
		if before[kit] != after[kit] {
			out = append(out, kit)
		}
	}
	return out
}
