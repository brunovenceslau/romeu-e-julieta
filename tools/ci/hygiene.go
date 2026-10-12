// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
)

// hookPath is the one file .githooks/ holds, and hookMode the mode git
// must record for it.
const (
	hookDir  = ".githooks/"
	hookPath = ".githooks/pre-push"
	hookMode = "100755"
)

// emDash is U+2014, written as an escape so this file passes the rule
// it implements.
const emDash = "\u2014"

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
// (isPrologue) because a macOS checkout holds prologue.md and
// PROLOGUE.md as one file, so the case a contributor typed says nothing
// about the file that other clones see.
const prologueName = "PROLOGUE.md"

// prologueMsg is the reason the rule gives, and repeats no path.
const prologueMsg = "the user's agreement file lives outside product repositories"

// isPrologue reports whether the last element of a git path is
// prologueName in any case.
func isPrologue(p string) bool {
	return strings.EqualFold(path.Base(p), prologueName)
}

// sandboxUser is the one name a home directory may carry: the user
// inside a sandbox, the same on every machine.
const sandboxUser = "agent"

// finding is one broken rule. It names a place and a rule and never
// repeats the text that matched: CI logs are public, and the output is
// pasted into pull requests.
type finding struct {
	where string
	rule  string
	msg   string
}

// String prints a finding with its path quoted by git.Printable when it
// does not print, and the whole line passed through git.SafeLines, so
// repository content cannot forge a line of the log or a workflow command.
func (f finding) String() string {
	return git.SafeLines(git.Printable(f.where) + ": " + f.rule + ": " + f.msg)
}

// treeFile is one entry of the tree at HEAD.
type treeFile struct {
	path    string
	mode    string
	content []byte // nil for a submodule, which has no content here
}

// hygiene applies the hygiene rules of 10 10.2 to the path and the
// content of each file of the tree at head (hygieneOf). The tree that
// is committed is the one a push publishes, so work in progress is not
// read. head is "HEAD", or the id of the commit fast or all judges
// (commitChecks).
func hygiene(ctx context.Context, repo git.Repo, head string) ([]finding, error) {
	files, err := headTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	return hygieneOf(files)
}

// hygieneOf applies the hygiene rules to files, the tree of one commit
// as headTree reads it, with the word lists and the denylist of that
// same tree: a denylist or a word list that is written and not
// committed is in no push. It reads nothing else, so a caller that
// read the tree before running anything judges what it read
// (commitChecks).
func hygieneOf(files []treeFile) ([]finding, error) {
	data, found, err := fileIn(files, prose.Path)
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
		where := git.Printable(f.path)
		if list.Match([]byte(f.path)) {
			where = fmt.Sprintf("(path withheld, tree entry %d)", i+1)
			findings = append(findings, finding{where, "name", "the path holds a listed name"})
		}
		if !printableASCII(f.path) {
			findings = append(findings, finding{where, "path", asciiPathMsg})
		}
		if strings.Contains(f.path, emDash) {
			findings = append(findings, finding{where, "em-dash", "the path holds U+2014; use a plain dash"})
		}
		// A second file there would run in every clone that enabled
		// the directory, with no rule of 10 10.2 covering it.
		if strings.HasPrefix(f.path, hookDir) && f.path != hookPath {
			findings = append(findings, finding{where, "githooks", hookDir + " holds exactly pre-push"})
		}
		if isPrologue(f.path) {
			findings = append(findings, finding{where, "prologue", prologueMsg})
		}
		// A symbolic link or a submodule leads a path to content that
		// no rule of this list reads by that path: a .config link to a
		// directory that holds mise/conf.d, say.
		if f.mode != "100644" && f.mode != "100755" {
			findings = append(findings, finding{where, "mode", "the mode is " + f.mode + "; the repository tracks regular files only, of mode 100644 or 100755"})
		}
		if isMiseFile(f.path) && !slices.Contains(pinnedMiseFiles, f.path) {
			findings = append(findings, finding{where, "mise", "mise reads this file as its configuration, and the repository tracks no mise file but mise.toml and mise.lock (tools/ci/misefiles.go)"})
		}
		if f.path == "go.work" || f.path == "go.work.sum" || strings.HasPrefix(f.path, "vendor/") {
			findings = append(findings, finding{where, "workspace", "go.work, go.work.sum and vendor/ are not tracked"})
		}
		at := func(line int) string { return where + ":" + strconv.Itoa(line) }
		for _, n := range list.MatchLines(f.content) {
			findings = append(findings, finding{at(n), "name", "the line holds a listed name"})
		}
		for _, n := range linesWith(f.content, func(line []byte) bool { return bytes.Contains(line, []byte(emDash)) }) {
			findings = append(findings, finding{at(n), "em-dash", "the line holds U+2014; use a plain dash"})
		}
		for _, n := range linesWith(f.content, hasPersonalPath) {
			findings = append(findings, finding{at(n), "personal-path", "the line holds a personal absolute path; write a <placeholder>"})
		}
		// The word lists hold the words they ban, so they are the one
		// file the lists are not applied to.
		if f.path != prose.Path {
			for _, h := range words.Find(f.content) {
				findings = append(findings, finding{at(h.Line), "prose", h.Rule + ": a listed word or phrase (ADR 0001, rule 4)"})
			}
		}
		if strings.HasSuffix(f.path, ".go") {
			for _, n := range toDoLines(f.content) {
				findings = append(findings, finding{at(n), "todo", "a to-do comment names its issue: TODO(#<issue>)"})
			}
		}
	}
	return findings, nil
}

// hygieneFile applies the name matcher, and no other rule, to one file
// that need not be in the tree: output a person is about to hand over.
// The denylist rule holds here too, so a clone without entries does
// not report a file as clean.
func hygieneFile(ctx context.Context, repo git.Repo, path string) ([]finding, error) {
	list, findings, err := loadDenylist(ctx, repo, "HEAD")
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, n := range list.MatchLines(content) {
		findings = append(findings, finding{"the file:" + strconv.Itoa(n), "name", "the line holds a listed name"})
	}
	return findings, nil
}

// loadDenylist reads the denylist committed at head, as hygieneOf
// reads it from a tree (denylistFindings).
func loadDenylist(ctx context.Context, repo git.Repo, head string) (*names.List, []finding, error) {
	list, found, err := denylistAt(ctx, repo, head)
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
func denylistFindings(list *names.List, found bool) []finding {
	const how = `; the maintainer adds entries with "go run ./tools/ci hygiene add" and commits the file`
	switch {
	case !found:
		return []finding{{names.Path, "denylist", "the denylist is missing at HEAD" + how}}
	case list.Len() == 0:
		return []finding{{names.Path, "denylist", "the denylist has no entry at HEAD" + how}}
	}
	return nil
}

// denylistAt reads the denylist committed at rev (parseDenylist).
func denylistAt(ctx context.Context, repo git.Repo, rev string) (*names.List, bool, error) {
	data, found, err := blobAt(ctx, repo, rev, names.Path)
	if err != nil {
		return nil, false, err
	}
	return parseDenylist(data, found)
}

// denylistIn reads the denylist of files, the tree of one commit
// (parseDenylist).
func denylistIn(files []treeFile) (*names.List, bool, error) {
	data, found, err := fileIn(files, names.Path)
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

// parseProse parses the word lists of a commit (hygieneOf).
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

// fileIn returns the content of the file at path among files, the
// tree of one commit, and whether it is there, as blobAt does for a
// commit in the repository: a submodule at path is not a file.
func fileIn(files []treeFile, path string) ([]byte, bool, error) {
	for _, f := range files {
		if f.path != path {
			continue
		}
		if f.mode == "160000" {
			return nil, false, fmt.Errorf("read %s: not a file", path)
		}
		return f.content, true, nil
	}
	return nil, false, nil
}

// blobAt returns the content of one file of the tree at rev, and
// whether it is there. rev is HEAD or an object id, never text a push
// supplies unchecked. It asks with cat-file's batch mode, which answers
// "missing" with exit 0, so a file that is absent is told apart from a
// git command that failed.
func blobAt(ctx context.Context, repo git.Repo, rev, path string) ([]byte, bool, error) {
	out, err := repo.Run(ctx, []byte(rev+":"+path+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, false, err
	}
	head, rest, _ := bytes.Cut(out, []byte{'\n'})
	if bytes.HasSuffix(head, []byte(" missing")) {
		return nil, false, nil
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
// rule, isMiseFile matches in ASCII lower case alone.
func printableASCII(path string) bool {
	for i := range len(path) {
		if path[i] < 0x20 || path[i] > 0x7e {
			return false
		}
	}
	return true
}

// hookFindings checks that the hook is tracked as an executable file.
func hookFindings(files []treeFile) []finding {
	for _, f := range files {
		if f.path != hookPath {
			continue
		}
		if f.mode != hookMode {
			return []finding{{hookPath, "githooks", "the mode is " + f.mode + ", want " + hookMode}}
		}
		return nil
	}
	return []finding{{hookPath, "githooks", "the hook is not tracked"}}
}

// headTree returns the files of the tree at head (hygiene) with their
// content, in the order git lists them.
func headTree(ctx context.Context, repo git.Repo, head string) ([]treeFile, error) {
	out, err := repo.Run(ctx, nil, "ls-tree", "-r", "-z", "--full-tree", head)
	if err != nil {
		return nil, err
	}
	var files []treeFile
	var ids bytes.Buffer
	for entry := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if entry == "" {
			continue
		}
		// "<mode> <type> <id>\t<path>"
		meta, path, _ := strings.Cut(entry, "\t")
		f := strings.Fields(meta)
		if len(f) != 3 {
			return nil, fmt.Errorf("read tree: unexpected entry of %d fields", len(f))
		}
		files = append(files, treeFile{path: path, mode: f[0]})
		if f[1] == "blob" {
			ids.WriteString(f[2] + "\n")
		}
	}
	blobs, err := repo.Run(ctx, ids.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].mode == "160000" { // a submodule: a commit, not a blob
			continue
		}
		// "<id> blob <size>\n<content>\n"
		head, rest, _ := bytes.Cut(blobs, []byte{'\n'})
		hf := strings.Fields(string(head))
		if len(hf) != 3 {
			return nil, fmt.Errorf("read blob of tree entry %d: unexpected header", i+1)
		}
		size, err := strconv.Atoi(hf[2])
		if err != nil || size+1 > len(rest) {
			return nil, fmt.Errorf("read blob of tree entry %d: unexpected size", i+1)
		}
		files[i].content = rest[:size:size]
		blobs = rest[size+1:]
	}
	return files, nil
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
