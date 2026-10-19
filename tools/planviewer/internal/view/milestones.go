// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"fmt"
	"strings"
)

// PlanFile is docs/plan.md, from the repository root: its milestone
// table names the goal of each milestone.
const PlanFile = "docs/plan.md"

// Milestone is one row of the milestone table of docs/plan.md.
type Milestone struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	EndsWhen string `json:"ends_when"`
	Delivers string `json:"delivers"`
}

// milestoneHeader is the header the parser requires, by cell prefix. A
// header that changes fails the run, so the page never shows a column
// under the wrong name.
var milestoneHeader = []string{"M", "Name", "Ends when", "Delivers"}

// cells splits a table row into trimmed cells, without the outer pipes.
// A pipe written \| (GFM) stays in its cell, as a plain pipe.
func cells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	if strings.HasSuffix(line, "|") && !strings.HasSuffix(line, `\|`) {
		line = line[:len(line)-1]
	}
	var out []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteByte('|')
			i++
		case line[i] == '|':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	return append(out, strings.TrimSpace(cur.String()))
}

func isRule(line string) bool {
	c := cells(line)
	for _, s := range c {
		if s == "" || strings.Trim(s, "-: ") != "" {
			return false
		}
	}
	return len(c) > 0
}

// parseMilestones reads the milestone table of the plan: the table whose
// header starts with M, Name, Ends when, Delivers, through its last row.
func parseMilestones(plan string) ([]Milestone, error) {
	lines := strings.Split(strings.ReplaceAll(plan, "\r\n", "\n"), "\n")
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "|") || strings.TrimSpace(cells(l)[0]) != "M" {
			continue
		}
		h := cells(l)
		if len(h) != len(milestoneHeader) {
			return nil, fmt.Errorf("%s: the milestone table header has %d columns, want %d (%s)", PlanFile, len(h), len(milestoneHeader), strings.Join(milestoneHeader, ", "))
		}
		for j, want := range milestoneHeader {
			if !strings.HasPrefix(h[j], want) {
				return nil, fmt.Errorf("%s: milestone table column %d is %q, want it to start with %q", PlanFile, j+1, h[j], want)
			}
		}
		if i+1 >= len(lines) || !isRule(lines[i+1]) {
			return nil, fmt.Errorf("%s: the milestone table has no separator row after its header", PlanFile)
		}
		var out []Milestone
		for _, r := range lines[i+2:] {
			if !strings.HasPrefix(strings.TrimSpace(r), "|") {
				break
			}
			c := cells(r)
			if len(c) != len(milestoneHeader) {
				return nil, fmt.Errorf("%s: milestone row %q has %d cells, want %d", PlanFile, c[0], len(c), len(milestoneHeader))
			}
			if c[0] == "" {
				return nil, fmt.Errorf("%s: a milestone row has an empty id (%q)", PlanFile, c[1])
			}
			for _, m := range out {
				if m.ID == c[0] {
					return nil, fmt.Errorf("%s: milestone id %q appears twice", PlanFile, c[0])
				}
			}
			out = append(out, Milestone{ID: c[0], Name: c[1], EndsWhen: c[2], Delivers: c[3]})
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%s: the milestone table has no row", PlanFile)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s: no milestone table (a table whose header starts with %s)", PlanFile, strings.Join(milestoneHeader, " | "))
}
