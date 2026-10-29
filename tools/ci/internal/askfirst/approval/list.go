// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package approval reads .github/ask-first.yaml, the one list of the
// paths whose changes need the maintainer's approval (05 5.3), and
// judges the approval lines of a pull request against it (12 12.4): the
// code of pr. It imports only the standard library, the git runner of
// tools/ci and the YAML library, so the ask-first job builds it without the
// framework of tools/ci (tools/ci/internal/check), and every package it
// builds is on the approvals surface (05 5.3, REQ-0539; TestPRImports
// holds it). The askfirst package wraps it for tools/ci: the CODEOWNERS
// generator and the pr subcommand.
package approval

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Path is the single list of paths whose changes need the
// maintainer's explicit approval (05 5.3), relative to the repository
// root.
const Path = ".github/ask-first.yaml"

// CodeownersPath is the file generate writes from the list (12 12.3).
const CodeownersPath = ".github/CODEOWNERS"

// List is .github/ask-first.yaml: the owner that the code-owner
// review names, and the surfaces whose paths need approval.
type List struct {
	Version  int       `yaml:"version"`
	Owner    string    `yaml:"owner"`
	Surfaces []Surface `yaml:"surfaces"`
}

// Surface is one group of paths that share a reason to ask first. The
// approval line of a pull request names it by id (12 12.4).
type Surface struct {
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
	// approvalLine is one checklist line of the approval alert of a pull
	// request body (12 12.4): the box, empty or ticked, a surface id, then
	// a phrase. The first character of the phrase is a letter, number, punctuation or symbol,
	// written as the complement of the other categories (C, M and Z)
	// because RE2 has no class subtraction; the phrase holds no control
	// or format character and none of the double quotation marks the
	// two classes list. The first-character class leans on Go's \p{C}
	// covering unassigned code points (Cn) as well as Cc, Cf, Co and
	// Cs; the case "a leading unassigned code point" of TestApprovalLine
	// fails if a Go or Unicode table update changes that.
	approvalLine = regexp.MustCompile(`^> - \[([ xX])\] Approval: ([a-z0-9-]+) - ([^"\x{AB}\x{BB}\x{201C}-\x{201F}\x{2033}\x{301D}-\x{301F}\x{FF02}\p{C}\p{M}\p{Z}][^"\x{AB}\x{BB}\x{201C}-\x{201F}\x{2033}\x{301D}-\x{301F}\x{FF02}\p{Cc}\p{Cf}]*)$`)
)

// approvalForm is the form of an approval line, as 12 12.4 and the
// pull request template show it.
const approvalForm = "> - [ ] Approval: <surface id> - <what is asked>"

// Parse reads .github/ask-first.yaml and checks it: the keys are
// the ones of 05 5.3, the version is 1, every id is unique and has the
// shape of an approval line, every glob is written in the dialect of
// 12 12.4, and the file lists itself.
func Parse(src []byte) (*List, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	var list List
	if err := dec.Decode(&list); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	var more yaml.Node
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: holds more than one YAML document", Path)
	}
	if err := list.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	return &list, nil
}

func (a *List) check() error {
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
			if err := CheckGlob(g); err != nil {
				return fmt.Errorf("surface %q: %w", s.ID, err)
			}
			listsItself = listsItself || g == Path
		}
	}
	if !listsItself {
		return fmt.Errorf("no surface lists %s: the file lists itself (05 5.3)", Path)
	}
	return nil
}

// CheckGlob holds a glob to the dialect of 12 12.4: "/"-separated
// segments of the characters above, where a segment may be "**" alone.
// It has no leading "/" (a path from the root is the only kind) and no
// empty, "." or ".." segment.
func CheckGlob(g string) error {
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

// GlobMatch reports whether a glob of the dialect above (CheckGlob)
// matches name, both relative to the top of the tree with forward
// slashes: "**" matches zero or more whole segments, and "*" any run of
// characters inside one segment (path.Match, which CheckGlob leaves
// nothing else to read). It fills a table over the segments of the glob
// and of the name, so its work grows with their product: a glob of many
// "**" segments, which a task file of a change may hold, cannot make it
// try every split of a deep name.
func GlobMatch(glob, name string) bool {
	g, n := strings.Split(glob, "/"), strings.Split(name, "/")
	// match[i][j] reports whether g[i:] matches n[j:].
	match := make([][]bool, len(g)+1)
	for i := range match {
		match[i] = make([]bool, len(n)+1)
	}
	match[len(g)][len(n)] = true
	for i, v := range slices.Backward(g) {
		for j := len(n); j >= 0; j-- {
			if v == "**" {
				match[i][j] = match[i+1][j] || (j < len(n) && match[i][j+1])
				continue
			}
			if j < len(n) {
				ok, err := path.Match(v, n[j])
				match[i][j] = err == nil && ok && match[i+1][j+1]
			}
		}
	}
	return match[0][0]
}

// GlobsMeet reports whether some name matches both globs a and b, each
// of the dialect of CheckGlob, in the sense of GlobMatch. The dialect has
// two wildcards, "**" over whole segments and "*" inside one, and every
// other character is itself, so the answer is exact: no name need be in
// the tree. Like GlobMatch, it fills a table over the segments of the
// two globs, and one over the characters of each pair of segments.
func GlobsMeet(a, b string) bool {
	return meet(strings.Split(a, "/"), strings.Split(b, "/"), "**", func(s, t string) bool {
		return meet([]rune(s), []rune(t), '*', func(c, d rune) bool { return c == d })
	})
}

// meet reports whether the token lists x and y can match one sequence:
// the token star matches zero or more elements of it, and any other
// token one element, which a token of the other list matches too when
// compatible holds for the two.
func meet[T comparable](x, y []T, star T, compatible func(T, T) bool) bool {
	// m[i][j] reports whether x[i:] and y[j:] can match one sequence.
	m := make([][]bool, len(x)+1)
	for i := range m {
		m[i] = make([]bool, len(y)+1)
	}
	m[len(x)][len(y)] = true
	for i := len(x); i >= 0; i-- {
		for j := len(y); j >= 0; j-- {
			switch {
			case i == len(x) && j == len(y):
				// Set above: both lists are matched to their ends.
			case i < len(x) && x[i] == star:
				// The star matches nothing more, or the element y[j]
				// matches (or y[j] is a star that matches nothing).
				m[i][j] = m[i+1][j] || (j < len(y) && m[i][j+1])
			case j < len(y) && y[j] == star:
				m[i][j] = m[i][j+1] || (i < len(x) && m[i+1][j])
			case i < len(x) && j < len(y):
				m[i][j] = compatible(x[i], y[j]) && m[i+1][j+1]
			}
		}
	}
	return m[0][0]
}

// CodeownersRules renders the rules of .github/CODEOWNERS, which the
// askfirst package writes under its generated header: one line per glob,
// anchored at the repository root with a leading "/", all for the one
// owner. Every owner is the same, so the order of the lines (the last
// match wins in CODEOWNERS) decides nothing.
func (a *List) CodeownersRules() []byte {
	var b bytes.Buffer
	b.WriteString("# A change to a path below needs the maintainer's review before it merges\n")
	fmt.Fprintf(&b, "# (%s lists the surfaces and their reasons).\n", Path)
	for _, s := range a.Surfaces {
		fmt.Fprintf(&b, "\n# %s\n", s.ID)
		for _, g := range s.Globs {
			fmt.Fprintf(&b, "/%s %s\n", g, a.Owner)
		}
	}
	return b.Bytes()
}
