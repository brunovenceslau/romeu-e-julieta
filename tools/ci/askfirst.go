// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// askFirstPath is the single list of paths whose changes need the
// maintainer's explicit approval (05 5.3), relative to the repository
// root.
const askFirstPath = ".github/ask-first.yaml"

// askFirst is .github/ask-first.yaml: the owner that the code-owner
// review names, and the surfaces whose paths need approval.
type askFirst struct {
	Version  int       `yaml:"version"`
	Owner    string    `yaml:"owner"`
	Surfaces []surface `yaml:"surfaces"`
}

// surface is one group of paths that share a reason to ask first. The
// approval line of a pull request names it by id (12 12.4).
type surface struct {
	ID     string   `yaml:"id"`
	Globs  []string `yaml:"globs"`
	Reason string   `yaml:"reason"`
}

var (
	// surfaceID is the shape of an id: the approval line of 12 12.4
	// matches it with [a-z0-9-]+.
	surfaceID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// ownerHandle is a GitHub user, or an organization and team.
	ownerHandle = regexp.MustCompile(`^@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:/[A-Za-z0-9_.-]+)?$`)
	// globSegment is one segment of a glob: the characters that match
	// themselves, and "*" inside a segment. A segment of "**" alone is
	// checked apart.
	globSegment = regexp.MustCompile(`^[A-Za-z0-9._*-]+$`)
	// approvalLine is one line of a pull request body that records an
	// approval (12 12.4): a surface id, then a phrase that starts with
	// neither a space nor a double quotation mark and holds no double
	// quotation mark, its look-alikes included, anywhere.
	approvalLine = regexp.MustCompile(`^Approval: ([a-z0-9-]+) - ([^"\x{AB}\x{BB}\x{201C}-\x{201F}\x{2033}\x{301D}-\x{301F}\x{FF02}\s\p{Z}][^"\x{AB}\x{BB}\x{201C}-\x{201F}\x{2033}\x{301D}-\x{301F}\x{FF02}\r\n]*)$`)
)

// approvalForm is the form of an approval line, as 12 12.4, the
// reference page and the pull request template show it.
const approvalForm = "Approval: <surface id> - <what was approved>"

// parseAskFirst reads .github/ask-first.yaml and checks it: the keys are
// the ones of 05 5.3, the version is 1, every id is unique and has the
// shape of an approval line, every glob is written in the dialect of
// 12 12.4, and the file lists itself.
func parseAskFirst(src []byte) (*askFirst, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	var list askFirst
	if err := dec.Decode(&list); err != nil {
		return nil, fmt.Errorf("%s: %w", askFirstPath, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: holds more than one YAML document", askFirstPath)
	}
	if err := list.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", askFirstPath, err)
	}
	return &list, nil
}

func (a *askFirst) check() error {
	if a.Version != 1 {
		return fmt.Errorf("version is %d, and this tool reads version 1", a.Version)
	}
	if !ownerHandle.MatchString(a.Owner) {
		return fmt.Errorf("owner %q is not a GitHub handle written with a leading @", a.Owner)
	}
	if len(a.Surfaces) == 0 {
		return errors.New("no surface is listed")
	}
	seen := map[string]bool{}
	listsItself := false
	for _, s := range a.Surfaces {
		if !surfaceID.MatchString(s.ID) {
			return fmt.Errorf("surface id %q is not lowercase words joined by hyphens", s.ID)
		}
		if seen[s.ID] {
			return fmt.Errorf("surface id %q is listed twice", s.ID)
		}
		seen[s.ID] = true
		if len(s.Globs) == 0 {
			return fmt.Errorf("surface %q has no glob", s.ID)
		}
		if strings.TrimSpace(s.Reason) == "" || strings.ContainsAny(s.Reason, "\r\n") {
			return fmt.Errorf("surface %q needs a reason written on one line", s.ID)
		}
		for _, g := range s.Globs {
			if err := checkGlob(g); err != nil {
				return fmt.Errorf("surface %q: %w", s.ID, err)
			}
			listsItself = listsItself || g == askFirstPath
		}
	}
	if !listsItself {
		return fmt.Errorf("no surface lists %s: the file lists itself (05 5.3)", askFirstPath)
	}
	return nil
}

// checkGlob holds a glob to the dialect of 12 12.4: "/"-separated
// segments of the characters above, where a segment may be "**" alone.
// It has no leading "/" (a path from the root is the only kind) and no
// empty, "." or ".." segment.
func checkGlob(g string) error {
	if g == "" {
		return errors.New("a glob is empty")
	}
	for seg := range strings.SplitSeq(g, "/") {
		switch {
		case seg == "**":
		case seg == "" || seg == "." || seg == "..":
			return fmt.Errorf("glob %q has an empty, \".\" or \"..\" segment, or starts or ends with \"/\"", g)
		case strings.Contains(seg, "**"):
			return fmt.Errorf("glob %q: \"**\" must be a whole segment", g)
		case !globSegment.MatchString(seg):
			return fmt.Errorf("glob %q: the segment %q holds a character other than letters, digits, \".\", \"_\", \"-\" and \"*\"", g, seg)
		}
	}
	return nil
}

// globMatch reports whether a glob of the dialect above (checkGlob)
// matches name, both relative to the top of the tree with forward
// slashes: "**" matches zero or more whole segments, and "*" any run of
// characters inside one segment (path.Match, which checkGlob leaves
// nothing else to read).
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

// generatedNotice is the line that every generated file carries (12
// 12.3): who writes it and from what, so nobody edits it by hand.
func generatedNotice(source string) string {
	return `Generated by "go generate ./..." from ` + source + `. Do not edit by hand.`
}

// codeowners renders .github/CODEOWNERS: one line per glob, anchored at
// the repository root with a leading "/", all for the one owner. Every
// owner is the same, so the order of the lines (the last match wins in
// CODEOWNERS) decides nothing.
func (a *askFirst) codeowners() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# %s\n", generatedNotice(askFirstPath))
	b.WriteString("# A change to a path below needs the maintainer's review before it merges\n")
	b.WriteString("# (docs/reference/ask-first.md lists the surfaces and their reasons).\n")
	for _, s := range a.Surfaces {
		fmt.Fprintf(&b, "\n# %s\n", s.ID)
		for _, g := range s.Globs {
			fmt.Fprintf(&b, "/%s %s\n", g, a.Owner)
		}
	}
	return b.Bytes()
}

// referencePage renders docs/reference/ask-first.md: the front matter of
// rule 1 of ADR 0001, the generated-file header, and one table row for
// each surface.
func (a *askFirst) referencePage() []byte {
	var b bytes.Buffer
	b.WriteString("---\ntype: reference\nreader: a contributor or reviewer deciding whether a change needs the maintainer's approval\n---\n\n")
	fmt.Fprintf(&b, "<!-- %s -->\n\n", generatedNotice(askFirstPath))
	b.WriteString("# Ask-first surfaces\n\n")
	b.WriteString("A change to a path below needs the maintainer's explicit approval.\n")
	b.WriteString("The pull request body carries one approval line for each surface the change touches,\n")
	b.WriteString("in the form that\n[12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change)\ndefines:\n\n")
	b.WriteString("```text\n" + approvalForm + "\n```\n\n")
	b.WriteString("The phrase names what was approved, in a few neutral words, and no person.\n")
	b.WriteString("It holds no double quotation mark and none of the look-alikes 12 12.4 lists; single quotation marks are left to review.\n\n")
	fmt.Fprintf(&b, "The code-owner review is requested from `%s`. [05 5.3](../spec/05-security.md#53-ask-first-surfaces)\nsays why each surface is on the list.\n\n", a.Owner)
	b.WriteString("| Surface | Paths | Why |\n|---|---|---|\n")
	for _, s := range a.Surfaces {
		globs := make([]string, len(s.Globs))
		for i, g := range s.Globs {
			globs[i] = "`" + g + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s.ID, strings.Join(globs, ", "), strings.ReplaceAll(strings.TrimSpace(s.Reason), "|", `\|`))
	}
	return b.Bytes()
}
