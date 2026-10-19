// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sample = `---
id: T007
title: "A title with ` + "`code`" + `"
milestone: M1
module: julieta-core
fate: kept
owns:
  - a/**
needs:
  - T001
after: []
ask_first:
  - checks
sitting: none
merged: 2026-10-01
pr: 12
---

# T007 - A title

Goal: The first line of the goal
and its second line.

More text.
`

func TestParse(t *testing.T) {
	task, err := parse("docs/plan/tasks/T007-x.md", []byte(sample))
	require.NoError(t, err)
	assert.Equal(t, "T007", task.ID)
	assert.Equal(t, "A title with `code`", task.Title)
	assert.Equal(t, []string{"T001"}, task.Needs)
	assert.Empty(t, task.After)
	assert.Equal(t, []string{"checks"}, task.AskFirst)
	assert.Equal(t, "2026-10-01", task.Merged)
	assert.Equal(t, "12", task.PR)
	assert.Equal(t, "The first line of the goal and its second line.", task.Goal)
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"no front matter": "# T001\n",
		"unclosed":        "---\nid: T001\n",
		"not yaml":        "---\nid: [\n---\n",
	} {
		_, err := parse("p.md", []byte(in))
		assert.Error(t, err, name)
	}
}

// task builds a task of the given id and fate with the given edges.
func task(id, fate string, needs []string, after []string) Task {
	return Task{ID: id, Path: "docs/plan/tasks/" + id + ".md", Fate: fate, Needs: needs, After: after}
}

// TestReadyRule pins the rule of Ready in tools/ci/internal/plan/views.go:
// an open task of fate kept or new whose needs and after are all merged.
func TestReadyRule(t *testing.T) {
	tasks := []Task{
		task("T001", "merged", nil, nil),
		task("T002", "kept", []string{"T001"}, nil),              // ready
		task("T003", "new", nil, nil),                            // ready: no edge
		task("T004", "kept", []string{"T001"}, []string{"T002"}), // waits on an after
		task("T005", "kept", []string{"T002"}, nil),              // waits on a need
		task("T006", "deferred", []string{"T001"}, nil),          // deferred is never ready
		task("T007", "folded", nil, nil),                         // closed
		task("T008", "kept", []string{"T007"}, nil),              // a closed edge never lets it start
		task("T009", "kept", []string{"T404"}, nil),              // an unknown id is not merged
		task("T010", "kept", []string{"T001"}, []string{"T001"}), // ready
		task("T011", "merged", []string{"T004"}, nil),            // merged is not ready
		task("T012", "removed", nil, nil),                        // closed
		task("T013", "left-v1", nil, nil),                        // closed
		task("T014", "kept", nil, []string{"T006"}),              // waits on a deferred
	}
	setStatus(tasks)
	assert.Equal(t, []string{"T002", "T003", "T010"}, ready(tasks))
	want := map[string]string{
		"T001": statusMerged, "T004": statusWaiting, "T006": statusDeferred,
		"T007": statusClosed, "T011": statusMerged, "T012": statusClosed, "T013": statusClosed,
	}
	for _, tk := range tasks {
		if s, ok := want[tk.ID]; ok {
			assert.Equal(t, s, tk.Status, tk.ID)
		}
	}
}

// TestRealPlan reads the plan of this repository and holds each status to
// its definition, so a change to the rule shows here. The ids it yields
// are what "go run ./tools/ci plan --ready" prints; the line
// "go run ./tools/planviewer -ready" prints them for a diff.
func TestRealPlan(t *testing.T) {
	tasks, err := load(filepath.Join("..", "..", "..", "..", TasksDir))
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	byID := map[string]Task{}
	for _, tk := range tasks {
		byID[tk.ID] = tk
	}
	for _, tk := range tasks {
		switch tk.Fate {
		case "merged":
			assert.Equal(t, statusMerged, tk.Status, tk.ID)
		case "deferred":
			assert.Equal(t, statusDeferred, tk.Status, tk.ID)
		case "kept", "new":
			open := false
			for _, id := range append(append([]string{}, tk.Needs...), tk.After...) {
				open = open || byID[id].Fate != "merged"
			}
			if open {
				assert.Equal(t, statusWaiting, tk.Status, tk.ID)
			} else {
				assert.Equal(t, statusReady, tk.Status, tk.ID)
			}
		default:
			assert.Equal(t, statusClosed, tk.Status, tk.ID)
		}
	}
}

// page reads the template of the package.
func page(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../template.html")
	require.NoError(t, err)
	return string(b)
}

func TestRenderEscapesData(t *testing.T) {
	tmpl := page(t)
	out, err := render(tmpl, []Task{{ID: "T001", Title: "</script><b>& <!-- x"}}, meta{Date: "2026-01-01"})
	require.NoError(t, err)
	assert.NotContains(t, out, "</script><b>")
	assert.NotContains(t, out, "<!-- x")
	// The closing tags are the template's own, none from the data.
	assert.Equal(t, strings.Count(tmpl, "</script>"), strings.Count(out, "</script>"))
	assert.Contains(t, out, "</script>")
	assert.NotContains(t, out, "/*DATA*/")
	assert.NotContains(t, out, "/*META*/")
	assert.Contains(t, out, `"date":"2026-01-01"`)
}

// A placeholder inside a task field is data, not a second placeholder.
func TestRenderPlaceholderInData(t *testing.T) {
	title := "x /*META*/{} y /*DATA*/[] z"
	out, err := render(page(t), []Task{{ID: "T001", Title: title}}, meta{Date: "2026-01-01"})
	require.NoError(t, err)
	assert.Contains(t, out, `const META = {"date":"2026-01-01","critical":[],"milestones":[],"trends":{"note":"","limit":0,"total":0,"snapshots":[]}};`)
	assert.Contains(t, out, `"title":"`+title+`"`)
	assert.Equal(t, 1, strings.Count(out, `"date":"2026-01-01"`))
}

// A file without needs, after or ask_first still gives arrays, which the
// page reads with map and length.
func TestOmittedListsAreArrays(t *testing.T) {
	task, err := parse("p.md", []byte("---\nid: T001\nfate: kept\n---\n"))
	require.NoError(t, err)
	out, err := render(page(t), []Task{task}, meta{})
	require.NoError(t, err)
	assert.Contains(t, out, `"needs":[],"after":[],"ask_first":[]`)
	assert.NotContains(t, out, `":null`)
}

// A task whose id is not T and three digits is left out, as the view of
// tools/ci plan does.
func TestBadIDIsLeftOut(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "T001-a.md"), []byte("---\nid: T001\nfate: kept\n---\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "T002-b.md"), []byte("---\nid: X2\nfate: kept\n---\n"), 0o644))
	ids, err := Ready(Options{Dir: dir})
	require.NoError(t, err)
	assert.Equal(t, []string{"T001"}, ids)
}

func TestRefusesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("keep"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "T001-a.md"), []byte("---\nid: T001\nfate: kept\n---\n"), 0o644))
	out := filepath.Join(dir, "out.html")
	require.NoError(t, os.Symlink(target, out))
	o := Options{Dir: dir, Template: "../../template.html", Plan: planFile(t), Out: out}
	_, _, err := Write(t.Context(), o, time.Now())
	require.ErrorContains(t, err, "symbolic link")
	b, _ := os.ReadFile(target)
	assert.Equal(t, "keep", string(b))

	// A task entry that is a link is refused too.
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "T002-b.md")))
	_, err = Ready(o)
	require.ErrorContains(t, err, "not a regular file")
}

func TestPageIsSelfContained(t *testing.T) {
	for _, bad := range []string{"<script src", "<link", "@import", "fonts.googleapis", "cdnjs", "url(http"} {
		assert.NotContains(t, page(t), bad)
	}
}

func TestWriteAndReady(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "T001-a.md"), []byte(sample), 0o644))
	o := Options{Dir: dir, Template: "../../template.html", Plan: planFile(t), Out: filepath.Join(dir, "out.html")}

	got, note, err := Write(t.Context(), o, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, o.Out, got)
	assert.Contains(t, note, "history is off")
	html, err := os.ReadFile(o.Out)
	require.NoError(t, err)
	assert.Contains(t, string(html), `"id":"T007"`)
	assert.Contains(t, string(html), `"date":"2026-01-02"`)
	assert.Contains(t, string(html), `"critical":["T007"]`)
	assert.Contains(t, string(html), `"id":"M1"`)

	ids, err := Ready(o)
	require.NoError(t, err)
	assert.Empty(t, ids) // T007 needs T001, which is not in the directory

	o.Dir = t.TempDir()
	_, err = Ready(o)
	require.Error(t, err)
	_, _, err = Write(t.Context(), o, time.Now())
	require.Error(t, err)
}

// A placeholder inside a plan.md milestone cell is data too.
func TestRenderPlaceholderInMilestone(t *testing.T) {
	ms := []Milestone{{ID: "M0", Name: "x /*META*/{} y", EndsWhen: "/*DATA*/[] z", Delivers: "</script>"}}
	out, err := render(page(t), []Task{{ID: "T001", Title: "t"}}, meta{Date: "2026-01-01", Milestones: ms})
	require.NoError(t, err)
	assert.Contains(t, out, `"name":"x /*META*/{} y","ends_when":"/*DATA*/[] z"`)
	assert.Contains(t, out, `\u003c/script\u003e`)
	assert.Equal(t, 1, strings.Count(out, `"id":"T001"`))
	assert.Equal(t, 1, strings.Count(out, `"date":"2026-01-01"`))
}

// The Goals view keeps a task whose milestone is not in the table, in a
// last card, and escapes what it shows.
func TestGoalsKeepsUnknownMilestones(t *testing.T) {
	p := page(t)
	assert.Contains(t, p, "Not in docs/plan.md")
	assert.Contains(t, p, "m.orphan ? lost")
	assert.Contains(t, p, "code(m.name)")
}

// The body is the Markdown after the front matter, without the blank
// lines around it, and the Goal paragraph stays in it.
func TestParseBody(t *testing.T) {
	task, err := parse("p.md", []byte(sample))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(task.Body, "# T007 - A title\n"), task.Body)
	assert.True(t, strings.HasSuffix(task.Body, "More text."), task.Body)
	assert.NotContains(t, task.Body, "id: T007")
	assert.False(t, task.BodyTruncated)

	empty, err := parse("p.md", []byte("---\nid: T001\n---\n"))
	require.NoError(t, err)
	assert.Empty(t, empty.Body)
}

// A huge body is cut to MaxBody bytes on a character boundary and says so.
func TestBodyIsBounded(t *testing.T) {
	for _, unit := range []string{"a", "\u00e9", "\u20ac", "\U0001F600"} {
		body := strings.Repeat(unit, MaxBody/len(unit)+2) // the cut falls inside a character for the wide units
		task, err := parse("p.md", []byte("---\nid: T001\n---\n"+body+"\n"))
		require.NoError(t, err)
		assert.LessOrEqual(t, len(task.Body), MaxBody, unit)
		assert.Greater(t, len(task.Body), MaxBody-utf8.UTFMax, unit)
		assert.True(t, utf8.ValidString(task.Body), unit)
		assert.True(t, task.BodyTruncated, unit)
	}
	exact := strings.Repeat("a", MaxBody)
	task, err := parse("p.md", []byte("---\nid: T001\n---\n"+exact))
	require.NoError(t, err)
	assert.Equal(t, exact, task.Body)
	assert.False(t, task.BodyTruncated)
}

// Hostile text in a body cannot leave the data script: encoding/json
// escapes it, and the placeholders in it are not replaced.
func TestRenderBodyIsEscaped(t *testing.T) {
	body := "```\n</script><script>alert(1)</script>\n```\n[x](javascript:alert(1)) /*DATA*/[] /*META*/{}"
	task, err := parse("p.md", []byte("---\nid: T001\nfate: kept\n---\n"+body))
	require.NoError(t, err)
	tmpl := page(t)
	out, err := render(tmpl, []Task{task}, meta{Date: "2026-01-01"})
	require.NoError(t, err)
	assert.Equal(t, strings.Count(tmpl, "</script>"), strings.Count(out, "</script>"))
	assert.Equal(t, strings.Count(tmpl, "<script>"), strings.Count(out, "<script>"))
	assert.Contains(t, out, `\u003c/script\u003e\u003cscript\u003ealert(1)`)
	assert.Contains(t, out, `/*DATA*/[] /*META*/{}`)
	assert.Equal(t, 1, strings.Count(out, `"date":"2026-01-01"`))
}

// The reader escapes the whole body before it adds any tag, and the page
// never puts task text into innerHTML except through that renderer. The
// renderer itself is run with hostile input under node by hand.
func TestReaderRendersThroughMarkdown(t *testing.T) {
	p := page(t)
	assert.Equal(t, 1, strings.Count(p, "md(t.body,"))
	assert.Equal(t, 0, strings.Count(p, "innerHTML = t.body"))
	assert.Equal(t, 0, strings.Count(p, "${t.body}"))
	start, end := strings.Index(p, "// md:begin"), strings.Index(p, "// md:end")
	require.True(t, start >= 0 && end > start)
	block := p[start:end]
	// The escape comes first in md(), before the lines reach a block.
	esc := strings.Index(block, `.replace(/[&<>"]/g`)
	blocks := strings.Index(block, "return mdBlocks(")
	assert.True(t, esc > 0 && esc < blocks)
	assert.Contains(t, block, `https:\/\/`)
	assert.NotContains(t, block, "innerHTML")
	assert.NotContains(t, block, "document.")
	for _, want := range []string{"<dialog", "showModal()", "data-open", "TASKHASH", "[A-Za-z0-9._~-]"} {
		assert.Contains(t, p, want)
	}
}
