// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The requirement ids of 12 12.11. A page is the index, docs/spec.md,
// or a chapter, docs/spec/NN-<name>.md; the document order is the index
// first, then the chapters by file name. An id is the code span of
// reqPrefix and four digits; the placeholder is the code span of
// reqPrefix and reqPlaceholder. The retired table is the table whose
// header is reqRetiredHeader, on whichever page holds it (12 12.11).
const (
	reqIndexPath     = "docs/spec.md"
	reqSpecDir       = "docs/spec"
	reqTaskDir       = "docs/plan/tasks"
	reqPrefix        = "REQ-"
	reqPlaceholder   = "new"
	reqWidth         = 4
	reqLast          = 9999
	reqRetiredHeader = "| Id | Retired on | Last text | Replaced by |"
	// reqCiteCap is the most tasks one id may be cited from (12 12.11):
	// an obligation has few implementers, and a wider citation is a
	// copy, not a trace.
	reqCiteCap = 5
	// reqIndexAid and reqOpeningAid are the reading aids of the index
	// and of a chapter's text before its first numbered section.
	reqIndexAid   = "index"
	reqOpeningAid = "(opening)"
)

var (
	// reqSpan is any code span that starts with the prefix; its body
	// decides whether it is an id, the placeholder or a malformed token.
	reqSpan = regexp.MustCompile("`" + reqPrefix + "([^`]*)`")
	// reqDigits is the body of an id.
	reqDigits = regexp.MustCompile(`^[0-9]{4}$`)
	// reqCite is an entry of a task's Implements line: the id, with or
	// without its code span, bounded on the left so that a longer token
	// is no id, then any digits that follow the four (a malformed id),
	// then the reading aid in parentheses, when written.
	reqCite = regexp.MustCompile("(?:^|[^0-9A-Za-z-])" + reqPrefix + "([0-9]{4})([0-9]*)`?(?:\\s*\\(([^()]*(?:\\([^()]*\\))?[^()]*)\\))?")
	// reqChapter is the name of a chapter file.
	reqChapter = regexp.MustCompile(`^[0-9]{2}-[^/]+\.md$`)
	// reqFence is a line that may open or close a fenced block: up to
	// three spaces, the run of backticks or tildes, and the rest.
	reqFence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	// reqSection is a numbered section heading of a chapter.
	reqSection = regexp.MustCompile(`^## ([0-9]+\.[0-9]+)\b`)
	// reqListMarker is the marker of a list item, which an id must
	// follow and never precede.
	reqListMarker = regexp.MustCompile(`^\s*([0-9]+\.|[-*+])\s`)
	// reqDelimiterRow is the row under a table's header.
	reqDelimiterRow = regexp.MustCompile(`^\|(\s*:?-+:?\s*\|)+\s*$`)
	// reqISODate is the form of the "Retired on" cell.
	reqISODate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// reqFate is the fate field of a task file's front matter.
	reqFate = regexp.MustCompile(`^fate:\s*(\S+)`)
	// reqSpaces is a run of whitespace, which a reading aid that wraps
	// a line holds in place of one space.
	reqSpaces = regexp.MustCompile(`\s+`)
)

// reqReadFates are the fates whose task files the coverage half reads
// (12 12.11): a folded, removed or left-v1 file cites nothing that
// counts, and its stale citations are no finding. reqOtherFates are
// the rest of the fates the plan writes; any other fate is a finding.
var (
	reqReadFates  = []string{"kept", "new", "deferred", "merged"}
	reqOtherFates = []string{"folded", "removed", "left-v1"}
)

// reqShared are the ids that more tasks than reqCiteCap may cite: an
// obligation on every record of a kind is cited from each record
// task, and the plan's own shape is cited from each task that lays it
// out (12 12.11).
var reqShared = []string{"1101", "1169", "1170", "1171"}

// reqPage is one specification page: its path, its lines and its line
// ending, kept so that allocate writes a page back as it found it.
type reqPage struct {
	path  string
	lines []string
	eol   string
}

// reqMark is one id or placeholder found on a page, or one citation
// of an id in a task file.
type reqMark struct {
	page string
	line int    // 1-based
	col  int    // the byte offset of the code span in its line
	body string // the digits, the placeholder or a malformed token
	// section is the reading aid of the section the mark sits in
	// ("index", "NN n.m" or "NN (opening)"), and, for a citation, the
	// aid the task wrote, or "" when it wrote none.
	section string
	// misplaced says why the mark's place is a finding (12 12.11), or
	// is "" where the mark is well placed.
	misplaced string
}

func (m reqMark) where() string { return m.page + ":" + strconv.Itoa(m.line) }

// runReqs is the "reqs" subcommand: the check over the tree at HEAD,
// or, with "allocate", the numbering of the placeholders in the
// working tree.
func runReqs(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) > 0 && args[0] == "allocate" {
		if len(args) > 1 {
			return false, fmt.Errorf("reqs allocate takes no argument\n%s", check.Usage())
		}
		return reqsAllocate(ctx, e)
	}
	if len(args) != 0 {
		return false, fmt.Errorf("reqs takes no argument, or allocate\n%s", check.Usage())
	}
	files, err := check.HeadTree(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	findings, deferred, err := reqsOf(files)
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("reqs", len(findings)))
	if len(deferred) > 0 {
		check.Say(e.Stdout, "%s", reqDeferredLines(deferred))
	}
	return len(findings) == 0, nil
}

// reqDeferredLines is the second summary of the check: the ids that
// only deferred tasks cite, which pass, eight to a line.
func reqDeferredLines(ids []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "reqs: %d id(s) cited only by deferred tasks:", len(ids))
	for i, id := range ids {
		if i%8 == 0 {
			b.WriteString("\n ")
		}
		b.WriteString(" " + id)
	}
	b.WriteString("\n")
	return b.String()
}

// reqsOf judges files, the tree of one commit as headTree reads it, by
// the rules of 12 12.11: the ids of the pages and the retired table,
// then, where task files exist, the citations of the task files. It
// returns the findings and the ids cited only by deferred tasks.
func reqsOf(files []check.TreeFile) ([]check.Finding, []string, error) {
	pages, err := reqPagesIn(files)
	if err != nil {
		return nil, nil, err
	}
	live, placeholders, findings := reqIDs(pages)
	for _, m := range placeholders {
		findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: "a placeholder is left; run \"go run ./tools/ci reqs allocate\""})
	}
	retired, retiredFindings := reqRetired(pages)
	findings = append(findings, retiredFindings...)
	findings = append(findings, reqConsistency(live, retired)...)
	findings = append(findings, reqOutsidePages(files)...)
	tasks, taskFindings, err := reqTasksIn(files)
	if err != nil {
		return nil, nil, err
	}
	findings = append(findings, taskFindings...)
	coverage, deferred := reqCoverage(live, retired, tasks)
	return append(findings, coverage...), deferred, nil
}

// reqPagesIn returns the pages among files, in document order.
func reqPagesIn(files []check.TreeFile) ([]reqPage, error) {
	var pages []reqPage
	for _, f := range files {
		if !reqIsPage(f.Path) {
			continue
		}
		if f.Mode == "160000" {
			return nil, fmt.Errorf("read %s: not a file", git.Printable(f.Path))
		}
		pages = append(pages, reqPageOf(f.Path, f.Content))
	}
	reqSortPages(pages)
	return pages, nil
}

// reqIsPage reports whether a path is the index or a chapter.
func reqIsPage(p string) bool {
	return p == reqIndexPath || (path.Dir(p) == reqSpecDir && reqChapter.MatchString(path.Base(p)))
}

// reqSortPages puts the pages in document order.
func reqSortPages(pages []reqPage) {
	slices.SortStableFunc(pages, func(a, b reqPage) int {
		if (a.path == reqIndexPath) != (b.path == reqIndexPath) {
			if a.path == reqIndexPath {
				return -1
			}
			return 1
		}
		return strings.Compare(a.path, b.path)
	})
}

// reqPageOf splits content into lines and remembers whether the page
// ends its lines with CR LF, so that allocate writes it back the same
// way.
func reqPageOf(p string, content []byte) reqPage {
	text := string(content)
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	return reqPage{path: p, lines: strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), eol: eol}
}

// reqMarks returns every id, placeholder or malformed token of a page,
// in order, each with the section it sits in and, where its place
// breaks the placement rule of 12 12.11, why. It reads a page in one
// pass whose work grows with the page's length: the backticks before a
// code span are counted from the span before it, never from the line's
// start.
func reqMarks(p reqPage) []reqMark {
	var marks []reqMark
	fence := "" // the run that opened the fenced block the line is in, or "" outside one
	ticks := 0  // the parity of the backticks since the last blank line: a code span may wrap a line, never a paragraph
	section := reqOpeningAid
	for i, line := range p.lines {
		if next, ok := reqFenceStep(fence, line); ok {
			fence, ticks = next, 0
			continue
		}
		fenced := fence != ""
		if line == "" {
			ticks = 0
		}
		if m := reqSection.FindStringSubmatch(line); m != nil && !fenced {
			section = m[1]
		}
		header := strings.HasPrefix(line, "|") && i+1 < len(p.lines) && reqDelimiterRow.MatchString(p.lines[i+1])
		before, from := ticks, 0 // the backticks of the paragraph before the span at hand
		for _, loc := range reqSpan.FindAllStringSubmatchIndex(line, -1) {
			before += strings.Count(line[from:loc[0]], "`")
			from = loc[0]
			marks = append(marks, reqMark{
				page:      p.path,
				line:      i + 1,
				col:       loc[0],
				body:      line[loc[2]:loc[3]],
				section:   reqAid(p.path, section),
				misplaced: reqMisplaced(line, loc[0], loc[1], reqPlace{fenced: fenced, inSpan: before%2 == 1, header: header}),
			})
		}
		ticks = (ticks + strings.Count(line, "`")) % 2
	}
	return marks
}

// reqFenceStep reads line as a fence of CommonMark: up to three spaces,
// then a run of three or more backticks or of three or more tildes.
// Outside a fenced block (fence is ""), such a run opens one, unless it
// is of backticks and a backtick follows it on the line; inside one, a
// run of the opener's character at least as long as the opener, with
// nothing but blanks after it, closes it. It returns the fence after
// the line and true when the line opens or closes a block, and false
// for any other line, a line inside a block included.
func reqFenceStep(fence, line string) (string, bool) {
	m := reqFence.FindStringSubmatch(line)
	if m == nil {
		return fence, false
	}
	run, rest := m[1], m[2]
	if fence == "" {
		if run[0] == '`' && strings.Contains(rest, "`") {
			return fence, false
		}
		return run, true
	}
	if run[0] == fence[0] && len(run) >= len(fence) && strings.TrimSpace(rest) == "" {
		return "", true
	}
	return fence, false
}

// reqAid is the reading aid of a section of a page, as the plan writes
// it after an id: "index" for the index, else the chapter's number
// and the section's, or "(opening)" before the first numbered one.
func reqAid(p, section string) string {
	if p == reqIndexPath {
		return reqIndexAid
	}
	return path.Base(p)[:2] + " " + section
}

// reqPlace is what the caller of reqMisplaced knows of a line: whether
// it sits in a fenced block, whether the code span at hand opens inside
// another one (counted across the lines of a paragraph), and whether
// the line is a table's header row.
type reqPlace struct {
	fenced, inSpan, header bool
}

// reqMisplaced applies the placement rule of 12 12.11 to the code span
// at [start, end) of line: an id is the first token of the text it
// governs, so it follows the marker of a list item, never precedes it,
// and sits in no fenced block, no other code span and no table header.
func reqMisplaced(line string, start, end int, at reqPlace) string {
	switch {
	case at.fenced:
		return "sits inside a fenced block"
	case at.inSpan:
		return "sits inside another code span"
	case strings.TrimSpace(line[:start]) == "" && reqListMarker.MatchString(line[end:]):
		return "precedes the marker of a list item"
	case at.header:
		return "sits in a table header row"
	}
	return ""
}

// reqIDs reads the ids of the pages. It returns the live ids, each with
// the place of its first occurrence, the placeholders in document
// order, and the findings of rules 1 to 3, 6 and 10 of 12 12.11: a
// malformed token, a duplicate, an id or a placeholder inside an
// excluded page or section, and one that is misplaced. A placeholder
// elsewhere is for the caller: the check reports it, allocate numbers
// it. A token whose body is empty is a mention of the prefix and
// nothing else.
func reqIDs(pages []reqPage) (map[string]reqMark, []reqMark, []check.Finding) {
	live := map[string]reqMark{}
	var placeholders []reqMark
	var findings []check.Finding
	for _, p := range pages {
		excluded := reqExcluded(p)
		rowFirst, rowLast := reqRetiredRows(p)
		for _, m := range reqMarks(p) {
			token := reqPrefix + m.body
			switch {
			case m.body == "":
			case m.misplaced != "":
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: token + " " + m.misplaced + " (12 12.11)"})
			case m.body == reqPlaceholder && excluded[m.line-1]:
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: "a placeholder sits in a page or section that carries no id (12 12.11)"})
			case m.body == reqPlaceholder:
				placeholders = append(placeholders, m)
			case m.line-1 >= rowFirst && m.line-1 < rowLast: // a retired row names its id
			case !reqDigits.MatchString(m.body):
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: "the token after " + reqPrefix + " is neither four digits nor " + reqPlaceholder})
			case excluded[m.line-1]:
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: token + " sits in a page or section that carries no id (12 12.11)"})
			default:
				if prior, dup := live[m.body]; dup {
					findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: token + " appears twice; its first place is " + prior.where()})
					continue
				}
				live[m.body] = m
			}
		}
	}
	return live, placeholders, findings
}

// reqExcluded marks the lines of a page where an id is a finding: the
// whole of chapter 13; chapter 09 from its first journey heading; the
// index from its Open questions heading to the end; and chapter 00
// from its section 0.5 to the next section.
func reqExcluded(p reqPage) []bool {
	out := make([]bool, len(p.lines))
	base := path.Base(p.path)
	from := func(prefix string, until func(string) bool) {
		in := false
		for i, l := range p.lines {
			switch {
			case !in && strings.HasPrefix(l, prefix):
				in = true
			case in && until != nil && until(l):
				in = false
			}
			out[i] = out[i] || in
		}
	}
	nextSection := func(l string) bool { return strings.HasPrefix(l, "## ") }
	switch {
	case strings.HasPrefix(base, "13-"):
		for i := range out {
			out[i] = true
		}
	case strings.HasPrefix(base, "09-"):
		from("## J", nil)
	case p.path == reqIndexPath:
		from("## Open questions", nil)
	case strings.HasPrefix(base, "00-"):
		from("## 0.5 ", nextSection)
	}
	return out
}

// reqRetired reads the retired table: the table whose header is
// reqRetiredHeader, on whichever page holds it. A row that does not
// parse is a finding (rule 7); a page without the table has no retired
// id.
func reqRetired(pages []reqPage) (map[string]reqMark, []check.Finding) {
	retired := map[string]reqMark{}
	var findings []check.Finding
	for _, p := range pages {
		first, last := reqRetiredRows(p)
		for i := first; i < last; i++ {
			m := reqMark{page: p.path, line: i + 1}
			cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(p.lines[i], "|"), "|"), "|")
			if len(cells) != 4 {
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: fmt.Sprintf("a retired row has %d cells, not 4", len(cells))})
				continue
			}
			id := strings.Trim(strings.TrimSpace(cells[0]), "`")
			date := strings.TrimSpace(cells[1])
			switch {
			case !strings.HasPrefix(id, reqPrefix) || !reqDigits.MatchString(strings.TrimPrefix(id, reqPrefix)):
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: "a retired row's first cell is not an id"})
			case !reqISODate.MatchString(date):
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: "a retired row's second cell is not an ISO date"})
			default:
				m.body = strings.TrimPrefix(id, reqPrefix)
				if _, dup := retired[m.body]; dup {
					findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: id + " is retired twice"})
					continue
				}
				retired[m.body] = m
			}
		}
	}
	return retired, findings
}

// reqRetiredRows returns the 0-based line range [first, last) of the
// rows of the retired table on a page: the lines after its header and
// separator up to the first line that is no row. A page without the
// table has an empty range.
func reqRetiredRows(p reqPage) (int, int) {
	start := slices.Index(p.lines, reqRetiredHeader)
	if start < 0 {
		return 0, 0
	}
	last := start + 2
	for last < len(p.lines) && strings.HasPrefix(p.lines[last], "|") {
		last++
	}
	return start + 2, last
}

// reqConsistency applies rules 4 and 5: an id both live and retired,
// and a number from 1 to the highest that is neither.
func reqConsistency(live, retired map[string]reqMark) []check.Finding {
	var findings []check.Finding
	for _, body := range slices.Sorted(maps.Keys(retired)) {
		if m, ok := live[body]; ok {
			findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: reqPrefix + body + " is live and in the retired table at " + retired[body].where()})
		}
	}
	for n := 1; n <= reqHighest(live, retired); n++ {
		body := reqBody(n)
		if _, ok := live[body]; ok {
			continue
		}
		if _, ok := retired[body]; ok {
			continue
		}
		findings = append(findings, check.Finding{Where: reqIndexPath, Rule: "reqs", Msg: reqPrefix + body + " is neither live nor retired; a dropped obligation is retired, never deleted"})
	}
	return findings
}

// reqHighest is the highest number among the live and retired ids, 0
// when there is none.
func reqHighest(live, retired map[string]reqMark) int {
	highest := 0
	for _, ids := range []map[string]reqMark{live, retired} {
		for body := range ids {
			if n, err := strconv.Atoi(body); err == nil && n > highest {
				highest = n
			}
		}
	}
	return highest
}

// reqBody is the body of the id of number n.
func reqBody(n int) string {
	return fmt.Sprintf("%0*d", reqWidth, n)
}

// reqOutsidePages applies rule 6 to the decision records: an id in a
// file under docs/adr/ is a finding, since a record carries none.
func reqOutsidePages(files []check.TreeFile) []check.Finding {
	var findings []check.Finding
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "docs/adr/") || f.Mode == "160000" {
			continue
		}
		for _, m := range reqMarks(reqPageOf(f.Path, f.Content)) {
			if reqDigits.MatchString(m.body) {
				findings = append(findings, check.Finding{Where: m.where(), Rule: "reqs", Msg: reqPrefix + m.body + " sits in a decision record, which carries no id (12 12.11)"})
			}
		}
	}
	return findings
}

// reqTask is what the coverage half reads of one task file: its fate,
// the line that states it, and the ids its Implements line cites, each
// with the reading aid written after it.
type reqTask struct {
	path     string
	fate     string
	fateLine int // 1-based
	cites    []reqMark
}

// reqTasksIn reads the task files among files. A file without a front
// matter or a fate is left out: that is a finding of the plan check,
// not of reqs. A fate outside the ones the plan writes is a finding
// here, since a misspelt fate would hide a file from the coverage
// half.
func reqTasksIn(files []check.TreeFile) ([]reqTask, []check.Finding, error) {
	var tasks []reqTask
	var findings []check.Finding
	for _, f := range files {
		if path.Dir(f.Path) != reqTaskDir || !strings.HasSuffix(f.Path, ".md") {
			continue
		}
		if f.Mode == "160000" {
			return nil, nil, fmt.Errorf("read %s: not a file", git.Printable(f.Path))
		}
		t, ok := reqParseTask(f.Path, reqPageOf(f.Path, f.Content).lines)
		if !ok {
			continue
		}
		if !slices.Contains(reqReadFates, t.fate) && !slices.Contains(reqOtherFates, t.fate) {
			where := f.Path + ":" + strconv.Itoa(t.fateLine)
			findings = append(findings, check.Finding{Where: where, Rule: "reqs", Msg: "the fate " + t.fate + " is none of " + strings.Join(slices.Concat(reqReadFates, reqOtherFates), ", ")})
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, findings, nil
}

// reqParseTask reads the fate and the Implements line of one task
// file. The front matter is the lines between the first "---" and the
// next; the Implements entry is the bullet "- Implements:" with its
// indented continuation lines, read whole, and its ids are read
// wherever they stand in it, with or without a code span, each with
// the reading aid in parentheses after it.
func reqParseTask(p string, lines []string) (reqTask, bool) {
	t := reqTask{path: p}
	if len(lines) == 0 || lines[0] != "---" {
		return t, false
	}
	end := slices.Index(lines[1:], "---")
	if end < 0 {
		return t, false
	}
	for i, l := range lines[1 : end+1] {
		if m := reqFate.FindStringSubmatch(l); m != nil {
			t.fate, t.fateLine = m[1], i+2
		}
	}
	if t.fate == "" {
		return t, false
	}
	start, block := reqImplements(lines, end+2)
	for _, loc := range reqCite.FindAllStringSubmatchIndex(block, -1) {
		m := reqMark{page: p, line: start + strings.Count(block[:loc[2]], "\n")}
		m.body = block[loc[2]:loc[3]] + block[loc[4]:loc[5]]
		if loc[6] >= 0 {
			m.section = reqSpaces.ReplaceAllString(block[loc[6]:loc[7]], " ")
		}
		t.cites = append(t.cites, m)
	}
	return t, true
}

// reqImplements returns the 1-based line of the Implements bullet of a
// task file, read from the 0-based line from, and the bullet with its
// continuation lines joined by newlines; a file without the bullet
// gives an empty block.
func reqImplements(lines []string, from int) (int, string) {
	start := -1
	var block []string
	for i := from; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "- Implements:"):
			start = i + 1
		case start >= 0 && (l == "" || !strings.HasPrefix(l, " ")):
			return start, strings.Join(block, "\n")
		case start < 0:
			continue
		}
		block = append(block, l)
	}
	return start, strings.Join(block, "\n")
}

// reqCoverage applies rules 8, 9 and 11 over the task files of the
// fates reqReadFates: an Implements entry whose id is not live, or
// whose reading aid names another section than the one the id sits
// in, is a finding, and so is a live id that no such task cites, and a
// live id that more tasks than reqCiteCap cite, the deferred tasks
// and the ids of reqShared aside. An id that only deferred tasks cite
// is returned apart and is no finding. Without task files the half is
// silent.
func reqCoverage(live, retired map[string]reqMark, tasks []reqTask) ([]check.Finding, []string) {
	if len(tasks) == 0 {
		return nil, nil
	}
	var findings []check.Finding
	cited := map[string]bool{} // by a task of a read fate
	citers := map[string]int{} // read tasks that are not deferred, per id
	for _, t := range tasks {
		if !slices.Contains(reqReadFates, t.fate) {
			continue
		}
		for _, c := range t.cites {
			token := reqPrefix + c.body
			id, ok := live[c.body]
			switch {
			case !reqDigits.MatchString(c.body):
				findings = append(findings, check.Finding{Where: c.where(), Rule: "reqs", Msg: "Implements cites " + token + ", which is no id"})
				continue
			case !ok:
				state := "unknown"
				if _, ok := retired[c.body]; ok {
					state = "retired"
				}
				findings = append(findings, check.Finding{Where: c.where(), Rule: "reqs", Msg: "Implements cites " + token + ", which is " + state})
				continue
			case c.section != "" && c.section != id.section:
				findings = append(findings, check.Finding{Where: c.where(), Rule: "reqs", Msg: "Implements cites " + token + " as (" + c.section + "); it sits in " + id.section})
			}
			cited[c.body] = true
			if t.fate != "deferred" {
				citers[c.body]++
			}
		}
	}
	var deferred []string
	for _, body := range slices.Sorted(maps.Keys(live)) {
		switch {
		case !cited[body]:
			findings = append(findings, check.Finding{Where: live[body].where(), Rule: "reqs", Msg: reqPrefix + body + " is cited by no task of fate " + strings.Join(reqReadFates, ", ")})
		case citers[body] == 0:
			deferred = append(deferred, reqPrefix+body)
		case citers[body] > reqCiteCap && !slices.Contains(reqShared, body):
			findings = append(findings, check.Finding{Where: live[body].where(), Rule: "reqs", Msg: fmt.Sprintf("%s%s is cited by %d tasks, more than the %d of 12 12.11; one obligation has few implementers", reqPrefix, body, citers[body], reqCiteCap)})
		}
	}
	return findings, deferred
}

// reqsAllocate numbers the placeholders of the pages of the working
// tree, in document order, from the highest live or retired id, and
// writes each changed page back with the line ending it had. It
// refuses, and writes nothing, when a page cannot be read, when an id
// or a retired row is a finding (a row that does not parse would hide
// its number, and the next placeholder would take it), or when a
// number would pass reqLast. A tree without a placeholder changes no
// byte.
func reqsAllocate(ctx context.Context, e check.Env) (bool, error) {
	dir, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	pages, err := reqPagesAt(root)
	if err != nil {
		return false, err
	}
	live, placeholders, findings := reqIDs(pages)
	retired, retiredFindings := reqRetired(pages)
	if findings = append(findings, retiredFindings...); len(findings) > 0 {
		return false, fmt.Errorf("%snothing was written", check.Lines(findings))
	}
	byPath := map[string]*reqPage{}
	for i := range pages {
		byPath[pages[i].path] = &pages[i]
	}
	next := reqHighest(live, retired)
	var written []string
	shift, last := 0, reqMark{} // the bytes earlier numbers added to the line at hand
	for _, m := range placeholders {
		next++
		if next > reqLast {
			return false, fmt.Errorf("%s: the next id would pass %s%d; nothing was written", git.Printable(m.where()), reqPrefix, reqLast)
		}
		if m.page != last.page || m.line != last.line {
			shift = 0
		}
		p := byPath[m.page]
		line := p.lines[m.line-1]
		at := m.col + shift
		old := "`" + reqPrefix + reqPlaceholder + "`"
		id := "`" + reqPrefix + reqBody(next) + "`"
		p.lines[m.line-1] = line[:at] + id + line[at+len(old):]
		shift += len(id) - len(old)
		last = m
		written = append(written, fmt.Sprintf("%s: %s%s", m.where(), reqPrefix, reqBody(next)))
	}
	changed := map[string]bool{}
	for _, m := range placeholders {
		changed[m.page] = true
	}
	for _, p := range pages {
		if !changed[p.path] {
			continue
		}
		if _, err := writeIfChanged(root, p.path, []byte(strings.Join(p.lines, p.eol))); err != nil {
			return false, err
		}
	}
	for _, w := range written {
		check.Say(e.Stdout, "%s\n", git.SafeLines(w))
	}
	check.Say(e.Stdout, "reqs: %s\n", reqAllocateSummary(len(written)))
	return true, nil
}

// reqAllocateSummary is the last line of allocate.
func reqAllocateSummary(n int) string {
	if n == 0 {
		return "ok"
	}
	return strconv.Itoa(n) + " placeholder(s) numbered"
}

// reqPagesAt reads the pages of the working tree below root, in
// document order. A page that is not a regular file is an error.
func reqPagesAt(root *os.Root) ([]reqPage, error) {
	paths := []string{reqIndexPath}
	entries, err := fs.ReadDir(root.FS(), reqSpecDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		if reqChapter.MatchString(entry.Name()) {
			paths = append(paths, path.Join(reqSpecDir, entry.Name()))
		}
	}
	var pages []reqPage
	for _, p := range paths {
		info, err := root.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("read %s: not a regular file", git.Printable(p))
		}
		content, err := root.ReadFile(p)
		if err != nil {
			return nil, err
		}
		pages = append(pages, reqPageOf(p, content))
	}
	reqSortPages(pages)
	return pages, nil
}
