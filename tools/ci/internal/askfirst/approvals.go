// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package askfirst

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// alertLine opens the approval alert of a pull request body (12 12.4).
const alertLine = "> [!IMPORTANT]"

// approvalPrefix is the prefix of an approval line. A line of the alert
// that holds it and is not an approval line is malformed (REQ-1157).
const approvalPrefix = "Approval: "

// grantPhrase is the phrase of an approval line that names a sitting
// grant (REQ-1161): the sitting, then the task.
var grantPhrase = regexp.MustCompile(`^granted at (\S+) for (\S+)$`)

// approval is one approval line of the alert: the surface id, the
// phrase, and whether the maintainer ticked its box.
type approval struct {
	id, phrase string
	ticked     bool
}

// bodyLines returns the lines of a pull request body as 12 12.4 reads
// them: each HTML comment removed, from "<!--" to the next "-->" or to
// the end of the body when none follows (REQ-1146), then the rest split
// on LF with one trailing CR dropped from each line (REQ-1147).
func bodyLines(body string) []string {
	var b strings.Builder
	for {
		before, after, found := strings.Cut(body, "<!--")
		b.WriteString(before)
		if !found {
			break
		}
		_, body, found = strings.Cut(after, "-->")
		if !found {
			break
		}
	}
	lines := strings.Split(b.String(), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

// alertBody is what the approval alert of a body holds: its approval
// lines, its lines that hold the prefix and are malformed, and the
// number of alerts the body opens.
type alertBody struct {
	approvals []approval
	malformed []string
	alerts    int
}

// readAlert reads the approval alert of body (REQ-1147, REQ-1158): it
// runs from the line alertLine to the last line before the first one
// that does not start with ">". Lines outside it are not read, nor lines
// of it without the prefix.
func readAlert(body string) alertBody {
	var a alertBody
	in := false
	for _, l := range bodyLines(body) {
		if l == alertLine {
			a.alerts++
			in = true
			continue
		}
		in = in && strings.HasPrefix(l, ">")
		if !in || !strings.Contains(l, approvalPrefix) {
			continue
		}
		m := approvalLine.FindStringSubmatch(l)
		if m == nil {
			a.malformed = append(a.malformed, l)
			continue
		}
		a.approvals = append(a.approvals, approval{id: m[2], phrase: m[3], ticked: m[1] != " "})
	}
	return a
}

// review is what the approval rules of 12 12.4 judge: the body, the
// ask-first lists at the base and at the head (head is nil when the
// head has no list), the paths the pull request changes, and the grants
// of the base.
type review struct {
	body       string
	base, head *List
	changed    []string
	grants     *Grants
}

// findings applies the rules of REQ-1157 and REQ-1161 to the body and
// returns one message per finding: the alert's malformed lines and a
// second alert first, then each approval line in order, then each
// touched surface, in the order of the base list and then of the head's.
func (r review) findings() []string {
	ids, globs := r.union()
	a := readAlert(r.body)
	var out []string
	for _, l := range a.malformed {
		out = append(out, fmt.Sprintf("malformed approval line: %q", l))
	}
	if a.alerts > 1 {
		out = append(out, fmt.Sprintf("the body opens %d approval alerts; it holds one, opened by the line %q", a.alerts, alertLine))
	}
	approved := map[string]bool{}
	lineFor := map[string]bool{}
	for _, l := range a.approvals {
		switch {
		case globs[l.id] == nil:
			out = append(out, fmt.Sprintf("an approval line names %q, which neither ask-first list has", l.id))
			continue
		case lineFor[l.id]:
			out = append(out, fmt.Sprintf("two approval lines name %q", l.id))
			continue
		}
		lineFor[l.id] = true
		if !r.touched(globs[l.id]) {
			// An untouched surface's line is accepted as it is (REQ-1157).
			continue
		}
		ok, msg := r.grant(l)
		if msg != "" {
			out = append(out, msg)
		}
		// A grant phrase that fails is reported once, above, and not
		// again as a line left unticked.
		approved[l.id] = ok || l.ticked || msg != ""
	}
	for _, id := range ids {
		if !r.touched(globs[id]) {
			continue
		}
		switch {
		case !lineFor[id]:
			out = append(out, fmt.Sprintf("the diff touches %q and the alert has no approval line for it", id))
		case !approved[id]:
			out = append(out, fmt.Sprintf("the diff touches %q and its approval line is not ticked: the maintainer ticks it", id))
		}
	}
	return out
}

// grant judges the phrase of an approval line as a sitting grant
// (REQ-1161). A phrase of another form is not a grant: ok is false and
// msg empty. A grant phrase is ok when the grants of the base hold that
// grant, and a finding otherwise.
func (r review) grant(l approval) (ok bool, msg string) {
	m := grantPhrase.FindStringSubmatch(l.phrase)
	if m == nil {
		return false, ""
	}
	sitting, task := m[1], m[2]
	if !slices.Contains(Grantable, l.id) {
		return false, fmt.Sprintf("the approval line for %q names a grant, and a sitting grants none on that surface", l.id)
	}
	if !r.grants.holds(sitting, l.id, task) {
		return false, fmt.Sprintf("the approval line for %q names a grant of %s for %s, which %s at the base does not hold", l.id, sitting, task, GrantsPath)
	}
	return true, ""
}

// union returns the surface ids of the base and the head lists, the
// base's first, and the globs of each id in either list, so a pull
// request that removes a surface or a glob still needs its line
// (REQ-1155).
func (r review) union() ([]string, map[string][]string) {
	var ids []string
	globs := map[string][]string{}
	for _, l := range []*List{r.base, r.head} {
		if l == nil {
			continue
		}
		for _, s := range l.Surfaces {
			if globs[s.ID] == nil {
				ids = append(ids, s.ID)
			}
			globs[s.ID] = append(globs[s.ID], s.Globs...)
		}
	}
	return ids, globs
}

// touched reports whether a changed path matches one of globs
// (REQ-1156).
func (r review) touched(globs []string) bool {
	for _, p := range r.changed {
		for _, g := range globs {
			if GlobMatch(g, p) {
				return true
			}
		}
	}
	return false
}
