// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package hygiene is the hygiene check of 10 10.2: the rules of
// Hygiene and Forbidden names over the path and the content of each
// file of the commit judged, read once before any step. It is the
// hygiene subcommand too, with "hygiene --file" and "hygiene add", the
// maintainer's way to add a denylist entry.
package hygiene

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
)

// hookPath is the one file .githooks/ holds, and HookMode the mode git
// must record for it.
const (
	hookDir  = ".githooks/"
	hookPath = ".githooks/pre-push"
	HookMode = "100755"
)

// EmDash is U+2014, written as an escape so this file passes the rule
// it implements.
const EmDash = "\u2014"

var (
	// personalPath finds a home directory of one person: the name is
	// one path segment. A placeholder such as <name> does not match,
	// because "<" is not in the class.
	personalPath = regexp.MustCompile(`/(?:Users|home)/([A-Za-z0-9._-]+)/`)
	// toDo finds the word; issueForm is what must follow it.
	toDo      = regexp.MustCompile(`\bTODO\b`)
	issueForm = regexp.MustCompile(`^\(#[0-9]+\)`)
)

// prologueName is the one file name no product path may carry. The
// user's agreement file lives outside product repositories, so the
// name is refused at any depth. It is compared without regard to case
// (IsPrologue) because a macOS checkout holds prologue.md and
// PROLOGUE.md as one file, so the case a contributor typed says nothing
// about the file that other clones see.
const prologueName = "PROLOGUE.md"

// PrologueMsg is the reason the rule gives, and repeats no path.
const PrologueMsg = "the user's agreement file lives outside product repositories"

// IsPrologue reports whether the last element of a git path is
// prologueName in any case.
func IsPrologue(p string) bool {
	return strings.EqualFold(path.Base(p), prologueName)
}

// sandboxUser is the one name a home directory may carry: the user
// inside a sandbox, the same on every machine.
const sandboxUser = "agent"

// At applies the At rules of 10 10.2 to the path and the
// content of each file of the tree at head (Of). The tree that
// is committed is the one a push publishes, so work in progress is not
// read. head is "HEAD", or the id of the commit fast or all judges
// (the runner).
func At(ctx context.Context, repo git.Repo, head string) ([]check.Finding, error) {
	files, err := check.HeadTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	return Of(files)
}

// Of applies the hygiene rules to files, the tree of one commit
// as check.HeadTree reads it, with the word lists and the denylist of that
// same tree: a denylist or a word list that is written and not
// committed is in no push. It reads nothing else, so a caller that
// read the tree before running anything judges what it read
// (the runner).
func Of(files []check.TreeFile) ([]check.Finding, error) {
	data, found, err := check.FileIn(files, prose.Path)
	if err != nil {
		return nil, err
	}
	words, err := parseProse(data, found)
	if err != nil {
		return nil, err
	}
	list, found, err := denylistIn(files)
	if err != nil {
		return nil, err
	}
	findings := denylistFindings(list, found)
	findings = append(findings, hookFindings(files)...)
	for i, f := range files {
		// A path that holds a listed name is not printed either.
		where := git.Printable(f.Path)
		if list.Match([]byte(f.Path)) {
			where = fmt.Sprintf("(path withheld, tree entry %d)", i+1)
			findings = append(findings, check.Finding{Where: where, Rule: "name", Msg: "the path holds a listed name"})
		}
		if !printableASCII(f.Path) {
			findings = append(findings, check.Finding{Where: where, Rule: "path", Msg: asciiPathMsg})
		}
		if strings.Contains(f.Path, EmDash) {
			findings = append(findings, check.Finding{Where: where, Rule: "em-dash", Msg: "the path holds U+2014; use a plain dash"})
		}
		// A second file there would run in every clone that enabled
		// the directory, with no rule of 10 10.2 covering it.
		if strings.HasPrefix(f.Path, hookDir) && f.Path != hookPath {
			findings = append(findings, check.Finding{Where: where, Rule: "githooks", Msg: hookDir + " holds exactly pre-push"})
		}
		if IsPrologue(f.Path) {
			findings = append(findings, check.Finding{Where: where, Rule: "prologue", Msg: PrologueMsg})
		}
		// A symbolic link or a submodule leads a path to content that
		// no rule of this list reads by that path: a .config link to a
		// directory that holds mise/conf.d, say.
		if f.Mode != "100644" && f.Mode != "100755" {
			findings = append(findings, check.Finding{Where: where, Rule: "mode", Msg: "the mode is " + f.Mode + "; the repository tracks regular files only, of mode 100644 or 100755"})
		}
		if mise.IsMiseFile(f.Path) && !slices.Contains(mise.PinnedMiseFiles, f.Path) {
			findings = append(findings, check.Finding{Where: where, Rule: "mise", Msg: "mise reads this file as its configuration, and the repository tracks no mise file but mise.toml and mise.lock (tools/ci/internal/mise/misefiles.go)"})
		}
		if f.Path == "go.work" || f.Path == "go.work.sum" || strings.HasPrefix(f.Path, "vendor/") {
			findings = append(findings, check.Finding{Where: where, Rule: "workspace", Msg: "go.work, go.work.sum and vendor/ are not tracked"})
		}
		at := func(line int) string { return where + ":" + strconv.Itoa(line) }
		for _, n := range list.MatchLines(f.Content) {
			findings = append(findings, check.Finding{Where: at(n), Rule: "name", Msg: "the line holds a listed name"})
		}
		for _, n := range linesWith(f.Content, func(line []byte) bool { return bytes.Contains(line, []byte(EmDash)) }) {
			findings = append(findings, check.Finding{Where: at(n), Rule: "em-dash", Msg: "the line holds U+2014; use a plain dash"})
		}
		for _, n := range linesWith(f.Content, hasPersonalPath) {
			findings = append(findings, check.Finding{Where: at(n), Rule: "personal-path", Msg: "the line holds a personal absolute path; write a <placeholder>"})
		}
		// The word lists hold the words they ban, so they are the one
		// file the lists are not applied to.
		if f.Path != prose.Path {
			for _, h := range words.Find(f.Content) {
				findings = append(findings, check.Finding{Where: at(h.Line), Rule: "prose", Msg: h.Rule + ": a listed word or phrase (ADR 0001, rule 4)"})
			}
		}
		if strings.HasSuffix(f.Path, ".go") {
			for _, n := range toDoLines(f.Content) {
				findings = append(findings, check.Finding{Where: at(n), Rule: "todo", Msg: "a to-do comment names its issue: TODO(#<issue>)"})
			}
		}
	}
	return findings, nil
}

// hygieneFile applies the name matcher, and no other rule, to one file
// that need not be in the tree: output a person is about to hand over.
// The denylist rule holds here too, so a clone without entries does
// not report a file as clean.
func hygieneFile(ctx context.Context, repo git.Repo, path string) ([]check.Finding, error) {
	list, findings, err := LoadDenylist(ctx, repo, "HEAD")
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, n := range list.MatchLines(content) {
		findings = append(findings, check.Finding{Where: "the file:" + strconv.Itoa(n), Rule: "name", Msg: "the line holds a listed name"})
	}
	return findings, nil
}

// LoadDenylist reads the denylist committed at head, as Of
// reads it from a tree (denylistFindings).
func LoadDenylist(ctx context.Context, repo git.Repo, head string) (*names.List, []check.Finding, error) {
	list, found, err := DenylistAt(ctx, repo, head)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", names.Path, err)
	}
	return list, denylistFindings(list, found), nil
}

// denylistFindings says what is wrong with the denylist of the commit
// judged: every check that matches names reads it there, and a
// pre-push run also at the remote's default branch (pushedRange). A
// list that is missing or has no entry is a finding and not an error:
// the check then proves nothing, and the maintainer is the one who can
// add an entry.
func denylistFindings(list *names.List, found bool) []check.Finding {
	const how = `; the maintainer adds entries with "go run ./tools/ci hygiene add" and commits the file`
	switch {
	case !found:
		return []check.Finding{{Where: names.Path, Rule: "denylist", Msg: "the denylist is missing at HEAD" + how}}
	case list.Len() == 0:
		return []check.Finding{{Where: names.Path, Rule: "denylist", Msg: "the denylist has no entry at HEAD" + how}}
	}
	return nil
}

// DenylistAt reads the denylist committed at rev (parseDenylist).
func DenylistAt(ctx context.Context, repo git.Repo, rev string) (*names.List, bool, error) {
	data, found, err := blobAt(ctx, repo, rev, names.Path)
	if err != nil {
		return nil, false, err
	}
	return parseDenylist(data, found)
}

// denylistIn reads the denylist of files, the tree of one commit
// (parseDenylist).
func denylistIn(files []check.TreeFile) (*names.List, bool, error) {
	data, found, err := check.FileIn(files, names.Path)
	if err == nil {
		var list *names.List
		if list, found, err = parseDenylist(data, found); err == nil {
			return list, found, nil
		}
	}
	return nil, false, fmt.Errorf("%s: %w", names.Path, err)
}

// parseDenylist parses a denylist as a commit holds it, and reports
// whether the commit has one; without one the list is empty. A
// denylist that is there and cannot be read is an error, never an
// empty list: the caller would pass a check that matched nothing.
func parseDenylist(data []byte, found bool) (*names.List, bool, error) {
	if !found {
		return &names.List{}, false, nil
	}
	list, err := names.Parse(data)
	if err != nil {
		return nil, false, err
	}
	return list, true, nil
}

// parseProse parses the word lists of a commit (Of).
func parseProse(data []byte, found bool) (*prose.Rules, error) {
	if !found {
		return nil, fmt.Errorf("%s is not in the tree at HEAD", prose.Path)
	}
	rules, err := prose.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", prose.Path, err)
	}
	return rules, nil
}

// blobAt returns the content of one file of the tree at rev, and
// whether it is there. rev is HEAD or an object id, never text a push
// supplies unchecked. The tree of rev says whether the file is there,
// and the object store is then asked for its blob: a partial clone
// holds every tree and may lack the blob, and a blob it lacks is an
// error that says how to fetch it, never a file that is not there.
func blobAt(ctx context.Context, repo git.Repo, rev, path string) ([]byte, bool, error) {
	entries, err := repo.Run(ctx, nil, "ls-tree", "-z", "--full-tree", rev, "--", path)
	if err != nil {
		return nil, false, err
	}
	var oid string
	for entry := range strings.SplitSeq(strings.TrimSuffix(string(entries), "\x00"), "\x00") {
		// "<mode> <type> <id>\t<path>"
		meta, name, ok := strings.Cut(entry, "\t")
		if !ok || name != path {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 3 || f[1] != "blob" {
			return nil, false, fmt.Errorf("read %s at %s: not a file", path, rev)
		}
		oid = f[2]
	}
	if oid == "" {
		return nil, false, nil
	}
	out, err := repo.Run(ctx, []byte(oid+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, false, err
	}
	head, rest, _ := bytes.Cut(out, []byte{'\n'})
	if string(head) == oid+" missing" {
		return nil, false, fmt.Errorf("read %s at %s: the object store lacks its blob %s; %s", path, rev, oid, git.FetchHint(oid))
	}
	// "<id> blob <size>"
	f := strings.Fields(string(head))
	if len(f) != 3 || f[1] != "blob" {
		return nil, false, fmt.Errorf("read %s at %s: not a file", path, rev)
	}
	size, err := strconv.Atoi(f[2])
	if err != nil || size > len(rest) {
		return nil, false, fmt.Errorf("read %s at %s: unexpected size", path, rev)
	}
	return rest[:size], true, nil
}

// asciiPathMsg is the reason of the path rule, which printableASCII
// decides.
const asciiPathMsg = "the path holds a byte outside printable ASCII (0x20 to 0x7E); name it with ASCII letters, digits and punctuation"

// printableASCII reports whether every byte of a git path is printable
// ASCII, 0x20 to 0x7E. A path outside that range can name, on another
// file system, the file a rule refuses by its ASCII name: macOS folds
// case beyond ASCII (U+017F, the long s, folds to s) and may compose or
// decompose accents, and a control byte hides in output. With this
// rule, mise.IsMiseFile matches in ASCII lower case alone.
func printableASCII(path string) bool {
	for i := range len(path) {
		if path[i] < 0x20 || path[i] > 0x7e {
			return false
		}
	}
	return true
}

// hookFindings checks that the hook is tracked as an executable file.
func hookFindings(files []check.TreeFile) []check.Finding {
	for _, f := range files {
		if f.Path != hookPath {
			continue
		}
		if f.Mode != HookMode {
			return []check.Finding{{Where: hookPath, Rule: "githooks", Msg: "the mode is " + f.Mode + ", want " + HookMode}}
		}
		return nil
	}
	return []check.Finding{{Where: hookPath, Rule: "githooks", Msg: "the hook is not tracked"}}
}

// linesWith returns the 1-based numbers of the lines that match.
func linesWith(content []byte, match func(line []byte) bool) []int {
	var out []int
	for n := 1; len(content) > 0; n++ {
		line, rest, _ := bytes.Cut(content, []byte{'\n'})
		if match(line) {
			out = append(out, n)
		}
		content = rest
	}
	return out
}

func hasPersonalPath(line []byte) bool {
	for _, m := range personalPath.FindAllSubmatch(line, -1) {
		if string(m[1]) != sandboxUser {
			return true
		}
	}
	return false
}

// toDoLines returns the lines of a Go file where a comment holds the
// to-do word without an issue number after it. It reads comments only,
// through the Go scanner, so a call of the context package and a
// string are left alone.
func toDoLines(src []byte) []int {
	var out []int
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	// A file that is not valid Go is still scanned as far as it goes;
	// vet and the compiler are the ones that report it.
	s.Init(file, src, func(token.Position, string) {}, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok != token.COMMENT {
			continue
		}
		first := fset.Position(pos).Line
		for _, loc := range toDo.FindAllStringIndex(lit, -1) {
			if !issueForm.MatchString(lit[loc[1]:]) {
				out = append(out, first+strings.Count(lit[:loc[0]], "\n"))
			}
		}
	}
}

// Run parses the arguments of the hygiene subcommand and prints
// what it finds.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) > 0 && args[0] == "add" {
		return true, add(ctx, e, args[1:])
	}
	fs := flag.NewFlagSet("hygiene", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "", "check one file with the name matcher")
	if err := fs.Parse(args); err != nil {
		return false, fmt.Errorf("hygiene: %w\n%s", err, check.Usage())
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("hygiene: unexpected argument %q\n%s", fs.Arg(0), check.Usage())
	}
	var findings []check.Finding
	var err error
	if *file != "" {
		findings, err = hygieneFile(ctx, e.Repo(), *file)
	} else {
		findings, err = At(ctx, e.Repo(), "HEAD")
	}
	if err != nil {
		return false, err
	}
	for _, f := range findings {
		check.Say(e.Stdout, "%s\n", f)
	}
	check.Say(e.Stdout, "%s\n", check.Summary("hygiene", len(findings)))
	return len(findings) == 0, nil
}
