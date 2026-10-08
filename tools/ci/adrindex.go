// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
)

// adrStatuses are the statuses of rule 7 of ADR 0001.
var adrStatuses = []string{"Proposed", "Accepted", "Rejected", "Deprecated", "Superseded"}

var (
	// adrTitleLine is the first line of a record: "# N. Title".
	adrTitleLine = regexp.MustCompile(`^# (\d+)\. (.+)$`)
)

// adr is what the index shows of one decision record.
type adr struct {
	file   string
	number int
	title  string
	status string
}

// parseADR reads the title line and the status of the record in
// content. The number of the title line must be the one of the filename.
// The status is the first word of the first line of the Status section,
// so a note on that line ("Accepted, with a note") is an error: rule 7
// puts a note on a line of its own. A line that starts with "Superseded
// by" at the left margin makes it Superseded, since rule 7 adds that
// line to a record a later one replaces; an indented one is part of a
// quotation or a list and does not count.
func parseADR(file string, content []byte) (adr, error) {
	m := adrdir.Name.FindStringSubmatch(file)
	if m == nil {
		return adr{}, fmt.Errorf("%s: the name is not NNNN-slug.md", file)
	}
	number, _ := strconv.Atoi(m[1])
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	t := adrTitleLine.FindStringSubmatch(lines[0])
	if t == nil {
		return adr{}, fmt.Errorf("%s: the first line is not \"# N. Title\"", file)
	}
	if n, _ := strconv.Atoi(t[1]); n != number {
		return adr{}, fmt.Errorf("%s: the title line numbers the record %s, and the filename %d", file, t[1], number)
	}
	status, err := adrStatus(lines[1:])
	if err != nil {
		return adr{}, fmt.Errorf("%s: %w", file, err)
	}
	return adr{file: file, number: number, title: strings.TrimSpace(t[2]), status: status}, nil
}

// adrStatus returns the status that the Status section among lines
// gives.
func adrStatus(lines []string) (string, error) {
	start := slices.Index(lines, "## Status")
	if start < 0 {
		return "", errors.New("no \"## Status\" section")
	}
	section := lines[start+1:]
	if end := slices.IndexFunc(section, func(l string) bool { return strings.HasPrefix(l, "## ") }); end >= 0 {
		section = section[:end]
	}
	if slices.ContainsFunc(section, func(l string) bool { return strings.HasPrefix(l, "Superseded by ") }) {
		return "Superseded", nil
	}
	i := slices.IndexFunc(section, func(l string) bool { return strings.TrimSpace(l) != "" })
	if i < 0 {
		return "", errors.New("no status under \"## Status\"")
	}
	word, _, _ := strings.Cut(strings.TrimSpace(section[i]), " ")
	if !slices.Contains(adrStatuses, word) {
		return "", fmt.Errorf("the status %q is not one of %s", word, strings.Join(adrStatuses, ", "))
	}
	return word, nil
}

// adrIndex renders docs/adr/README.md from the records, in the order
// given: the generated-file header, then one row for each record.
func adrIndex(records []adr) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<!-- %s -->\n\n", generatedNotice("the titles and statuses of the records in this directory"))
	b.WriteString("# Decision records\n\n")
	b.WriteString("Each record holds one decision with its context and consequences. A record changes only\n")
	b.WriteString("with the maintainer's approval; a reversal is a new record that supersedes it. The layout and the statuses are the ones of rules 6 and 7 of\n")
	b.WriteString("[ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md).\n\n")
	b.WriteString("| Number | Title | Status |\n|---|---|---|\n")
	escape := strings.NewReplacer("|", `\|`, "[", `\[`, "]", `\]`)
	for _, r := range records {
		fmt.Fprintf(&b, "| %d | [%s](%s) | %s |\n", r.number, escape.Replace(r.title), r.file, r.status)
	}
	return b.Bytes()
}
