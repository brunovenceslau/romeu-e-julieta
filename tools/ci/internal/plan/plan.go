// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package plan is the "plan" check of tools/ci: the task files of
// docs/plan/tasks, read from the tree of one commit, keep the rules of
// docs/plan.md (rules 1 to 3 and the shape of a task) and of 10 10.2
// (REQ-0826, the task half that "plan" holds until "sequences" lands).
// It also prints two views of the plan, the critical path and the
// ready tasks.
package plan

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Dir is the directory of the task files (02 2.1).
const Dir = "docs/plan/tasks"

// askFirstPath is the list whose surfaces the ask_first forecasts name.
const askFirstPath = ".github/ask-first.yaml"

// maxImplements is the most ids one Implements line may cite: the
// mirror of the citation cap of 12 12.11.
const maxImplements = 40

// OverCap are the tasks whose Implements line cited more than
// maxImplements ids when the check landed, decided on 2026-10-17: a
// ratchet, not an exemption to grow. A test fails when one of them comes
// down to the cap, so the list only shrinks; splitting the open ones is
// a re-plan.
var OverCap = []string{"T001", "T002", "T005", "T008", "T021", "T033", "T043", "T049", "T051", "T061", "T066", "T068", "T069"}

// Fates of the plan's field table. open are the fates of a task that
// is still to be built; read are the fates whose Implements line counts
// (the coverage half of reqs reads the same ones).
var (
	fates = []string{"kept", "new", "deferred", "folded", "merged", "left-v1", "removed"}
	open  = []string{"kept", "new", "deferred"}
	read  = []string{"kept", "new", "deferred", "merged"}
	// startable are the open fates whose task waits only for its
	// edges: a deferred task also waits for its trigger.
	startable = []string{"kept", "new"}
)

// Generated are the generated outputs of rule 1 of docs/plan.md, which
// no task owns and the overlap rule leaves out. A test of tools/ci holds
// every output a generator registers to one of these globs.
var Generated = []string{
	"docs/reference/**",
	"schemas/*.json",
	".github/CODEOWNERS",
	"docs/adr/README.md",
	"docs/diagrams/**",
}

// The fields of a task's front matter, in the order the plan writes
// them; required are the ones every task file holds.
var (
	required = []string{"id", "title", "milestone", "module", "fate", "owns", "needs", "after", "ask_first", "sitting"}
	optional = []string{"absorbed_by", "merged", "pr"}
)

var (
	fileName  = regexp.MustCompile(`^T([0-9]{3})-[a-z0-9]+(-[a-z0-9]+)*\.md$`)
	taskID    = regexp.MustCompile(`^T[0-9]{3}$`)
	milestone = regexp.MustCompile(`^(M[0-6]|none)$`)
	sitting   = regexp.MustCompile(`^(none|M[0-5]|O[1-8])$`)
	date      = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// entry is one Implements entry: an id and its reading aid, in the
	// forms the plan writes: "<page> <n>.<m>", "<page> (opening)", or
	// "index" for the index page.
	entry = regexp.MustCompile(`^REQ-[0-9]{4} \((index|[0-9]{2} [0-9]+\.[0-9]+|[0-9]{2} \(opening\))\)$`)
	// none is the form of a task with no obligation: "none" with its
	// reason after a colon or in parentheses.
	none  = regexp.MustCompile(`^none(:| \(|$)`)
	reqID = regexp.MustCompile(`REQ-[0-9]{4}`)
)

// Task is one task file as the check reads it.
type Task struct {
	Path       string
	ID         string
	Title      string
	Milestone  string
	Module     string
	Fate       string
	AbsorbedBy string
	Owns       []string
	Needs      []string
	After      []string
	AskFirst   []string
	Sitting    string
	Merged     string
	PR         string
	// Implements holds the requirement ids the Implements line cites,
	// and implLine the 1-based line of that bullet (0 when absent).
	Implements []string
	implLine   int
	// line is the 1-based line of each front-matter key.
	line map[string]int
}

func (t Task) where(key string) string {
	if n := t.line[key]; n > 0 {
		return t.Path + ":" + strconv.Itoa(n)
	}
	return t.Path
}

func (t Task) is(fates []string) bool { return slices.Contains(fates, t.Fate) }

// Read returns the task files among files, the tree of one commit, in
// path order, with the findings of each file read alone: its name, its
// front matter and its Implements line.
func Read(files []check.TreeFile) ([]Task, []check.Finding, error) {
	var tasks []Task
	var findings []check.Finding
	for _, f := range files {
		if path.Dir(f.Path) != Dir || path.Base(f.Path) == ".keep" {
			continue
		}
		if f.Mode == "160000" {
			return nil, nil, fmt.Errorf("read %s: not a file", git.Printable(f.Path))
		}
		t, fs := parse(f.Path, f.Content)
		findings = append(findings, fs...)
		if t != nil {
			tasks = append(tasks, *t)
		}
	}
	return tasks, findings, nil
}

// parse reads one task file; a nil task means its front matter could
// not be read, and the findings say why.
func parse(p string, content []byte) (*Task, []check.Finding) {
	find := func(where, msg string) check.Finding { return check.Finding{Where: where, Rule: "plan", Msg: msg} }
	var findings []check.Finding
	m := fileName.FindStringSubmatch(path.Base(p))
	if m == nil {
		findings = append(findings, find(p, "a task file is named T<three digits>-<slug>.md, the slug in lower-case ASCII letters, digits and dashes"))
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, append(findings, find(p, "the file does not start with a front matter (---)"))
	}
	end := slices.Index(lines[1:], "---")
	if end < 0 {
		return nil, append(findings, find(p, "the front matter has no closing ---"))
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end+1], "\n")), &doc); err != nil {
		return nil, append(findings, find(p, "the front matter is not YAML: "+err.Error()))
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, append(findings, find(p, "the front matter is not a mapping"))
	}
	t := &Task{Path: p, line: map[string]int{}}
	fields := doc.Content[0].Content
	for i := 0; i+1 < len(fields); i += 2 {
		key, val := fields[i].Value, fields[i+1]
		line := fields[i].Line + 1 // the front matter starts on the file's second line
		where := p + ":" + strconv.Itoa(line)
		if _, dup := t.line[key]; dup {
			findings = append(findings, find(where, "the field "+key+" appears twice"))
			continue
		}
		t.line[key] = line
		if !slices.Contains(required, key) && !slices.Contains(optional, key) {
			findings = append(findings, find(where, "the field "+key+" is none of "+strings.Join(slices.Concat(required, optional), ", ")))
			continue
		}
		switch key {
		case "owns", "needs", "after", "ask_first":
			list, ok := stringList(val)
			if !ok {
				findings = append(findings, find(where, "the field "+key+" is a list of strings"))
			}
			switch key {
			case "owns":
				t.Owns = list
			case "needs":
				t.Needs = list
			case "after":
				t.After = list
			case "ask_first":
				t.AskFirst = list
			}
		default:
			if val.Kind != yaml.ScalarNode || val.Value == "" {
				findings = append(findings, find(where, "the field "+key+" is one non-empty value"))
				continue
			}
			v := val.Value
			switch key {
			case "id":
				t.ID = v
			case "title":
				t.Title = v
			case "milestone":
				t.Milestone = v
			case "module":
				t.Module = v
			case "fate":
				t.Fate = v
			case "absorbed_by":
				t.AbsorbedBy = v
			case "sitting":
				t.Sitting = v
			case "merged":
				t.Merged = v
			case "pr":
				t.PR = v
			}
		}
	}
	for _, key := range required {
		if _, ok := t.line[key]; !ok {
			findings = append(findings, find(p, "the front matter has no field "+key))
		}
	}
	findings = append(findings, fieldFindings(*t, m)...)
	t.implLine, t.Implements, findings = implements(*t, lines, end+2, findings)
	return t, findings
}

// stringList reads a YAML sequence of non-empty strings.
func stringList(n *yaml.Node) ([]string, bool) {
	if n.Kind != yaml.SequenceNode {
		return nil, false
	}
	var list []string
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode || c.Value == "" {
			return list, false
		}
		list = append(list, c.Value)
	}
	return list, true
}

// fieldFindings checks the values of one task's fields against the
// plan's field table; m is the match of its filename.
func fieldFindings(t Task, m []string) []check.Finding {
	var findings []check.Finding
	add := func(key, msg string) {
		findings = append(findings, check.Finding{Where: t.where(key), Rule: "plan", Msg: msg})
	}
	if t.ID != "" && !taskID.MatchString(t.ID) {
		add("id", "the id "+t.ID+" is T and three digits")
	}
	if m != nil && t.ID != "" && t.ID != "T"+m[1] {
		add("id", "the id "+t.ID+" is not the one the filename carries, T"+m[1])
	}
	if t.Fate != "" && !slices.Contains(fates, t.Fate) {
		add("fate", "the fate "+t.Fate+" is none of "+strings.Join(fates, ", "))
	}
	if t.Milestone != "" && !milestone.MatchString(t.Milestone) {
		add("milestone", "the milestone "+t.Milestone+" is M0 to M6, or none")
	}
	if t.Sitting != "" && !sitting.MatchString(t.Sitting) {
		add("sitting", "the sitting "+t.Sitting+" is none, M0 to M5, or O1 to O8")
	}
	if (t.Fate == "folded") != (t.AbsorbedBy != "") {
		if t.Fate == "folded" {
			add("fate", "a folded task names the task that absorbed it in absorbed_by")
		} else {
			add("absorbed_by", "absorbed_by belongs to a folded task only")
		}
	}
	switch {
	case t.Merged != "" && t.PR == "":
		add("merged", "merged: comes with pr:, the number of the pull request")
	case t.PR != "" && t.Merged == "":
		add("pr", "pr: comes with merged:, the date of the merge")
	}
	if (t.Fate == "merged") != (t.Merged != "") {
		if t.Fate == "merged" {
			add("fate", "a merged task carries merged: and pr:")
		} else {
			add("merged", "merged: and pr: belong to a task of fate merged")
		}
	}
	if t.Merged != "" && !date.MatchString(t.Merged) {
		add("merged", "merged: is a date, YYYY-MM-DD")
	}
	if t.PR != "" {
		if n, err := strconv.Atoi(t.PR); err != nil || n <= 0 {
			add("pr", "pr: is the number of the pull request")
		}
	}
	return findings
}

// implements reads the "- Implements:" bullet of a task body, from the
// 0-based line from, with its indented continuation lines, and checks
// its shape for a task of a read fate: each entry an id and its reading
// aid, or the none form, and at most maxImplements ids. It returns the
// bullet's line, the ids it cites and the findings so far.
func implements(t Task, lines []string, from int, findings []check.Finding) (int, []string, []check.Finding) {
	start := -1
	var parts []string
	for i := from; i < len(lines); i++ {
		l := lines[i]
		if start < 0 {
			if rest, ok := strings.CutPrefix(l, "- Implements:"); ok {
				start = i + 1
				parts = append(parts, strings.TrimSpace(rest))
			}
			continue
		}
		if l == "" || !strings.HasPrefix(l, " ") {
			break
		}
		parts = append(parts, strings.TrimSpace(l))
	}
	block := strings.Join(parts, " ")
	ids := reqID.FindAllString(block, -1)
	if !t.is(read) {
		return max(start, 0), ids, findings
	}
	where := t.Path
	if start > 0 {
		where += ":" + strconv.Itoa(start)
	}
	add := func(msg string) { findings = append(findings, check.Finding{Where: where, Rule: "plan", Msg: msg}) }
	if start < 0 {
		add("the body has no \"- Implements:\" line")
		return 0, nil, findings
	}
	if none.MatchString(block) {
		return start, nil, findings
	}
	for _, e := range strings.Split(strings.TrimSuffix(block, "."), ", ") {
		if !entry.MatchString(e) {
			add(fmt.Sprintf("the Implements entry %q is not an id with its reading aid, \"REQ-nnnn (<page> <n>.<m>)\", \"(<page> (opening))\" or \"(index)\", nor the none form", e))
		}
	}
	if len(ids) > maxImplements && !slices.Contains(OverCap, t.ID) {
		add(fmt.Sprintf("the Implements line cites %d ids, more than %d", len(ids), maxImplements))
	}
	return start, ids, findings
}

// Of judges files, the tree of one commit, and returns the findings.
// generated are the globs of the generated outputs that the overlap rule
// leaves out.
func Of(files []check.TreeFile, generated []string) ([]check.Finding, error) {
	tasks, findings, err := Read(files)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 && len(findings) == 0 {
		return nil, nil
	}
	findings = append(findings, ids(tasks)...)
	byID := index(tasks)
	findings = append(findings, edges(tasks, byID)...)
	findings = append(findings, absorbed(tasks, byID)...)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	findings = append(findings, overlaps(tasks, byID, paths, generated)...)
	af, err := forecasts(files, tasks, paths)
	if err != nil {
		return nil, err
	}
	return append(findings, af...), nil
}

// index maps each id to its first task.
func index(tasks []Task) map[string]Task {
	byID := map[string]Task{}
	for _, t := range tasks {
		if _, ok := byID[t.ID]; !ok && t.ID != "" {
			byID[t.ID] = t
		}
	}
	return byID
}

// ids checks that every id is held by one file, and that the ids run
// from T001 to the highest with no gap (REQ-0826).
func ids(tasks []Task) []check.Finding {
	var findings []check.Finding
	seen := map[string]string{}
	highest := 0
	for _, t := range tasks {
		if !taskID.MatchString(t.ID) {
			continue
		}
		if first, dup := seen[t.ID]; dup {
			findings = append(findings, check.Finding{Where: t.where("id"), Rule: "plan", Msg: "the id " + t.ID + " is also the id of " + first})
			continue
		}
		seen[t.ID] = t.Path
		n, _ := strconv.Atoi(t.ID[1:])
		highest = max(highest, n)
	}
	for n := 1; n <= highest; n++ {
		if id := fmt.Sprintf("T%03d", n); seen[id] == "" {
			findings = append(findings, check.Finding{Where: Dir, Rule: "plan", Msg: "the ids are not contiguous: no task file holds " + id})
		}
	}
	return findings
}

// edges checks needs and after: each names a task of the tree, not the
// task itself, no id is in both lists, and the edges form no cycle. A
// task of every fate is checked, a deferred one included.
func edges(tasks []Task, byID map[string]Task) []check.Finding {
	var findings []check.Finding
	for _, t := range tasks {
		for _, key := range []string{"needs", "after"} {
			list := t.Needs
			if key == "after" {
				list = t.After
			}
			for _, id := range list {
				switch _, ok := byID[id]; {
				case id == t.ID:
					findings = append(findings, check.Finding{Where: t.where(key), Rule: "plan", Msg: key + " names the task itself, " + id})
				case !ok:
					findings = append(findings, check.Finding{Where: t.where(key), Rule: "plan", Msg: key + " names " + id + ", which no task file holds"})
				}
			}
		}
		for _, id := range t.Needs {
			if slices.Contains(t.After, id) {
				findings = append(findings, check.Finding{Where: t.where("after"), Rule: "plan", Msg: id + " is in both needs and after"})
			}
		}
	}
	if cycle := findCycle(tasks, byID); cycle != nil {
		first := byID[cycle[0]]
		findings = append(findings, check.Finding{Where: first.where("needs"), Rule: "plan", Msg: "needs and after form a cycle: " + strings.Join(cycle, " -> ")})
	}
	return findings
}

// waits returns the ids t waits for, needs then after, that are tasks
// of the tree.
func waits(t Task, byID map[string]Task) []string {
	var out []string
	for _, id := range slices.Concat(t.Needs, t.After) {
		if _, ok := byID[id]; ok && id != t.ID && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// findCycle returns one cycle of the edges, as the ids along it with
// the first repeated at the end, or nil.
func findCycle(tasks []Task, byID map[string]Task) []string {
	const (
		unseen = iota
		onPath
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(id string) []string
	visit = func(id string) []string {
		state[id] = onPath
		stack = append(stack, id)
		for _, next := range waits(byID[id], byID) {
			switch state[next] {
			case onPath:
				i := slices.Index(stack, next)
				return append(slices.Clone(stack[i:]), next)
			case unseen:
				if c := visit(next); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return nil
	}
	for _, t := range tasks {
		if state[t.ID] == unseen && t.ID != "" {
			if c := visit(t.ID); c != nil {
				return c
			}
		}
	}
	return nil
}

// absorbed checks each folded task: absorbed_by names a task of the
// tree, and that task's Implements cites every id the folded one cites.
func absorbed(tasks []Task, byID map[string]Task) []check.Finding {
	var findings []check.Finding
	for _, t := range tasks {
		if t.AbsorbedBy == "" {
			continue
		}
		into, ok := byID[t.AbsorbedBy]
		if !ok {
			findings = append(findings, check.Finding{Where: t.where("absorbed_by"), Rule: "plan", Msg: "absorbed_by names " + t.AbsorbedBy + ", which no task file holds"})
			continue
		}
		var missing []string
		for _, id := range t.Implements {
			if !slices.Contains(into.Implements, id) && !slices.Contains(missing, id) {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			findings = append(findings, check.Finding{Where: t.where("absorbed_by"), Rule: "plan", Msg: into.ID + " absorbs " + t.ID + " but does not implement " + strings.Join(missing, ", ")})
		}
	}
	return findings
}

// reach returns, for each task, the set of ids it waits for directly or
// through other tasks.
func reach(tasks []Task, byID map[string]Task) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	var walk func(id string) map[string]bool
	walk = func(id string) map[string]bool {
		if r, ok := out[id]; ok {
			return r
		}
		r := map[string]bool{}
		out[id] = r // a cycle stops here; edges reports it
		for _, next := range waits(byID[id], byID) {
			r[next] = true
			for k := range walk(next) {
				r[k] = true
			}
		}
		return r
	}
	for _, t := range tasks {
		walk(t.ID)
	}
	return out
}

// overlaps applies rule 3: two open tasks that no edge orders, directly
// or through other tasks, may not own one path, a path of the tree or a
// literal path matched by both, the generated outputs left out.
func overlaps(tasks []Task, byID map[string]Task, paths, generated []string) []check.Finding {
	isGenerated := func(p string) bool {
		return slices.ContainsFunc(generated, func(g string) bool { return askfirst.GlobMatch(g, p) })
	}
	var openTasks []Task
	owned := map[string][]string{} // the tree paths each open task owns
	for _, t := range tasks {
		if !t.is(open) || len(t.Owns) == 0 || byID[t.ID].Path != t.Path {
			continue
		}
		openTasks = append(openTasks, t)
		for _, p := range paths {
			if !isGenerated(p) && slices.ContainsFunc(t.Owns, func(g string) bool { return askfirst.GlobMatch(g, p) }) {
				owned[t.ID] = append(owned[t.ID], p)
			}
		}
	}
	r := reach(tasks, byID)
	var findings []check.Finding
	for i, a := range openTasks {
		for _, b := range openTasks[i+1:] {
			if r[a.ID][b.ID] || r[b.ID][a.ID] {
				continue
			}
			if p, ok := shared(a, b, owned, isGenerated); ok {
				findings = append(findings, check.Finding{Where: b.where("owns"), Rule: "plan", Msg: fmt.Sprintf("%s and %s both own %s, and no needs or after orders them", a.ID, b.ID, p)})
			}
		}
	}
	return findings
}

// shared returns a path both tasks own: a tree path, or a literal owns
// entry of one that a glob of the other matches.
func shared(a, b Task, owned map[string][]string, isGenerated func(string) bool) (string, bool) {
	for _, p := range owned[a.ID] {
		if slices.Contains(owned[b.ID], p) {
			return p, true
		}
	}
	for _, pair := range [][2]Task{{a, b}, {b, a}} {
		for _, lit := range pair[0].Owns {
			if isGenerated(lit) {
				continue
			}
			for _, g := range pair[1].Owns {
				if askfirst.GlobMatch(g, lit) {
					return lit, true
				}
			}
		}
	}
	return "", false
}

// forecasts checks each open task's ask_first against its owns: a
// surface of .github/ask-first.yaml whose globs match a path the task
// owns, a tree path or the owns entry itself, is in the forecast. A
// forecast may name a surface the list does not hold yet: the task that
// adds it lands in its own time.
func forecasts(files []check.TreeFile, tasks []Task, paths []string) ([]check.Finding, error) {
	src, ok, err := check.FileIn(files, askFirstPath)
	if err != nil || !ok {
		return nil, err
	}
	list, err := askfirst.Parse(src)
	if err != nil {
		return []check.Finding{{Where: askFirstPath, Rule: "plan", Msg: "the forecasts cannot be checked: " + err.Error()}}, nil
	}
	var findings []check.Finding
	for _, t := range tasks {
		if !t.is(open) {
			continue
		}
		for _, s := range list.Surfaces {
			if slices.Contains(t.AskFirst, s.ID) {
				continue
			}
			if p, ok := touches(t.Owns, s.Globs, paths); ok {
				findings = append(findings, check.Finding{Where: t.where("ask_first"), Rule: "plan", Msg: fmt.Sprintf("ask_first misses %s: the task owns %s, which the surface's globs match", s.ID, p)})
			}
		}
	}
	return findings, nil
}

// touches returns a path that an owns entry and a surface glob both
// match: a tree path, or the entry itself read as a path.
func touches(owns, globs, paths []string) (string, bool) {
	for _, o := range owns {
		for _, g := range globs {
			if askfirst.GlobMatch(g, o) {
				return o, true
			}
		}
		for _, p := range paths {
			if askfirst.GlobMatch(o, p) && slices.ContainsFunc(globs, func(g string) bool { return askfirst.GlobMatch(g, p) }) {
				return p, true
			}
		}
	}
	return "", false
}

// Judge is the plan check of the commit fast and all read before any
// step.
func Judge(generated []string) func(context.Context, check.Commit) ([]check.Verdict, error) {
	return func(_ context.Context, c check.Commit) ([]check.Verdict, error) {
		findings, err := Of(c.Files, generated)
		return []check.Verdict{{Name: "plan", Findings: findings, Err: err}}, nil
	}
}
