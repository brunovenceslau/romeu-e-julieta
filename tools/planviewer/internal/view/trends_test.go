// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fake is a Source over fixed commits, newest first, and the files at each.
type fake struct {
	commits  []Commit
	files    map[string][]File
	logErr   error
	tasksErr map[string]error
}

func (f fake) Log(context.Context) ([]Commit, error) { return f.commits, f.logErr }

func (f fake) Tasks(_ context.Context, oid string) ([]File, error) {
	return f.files[oid], f.tasksErr[oid]
}

func taskFile(name, id, fate string, needs ...string) File {
	body := "---\nid: " + id + "\nfate: " + fate + "\n"
	if len(needs) > 0 {
		body += "needs:\n"
		for _, n := range needs {
			body += "  - " + n + "\n"
		}
	}
	return File{Name: name, Data: []byte(body + "---\n")}
}

func at(day int) time.Time { return time.Date(2026, 3, day, 12, 0, 0, 0, time.UTC) }

// history is four commits, oldest c1: the third one cannot be parsed.
func history() fake {
	return fake{
		commits: []Commit{
			{OID: "c4", Time: at(4), Subject: "plan: defer T002, fold T003, drop T006, reopen T001"},
			{OID: "c3", Time: at(3), Subject: "plan: broken file"},
			{OID: "c2", Time: at(2), Subject: "plan: merge T001, add T003 and T006"},
			{OID: "c1", Time: at(1), Subject: "plan: start"},
		},
		files: map[string][]File{
			"c1": {taskFile("T001-a.md", "T001", "kept"), taskFile("T002-b.md", "T002", "kept", "T001")},
			"c2": {taskFile("T001-a.md", "T001", "merged"), taskFile("T002-b.md", "T002", "kept", "T001"), taskFile("T003-c.md", "T003", "kept", "T002"), taskFile("T006-f.md", "T006", "kept")},
			"c3": {{Name: "T001-a.md", Data: []byte("no front matter")}},
			"c4": {taskFile("T001-a.md", "T001", "kept"), taskFile("T002-b.md", "T002", "deferred", "T001"), taskFile("T003-c.md", "T003", "folded", "T002"), {Name: "README.md", Data: []byte("x")}},
		},
	}
}

func TestBuildTrendsSnapshots(t *testing.T) {
	tr := buildTrends(t.Context(), history(), 10)
	require.Empty(t, tr.Note)
	require.Len(t, tr.Snapshots, 4)
	assert.Equal(t, 4, tr.Total)
	// Oldest first.
	assert.Equal(t, []string{"c1", "c2", "c3", "c4"}, []string{tr.Snapshots[0].OID, tr.Snapshots[1].OID, tr.Snapshots[2].OID, tr.Snapshots[3].OID})

	c1, c2, c3, c4 := tr.Snapshots[0], tr.Snapshots[1], tr.Snapshots[2], tr.Snapshots[3]
	assert.Equal(t, Counts{Ready: 1, Waiting: 1, Scope: 2, Critical: 2}, c1.Counts)
	assert.Nil(t, c1.Delta, "the first snapshot has nothing to compare with")
	assert.Equal(t, "2026-03-01T12:00:00Z", c1.Time)

	assert.Equal(t, Counts{Merged: 1, Ready: 2, Waiting: 1, Scope: 4, Critical: 2}, c2.Counts)
	assert.Equal(t, &Delta{
		Added: []string{"T003", "T006"}, Removed: []string{}, Merged: []string{"T001"}, Deferred: []string{}, Closed: []string{}, Reopened: []string{},
		Entered: []string{"T003"}, Left: []string{"T001"}, CritFrom: 2, CritTo: 2,
	}, c2.Delta)

	assert.Contains(t, c3.Gap, "front matter")
	assert.Equal(t, Counts{}, c3.Counts)
	assert.Nil(t, c3.Delta)

	// c4 reads against c2, the readable snapshot before the gap.
	assert.Equal(t, Counts{Ready: 1, Deferred: 1, Closed: 1, Scope: 2, Critical: 2}, c4.Counts)
	assert.Equal(t, &Delta{
		Added: []string{}, Removed: []string{"T006"}, Merged: []string{}, Deferred: []string{"T002"}, Closed: []string{"T003"}, Reopened: []string{"T001"},
		Entered: []string{"T001"}, Left: []string{"T003"}, CritFrom: 2, CritTo: 2,
	}, c4.Delta)
}

func TestBuildTrendsLimit(t *testing.T) {
	tr := buildTrends(t.Context(), history(), 2)
	assert.Equal(t, 4, tr.Total)
	assert.Equal(t, 2, tr.Limit)
	require.Len(t, tr.Snapshots, 2)
	assert.Equal(t, "c3", tr.Snapshots[0].OID)
	assert.Equal(t, "c4", tr.Snapshots[1].OID)
	assert.Nil(t, tr.Snapshots[1].Delta, "the only readable snapshot of the window has no predecessor")
}

func TestBuildTrendsFailures(t *testing.T) {
	f := history()
	f.tasksErr = map[string]error{"c2": errors.New("blob missing")}
	tr := buildTrends(t.Context(), f, 10)
	require.Len(t, tr.Snapshots, 4)
	assert.Contains(t, tr.Snapshots[1].Gap, "blob missing")

	empty := fake{commits: []Commit{{OID: "e1", Time: at(1), Subject: "s"}}}
	tr = buildTrends(t.Context(), empty, 10)
	require.Len(t, tr.Snapshots, 1)
	assert.Contains(t, tr.Snapshots[0].Gap, "no task file")

	tr = buildTrends(t.Context(), fake{logErr: errors.New("shallow repository")}, 10)
	assert.Contains(t, tr.Note, "shallow repository")
	assert.NotNil(t, tr.Snapshots)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	tr = buildTrends(ctx, history(), 10)
	assert.Contains(t, tr.Note, "time budget")
}

func TestDiffEmpty(t *testing.T) {
	tr := buildTrends(t.Context(), fake{
		commits: []Commit{{OID: "b", Time: at(2), Subject: "s"}, {OID: "a", Time: at(1), Subject: "s"}},
		files:   map[string][]File{"a": {taskFile("T001-a.md", "T001", "kept")}, "b": {taskFile("T001-a.md", "T001", "kept")}},
	}, 10)
	require.NotNil(t, tr.Snapshots[1].Delta)
	assert.True(t, tr.Snapshots[1].Delta.Empty())
}

func TestHistoryTrendsOffAndNoRepository(t *testing.T) {
	tr := historyTrends(t.Context(), Options{History: false, HistoryLimit: 5})
	assert.Contains(t, tr.Note, "history is off")
	assert.Empty(t, tr.Snapshots)

	tr = historyTrends(t.Context(), Options{History: true, HistoryLimit: 5, Repo: t.TempDir()})
	assert.Contains(t, tr.Note, "not the root of a git repository")
	assert.NotNil(t, tr.Snapshots)

	tr = historyTrends(t.Context(), Options{History: true, HistoryLimit: 5, Source: history()})
	assert.Len(t, tr.Snapshots, 4)
}

// A subject or a task field with markup stays data in the page: the
// JSON cannot close the script element, and the template escapes it.
func TestTrendsHostileText(t *testing.T) {
	hostile := `</script><img src=x onerror=alert(1)> /*DATA*/[] /*META*/{}`
	f := fake{
		commits: []Commit{{OID: "a", Time: at(1), Subject: hostile}},
		files:   map[string][]File{"a": {{Name: "T001-a.md", Data: []byte("---\nid: T001\ntitle: '" + strings.ReplaceAll(hostile, "'", "''") + "'\nfate: kept\n---\nGoal: " + hostile + "\n")}}},
	}
	tr := buildTrends(t.Context(), f, 5)
	require.Len(t, tr.Snapshots, 1)
	assert.Equal(t, hostile, tr.Snapshots[0].Subject)

	tmpl := page(t)
	out, err := render(tmpl, []Task{{ID: "T001", Title: hostile, Goal: hostile}}, meta{Date: "2026-01-01", Trends: tr})
	require.NoError(t, err)
	assert.Equal(t, strings.Count(tmpl, "</script>"), strings.Count(out, "</script>"))
	assert.NotContains(t, out, "<img src=x")
	assert.Equal(t, 1, strings.Count(out, `"date":"2026-01-01"`))
	assert.Contains(t, out, `</script>`)
}

// The page escapes what it shows from a commit and links only a valid object id.
func TestTrendsTemplateEscapes(t *testing.T) {
	p := page(t)
	assert.Contains(t, p, `esc(s.subject)`)
	assert.Contains(t, p, `const OID = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/`)
	assert.Contains(t, p, `OID.test(oid)`)
	assert.Contains(t, p, `https://github.com/brunovenceslau/romeu-e-julieta/commit/${oid}`)
	assert.Contains(t, p, `esc(s.gap)`)
	assert.Contains(t, p, `esc(TR.note)`)
	assert.Contains(t, p, `["trends", "Trends", trendsView]`)
	assert.Contains(t, p, `id="v-trends"`)
}

// cutter cancels the context while a snapshot is being read.
type cutter struct {
	fake
	cancel context.CancelFunc
}

func (c cutter) Tasks(ctx context.Context, oid string) ([]File, error) {
	fs, err := c.fake.Tasks(ctx, oid)
	if oid == "c2" {
		c.cancel()
	}
	return fs, err
}

// A cut in the middle of a snapshot is the time budget, not a gap.
func TestBuildTrendsCutMidLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	tr := buildTrends(ctx, cutter{history(), cancel}, 10)
	assert.Contains(t, tr.Note, "time budget")
	require.Len(t, tr.Snapshots, 2, "c4 and c3 were read before the cut at c2")
	assert.Equal(t, "c3", tr.Snapshots[0].OID)
	assert.Equal(t, "c4", tr.Snapshots[1].OID)
	assert.Contains(t, tr.Snapshots[0].Gap, "front matter") // its own gap, not the cut's
}

func TestSnapshotTooManyFiles(t *testing.T) {
	var files []File
	for i := 0; i <= maxTaskFiles; i++ {
		files = append(files, File{Name: "T.md", Data: []byte("---\nid: T001\nfate: kept\n---\n")})
	}
	s := snapshotOf(t.Context(), fake{files: map[string][]File{"a": files}}, Commit{OID: "a", Time: at(1)})
	assert.Contains(t, s.Gap, "more than 1000 task files")
}

// Two files with one id in a commit count once, the first.
func TestSnapshotDuplicateID(t *testing.T) {
	f := fake{files: map[string][]File{"a": {taskFile("T001-a.md", "T001", "kept"), taskFile("T001-b.md", "T001", "merged")}}}
	s := snapshotOf(t.Context(), f, Commit{OID: "a", Time: at(1)})
	assert.Empty(t, s.Gap)
	assert.Equal(t, 1, s.Scope)
	assert.Equal(t, 1, s.Ready)
}
