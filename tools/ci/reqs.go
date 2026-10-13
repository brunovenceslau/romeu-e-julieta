// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The requirement ids of 12 12.11. A page is the index, docs/spec.md,
// or a chapter, docs/spec/NN-<name>.md; the document order is the index
// first, then the chapters by file name. An id is the code span of
// reqPrefix and four digits; the placeholder is the code span of
// reqPrefix and reqPlaceholder. The retired table sits in the chapter
// that carries reqRetiredHeader, under the heading of 12 12.11.
const (
	reqIndexPath     = "docs/spec.md"
	reqSpecDir       = "docs/spec"
	reqTaskDir       = "docs/plan/tasks"
	reqPrefix        = "REQ-"
	reqPlaceholder   = "new"
	reqWidth         = 4
	reqLast          = 9999
	reqRetiredHeader = "| Id | Retired on | Last text | Replaced by |"
	reqRetiredRule   = "**Retired requirements.**"
)

var (
	// reqSpan is any code span that starts with the prefix; its body
	// decides whether it is an id, the placeholder or a malformed token.
	reqSpan = regexp.MustCompile("`" + reqPrefix + "([^`]*)`")
	// reqDigits is the body of an id.
	reqDigits = regexp.MustCompile(`^[0-9]{4}$`)
	// reqIDToken finds an id in a task's Implements line, where the
	// code span is optional.
	reqIDToken = regexp.MustCompile(reqPrefix + `[0-9]{4}`)
	// reqChapter is the name of a chapter file.
	reqChapter = regexp.MustCompile(`^[0-9]{2}-[^/]+\.md$`)
	// reqISODate is the form of the "Retired on" cell.
	reqISODate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// reqFate is the fate field of a task file's front matter.
	reqFate = regexp.MustCompile(`^fate:\s*(\S+)`)
)

// reqReadFates are the fates whose task files the coverage half reads
// (12 12.11): a folded, removed or left-v1 file cites nothing that
// counts, and its stale citations are no finding.
var reqReadFates = []string{"kept", "new", "deferred", "merged"}

// reqPage is one specification page: its path and its lines.
type reqPage struct {
	path  string
	lines []string
}

// reqMark is one id or placeholder found on a page.
type reqMark struct {
	page string
	line int // 1-based
	body string
}

func (m reqMark) where() string { return m.page + ":" + strconv.Itoa(m.line) }

// runReqs is the "reqs" subcommand: the check over the tree at HEAD,
// or, with "allocate", the numbering of the placeholders in the
// working tree.
func runReqs(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) > 0 && args[0] == "allocate" {
		if len(args) > 1 {
			return false, fmt.Errorf("reqs allocate takes no argument\n%s", usage)
		}
		return reqsAllocate(ctx, e)
	}
	if len(args) != 0 {
		return false, fmt.Errorf("reqs takes no argument, or allocate\n%s", usage)
	}
	files, err := headTree(ctx, e.repo(), "HEAD")
	if err != nil {
		return false, err
	}
	findings, deferred, err := reqsOf(files)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("reqs", len(findings)))
	if len(deferred) > 0 {
		say(e.stdout, "%s\n", reqDeferredLine(deferred))
	}
	return len(findings) == 0, nil
}

// reqDeferredLine is the second summary line: the ids that only
// deferred tasks cite, which pass.
func reqDeferredLine(ids []string) string {
	return fmt.Sprintf("reqs: %d id(s) cited only by deferred tasks: %s", len(ids), strings.Join(ids, ", "))
}

// reqsOf judges files, the tree of one commit as headTree reads it, by
// the rules of 12 12.11: the ids of the pages and the retired table,
// then, where task files exist, the citations of the task files. It
// returns the findings and the ids cited only by deferred tasks.
func reqsOf(files []treeFile) ([]finding, []string, error) {
	pages, err := reqPagesIn(files)
	if err != nil {
		return nil, nil, err
	}
	live, findings := reqIDs(pages)
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
func reqPagesIn(files []treeFile) ([]reqPage, error) {
	var pages []reqPage
	for _, f := range files {
		if !reqIsPage(f.path) {
			continue
		}
		if f.mode == "160000" {
			return nil, fmt.Errorf("read %s: not a file", f.path)
		}
		pages = append(pages, reqPage{path: f.path, lines: reqLines(f.content)})
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
	sort.SliceStable(pages, func(i, j int) bool {
		a, b := pages[i].path, pages[j].path
		if (a == reqIndexPath) != (b == reqIndexPath) {
			return a == reqIndexPath
		}
		return a < b
	})
}

// reqLines splits content into lines, with CR LF read as LF.
func reqLines(content []byte) []string {
	return strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
}

// reqMarks returns every id, placeholder or malformed token of a page,
// in order, with the lines the ids are not allowed on (reqExcluded)
// reported as findings by the caller.
func reqMarks(p reqPage) []reqMark {
	var marks []reqMark
	for i, line := range p.lines {
		for _, m := range reqSpan.FindAllStringSubmatch(line, -1) {
			marks = append(marks, reqMark{page: p.path, line: i + 1, body: m[1]})
		}
	}
	return marks
}

// reqIDs reads the ids of the pages. It returns the live ids, each with
// the place of its first occurrence, and the findings of rules 1 to 3
// and 6 of 12 12.11: a placeholder, a malformed token, a duplicate and
// an id inside an excluded page or section. A token whose body is
// empty is a mention of the prefix and nothing else.
func reqIDs(pages []reqPage) (map[string]reqMark, []finding) {
	live := map[string]reqMark{}
	var findings []finding
	for _, p := range pages {
		excluded := reqExcluded(p)
		first, last := reqRetiredRows(p)
		for _, m := range reqMarks(p) {
			switch {
			case m.body == "":
			case m.line-1 >= first && m.line-1 < last: // a retired row names its id
			case m.body == reqPlaceholder:
				findings = append(findings, finding{m.where(), "reqs", "a placeholder is left; run \"go run ./tools/ci reqs allocate\""})
			case !reqDigits.MatchString(m.body):
				findings = append(findings, finding{m.where(), "reqs", "the token after " + reqPrefix + " is neither four digits nor " + reqPlaceholder})
			case excluded[m.line-1]:
				findings = append(findings, finding{m.where(), "reqs", reqPrefix + m.body + " sits in a page or section that carries no id (12 12.11)"})
			default:
				if first, dup := live[m.body]; dup {
					findings = append(findings, finding{m.where(), "reqs", reqPrefix + m.body + " appears twice; its first place is " + first.where()})
					continue
				}
				live[m.body] = m
			}
		}
	}
	return live, findings
}

// reqExcluded marks the lines of a page where an id is a finding: the
// whole of chapter 13; chapter 09 from its first journey heading; the
// index from its Open questions heading; and chapter 00 from its
// section 0.5 to the next section.
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
// reqRetiredHeader, under the paragraph reqRetiredRule, on whichever
// page holds it. A row that does not parse is a finding (rule 7); a
// page without the table has no retired id.
func reqRetired(pages []reqPage) (map[string]reqMark, []finding) {
	retired := map[string]reqMark{}
	var findings []finding
	for _, p := range pages {
		first, last := reqRetiredRows(p)
		for i := first; i < last; i++ {
			m := reqMark{page: p.path, line: i + 1}
			cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(p.lines[i], "|"), "|"), "|")
			if len(cells) != 4 {
				findings = append(findings, finding{m.where(), "reqs", fmt.Sprintf("a retired row has %d cells, not 4", len(cells))})
				continue
			}
			id := strings.Trim(strings.TrimSpace(cells[0]), "`")
			date := strings.TrimSpace(cells[1])
			switch {
			case !strings.HasPrefix(id, reqPrefix) || !reqDigits.MatchString(strings.TrimPrefix(id, reqPrefix)):
				findings = append(findings, finding{m.where(), "reqs", "a retired row's first cell is not an id"})
			case !reqISODate.MatchString(date):
				findings = append(findings, finding{m.where(), "reqs", "a retired row's second cell is not an ISO date"})
			default:
				m.body = strings.TrimPrefix(id, reqPrefix)
				if _, dup := retired[m.body]; dup {
					findings = append(findings, finding{m.where(), "reqs", id + " is retired twice"})
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
func reqConsistency(live, retired map[string]reqMark) []finding {
	var findings []finding
	for _, body := range slices.Sorted(func(yield func(string) bool) {
		for b := range retired {
			if !yield(b) {
				return
			}
		}
	}) {
		if m, ok := live[body]; ok {
			findings = append(findings, finding{m.where(), "reqs", reqPrefix + body + " is live and in the retired table at " + retired[body].where()})
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
		findings = append(findings, finding{reqIndexPath, "reqs", reqPrefix + body + " is neither live nor retired; a dropped obligation is retired, never deleted"})
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
func reqOutsidePages(files []treeFile) []finding {
	var findings []finding
	for _, f := range files {
		if !strings.HasPrefix(f.path, "docs/adr/") || f.mode == "160000" {
			continue
		}
		for _, m := range reqMarks(reqPage{path: f.path, lines: reqLines(f.content)}) {
			if reqDigits.MatchString(m.body) {
				findings = append(findings, finding{m.where(), "reqs", reqPrefix + m.body + " sits in a decision record, which carries no id (12 12.11)"})
			}
		}
	}
	return findings
}

// reqTask is what the coverage half reads of one task file: its fate
// and the ids its Implements line cites.
type reqTask struct {
	path  string
	fate  string
	cites []reqMark
}

// reqTasksIn reads the task files among files. A file without a front
// matter or a fate is left out: that is a finding of the plan check,
// not of reqs.
func reqTasksIn(files []treeFile) ([]reqTask, []finding, error) {
	var tasks []reqTask
	for _, f := range files {
		if path.Dir(f.path) != reqTaskDir || !strings.HasSuffix(f.path, ".md") {
			continue
		}
		if f.mode == "160000" {
			return nil, nil, fmt.Errorf("read %s: not a file", f.path)
		}
		t, ok := reqParseTask(f.path, reqLines(f.content))
		if ok {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil, nil
}

// reqParseTask reads the fate and the Implements line of one task
// file. The front matter is the lines between the first "---" and the
// next; the Implements entry is the bullet "- Implements:" with its
// indented continuation lines, and its ids are read wherever they
// stand in it, with or without a code span: the parenthetical after
// each id is a reading aid.
func reqParseTask(p string, lines []string) (reqTask, bool) {
	t := reqTask{path: p}
	if len(lines) == 0 || lines[0] != "---" {
		return t, false
	}
	end := slices.Index(lines[1:], "---")
	if end < 0 {
		return t, false
	}
	for _, l := range lines[1 : end+1] {
		if m := reqFate.FindStringSubmatch(l); m != nil {
			t.fate = m[1]
		}
	}
	if t.fate == "" {
		return t, false
	}
	in := false
	for i, l := range lines[end+2:] {
		switch {
		case strings.HasPrefix(l, "- Implements:"):
			in = true
		case in && (l == "" || !strings.HasPrefix(l, " ")):
			in = false
		}
		if !in {
			continue
		}
		for _, id := range reqIDToken.FindAllString(l, -1) {
			t.cites = append(t.cites, reqMark{page: p, line: end + 3 + i, body: strings.TrimPrefix(id, reqPrefix)})
		}
	}
	return t, true
}

// reqCoverage applies rules 8 and 9 over the task files of the fates
// reqReadFates: an Implements entry whose id is not live is a finding,
// and so is a live id that no such task cites. An id that only deferred
// tasks cite is returned apart and is no finding. Without task files
// the half is silent.
func reqCoverage(live, retired map[string]reqMark, tasks []reqTask) ([]finding, []string) {
	if len(tasks) == 0 {
		return nil, nil
	}
	var findings []finding
	cited := map[string]bool{}        // by a task of a read fate
	citedByOther := map[string]bool{} // by a read task that is not deferred
	for _, t := range tasks {
		if !slices.Contains(reqReadFates, t.fate) {
			continue
		}
		for _, c := range t.cites {
			if _, ok := live[c.body]; !ok {
				state := "unknown"
				if _, ok := retired[c.body]; ok {
					state = "retired"
				}
				findings = append(findings, finding{c.where(), "reqs", "Implements cites " + reqPrefix + c.body + ", which is " + state})
				continue
			}
			cited[c.body] = true
			if t.fate != "deferred" {
				citedByOther[c.body] = true
			}
		}
	}
	var deferred []string
	for _, body := range slices.Sorted(func(yield func(string) bool) {
		for b := range live {
			if !yield(b) {
				return
			}
		}
	}) {
		switch {
		case !cited[body]:
			findings = append(findings, finding{live[body].where(), "reqs", reqPrefix + body + " is cited by no task of fate " + strings.Join(reqReadFates, ", ")})
		case !citedByOther[body]:
			deferred = append(deferred, reqPrefix+body)
		}
	}
	return findings, deferred
}

// reqsAllocate numbers the placeholders of the pages of the working
// tree, in document order, from the highest live or retired id, and
// writes each changed page back. It refuses, and writes nothing, when a
// page cannot be read, when a number would pass reqLast, or when a
// placeholder sits where an id is a finding. A tree without a
// placeholder changes no byte.
func reqsAllocate(ctx context.Context, e env) (bool, error) {
	dir, err := repoRoot(ctx, e)
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
	live, _ := reqIDs(pages)
	retired, _ := reqRetired(pages)
	next := reqHighest(live, retired)
	var written []string
	for i := range pages {
		p := &pages[i]
		excluded := reqExcluded(*p)
		changed := false
		for j, line := range p.lines {
			if !strings.Contains(line, "`"+reqPrefix+reqPlaceholder+"`") {
				continue
			}
			if excluded[j] {
				return false, fmt.Errorf("%s:%d: a placeholder sits in a page or section that carries no id (12 12.11); nothing was written", p.path, j+1)
			}
			var b strings.Builder
			rest := line
			for {
				before, after, found := strings.Cut(rest, "`"+reqPrefix+reqPlaceholder+"`")
				b.WriteString(before)
				if !found {
					break
				}
				next++
				if next > reqLast {
					return false, fmt.Errorf("%s:%d: the next id would pass %s%d; nothing was written", p.path, j+1, reqPrefix, reqLast)
				}
				b.WriteString("`" + reqPrefix + reqBody(next) + "`")
				written = append(written, fmt.Sprintf("%s:%d: %s%s", p.path, j+1, reqPrefix, reqBody(next)))
				rest = after
			}
			p.lines[j] = b.String()
			changed = true
		}
		if changed {
			if _, err := writeIfChanged(root, p.path, []byte(strings.Join(p.lines, "\n"))); err != nil {
				return false, err
			}
		}
	}
	for _, w := range written {
		say(e.stdout, "%s\n", git.SafeLines(w))
	}
	say(e.stdout, "reqs: %s\n", reqAllocateSummary(len(written)))
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
			return nil, fmt.Errorf("read %s: not a regular file", p)
		}
		content, err := root.ReadFile(p)
		if err != nil {
			return nil, err
		}
		pages = append(pages, reqPage{path: p, lines: reqLines(content)})
	}
	reqSortPages(pages)
	return pages, nil
}
