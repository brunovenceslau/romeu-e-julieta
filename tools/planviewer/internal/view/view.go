// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package view reads the task files of docs/plan/tasks and renders them
// as one self-contained HTML page: the plan as a dependency graph, each
// task colored by its status. The page loads nothing from the network
// and opens from file://. The package writes no stream; the command
// above it, tools/planviewer, hands it paths and prints what it returns.
//
// The task files are read from the working tree, so the page shows the
// plan as it stands in the checkout, not only as committed. It reads the
// files named T*.md of the directory and, like "tools/ci plan --ready",
// leaves out a task whose id is not T and three digits; "tools/ci plan"
// reports a file of another name or a bad id, which this package does not.
package view

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/renameio/v2"
	"go.yaml.in/yaml/v3"
)

// TasksDir is the directory of the task files, from the repository root.
const TasksDir = "docs/plan/tasks"

// TemplateFile is the page with two placeholders, from the repository
// root. It is read when the tool runs and not embedded: the lint step
// admits no go:embed directive (tools/ci/internal/lint, directives).
const TemplateFile = "tools/planviewer/template.html"

// Task is one task file as the page needs it.
type Task struct {
	ID        string   `json:"id" yaml:"id"`
	Title     string   `json:"title" yaml:"title"`
	Milestone string   `json:"milestone" yaml:"milestone"`
	Module    string   `json:"module" yaml:"module"`
	Fate      string   `json:"fate" yaml:"fate"`
	Needs     []string `json:"needs" yaml:"needs"`
	After     []string `json:"after" yaml:"after"`
	AskFirst  []string `json:"ask_first" yaml:"ask_first"`
	Sitting   string   `json:"sitting" yaml:"sitting"`
	PR        string   `json:"pr" yaml:"pr"`
	Merged    string   `json:"merged" yaml:"merged"`
	Goal      string   `json:"goal" yaml:"-"`
	Path      string   `json:"path" yaml:"-"`
	Status    string   `json:"status" yaml:"-"`

	// Body is the Markdown after the front matter, cut to MaxBody bytes;
	// BodyTruncated says it was cut.
	Body          string `json:"body" yaml:"-"`
	BodyTruncated bool   `json:"body_truncated" yaml:"-"`
}

// MaxBody bounds the body one task carries into the page, so a huge file
// cannot swell it. The cut falls on a character boundary.
const MaxBody = 64 << 10

// The statuses of the page. A closed task is folded, removed or left to
// a later version.
const (
	statusMerged   = "merged"
	statusReady    = "ready"
	statusWaiting  = "waiting"
	statusDeferred = "deferred"
	statusClosed   = "closed"
)

var taskID = regexp.MustCompile(`^T[0-9]{3}$`)

var goalLine = regexp.MustCompile(`(?s)(?:^|\n)Goal:\s*(.+?)(?:\n\s*\n|\z)`)

// parse reads one task file: the YAML between the first two --- lines,
// and the first paragraph that starts with "Goal:".
func parse(path string, content []byte) (Task, error) {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return Task{}, fmt.Errorf("%s: the file does not start with a front matter (---)", path)
	}
	head, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		head, ok = strings.CutSuffix(rest, "\n---")
		if !ok {
			return Task{}, fmt.Errorf("%s: the front matter has no closing ---", path)
		}
	}
	var t Task
	if err := yaml.Unmarshal([]byte(head), &t); err != nil {
		return Task{}, fmt.Errorf("%s: the front matter is not YAML: %w", path, err)
	}
	t.Path = path
	t.Body, t.BodyTruncated = boundBody(body)
	// The page reads a list as an array, never null.
	for _, l := range []*[]string{&t.Needs, &t.After, &t.AskFirst} {
		if *l == nil {
			*l = []string{}
		}
	}
	if m := goalLine.FindStringSubmatch(body); m != nil {
		t.Goal = strings.Join(strings.Fields(m[1]), " ")
	}
	return t, nil
}

// boundBody trims the blank lines around body and cuts it to MaxBody
// bytes at a character boundary.
func boundBody(body string) (string, bool) {
	body = strings.Trim(body, "\n")
	if len(body) <= MaxBody {
		return body, false
	}
	cut := MaxBody
	for cut > 0 && !utf8.RuneStart(body[cut]) {
		cut--
	}
	return body[:cut], true
}

// load reads every T*.md under dir, in path order, with its status set.
func load(dir string) ([]Task, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "T*.md"))
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	var tasks []Task
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular file", p)
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		// The path on the page is dir joined with the name, with slashes.
		t, err := parse(filepath.ToSlash(p), content)
		if err != nil {
			return nil, err
		}
		if taskID.MatchString(t.ID) {
			tasks = append(tasks, t)
		}
	}
	setStatus(tasks)
	return tasks, nil
}

// startable are the open fates whose task waits only for its edges.
var startable = []string{"kept", "new"}

// isReady is the rule of Ready in tools/ci/internal/plan/views.go: the
// fate is kept or new, and every need and every after is merged. An id
// that names no task is not merged. byID holds the first task of each id.
func isReady(t Task, byID map[string]Task) bool {
	if !slices.Contains(startable, t.Fate) {
		return false
	}
	return !slices.ContainsFunc(slices.Concat(t.Needs, t.After), func(id string) bool {
		return byID[id].Fate != "merged"
	})
}

// setStatus gives each task the status the page colors it by.
func setStatus(tasks []Task) {
	byID := map[string]Task{}
	for _, t := range tasks {
		if _, ok := byID[t.ID]; !ok && t.ID != "" {
			byID[t.ID] = t
		}
	}
	for i, t := range tasks {
		switch {
		case t.Fate == "merged":
			tasks[i].Status = statusMerged
		case t.Fate == "deferred":
			tasks[i].Status = statusDeferred
		case isReady(t, byID) && byID[t.ID].Path == t.Path:
			tasks[i].Status = statusReady
		case slices.Contains(startable, t.Fate):
			tasks[i].Status = statusWaiting
		default:
			tasks[i].Status = statusClosed
		}
	}
}

// ready returns the ids of the ready tasks, in id order.
func ready(tasks []Task) []string {
	var ids []string
	for _, t := range tasks {
		if t.Status == statusReady {
			ids = append(ids, t.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

// meta is what the page shows about the checkout it was made from.
type meta struct {
	Date       string      `json:"date"`
	Critical   []string    `json:"critical"`
	Milestones []Milestone `json:"milestones"`
	Trends     Trends      `json:"trends"`
}

// render returns page, the template, with the tasks and the meta data in place.
// encoding/json escapes <, > and & in strings, so the data cannot close
// the script element it sits in.
func render(page string, tasks []Task, m meta) (string, error) {
	if m.Critical == nil {
		m.Critical = []string{}
	}
	if m.Milestones == nil {
		m.Milestones = []Milestone{}
	}
	if m.Trends.Snapshots == nil {
		m.Trends.Snapshots = []Snapshot{}
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return "", err
	}
	md, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	// One pass, so a placeholder inside the data is not replaced.
	return strings.NewReplacer("/*DATA*/[]", string(data), "/*META*/{}", string(md)).Replace(page), nil
}

// Options are the paths of one run.
type Options struct {
	Dir      string // the directory of the task files
	Template string // the page template
	Plan     string // docs/plan.md, for the milestone table
	Out      string // the file to write

	History      bool   // read the git history for the Trends view
	HistoryLimit int    // the newest commits of the history the page shows
	Repo         string // the root of the repository the history is read from
	Source       Source // the history to read; nil reads Repo through internal/gitsafe
}

// Defaults returns the paths of a run from the root of the repository.
func Defaults() Options {
	return Options{Dir: TasksDir, Template: TemplateFile, Plan: PlanFile, Out: "plan-viewer.html",
		History: true, HistoryLimit: DefaultHistoryLimit, Repo: "."}
}

// Ready returns the ids of the ready tasks under o.Dir, in id order,
// by the rule of "go run ./tools/ci plan --ready".
func Ready(o Options) ([]string, error) {
	tasks, err := read(o.Dir)
	if err != nil {
		return nil, err
	}
	return ready(tasks), nil
}

// CriticalPath returns the ids of the critical path under o.Dir, by the
// rule of "go run ./tools/ci plan --critical-path".
func CriticalPath(o Options) ([]string, error) {
	tasks, err := read(o.Dir)
	if err != nil {
		return nil, err
	}
	return criticalPath(tasks), nil
}

// Write renders the page into o.Out and returns its absolute path and
// the note of the Trends view: why it has no history, or "". A history
// that cannot be read is a note and never an error, so the other views
// still render.
func Write(ctx context.Context, o Options, now time.Time) (path, note string, err error) {
	tasks, err := read(o.Dir)
	if err != nil {
		return "", "", err
	}
	page, err := os.ReadFile(o.Template)
	if err != nil {
		return "", "", fmt.Errorf("%w; run from the root of the repository", err)
	}
	plan, err := os.ReadFile(o.Plan)
	if err != nil {
		return "", "", fmt.Errorf("%w; run from the root of the repository", err)
	}
	ms, err := parseMilestones(string(plan))
	if err != nil {
		return "", "", err
	}
	trends := historyTrends(ctx, o)
	html, err := render(string(page), tasks, meta{Date: now.Format("2006-01-02"), Critical: criticalPath(tasks), Milestones: ms, Trends: trends})
	if err != nil {
		return "", "", err
	}
	if err := writeFile(o.Out, []byte(html)); err != nil {
		return "", "", err
	}
	abs, err := filepath.Abs(o.Out)
	if err != nil {
		abs = o.Out
	}
	return abs, trends.Note, nil
}

// read loads the tasks and refuses an empty directory.
func read(dir string) ([]Task, error) {
	tasks, err := load(dir)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("no task file in %s; run from the root of the repository", dir)
	}
	return tasks, nil
}

// writeFile replaces path with data through a temporary file in the same
// directory, and refuses a path that is a symbolic link.
func writeFile(path string, data []byte) (err error) {
	if info, lerr := os.Lstat(path); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symbolic link; refusing to write through it", path)
	}
	t, err := renameio.NewPendingFile(path,
		renameio.WithTempDir(filepath.Dir(path)), renameio.WithStaticPermissions(0o644))
	if err != nil {
		return err
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return err
	}
	return t.CloseAtomicallyReplace()
}
