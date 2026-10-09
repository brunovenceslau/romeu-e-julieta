// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// runGenerated is the "generated" subcommand (10 10.2): it first checks
// that the tree holds no go:generate line but the one of the repository
// (checkGenerateDirectives), then runs
// "go generate ./..." with the go command that mise.lock locks and
// fails when that changed, created or removed a file of the working
// tree, or when a file that generate writes is not tracked as a regular
// file with the bytes that generate gives.
func runGenerated(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generated takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	// Before anything runs: go generate runs every directive it finds, so
	// the tree must hold the one directive of the repository first.
	problems, err := checkGenerateDirectives(root)
	if err != nil {
		return false, err
	}
	if len(problems) != 0 {
		say(e.stdout, "%s%s\n", lines(problems), summary("generated", len(problems)))
		return false, nil
	}
	goBin, err := whichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return false, err
	}
	goDir := filepath.Dir(goBin)
	gen := step{name: "go generate", argv: []string{goBin, "generate", "./..."}, environ: stepEnv(goDir)}
	repo := e.repo()
	repo.Dir = root
	findings, err := generated(ctx, repo, gen, generateOutputs)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("generated", len(findings)))
	return len(findings) == 0, nil
}

// generated runs gen in the root of repo and compares the working tree
// before and after, by content. The tree is the tracked files and the
// untracked ones that are not ignored, so a changed, a new and a removed
// file each fail, and a tree with uncommitted work can be checked.
//
// That comparison cannot see a generated file that is the same before
// and after: one that is ignored, untracked, or a symbolic link to a copy
// of the right bytes. The workflow's checkout would hold something else,
// so wanted, when it is not nil, names the files that gen writes and the
// bytes it gives them, and each must be tracked in the index as a
// regular file (mode 100644) with exactly those bytes and match no ignore
// rule. The index is what a commit takes, so a file is judged as the
// merged tree holds it.
func generated(ctx context.Context, repo git.Repo, gen step, wanted func(*os.Root) ([]output, error)) ([]finding, error) {
	root, err := os.OpenRoot(repo.Dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := snapshot(ctx, repo, root)
	if err != nil {
		return nil, err
	}
	if out, err := gen.run(ctx, repo.Dir); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", gen.name, err, out)
	}
	after, err := snapshot(ctx, repo, root)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for _, path := range slices.Sorted(maps.Keys(before)) {
		switch had, has := before[path], after[path]; {
		case has == "":
			findings = append(findings, finding{path, "generated", "go generate removed this file; commit the removal"})
		case has != had:
			findings = append(findings, finding{path, "generated", "go generate changed this file; run it and commit the result"})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if _, had := before[path]; !had {
			findings = append(findings, finding{path, "generated", "go generate created this file; run it and commit the result"})
		}
	}
	if wanted != nil {
		outs, err := wanted(root)
		if err != nil {
			return nil, err
		}
		tracked, err := trackedOutputs(ctx, repo, outs)
		if err != nil {
			return nil, err
		}
		findings = append(findings, tracked...)
		converted, err := attributeProblems(ctx, repo, outs)
		if err != nil {
			return nil, err
		}
		findings = append(findings, converted...)
	}
	slices.SortStableFunc(findings, func(a, b finding) int { return strings.Compare(a.where, b.where) })
	return findings, nil
}

// attributesPath is the base name of the files whose lines set
// attributes. The root one lists the outputs of generate, one
// "<path> -text" line each, so that no line-ending rule converts them.
const attributesPath = ".gitattributes"

// checkoutAttrs are the attributes that can change the bytes of a file
// between the blob and the checkout: the line-ending ones (text, eol),
// the keyword expansion of ident, a filter (smudge) and the encoding of
// the working tree. A generated file has text unset and the rest
// unspecified.
var checkoutAttrs = []string{"text", "eol", "ident", "filter", "working-tree-encoding"}

// attributeProblems holds the list in the root .gitattributes and the
// outputs of generate to each other, and then asks git what attribute
// each output really has.
//
// The list is compared in both directions: an output with no
// "<path> -text" line fails, and so does a "-text" line that names a
// path generate does not write. Only a line whose pattern is a path
// written out in full counts as a list entry; any other "-text" line
// (a glob, say) is not on the list, and so it fails. The list uses bare
// paths with an explicit -text; a macro line ("[attr]name ...") is not an
// entry.
//
// The list alone guarantees nothing, because a later line, a
// .gitattributes in a directory or a rule in a clone's
// .git/info/attributes can set the attribute again. So the effective
// "text" attribute of each output is read with "git check-attr
// --cached": that reads the .gitattributes files of the index, nested
// ones included, which is what a commit takes and what a fresh checkout
// holds, in the same way trackedOutputs judges the index and not the
// working tree. It also honours .git/info/attributes of this clone, which
// outranks the tracked files and which no checkout carries, so such a
// rule fails here on the machine that holds it. Each output must be
// "unset", and the others of checkoutAttrs unspecified. The attribute
// files of the user and of the system rank below the tracked ones, so
// they cannot undo a "-text" the tracked files set. What the effective
// check cannot see, a string such as text=unset that reads like -text,
// is found by scanAttributes in every tracked .gitattributes and in
// info/attributes of the clone.
func attributeProblems(ctx context.Context, repo git.Repo, outs []output) ([]finding, error) {
	var findings []finding
	root, listed, valued, err := scanAttributes(ctx, repo)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, o := range outs {
		want[o.path] = true
		if !listed[o.path] {
			advice := fmt.Sprintf("add the line %q", o.path+" -text")
			if root != attributesPath {
				// Git on a case-sensitive filesystem ignores a root file in other case.
				advice = "rename it to " + attributesPath + " and " + advice
			}
			findings = append(findings, finding{o.path, "generated", root + " does not list this file with -text; " + advice})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(listed)) {
		if !want[path] {
			findings = append(findings, finding{attributesPath, "generated", fmt.Sprintf("lists %q with -text, and go generate does not write it; remove the line", path)})
		}
	}
	findings = append(findings, valued...)
	if len(outs) == 0 {
		return findings, nil
	}
	args := []string{"check-attr", "--cached", "-z"}
	args = append(args, checkoutAttrs...)
	args = append(args, "--")
	for _, o := range outs {
		args = append(args, o.path)
	}
	got, err := repo.Run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	values, err := parseCheckAttr(got, outs)
	if err != nil {
		return nil, err
	}
	for _, o := range outs {
		for _, attr := range checkoutAttrs {
			value := values[[2]string{o.path, attr}]
			want := "unspecified"
			if attr == "text" {
				want = "unset"
			}
			// An unlisted output with no text rule at all is said once, above.
			if value == want || (attr == "text" && value == "unspecified" && !listed[o.path]) {
				continue
			}
			findings = append(findings, finding{o.path, "generated", fmt.Sprintf("has the %s attribute %q, and a generated file has text unset and the others unspecified; a later line or another .gitattributes sets it", attr, value)})
		}
	}
	return findings, nil
}

// parseCheckAttr reads the output of "git check-attr -z" for outs and
// checkoutAttrs: "<path>\0<attribute>\0<value>\0", one record for each
// path and attribute. It is an error unless every output has a value
// for every attribute, under the path it was asked for.
func parseCheckAttr(got []byte, outs []output) (map[[2]string]string, error) {
	fields := strings.Split(strings.TrimSuffix(string(got), "\x00"), "\x00")
	if len(fields) != 3*len(outs)*len(checkoutAttrs) {
		return nil, fmt.Errorf("git check-attr gave %d fields for %d paths and %d attributes", len(fields), len(outs), len(checkoutAttrs))
	}
	values := map[[2]string]string{}
	for i := 0; i < len(fields); i += 3 {
		values[[2]string{fields[i], fields[i+1]}] = fields[i+2]
	}
	for _, o := range outs {
		for _, attr := range checkoutAttrs {
			if _, ok := values[[2]string{o.path, attr}]; !ok {
				return nil, fmt.Errorf("git check-attr gave no %s for %q", attr, o.path)
			}
		}
	}
	return values, nil
}

// infoAttributes is the name that "info/attributes" of the clone has in
// a finding.
const infoAttributes = "info/attributes"

// scanAttributes reads every .gitattributes of the index, the root one
// and the nested ones, and the info/attributes of the clone, and
// returns the name of the root file, the paths it lists with -text and the
// problems of the lines of all of them.
//
// A file counts when its base name is ".gitattributes" in any case,
// since git on a case-insensitive filesystem may read it, and when the index
// holds it as a regular file (mode 100644 or 100755); an entry with a
// stage other than 0 is a problem, "resolve it", and is not read. The
// file is read from its blob in the index.
//
// A line is a pattern and attributes, or a macro definition
// ("[attr]name attributes"); comment and blank lines are skipped. The
// pattern of a list entry is a bare path, so a pattern in double quotes
// (git's C-style quoting, whose escapes differ in small ways from
// Go's) is a problem, "write the path unquoted", and the line is not
// read further. The list is the pattern of each root line that has -text
// among its attributes.
//
// An attribute of checkoutAttrs with the value set, unset or
// unspecified is a problem on any line, a macro definition included:
// "git check-attr" prints that string as it prints the state, so the
// effective check cannot tell the string from the state, and a checkout
// with core.autocrlf may convert the file. Another value, such as
// text=auto, is not a problem here: on a line that matches an output
// the effective check reports it.
func scanAttributes(ctx context.Context, repo git.Repo) (string, map[string]bool, []finding, error) {
	staged, err := repo.Run(ctx, nil, "ls-files", "--stage", "-z")
	if err != nil {
		return "", nil, nil, err
	}
	// The blob of the entry of each file at stage 0, and the files that
	// have an entry at another stage.
	blobs := map[string]string{}
	unmerged := map[string]bool{}
	for rec := range strings.SplitSeq(strings.TrimSuffix(string(staged), "\x00"), "\x00") {
		meta, name, ok := strings.Cut(rec, "\t")
		if !ok || !strings.EqualFold(path.Base(name), attributesPath) {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			continue
		}
		switch mode, object, stage := fields[0], fields[1], fields[2]; {
		case stage != "0":
			unmerged[name] = true
		case mode == "100644" || mode == "100755":
			blobs[name] = object
		}
	}
	// The name of the root file as the index holds it, the one the list
	// is read from; the plain name when the index holds none.
	rootName := attributesPath
	listed := map[string]bool{}
	var problems []finding
	for _, file := range slices.Sorted(maps.Keys(unmerged)) {
		problems = append(problems, finding{file, "generated", "has an unmerged entry in the index; resolve it"})
	}
	for _, file := range slices.Sorted(maps.Keys(blobs)) {
		data, err := repo.Run(ctx, nil, "cat-file", "blob", blobs[file])
		if err != nil {
			return "", nil, nil, err
		}
		root := strings.EqualFold(file, attributesPath)
		if root {
			rootName = file
		}
		scanLines(file, string(data), root, listed, &problems)
	}
	// Absolute, so that the path is the same from the main clone and from
	// a linked worktree, where git gives the path in the common directory.
	info, err := repo.Run(ctx, nil, "rev-parse", "--path-format=absolute", "--git-path", infoAttributes)
	if err != nil {
		return "", nil, nil, err
	}
	switch data, err := os.ReadFile(strings.TrimSuffix(string(info), "\n")); {
	case err == nil:
		scanLines(infoAttributes, string(data), false, listed, &problems)
	case !os.IsNotExist(err):
		return "", nil, nil, err
	}
	return rootName, listed, problems, nil
}

// scanLines reads the lines of one attributes file, called file in a
// finding. When root is true, the pattern of each line with -text is
// added to listed.
func scanLines(file, data string, root bool, listed map[string]bool, problems *[]finding) {
	for line := range strings.SplitSeq(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, `"`) {
			*problems = append(*problems, finding{file, "generated", fmt.Sprintf("quotes a pattern in %q; write the path unquoted", line)})
			continue
		}
		pattern, attrs, _ := strings.Cut(line, " ")
		if p, a, ok := strings.Cut(line, "\t"); ok && len(p) < len(pattern) {
			pattern, attrs = p, a
		}
		fields := strings.Fields(attrs)
		if root && !strings.HasPrefix(pattern, "[attr]") && slices.Contains(fields, "-text") {
			listed[pattern] = true
		}
		for _, attr := range checkoutAttrs {
			fix := fmt.Sprintf("write the state as %s, -%s or !%s", attr, attr, attr)
			for _, state := range []string{"set", "unset", "unspecified"} {
				if slices.Contains(fields, attr+"="+state) {
					*problems = append(*problems, finding{file, "generated", fmt.Sprintf("gives %q the value %s=%s; git prints that string as it prints a state, so %s", pattern, attr, state, fix)})
				}
			}
		}
	}
}

// regularMode is the index mode of a regular file without the execute
// bit, the one mode a generated file has.
const regularMode = "100644"

// trackedOutputs returns a finding for each of outs that the index does
// not hold as a regular file with the bytes of the output, or that
// matches an ignore rule.
func trackedOutputs(ctx context.Context, repo git.Repo, outs []output) ([]finding, error) {
	// ":(literal)" keeps a glob character of a path from being one.
	pathspec := []string{"--"}
	for _, o := range outs {
		pathspec = append(pathspec, ":(literal)"+o.path)
	}
	staged, err := repo.Run(ctx, nil, append([]string{"ls-files", "--stage", "-z"}, pathspec...)...)
	if err != nil {
		return nil, err
	}
	// "<mode> <object> <stage>\t<path>", one entry for each stage.
	entries := map[string][]string{}
	for _, rec := range strings.Split(strings.TrimSuffix(string(staged), "\x00"), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if ok {
			entries[path] = append(entries[path], meta)
		}
	}
	ignored, err := repo.Run(ctx, nil, append([]string{"ls-files", "--cached", "--ignored", "--exclude-standard", "-z"}, pathspec...)...)
	if err != nil {
		return nil, err
	}
	isIgnored := map[string]bool{}
	for _, path := range strings.Split(string(ignored), "\x00") {
		isIgnored[path] = true
	}
	var findings []finding
	for _, o := range outs {
		msg, err := trackedProblem(ctx, repo, o, entries[o.path])
		if err != nil {
			return nil, err
		}
		if msg == "" && isIgnored[o.path] {
			msg = "is tracked and matches an ignore rule; a generated file is not ignored"
		}
		if msg != "" {
			findings = append(findings, finding{o.path, "generated", msg})
		}
	}
	return findings, nil
}

// trackedProblem says what is wrong with the index entries of one
// output, or returns "" when they are right.
func trackedProblem(ctx context.Context, repo git.Repo, o output, entries []string) (string, error) {
	if len(entries) == 0 {
		return "is not tracked; run go generate, add the file and commit it", nil
	}
	want, err := repo.Run(ctx, o.content, "hash-object", "--stdin")
	if err != nil {
		return "", err
	}
	for _, meta := range entries {
		mode, rest, _ := strings.Cut(meta, " ")
		object, stage, _ := strings.Cut(rest, " ")
		switch {
		case stage != "0":
			return "has an unmerged entry in the index; resolve it", nil
		case mode != regularMode:
			return fmt.Sprintf("is tracked with mode %s; a generated file is a regular file (%s), not a link and not executable", mode, regularMode), nil
		case object != strings.TrimSpace(string(want)):
			return "is tracked with other bytes than go generate gives; run it, add the file and commit it", nil
		}
	}
	return "", nil
}

// snapshot maps each file of the working tree, tracked or untracked and
// not ignored, to a digest of its type, mode and content. A tracked file
// deleted from disk is not in it. Files are read through root, so a path
// that reaches outside it is an error and not a read.
func snapshot(ctx context.Context, repo git.Repo, root *os.Root) (map[string]string, error) {
	out, err := repo.Run(ctx, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, name := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(name) == 0 {
			continue
		}
		digest, ok, err := fileDigest(root, string(name))
		if err != nil {
			return nil, err
		}
		if ok {
			files[string(name)] = digest
		}
	}
	return files, nil
}

// fileDigest returns the digest of the regular file or symbolic link at
// path below root, and false for anything else, a missing file and a
// directory (a submodule) included. The mode of a regular file is part
// of the digest, so a chmod is a change.
func fileDigest(root *os.Root, path string) (string, bool, error) {
	info, err := root.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	h := sha256.New()
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := root.Readlink(path)
		if err != nil {
			return "", false, err
		}
		_, _ = io.WriteString(h, "link "+target)
	case info.Mode().IsRegular():
		f, err := root.Open(path)
		if err != nil {
			return "", false, err
		}
		defer func() { _ = f.Close() }()
		_, _ = io.WriteString(h, "file "+info.Mode().Perm().String()+" ")
		if _, err := io.Copy(h, f); err != nil {
			return "", false, err
		}
	default:
		return "", false, nil
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}
