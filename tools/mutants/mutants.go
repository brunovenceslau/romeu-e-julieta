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
	"unicode"

	"go.yaml.in/yaml/v3"
)

// mutant is one entry of a mutants file.
type mutant struct {
	ID      string `yaml:"id"`
	File    string `yaml:"file"`
	Old     string `yaml:"old"`
	New     string `yaml:"new"`
	Package string `yaml:"package"`
	Run     string `yaml:"run"`
}

// target is the test run a mutant names: a package pattern and the
// pattern of "-run".
type target struct {
	pkg, run string
}

func (m mutant) target() target {
	return target{pkg: m.Package, run: m.Run}
}

// mutantsFile is the shape of a mutants file.
type mutantsFile struct {
	Mutants []mutant `yaml:"mutants"`
}

// mutantID is the shape of an id. It starts each line of the output, so
// it holds no space and nothing that does not print.
var mutantID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// parseMutants decodes a mutants file and checks every entry. It
// refuses a key it does not know and a second YAML document, so a
// typing error does not load as a mutant that silently tests nothing.
func parseMutants(data []byte) ([]mutant, error) {
	var f mutantsFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the mutants file is empty")
		}
		return nil, fmt.Errorf("decode the mutants file: %w", err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("the mutants file holds more than one YAML document")
	}
	if len(f.Mutants) == 0 {
		return nil, errors.New("the mutants file lists no mutant")
	}
	seen := map[string]bool{}
	for i, m := range f.Mutants {
		if err := m.check(); err != nil {
			return nil, fmt.Errorf("mutant %d: %w", i+1, err)
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("mutant %d: the id %q is listed twice", i+1, m.ID)
		}
		seen[m.ID] = true
	}
	return f.Mutants, nil
}

// check refuses an entry that cannot be run as it is written.
func (m mutant) check() error {
	if !mutantID.MatchString(m.ID) {
		return fmt.Errorf("the id %q is not letters, digits, dots, underscores and hyphens", m.ID)
	}
	if err := checkRelPath("file", m.File); err != nil {
		return fmt.Errorf("%s: %w", m.ID, err)
	}
	if m.Old == "" {
		return fmt.Errorf("%s: old is empty", m.ID)
	}
	if m.Old == m.New {
		return fmt.Errorf("%s: new is the same as old", m.ID)
	}
	if !strings.HasPrefix(m.Package, "./") {
		return fmt.Errorf("%s: the package %q does not start with ./", m.ID, m.Package)
	}
	if rest := strings.TrimPrefix(m.Package, "./"); rest != "..." {
		if err := checkRelPath("package", rest); err != nil {
			return fmt.Errorf("%s: %w", m.ID, err)
		}
	}
	if m.Run == "" {
		return fmt.Errorf("%s: run is empty; name the tests the mutant is for", m.ID)
	}
	if strings.ContainsFunc(m.Run, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("%s: run %q holds a character that does not print", m.ID, m.Run)
	}
	return nil
}

// checkRelPath refuses a path that is not a clean path below the top of
// the repository, written with slashes, with every character printable
// and no space.
func checkRelPath(what, p string) error {
	switch {
	case p == "":
		return fmt.Errorf("%s is empty", what)
	case strings.ContainsFunc(p, func(r rune) bool { return r == '\\' || unicode.IsSpace(r) || !unicode.IsPrint(r) }):
		return fmt.Errorf("the %s %q holds a backslash, a space or a character that does not print", what, p)
	case path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../"):
		return fmt.Errorf("the %s %q is not a clean path below the top of the repository", what, p)
	}
	return nil
}

// occurrences counts where sub starts in s, overlapping matches
// included: "xx" occurs twice in "xxx", so it is not unique there, as
// a replacement of one of them would be ambiguous.
func occurrences(s, sub []byte) int {
	n := 0
	for i := 0; i <= len(s)-len(sub); {
		j := bytes.Index(s[i:], sub)
		if j < 0 {
			break
		}
		n++
		i += j + 1
	}
	return n
}

// targets returns the distinct targets of the mutants, in the order
// they first appear.
func targets(mutants []mutant) []target {
	var out []target
	seen := map[target]bool{}
	for _, m := range mutants {
		if t := m.target(); !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
