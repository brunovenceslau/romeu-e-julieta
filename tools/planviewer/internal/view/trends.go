// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The bounds of the history read. The git verbs bound the size of one
// read (internal/gitsafe); these bound how many reads one run makes and
// for how long, so the page of a long history still builds.
const (
	// DefaultHistoryLimit is the number of newest commits the Trends view
	// reads when -history-limit is not given.
	DefaultHistoryLimit = 200
	// historyBudget is the time the whole history read may take.
	historyBudget = 2 * time.Minute
	// maxTaskFiles bounds the task files parsed for one snapshot.
	maxTaskFiles = 1000
)

// Commit is one commit that touched the task files.
type Commit struct {
	OID     string
	Time    time.Time
	Subject string
}

// File is one file of the task directory at a commit.
type File struct {
	Name string
	Data []byte
}

// Source is the read-only history the Trends view rests on. The command
// reads it through internal/gitsafe (history.go); the tests give a fake.
type Source interface {
	// Log lists the commits that touched the task directory, newest first.
	Log(ctx context.Context) ([]Commit, error)
	// Tasks lists the regular files directly under the task directory at
	// the commit.
	Tasks(ctx context.Context, oid string) ([]File, error)
}

// Counts are the tasks of one snapshot by status. Scope is every task
// that is not closed: open and merged.
type Counts struct {
	Merged   int `json:"merged"`
	Ready    int `json:"ready"`
	Waiting  int `json:"waiting"`
	Deferred int `json:"deferred"`
	Closed   int `json:"closed"`
	Scope    int `json:"scope"`
	Critical int `json:"critical"`
}

// Delta is what changed from the previous readable snapshot. The lists
// hold task ids in id order.
type Delta struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Merged   []string `json:"merged"`
	Deferred []string `json:"deferred"`
	Closed   []string `json:"closed"`
	Reopened []string `json:"reopened"`
	Entered  []string `json:"entered"` // ids that joined the critical path
	Left     []string `json:"left"`    // ids that left it
	CritFrom int      `json:"crit_from"`
	CritTo   int      `json:"crit_to"`
}

// Empty reports whether nothing changed.
func (d Delta) Empty() bool {
	return len(d.Added)+len(d.Removed)+len(d.Merged)+len(d.Deferred)+len(d.Closed)+len(d.Reopened)+len(d.Entered)+len(d.Left) == 0 && d.CritFrom == d.CritTo
}

// Snapshot is the plan at one commit. A snapshot whose files cannot be
// read or parsed holds Gap, the reason, and no counts.
type Snapshot struct {
	OID     string `json:"oid"`
	Time    string `json:"time"` // RFC 3339, UTC
	Subject string `json:"subject"`
	Gap     string `json:"gap"`
	Counts
	Delta *Delta `json:"delta"` // nil for the first readable snapshot

	crit  []string
	state map[string]string // id -> status
}

// Trends is the history of the plan, oldest snapshot first.
type Trends struct {
	// Note says why there is no history (it is off, or it cannot be read),
	// or that the read was cut short; empty when the read is whole.
	Note      string     `json:"note"`
	Limit     int        `json:"limit"`
	Total     int        `json:"total"` // commits that touched the task files
	Snapshots []Snapshot `json:"snapshots"`
}

// buildTrends reads the newest limit commits of src and turns each into
// a snapshot with the delta from the one before it. It never fails: a
// history that cannot be read is a Note, a commit that cannot be parsed
// is a gap.
func buildTrends(ctx context.Context, src Source, limit int) Trends {
	t := Trends{Limit: limit, Snapshots: []Snapshot{}}
	ctx, cancel := context.WithTimeout(ctx, historyBudget)
	defer cancel()
	commits, err := src.Log(ctx)
	if err != nil {
		t.Note = "The history cannot be read: " + err.Error()
		return t
	}
	t.Total = len(commits)
	if limit > 0 && len(commits) > limit {
		commits = commits[:limit]
	}
	// Newest first, so a cut at the time budget loses the oldest.
	for _, c := range commits {
		if ctx.Err() != nil {
			t.Note = budgetNote()
			break
		}
		s := snapshotOf(ctx, src, c)
		if ctx.Err() != nil {
			// Cut mid-snapshot: it is not a gap, the budget ran out.
			t.Note = budgetNote()
			break
		}
		t.Snapshots = append(t.Snapshots, s)
	}
	// Oldest first, and each delta reads against the readable snapshot before it.
	slices.Reverse(t.Snapshots)
	var prev *Snapshot
	for i := range t.Snapshots {
		s := &t.Snapshots[i]
		if s.Gap != "" {
			continue
		}
		if prev != nil {
			s.Delta = diff(*prev, *s)
		}
		prev = s
	}
	return t
}

func budgetNote() string {
	return fmt.Sprintf("The read stopped at the time budget (%s): the oldest snapshots are missing.", historyBudget)
}

// snapshotOf reads and parses the task files at one commit.
func snapshotOf(ctx context.Context, src Source, c Commit) Snapshot {
	s := Snapshot{OID: c.OID, Time: c.Time.UTC().Format(time.RFC3339), Subject: c.Subject}
	files, err := src.Tasks(ctx, c.OID)
	if ctx.Err() != nil {
		return s // the caller reports the budget
	}
	if err != nil {
		s.Gap = "the task files cannot be read: " + err.Error()
		return s
	}
	var tasks []Task
	names := 0
	for _, f := range files {
		if ctx.Err() != nil {
			return s
		}
		if !strings.HasPrefix(f.Name, "T") || !strings.HasSuffix(f.Name, ".md") {
			continue
		}
		if names++; names > maxTaskFiles {
			s.Gap = fmt.Sprintf("the commit holds more than %d task files", maxTaskFiles)
			return s
		}
		task, err := parse(TasksDir+"/"+f.Name, f.Data)
		if err != nil {
			s.Gap = "a task file cannot be parsed: " + err.Error()
			return s
		}
		if taskID.MatchString(task.ID) {
			tasks = append(tasks, task)
		}
	}
	if len(tasks) == 0 {
		s.Gap = "the commit holds no task file"
		return s
	}
	slices.SortFunc(tasks, func(a, b Task) int { return strings.Compare(a.Path, b.Path) })
	setStatus(tasks)
	s.state = map[string]string{}
	for _, t := range tasks {
		if _, dup := s.state[t.ID]; dup {
			continue
		}
		s.state[t.ID] = t.Status
		switch t.Status {
		case statusMerged:
			s.Merged++
		case statusReady:
			s.Ready++
		case statusWaiting:
			s.Waiting++
		case statusDeferred:
			s.Deferred++
		default:
			s.Closed++
		}
	}
	s.Scope = s.Merged + s.Ready + s.Waiting + s.Deferred
	s.crit = criticalPath(tasks)
	s.Critical = len(s.crit)
	return s
}

// isOpenStatus reports whether a status is a task that is not built.
func isOpenStatus(s string) bool {
	return s == statusReady || s == statusWaiting || s == statusDeferred
}

// diff compares two readable snapshots, old then new.
func diff(old, cur Snapshot) *Delta {
	d := &Delta{
		Added: []string{}, Removed: []string{}, Merged: []string{}, Deferred: []string{},
		Closed: []string{}, Reopened: []string{}, Entered: []string{}, Left: []string{},
		CritFrom: old.Critical, CritTo: cur.Critical,
	}
	for id, now := range cur.state {
		was, existed := old.state[id]
		switch {
		case !existed:
			d.Added = append(d.Added, id)
			continue
		case was == now:
			continue
		}
		switch {
		case now == statusMerged:
			d.Merged = append(d.Merged, id)
		case now == statusClosed:
			d.Closed = append(d.Closed, id)
		case now == statusDeferred && (was == statusReady || was == statusWaiting):
			d.Deferred = append(d.Deferred, id)
		case isOpenStatus(now) && (was == statusMerged || was == statusClosed):
			d.Reopened = append(d.Reopened, id)
		}
	}
	for id := range old.state {
		if _, ok := cur.state[id]; !ok {
			d.Removed = append(d.Removed, id)
		}
	}
	for _, id := range cur.crit {
		if !slices.Contains(old.crit, id) {
			d.Entered = append(d.Entered, id)
		}
	}
	for _, id := range old.crit {
		if !slices.Contains(cur.crit, id) {
			d.Left = append(d.Left, id)
		}
	}
	for _, l := range [][]string{d.Added, d.Removed, d.Merged, d.Deferred, d.Closed, d.Reopened, d.Entered, d.Left} {
		slices.Sort(l)
	}
	return d
}

// historyTrends is the Trends of a run: off, without a repository, or
// read through the options' source (the command's own, from the
// repository, when none is given).
func historyTrends(ctx context.Context, o Options) Trends {
	off := func(note string) Trends { return Trends{Note: note, Limit: o.HistoryLimit, Snapshots: []Snapshot{}} }
	if !o.History {
		return off("The history is off (-history=false).")
	}
	src := o.Source
	if src == nil {
		repo, err := filepath.Abs(o.Repo)
		if err != nil {
			return off("The history cannot be read: " + err.Error())
		}
		if _, err := os.Lstat(filepath.Join(repo, ".git")); err != nil {
			return off(fmt.Sprintf("%s is not the root of a git repository, so there is no history; run from the root of a clone, or pass -history=false.", repo))
		}
		src, err = gitSource(repo)
		if err != nil {
			return off("The history cannot be read: " + err.Error())
		}
	}
	return buildTrends(ctx, src, o.HistoryLimit)
}
