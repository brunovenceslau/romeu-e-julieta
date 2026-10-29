// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package plan

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// spec is one task of a fixture tree. The zero value is an open task
// of fate new that owns nothing and waits for nothing.
type spec struct {
	id, fate, absorbedBy, merged, pr string
	owns, needs, after, askFirst     []string
	implements                       string
}

// file writes s as a task file.
func (s spec) file() (string, string) {
	fate := s.fate
	if fate == "" {
		fate = "new"
	}
	impl := s.implements
	if impl == "" {
		impl = "none: a fixture."
	}
	list := func(xs []string) string {
		if len(xs) == 0 {
			return " []\n"
		}
		var b strings.Builder
		for _, x := range xs {
			b.WriteString("\n  - " + strconv.Quote(x))
		}
		return b.String() + "\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %s\ntitle: \"Task %s\"\nmilestone: M0\nmodule: none\nfate: %s\n", s.id, s.id, fate)
	if s.absorbedBy != "" {
		fmt.Fprintf(&b, "absorbed_by: %s\n", s.absorbedBy)
	}
	fmt.Fprintf(&b, "owns:%sneeds:%safter:%sask_first:%ssitting: none\n", list(s.owns), list(s.needs), list(s.after), list(s.askFirst))
	if s.merged != "" {
		fmt.Fprintf(&b, "merged: %s\n", s.merged)
	}
	if s.pr != "" {
		fmt.Fprintf(&b, "pr: %s\n", s.pr)
	}
	fmt.Fprintf(&b, "---\n\n# %s - Task %s\n\nGoal: a fixture.\n\n- Implements: %s\n- Operator: no\n", s.id, s.id, impl)
	return Dir + "/" + s.id + "-task-" + strings.ToLower(s.id) + ".md", b.String()
}

// merged is a closed task, merged in pull request 1.
func merged(id string) spec { return spec{id: id, fate: "merged", merged: "2026-10-01", pr: "1"} }

// tree returns the files of a fixture commit: the tasks, an ask-first
// list with one surface on tools/ci/**, and the extra files.
func tree(tasks []spec, extra ...string) []check.TreeFile {
	files := []check.TreeFile{{Path: askFirstPath, Mode: "100644", Content: []byte(citest.ValidAskFirst + `  - id: checks
    globs: [tools/ci/**]
    reason: the gates
`)}}
	for _, s := range tasks {
		p, c := s.file()
		files = append(files, check.TreeFile{Path: p, Mode: "100644", Content: []byte(c)})
	}
	for _, p := range extra {
		files = append(files, check.TreeFile{Path: p, Mode: "100644", Content: []byte("x\n")})
	}
	return files
}

// messages runs the check over files and returns the message of each
// finding.
func messages(t *testing.T, files []check.TreeFile) []string {
	t.Helper()
	findings, err := Of(files, Generated)
	require.NoError(t, err)
	var out []string
	for _, f := range findings {
		out = append(out, f.Msg)
	}
	return out
}

// TestTheTreePasses runs the check over the committed tree at HEAD.
func TestTheTreePasses(t *testing.T) {
	files, err := check.HeadTree(t.Context(), git.Repo{Dir: citest.ModuleRoot(t)}, "HEAD")
	require.NoError(t, err)
	findings, err := Of(files, Generated)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestOverCapOnlyShrinks holds the ratchet: each task of OverCap still
// cites more than maxImplements ids at HEAD, so one that comes down to
// the cap leaves the list in the same pull request.
func TestOverCapOnlyShrinks(t *testing.T) {
	files, err := check.HeadTree(t.Context(), git.Repo{Dir: citest.ModuleRoot(t)}, "HEAD")
	require.NoError(t, err)
	tasks, _, err := Read(files)
	require.NoError(t, err)
	byID := index(tasks)
	for _, id := range OverCap {
		require.Contains(t, byID, id)
		assert.Greater(t, len(byID[id].Implements), maxImplements, "%s is at or under the cap: take it off OverCap", id)
	}
}

// TestFindings has one fixture per rule; each fails with its message,
// and the passing fixtures show what each rule admits.
func TestFindings(t *testing.T) {
	over := make([]string, maxImplements+1)
	for i := range over {
		over[i] = fmt.Sprintf("REQ-%04d (10 10.2)", i+1)
	}
	tests := []struct {
		name  string
		tasks []spec
		extra []string
		want  []string
	}{
		{name: "a tree of open and merged tasks passes", tasks: []spec{merged("T001"), {id: "T002", needs: []string{"T001"}}}},
		{name: "a duplicate id", tasks: []spec{{id: "T001"}, {id: "T001", fate: "kept"}},
			want: []string{"the id T001 is also the id of docs/plan/tasks/T001-task-t001.md"}},
		{name: "a gap in the ids", tasks: []spec{{id: "T001"}, {id: "T003"}},
			want: []string{"the ids are not contiguous: no task file holds T002"}},
		{name: "an unknown needs", tasks: []spec{{id: "T001", needs: []string{"T009"}}},
			want: []string{"needs names T009, which no task file holds"}},
		{name: "an unknown after", tasks: []spec{{id: "T001", after: []string{"T009"}}},
			want: []string{"after names T009, which no task file holds"}},
		{name: "a deferred task's edges are checked", tasks: []spec{{id: "T001", fate: "deferred", needs: []string{"T009"}}},
			want: []string{"needs names T009, which no task file holds"}},
		{name: "an id in both needs and after", tasks: []spec{{id: "T001"}, {id: "T002", needs: []string{"T001"}, after: []string{"T001"}}},
			want: []string{"T001 is in both needs and after"}},
		{name: "a cycle", tasks: []spec{{id: "T001", needs: []string{"T002"}}, {id: "T002", after: []string{"T001"}}},
			want: []string{"needs and after form a cycle: T001 -> T002 -> T001"}},
		{name: "two unordered open tasks own one tree path",
			tasks: []spec{{id: "T001", owns: []string{"internal/a/**"}}, {id: "T002", owns: []string{"internal/a/x.go"}}},
			extra: []string{"internal/a/x.go"},
			want:  []string{"T001 and T002 both own internal/a/x.go, and no needs or after orders them"}},
		{name: "two unordered open tasks own one literal path that is not in the tree",
			tasks: []spec{{id: "T001", owns: []string{"internal/b/**"}}, {id: "T002", fate: "kept", owns: []string{"internal/b/new.go"}}},
			want:  []string{"T001 and T002 both own internal/b/new.go, and no needs or after orders them"}},
		{name: "two unordered open tasks own globs that meet in a file neither has made",
			tasks: []spec{{id: "T001", owns: []string{"internal/c/migrate*.go"}}, {id: "T002", owns: []string{"internal/c/*_test.go"}}},
			want:  []string{"T001 and T002 both own a path that internal/c/migrate*.go and internal/c/*_test.go both match, and no needs or after orders them"}},
		{name: "two globs that no path matches both are no overlap",
			tasks: []spec{{id: "T001", owns: []string{"internal/c/kits*.go"}}, {id: "T002", owns: []string{"internal/c/promote*.go", "internal/d/*_test.go"}}}},
		{name: "a glob that names a generated output meets no other",
			tasks: []spec{{id: "T001", owns: []string{"docs/reference/**"}}, {id: "T002", owns: []string{"docs/reference/*.md"}}}},
		{name: "a generated glob on the first side meets no other",
			tasks: []spec{{id: "T001", owns: []string{"docs/reference/**"}}, {id: "T002", owns: []string{"docs/**"}}}},
		{name: "a generated glob on the second side meets no other",
			tasks: []spec{{id: "T001", owns: []string{"docs/**"}}, {id: "T002", owns: []string{"docs/reference/**"}}}},
		{name: "a literal of the first task that a glob of the second matches",
			tasks: []spec{{id: "T001", owns: []string{"internal/b/new.go"}}, {id: "T002", owns: []string{"internal/b/**"}}},
			want:  []string{"T001 and T002 both own internal/b/new.go, and no needs or after orders them"}},
		{name: "a literal is named before two globs that meet earlier in the pair",
			tasks: []spec{{id: "T001", owns: []string{"internal/c/x_*.go", "internal/e/f.go"}}, {id: "T002", owns: []string{"internal/c/*_test.go", "internal/e/*.go"}}},
			want:  []string{"T001 and T002 both own internal/e/f.go, and no needs or after orders them"}},
		{name: "an owns entry longer than the bound",
			tasks: []spec{{id: "T001", owns: []string{"internal/" + strings.Repeat("a", 260) + ".go"}}},
			want:  []string{"an owns entry is longer than 256 bytes"}},
		{name: "an edge orders two globs that meet",
			tasks: []spec{{id: "T001", owns: []string{"internal/c/migrate*.go"}}, {id: "T002", after: []string{"T001"}, owns: []string{"internal/c/*_test.go"}}}},
		{name: "an edge through another task orders the pair",
			tasks: []spec{{id: "T001", owns: []string{"internal/a/**"}}, {id: "T002", needs: []string{"T001"}}, {id: "T003", after: []string{"T002"}, owns: []string{"internal/a/x.go"}}},
			extra: []string{"internal/a/x.go"}},
		{name: "a generated output is owned by no task",
			tasks: []spec{{id: "T001", owns: []string{"docs/reference/**"}}, {id: "T002", owns: []string{"docs/reference/checks.md"}}},
			extra: []string{"docs/reference/checks.md"}},
		{name: "a merged task owns nothing any more",
			tasks: []spec{{id: "T001", fate: "merged", merged: "2026-10-01", pr: "1", owns: []string{"internal/a/**"}}, {id: "T002", owns: []string{"internal/a/x.go"}}},
			extra: []string{"internal/a/x.go"}},
		{name: "merged without pr", tasks: []spec{{id: "T001", fate: "merged", merged: "2026-10-01"}},
			want: []string{"merged: comes with pr:, the number of the pull request"}},
		{name: "pr without merged", tasks: []spec{{id: "T001", fate: "merged", pr: "3"}},
			want: []string{"pr: comes with merged:, the date of the merge", "a merged task carries merged: and pr:"}},
		{name: "merged on a task of another fate", tasks: []spec{{id: "T001", merged: "2026-10-01", pr: "3"}},
			want: []string{"merged: and pr: belong to a task of fate merged"}},
		{name: "a fate outside the table", tasks: []spec{{id: "T001", fate: "done"}},
			want: []string{"the fate done is none of kept, new, deferred, folded, merged, left-v1, removed"}},
		{name: "a folded task without absorbed_by", tasks: []spec{{id: "T001", fate: "folded"}},
			want: []string{"a folded task names the task that absorbed it in absorbed_by"}},
		{name: "an absorbing task that does not implement what it absorbs",
			tasks: []spec{{id: "T001", fate: "folded", absorbedBy: "T002", implements: "REQ-0001 (10 10.2), REQ-0002 (10 10.2)."}, {id: "T002", implements: "REQ-0001 (10 10.2)."}},
			want:  []string{"T002 absorbs T001 but does not implement REQ-0002"}},
		{name: "an absorbing task that implements what it absorbs",
			tasks: []spec{{id: "T001", fate: "folded", absorbedBy: "T002", implements: "REQ-0001 (10 10.2)."}, {id: "T002", implements: "REQ-0001 (10 10.2), REQ-0003 (index)."}}},
		{name: "the forms of a reading aid", tasks: []spec{{id: "T001", implements: "REQ-0001 (index), REQ-0002 (10 (opening)), REQ-0003 (12 12.11)."}}},
		{name: "an entry without its reading aid", tasks: []spec{{id: "T001", implements: "REQ-0001, REQ-0002 (10 10.2)."}},
			want: []string{`the Implements entry "REQ-0001" is not an id with its reading aid, "REQ-nnnn (<page> <n>.<m>)", "(<page> (opening))" or "(index)", nor the none form`}},
		{name: "more than 40 ids", tasks: []spec{{id: "T001"}, {id: "T002"}, {id: "T003", implements: strings.Join(over, ", ") + "."}},
			want: []string{"the Implements line cites 41 ids, more than 40"}},
		{name: "a folded task's Implements is not read", tasks: []spec{{id: "T001", fate: "folded", absorbedBy: "T002", implements: "10 10.3, 05 5.2 (I4)."}, {id: "T002"}}},
		{name: "a forecast that misses a surface the owns touch", tasks: []spec{{id: "T001", owns: []string{"tools/ci/internal/plan/**"}}},
			want: []string{"ask_first misses checks: the task owns tools/ci/internal/plan/**, which the surface's globs match"}},
		{name: "a forecast that names the surface", tasks: []spec{{id: "T001", owns: []string{"tools/ci/internal/plan/**"}, askFirst: []string{"checks"}}}},
		{name: "a forecast may name a surface the list does not hold yet", tasks: []spec{{id: "T001", askFirst: []string{"sbxdrv"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, messages(t, tree(tt.tasks, tt.extra...)))
		})
	}
}

// TestOverCapTaskPasses checks that a task of OverCap may cite more than
// maxImplements ids.
func TestOverCapTaskPasses(t *testing.T) {
	ids := make([]string, maxImplements+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("REQ-%04d (10 10.2)", i+1)
	}
	var tasks []spec
	for n := 1; n <= 3; n++ {
		tasks = append(tasks, spec{id: fmt.Sprintf("T%03d", n), implements: strings.Join(ids, ", ") + "."})
	}
	require.Subset(t, OverCap, []string{"T001", "T002"})
	require.NotContains(t, OverCap, "T003")
	assert.Equal(t, []string{"the Implements line cites 41 ids, more than 40"}, messages(t, tree(tasks)), "T003 is not on the list")
}

// TestFileShape covers a file the check cannot read as a task, and a
// filename that is not the plan's.
func TestFileShape(t *testing.T) {
	_, good := spec{id: "T001"}.file()
	tests := []struct {
		name, path, content, want string
	}{
		{"no front matter", Dir + "/T001-a.md", "# T001\n", "the file does not start with a front matter (---)"},
		{"no closing line", Dir + "/T001-a.md", "---\nid: T001\n", "the front matter has no closing ---"},
		{"not YAML", Dir + "/T001-a.md", "---\nid: [\n---\n", "the front matter is not YAML"},
		{"a filename in capitals", Dir + "/T001-Plan.md", good, "a task file is named T<three digits>-<slug>.md"},
		{"an id the filename does not carry", Dir + "/T002-a.md", good, "the id T001 is not the one the filename carries, T002"},
		{"a missing field", Dir + "/T001-a.md", strings.Replace(good, "sitting: none\n", "", 1), "the front matter has no field sitting"},
		{"an unknown field", Dir + "/T001-a.md", strings.Replace(good, "sitting: none\n", "sitting: none\nowner: x\n", 1), "the field owner is none of"},
		{"a field twice", Dir + "/T001-a.md", strings.Replace(good, "sitting: none\n", "sitting: none\nsitting: M0\n", 1), "the field sitting appears twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messages(t, []check.TreeFile{{Path: tt.path, Mode: "100644", Content: []byte(tt.content)}})
			assert.True(t, slices.ContainsFunc(got, func(m string) bool { return strings.Contains(m, tt.want) }), "%q in %q", tt.want, got)
		})
	}
}

// TestAnUntrackedTaskFileThatSatisfiesANeedsTheCommittedTreeLacksFailsInAPrePushRun
// is the acceptance fixture: the judge reads the commit, as a pre-push
// run does, so a task file in the working tree alone satisfies no edge.
func TestAnUntrackedTaskFileThatSatisfiesANeedsTheCommittedTreeLacksFailsInAPrePushRun(t *testing.T) {
	r := gittest.New(t)
	p, c := spec{id: "T001", needs: []string{"T002"}}.file()
	r.Write(p, c)
	r.Commit("T001 needs T002")
	p, c = spec{id: "T002"}.file()
	r.Write(p, c) // untracked
	files, err := check.HeadTree(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	verdicts, err := Judge(Generated)(t.Context(), check.Commit{Repo: r.Repo, Files: files})
	require.NoError(t, err)
	require.Len(t, verdicts, 1)
	require.NoError(t, verdicts[0].Err)
	require.Len(t, verdicts[0].Findings, 1)
	assert.Equal(t, "needs names T002, which no task file holds", verdicts[0].Findings[0].Msg)
}

// TestCriticalPathCountsAfterEdges is the fixture graph of the
// acceptance: the longest chain runs through an after edge, merged tasks
// add nothing to it, and Ready lists the open tasks whose edges are all
// merged.
func TestCriticalPathCountsAfterEdges(t *testing.T) {
	files := tree([]spec{
		merged("T001"),
		{id: "T002", needs: []string{"T001"}},
		{id: "T003", after: []string{"T002"}},
		{id: "T004", needs: []string{"T003"}},
		{id: "T005", needs: []string{"T001"}},
		{id: "T006", fate: "deferred"},
		{id: "T007", fate: "kept", needs: []string{"T005"}},
	})
	tasks, findings, err := Read(files)
	require.NoError(t, err)
	require.Empty(t, findings)
	assert.Equal(t, []string{"T002", "T003", "T004"}, CriticalPath(tasks), "the after edge of T003 counts")
	assert.Equal(t, []string{"T002", "T005"}, Ready(tasks), "a deferred task is not ready, and T007 waits for T005")
}

// commitTree writes files as one commit of a new repository.
func commitTree(t *testing.T, files []check.TreeFile) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	for _, f := range files {
		r.Write(f.Path, string(f.Content))
	}
	r.Commit("the plan")
	return r
}

// TestRun covers the subcommand: the check and its exit status, the two
// views one id per line with status 0, and a wrong argument.
func TestRun(t *testing.T) {
	good := commitTree(t, tree([]spec{merged("T001"), {id: "T002", needs: []string{"T001"}}, {id: "T003", after: []string{"T002"}}}))
	bad := commitTree(t, tree([]spec{{id: "T001", needs: []string{"T009"}}}))
	tests := []struct {
		name string
		r    *gittest.Repo
		args []string
		code int
		out  string
	}{
		{"the check passes", good, nil, check.ExitOK, "plan: ok\n"},
		{"the check fails", bad, nil, check.ExitFail, "plan: 1 finding(s)"},
		{"the ready tasks", good, []string{"--ready"}, check.ExitOK, "T002\n"},
		{"the critical path", good, []string{"--critical-path"}, check.ExitOK, "T002\nT003\n"},
		{"a wrong argument", good, []string{"--bogus"}, check.ExitError, "plan takes no argument, --critical-path or --ready"},
		{"two arguments", good, []string{"--ready", "--critical-path"}, check.ExitError, "plan takes no argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out := citest.Run(t, tt.r, nil, Run, tt.args...)
			assert.Equal(t, tt.code, code, out)
			if tt.code == check.ExitOK && tt.args != nil {
				assert.Equal(t, tt.out, out)
			} else {
				assert.Contains(t, out, tt.out)
			}
		})
	}
}

// TestViewsLeaveOutAnIDOfAnotherShape checks that a task whose id is
// not the plan's shape, such as one that would print a second line or
// an escape, is in neither view.
func TestViewsLeaveOutAnIDOfAnotherShape(t *testing.T) {
	files := tree([]spec{{id: "T001"}})
	p, c := spec{id: "T002"}.file()
	c = strings.Replace(c, "id: T002\n", "id: \"T002\\n\\e[31mT999\"\n", 1)
	files = append(files, check.TreeFile{Path: p, Mode: "100644", Content: []byte(c)})
	r := commitTree(t, files)
	for _, view := range []string{"--ready", "--critical-path"} {
		code, out := citest.Run(t, r, nil, Run, view)
		assert.Equal(t, check.ExitOK, code)
		assert.Equal(t, "T001\n", out, view)
	}
}

// TestCriticalPathTiesAndCycles checks that of two chains of one length
// the one whose ids come first wins, on every run, and that a cycle,
// which the check reports, still gives a chain.
func TestCriticalPathTiesAndCycles(t *testing.T) {
	tasks, _, err := Read(tree([]spec{
		{id: "T001"}, {id: "T002", needs: []string{"T001"}},
		{id: "T003"}, {id: "T004", needs: []string{"T003"}},
	}))
	require.NoError(t, err)
	for range 5 {
		assert.Equal(t, []string{"T001", "T002"}, CriticalPath(tasks))
	}
	tasks, _, err = Read(tree([]spec{{id: "T001", needs: []string{"T002"}}, {id: "T002", needs: []string{"T001"}}}))
	require.NoError(t, err)
	assert.Len(t, CriticalPath(tasks), 2, "a cycle ends a chain where it closes")
}

// TestMoreFindings covers the rules a review added: an owns entry that
// is no glob, an id named twice, an open task waiting for a closed one,
// an absorbed_by that names the task itself or a closed task, a date
// that is no day, and a plan page without task files.
func TestMoreFindings(t *testing.T) {
	tests := []struct {
		name  string
		files []check.TreeFile
		want  []string
	}{
		{"an owns entry that is no glob", tree([]spec{{id: "T001", owns: []string{"a**b/x"}}}),
			[]string{`an owns entry is not a glob of .github/ask-first.yaml: glob "a**b/x": "**" must be a whole segment`}},
		{"an id named twice in needs", tree([]spec{{id: "T001"}, {id: "T002", needs: []string{"T001", "T001"}}}),
			[]string{"needs names T001 twice"}},
		{"an open task that waits for a folded one", tree([]spec{{id: "T001", fate: "folded", absorbedBy: "T002"}, {id: "T002"}, {id: "T003", needs: []string{"T001"}}}),
			[]string{"needs names T001, a task of fate folded, which is never merged"}},
		{"absorbed_by names the task itself", tree([]spec{{id: "T001", fate: "folded", absorbedBy: "T001"}}),
			[]string{"absorbed_by names the task itself"}},
		{"absorbed_by names a removed task", tree([]spec{{id: "T001", fate: "folded", absorbedBy: "T002"}, {id: "T002", fate: "removed"}}),
			[]string{"absorbed_by names T002, a task of fate removed"}},
		{"a merged date that is no day", tree([]spec{{id: "T001", fate: "merged", merged: "2026-13-45", pr: "1"}}),
			[]string{"merged: is a date, YYYY-MM-DD"}},
		{"a plan page without task files", []check.TreeFile{{Path: "docs/plan.md", Mode: "100644", Content: []byte("# Plan\n")}},
			[]string{"docs/plan.md is there, and no task file is"}},
		{"a tree with neither", []check.TreeFile{{Path: "README.md", Mode: "100644", Content: []byte("x\n")}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, messages(t, tt.files))
		})
	}
}

// TestOverlapOfManyDoubleStars is the case the audit found: an owns
// glob of many "**" against a deep literal path of another task. The
// check answers, and reports the overlap.
func TestOverlapOfManyDoubleStars(t *testing.T) {
	deep := strings.TrimSuffix(strings.Repeat("d/", 30), "/")
	got := messages(t, tree([]spec{{id: "T001", owns: []string{strings.Repeat("**/", 16) + "zzz"}}, {id: "T002", owns: []string{deep + "/zzz"}}}))
	assert.Equal(t, []string{"T001 and T002 both own " + deep + "/zzz, and no needs or after orders them"}, got)
}

// TestGeneratedHasNoLoneStarSegment holds the skip of rule 3 to what
// it means: shared leaves out an owns entry that a Generated glob
// matches as text, and a lone "*" segment of a Generated glob would
// match an entry's "**" (gen/* against gen/**), skipping an entry that
// also covers paths deeper than any generated output.
func TestGeneratedHasNoLoneStarSegment(t *testing.T) {
	for _, g := range Generated {
		assert.NotContains(t, strings.Split(g, "/"), "*", g)
	}
}
