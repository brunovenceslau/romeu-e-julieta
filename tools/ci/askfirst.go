// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// askFirstPath is the one list of the paths whose change asks first
// (05 5.3).
const askFirstPath = ".github/ask-first.yaml"

// Threat model of the ask-first list. It protects the paths whose
// change can weaken the product's security model or the gates
// themselves (05 5.3), against a change to one of them that reaches
// the default branch without anyone being asked: an agent or a
// contributor who edits a gate, a pin or a decision record as a side
// effect of other work. The list is the one source, and its consumers
// make the asking mechanical (12 12.3): CODEOWNERS routes the review,
// and the pr check, which requires an approval line per touched
// surface (12 12.4), and the mutate check, which runs mutation testing
// on the Go code the list names, read it once they land.
//
// What this code guards against:
//
//  1. Consumers that drift from the list, in a commit. The generated
//     step judges one commit and reads the list and each generated
//     file of it from the object store, once, before fast or all
//     starts mise or any step (commitChecks): a hand edit, a list
//     changed without its consumers, a generated file deleted,
//     untracked or ignored, and one committed as a symbolic link or an
//     executable each fail, and nothing a step writes to the working
//     tree, the index, HEAD or the object store while it runs changes
//     what the step judges. git and the object store are trusted up to
//     that read, and in the hosted workflow mise runs before tools/ci
//     starts (the threat model in misefiles.go). A source committed as
//     a link is refused, never followed. CODEOWNERS is on the list
//     too, as defence in depth, so a change to it asks for the review
//     even where the step does not run. The step does not judge
//     the generators: a pull request that changes tools/ci runs its own
//     generators and checks, which is what the checks surface is for.
//  2. A list that two readers read in two ways. A glob is held to
//     characters that mean the same to CODEOWNERS, whose gitignore-like
//     syntax gives "!", "#", "?", "[", "\" and spaces a meaning of their
//     own, and to the matcher of 12 12.4, which lands with the pr
//     check. CODEOWNERS anchors each glob at the top of the repository,
//     so "go.mod" names the root file there as in the matcher, and a
//     glob without "*" may not name a directory, which CODEOWNERS
//     reads as the whole subtree and the matcher as one path. GitHub
//     skips a CODEOWNERS line it cannot read without failing anything,
//     which is why the form is fixed here. YAML that could read twice
//     (an unknown key, a key written twice) fails, and so does a glob
//     that two surfaces share.
//  3. A review routed to the wrong account. The owner must be the
//     <owner> of the module path, which is public (05 5.3), so a typo
//     or a swapped handle fails rather than inviting a stranger.
//  4. A list that removes itself from the asking. The list must name
//     its own path, so a change to it asks first too; the pr check also
//     reads the list at the base and at the head (12 12.4), so a pull
//     request that drops a surface still needs that surface's line.
//  5. Code that runs. The generators read and write files of the tree,
//     and the generated step starts no program but git, which reads
//     the objects of the judged commit. It runs in fast and in all: in
//     CI with the workflow's read-only contents permission and no
//     persisted credentials, and in the pre-push hook, with no
//     privilege beyond the other checks'.
//
// Out of scope: who typed an approval line (the author does; 05 5.4);
// whether a ruleset requires the code-owner review and these checks,
// without which all of this is advisory (GitHub settings, read by no
// check; 05 5.4); the owner's write access to the repository, without
// which GitHub ignores the CODEOWNERS line; whether the list is
// complete, since a path that should ask first and is missing is a
// review judgment; gate logic outside the globs, such as the scenario
// test 05 5.3 names; and a pull request that edits this code, since its
// own run uses its own files, which is why tools/ci/** is a surface.

// askFirst is the list, as .github/ask-first.yaml writes it.
type askFirst struct {
	Version  int       `yaml:"version"`
	Owner    string    `yaml:"owner"`
	Surfaces []surface `yaml:"surfaces"`
}

// surface is one entry of the list.
type surface struct {
	ID     string   `yaml:"id"`
	Globs  []string `yaml:"globs"`
	Reason string   `yaml:"reason"`
}

var (
	// surfaceID is the form of an id: the class an approval line
	// names it with (12 12.4).
	surfaceID = regexp.MustCompile(`^[a-z0-9-]+$`)
	// handle is a GitHub account name after "@": letters, digits and
	// inner hyphens.
	handle = regexp.MustCompile(`^@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
	// globSegment is one segment of a glob: "**", or a name of
	// characters that mean the same thing to the matcher of 12 12.4 and
	// to CODEOWNERS, where "*" matches inside one segment.
	globSegment = regexp.MustCompile(`^(\*\*|[A-Za-z0-9._*-]+)$`)
)

// parseAskFirst reads the list and refuses one its consumers could read
// in two ways (the threat model above). module is the module path of
// go.mod, whose <owner> the list's owner must be.
func parseAskFirst(data []byte, module string) (askFirst, error) {
	var list askFirst
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&list); err != nil && !errors.Is(err, io.EOF) {
		return askFirst{}, fmt.Errorf("%s: %w", askFirstPath, err)
	}
	if err := list.check(module); err != nil {
		return askFirst{}, fmt.Errorf("%s: %w", askFirstPath, err)
	}
	return list, nil
}

func (l askFirst) check(module string) error {
	if l.Version != 1 {
		return fmt.Errorf("version %d; this code reads version 1", l.Version)
	}
	host, rest, _ := strings.Cut(module, "/")
	owner, _, _ := strings.Cut(rest, "/")
	if host != "github.com" || owner == "" {
		return fmt.Errorf("the module path %q is not github.com/<owner>/..., which names the owner", module)
	}
	if !handle.MatchString(l.Owner) {
		return fmt.Errorf("the owner %q is not a GitHub handle such as \"@%s\"", l.Owner, owner)
	}
	if l.Owner != "@"+owner {
		return fmt.Errorf("the owner %q is not %q, the <owner> of the module path %s", l.Owner, "@"+owner, module)
	}
	var ids []string
	globs := map[string]string{} // the surface of each glob
	listsItself := false
	for i, s := range l.Surfaces {
		if !surfaceID.MatchString(s.ID) {
			return fmt.Errorf("surface %d: the id %q is not lowercase letters, digits and hyphens", i+1, s.ID)
		}
		if slices.Contains(ids, s.ID) {
			return fmt.Errorf("surface %d: the id %q is used twice", i+1, s.ID)
		}
		ids = append(ids, s.ID)
		switch {
		case len(s.Globs) == 0:
			return fmt.Errorf("surface %s: no glob", s.ID)
		case strings.TrimSpace(s.Reason) == "":
			return fmt.Errorf("surface %s: no reason", s.ID)
		case strings.ContainsFunc(s.Reason, func(r rune) bool { return !unicode.IsPrint(r) }):
			return fmt.Errorf("surface %s: the reason is one line of printable characters", s.ID)
		}
		for _, g := range s.Globs {
			if err := checkGlob(g); err != nil {
				return fmt.Errorf("surface %s: glob %q: %w", s.ID, g, err)
			}
			if other, ok := globs[g]; ok {
				return fmt.Errorf("surface %s: glob %q is a glob of %s too: a path belongs to one surface", s.ID, g, other)
			}
			globs[g] = s.ID
			listsItself = listsItself || g == askFirstPath
		}
	}
	if !listsItself {
		return fmt.Errorf("no surface names %s: the list lists itself, so a change to it asks first too", askFirstPath)
	}
	return nil
}

// checkGlob holds a glob to the form both its readers share: a
// relative path of segments, each "**" or a name of the characters of
// globSegment, with no "." or ".." segment.
func checkGlob(g string) error {
	for seg := range strings.SplitSeq(g, "/") {
		switch {
		case seg == "":
			return errors.New("an empty segment: no leading, trailing or double slash")
		case seg == "." || seg == "..":
			return errors.New(`a "." or ".." segment`)
		case !globSegment.MatchString(seg):
			return errors.New(`a segment is "**", or ASCII letters, digits and "._-*"`)
		case seg != "**" && strings.Contains(seg, "**"):
			return errors.New(`"**" is a whole segment`)
		}
	}
	return nil
}

// globMatch reports whether the glob of the ask-first grammar matches
// path, both relative to the top of the tree with forward slashes:
// "**" matches zero or more whole segments, and "*" any run of
// characters inside one segment (path.Match, which checkGlob leaves
// nothing else to read). It is the matcher of 12 12.4.
func globMatch(glob, name string) bool {
	return segmentsMatch(strings.Split(glob, "/"), strings.Split(name, "/"))
}

func segmentsMatch(glob, name []string) bool {
	if len(glob) == 0 {
		return len(name) == 0
	}
	if glob[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if segmentsMatch(glob[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(glob[0], name[0])
	return err == nil && ok && segmentsMatch(glob[1:], name[1:])
}

// checkDirs refuses a glob without "*" that names a directory of fsys,
// the tree the sources are read from: CODEOWNERS reads it as the whole
// subtree, and the matcher of 12 12.4 as that one path. "dir/**" says
// the subtree to both.
func (l askFirst) checkDirs(fsys fs.FS) error {
	for _, s := range l.Surfaces {
		for _, g := range s.Globs {
			if strings.Contains(g, "*") {
				continue
			}
			if info, err := fs.Stat(fsys, g); err == nil && info.IsDir() {
				return fmt.Errorf("surface %s: glob %q names a directory, which CODEOWNERS reads as its subtree and the matcher of 12 12.4 as one path; write %q", s.ID, g, g+"/**")
			}
		}
	}
	return nil
}

// generatedFrom is the header line of a generated file, without its
// comment marker.
func generatedFrom(source string) string {
	return `Generated by "go generate ./..." from ` + source + "; edit the source, not this file."
}

// mdEscape escapes the characters that would end a table cell or a
// link's text in Markdown.
func mdEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "|", `\|`, "[", `\[`, "]", `\]`).Replace(s)
}

// codeowners returns .github/CODEOWNERS for the list: each glob,
// anchored at the top of the repository with a leading "/", owned by
// the list's owner, grouped by surface.
func codeowners(l askFirst) []byte {
	var b bytes.Buffer
	b.WriteString("# " + generatedFrom(askFirstPath) + "\n#\n")
	b.WriteString("# Each path of an ask-first surface (05 5.3) asks for the review of\n# its owner.\n")
	for _, s := range l.Surfaces {
		b.WriteString("\n# " + s.ID + "\n")
		for _, g := range s.Globs {
			b.WriteString("/" + g + " " + l.Owner + "\n")
		}
	}
	return b.Bytes()
}

// askFirstPage returns docs/reference/ask-first.md for the list, with
// the front matter and the header its generator writes (ADR 0001,
// rules 1 and 10).
func askFirstPage(l askFirst) []byte {
	var b bytes.Buffer
	b.WriteString("---\ntype: reference\nreader: a contributor whose change touches a path that asks first\n---\n\n")
	b.WriteString("<!-- " + generatedFrom(askFirstPath) + " -->\n\n")
	b.WriteString("# Ask-first surfaces\n\n")
	b.WriteString("A pull request that changes a path below carries, for each surface it\n" +
		"touches, one approval line in its body:\n\n" +
		"```text\nApproval: <surface id> - \"<the maintainer's words>\"\n```\n\n" +
		"The form of the line and the check that reads it are in\n" +
		"[12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change);\n" +
		"why these paths are on the list is in\n" +
		"[05 5.3](../spec/05-security.md#53-ask-first-surfaces). In a path, `**`\n" +
		"matches zero or more path segments and `*` matches inside one segment.\n" +
		"`.github/CODEOWNERS` names `" + l.Owner + "` as the code owner of each path.\n\n")
	b.WriteString("| Surface | Paths | Why it asks first |\n|---|---|---|\n")
	for _, s := range l.Surfaces {
		globs := make([]string, len(s.Globs))
		for i, g := range s.Globs {
			globs[i] = "`" + g + "`"
		}
		b.WriteString("| `" + s.ID + "` | " + strings.Join(globs, ", ") + " | " + mdEscape(s.Reason) + " |\n")
	}
	return b.Bytes()
}
