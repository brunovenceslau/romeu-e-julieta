// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// reqTable is the retired table of a fixture, with the rows given.
func reqTable(rows ...string) string {
	return strings.Join(append([]string{reqRetiredRule, "", reqRetiredHeader, "|---|---|---|---|"}, rows...), "\n") + "\n"
}

// reqFixture is a tree that passes reqs: an index with two ids, a
// chapter with one, and the retired table with one retired id.
func reqFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write("docs/spec.md", "# Index\n\n- `REQ-0001` One.\n- `REQ-0002` Two.\n\n## Open questions\n\n| Q | Default |\n|---|---|\n| Q1 | none |\n")
	r.Write("docs/spec/00-scope.md", "# 00\n\n## 0.1 Objective\n\n`REQ-0003` Three.\n\n## 0.5 Scope\n\n| Feature | Needed by |\n|---|---|\n| a | S1 |\n\n## 0.6 Next\n\nText.\n")
	r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-0004` | 2026-10-12 | Four. | none |"))
	return r
}

// reqTaskFile is a task file of the given fate that cites ids.
func reqTaskFile(fate string, ids ...string) string {
	return "---\nid: T001\nfate: " + fate + "\n---\n\n# T001\n\n- Implements: " + strings.Join(ids, ", ") + "\n- Verify: go test\n"
}

func TestReqsPasses(t *testing.T) {
	r := reqFixture(t)
	r.Commit("fixture")
	code, out := runCI(t, r, nil, nil, "reqs")
	assert.Equal(t, exitOK, code, out)
	assert.Equal(t, "reqs: ok\n", out)
}

// TestReqsRules has one fixture for each failure of 12 12.11: each case
// changes a tree that passes and names the finding it expects.
func TestReqsRules(t *testing.T) {
	tests := []struct {
		name   string
		change func(r *gittest.Repo)
		want   []string
	}{
		{
			name:   "a placeholder left in a page",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Five.\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: a placeholder is left; run \"go run ./tools/ci reqs allocate\""},
		},
		{
			name:   "a token after the prefix that is neither four digits nor new",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-12` Five; `REQ-abcd` six.\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: the token after REQ- is neither four digits nor new"},
		},
		{
			name:   "an id that appears twice",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-0002` Again.\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: REQ-0002 appears twice; its first place is docs/spec.md:4"},
		},
		{
			name:   "an id both live and retired",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-0004` Four again.\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: REQ-0004 is live and in the retired table at docs/spec/12-engineering.md:9"},
		},
		{
			name:   "a number below the highest that is neither live nor retired",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-0007` Seven.\n") },
			want: []string{
				"docs/spec.md: reqs: REQ-0005 is neither live nor retired; a dropped obligation is retired, never deleted",
				"docs/spec.md: reqs: REQ-0006 is neither live nor retired",
			},
		},
		{
			name:   "an id in chapter 13",
			change: func(r *gittest.Repo) { r.Write("docs/spec/13-ledger.md", "# 13\n\n`REQ-0005` A design.\n") },
			want:   []string{"docs/spec/13-ledger.md:3: reqs: REQ-0005 sits in a page or section that carries no id (12 12.11)"},
		},
		{
			name: "an id in a journey of chapter 09",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/09-journeys.md", "# 09\n\n`REQ-0005` The opening keeps ids.\n\n## J1 Onboarding\n\n`REQ-0006` A step.\n")
			},
			want: []string{"docs/spec/09-journeys.md:7: reqs: REQ-0006 sits in a page or section that carries no id (12 12.11)"},
		},
		{
			name: "an id in the open questions of the index",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec.md", "# Index\n\n- `REQ-0001` One.\n- `REQ-0002` Two.\n\n## Open questions\n\n| Q | Default |\n|---|---|\n| Q1 | `REQ-0005` none |\n")
			},
			want: []string{"docs/spec.md:10: reqs: REQ-0005 sits in a page or section that carries no id (12 12.11)"},
		},
		{
			name: "an id in section 0.5 of chapter 00",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/00-scope.md", "# 00\n\n## 0.1 Objective\n\n`REQ-0003` Three.\n\n## 0.5 Scope\n\n| Feature | Needed by |\n|---|---|\n| `REQ-0005` a | S1 |\n\n## 0.6 Next\n\n`REQ-0006` Text.\n")
			},
			want: []string{"docs/spec/00-scope.md:11: reqs: REQ-0005 sits in a page or section that carries no id (12 12.11)"},
		},
		{
			name:   "an id in a decision record",
			change: func(r *gittest.Repo) { r.Write("docs/adr/0001-first.md", "# 1. First\n\n`REQ-0001` is cited here.\n") },
			want:   []string{"docs/adr/0001-first.md:3: reqs: REQ-0001 sits in a decision record, which carries no id (12 12.11)"},
		},
		{
			name: "a retired row that does not parse",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable(
					"| `REQ-0004` | 2026-10-12 | Four. |",
					"| four | 2026-10-12 | Four. | none |",
					"| `REQ-0005` | yesterday | Five. | none |"))
			},
			want: []string{
				"docs/spec/12-engineering.md:9: reqs: a retired row has 3 cells, not 4",
				"docs/spec/12-engineering.md:10: reqs: a retired row's first cell is not an id",
				"docs/spec/12-engineering.md:11: reqs: a retired row's second cell is not an ISO date",
			},
		},
		{
			name: "an Implements entry whose id is retired or unknown",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)", "REQ-0004 (12 12.11)", "REQ-0099 (none)"))
			},
			want: []string{
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0004, which is retired",
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0099, which is unknown",
			},
		},
		{
			name: "a live id that no task cites",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)"))
			},
			want: []string{"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task of fate kept, new, deferred, merged"},
		},
		{
			name: "a folded task is not read",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)"))
				r.Write("docs/plan/tasks/T002-b.md", strings.Replace(reqTaskFile("folded", "REQ-0003 (00 0.1)", "REQ-0099 (none)"), "T001", "T002", 2))
			},
			want: []string{"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := reqFixture(t)
			tt.change(r)
			r.Commit("fixture")
			code, out := runCI(t, r, nil, nil, "reqs")
			assert.Equal(t, exitFail, code, out)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			assert.NotContains(t, out, "reqs: ok")
		})
	}
}

func TestReqsCoverage(t *testing.T) {
	t.Run("the coverage half is silent without task files", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan.md", "# Plan\n\nNo task file yet.\n")
		r.Commit("fixture")
		code, out := runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a deferred-only id is reported and passes", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)"))
		r.Write("docs/plan/tasks/T002-b.md", strings.Replace(reqTaskFile("deferred", "REQ-0002 (index)", "REQ-0003 (00 0.1)"), "T001", "T002", 2))
		r.Commit("fixture")
		code, out := runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitOK, code, out)
		assert.Equal(t, "reqs: ok\nreqs: 1 id(s) cited only by deferred tasks: REQ-0003\n", out)
	})
	t.Run("an Implements line that wraps is read whole", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", "---\nid: T001\nfate: new\n---\n\n- Implements: `REQ-0001` (index), REQ-0002\n  (index),\n  REQ-0003 (00 0.1)\n- Verify: go test\n")
		r.Commit("fixture")
		code, out := runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitOK, code, out)
	})
	t.Run("an untracked task file does not satisfy the coverage half", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)"))
		r.Commit("fixture")
		r.Write("docs/plan/tasks/T002-b.md", strings.Replace(reqTaskFile("new", "REQ-0003 (00 0.1)"), "T001", "T002", 2))
		code, out := runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "REQ-0003 is cited by no task")
	})
	t.Run("a task file without a fate is left to the plan check", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
		r.Write("docs/plan/tasks/T002-b.md", "# T002\n\n- Implements: REQ-0099 (none)\n")
		r.Commit("fixture")
		code, out := runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitOK, code, out)
	})
}

func TestReqsAllocate(t *testing.T) {
	read := func(t *testing.T, r *gittest.Repo, p string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(p)))
		require.NoError(t, err)
		return string(b)
	}
	t.Run("allocate numbers in document order from the highest live or retired id", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec.md", "# Index\n\n- `REQ-new` Five, then `REQ-new` six.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n")
		r.Write("docs/spec/00-scope.md", "# 00\n\n`REQ-0003` Three; `REQ-new` seven.\n")
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Eight.\n")
		r.Commit("fixture")
		code, out := runCI(t, r, nil, nil, "reqs", "allocate")
		assert.Equal(t, exitOK, code, out)
		assert.Equal(t, "docs/spec.md:3: REQ-0005\ndocs/spec.md:3: REQ-0006\ndocs/spec/00-scope.md:3: REQ-0007\ndocs/spec/01-model.md:3: REQ-0008\nreqs: 4 placeholder(s) numbered\n", out)
		assert.Equal(t, "# Index\n\n- `REQ-0005` Five, then `REQ-0006` six.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n", read(t, r, "docs/spec.md"))
		assert.Equal(t, "# 00\n\n`REQ-0003` Three; `REQ-0007` seven.\n", read(t, r, "docs/spec/00-scope.md"))
		assert.Equal(t, "# 01\n\n`REQ-0008` Eight.\n", read(t, r, "docs/spec/01-model.md"))
		assert.Equal(t, "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-0004` | 2026-10-12 | Four. | none |"), read(t, r, "docs/spec/12-engineering.md"), "a page without a placeholder is not rewritten")
		r.Commit("allocated")
		code, out = runCI(t, r, nil, nil, "reqs")
		assert.Equal(t, exitOK, code, out)
	})
	t.Run("allocate is idempotent", func(t *testing.T) {
		r := reqFixture(t)
		before := read(t, r, "docs/spec.md")
		info, err := os.Stat(filepath.Join(r.Dir, "docs", "spec.md"))
		require.NoError(t, err)
		code, out := runCI(t, r, nil, nil, "reqs", "allocate")
		assert.Equal(t, exitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
		assert.Equal(t, before, read(t, r, "docs/spec.md"))
		after, err := os.Stat(filepath.Join(r.Dir, "docs", "spec.md"))
		require.NoError(t, err)
		assert.Equal(t, info.ModTime(), after.ModTime(), "no byte changed, so no write")
	})
	t.Run("a placeholder in an excluded section refuses, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec.md", "# Index\n\n- `REQ-new` Five.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n\n## Open questions\n\n`REQ-new` A question.\n")
		code, out := runCI(t, r, nil, nil, "reqs", "allocate")
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "ci: docs/spec.md:9: a placeholder sits in a page or section that carries no id (12 12.11); nothing was written")
		assert.Contains(t, read(t, r, "docs/spec.md"), "- `REQ-new` Five.")
	})
	t.Run("a number past the last refuses, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-9999` | 2026-10-12 | Last. | none |"))
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Too many.\n")
		code, out := runCI(t, r, nil, nil, "reqs", "allocate")
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "ci: docs/spec/01-model.md:3: the next id would pass REQ-9999; nothing was written")
		assert.Contains(t, read(t, r, "docs/spec/01-model.md"), "`REQ-new`")
	})
	t.Run("the duplicate check catches a double allocation", func(t *testing.T) {
		// Two branches allocate from one base: each numbers its own
		// placeholder REQ-0005, and the merged tree holds both.
		base := reqFixture(t)
		base.Commit("base")
		branch := func(page, text string) string {
			r := gittest.New(t)
			r.Write("docs/spec.md", read(t, base, "docs/spec.md"))
			r.Write("docs/spec/00-scope.md", read(t, base, "docs/spec/00-scope.md"))
			r.Write("docs/spec/12-engineering.md", read(t, base, "docs/spec/12-engineering.md"))
			r.Write(page, text)
			code, out := runCI(t, r, nil, nil, "reqs", "allocate")
			require.Equal(t, exitOK, code, out)
			return read(t, r, page)
		}
		a := branch("docs/spec/01-model.md", "# 01\n\n`REQ-new` From branch a.\n")
		b := branch("docs/spec/02-layouts.md", "# 02\n\n`REQ-new` From branch b.\n")
		base.Write("docs/spec/01-model.md", a)
		base.Write("docs/spec/02-layouts.md", b)
		base.Commit("merged")
		code, out := runCI(t, base, nil, nil, "reqs")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "docs/spec/02-layouts.md:3: reqs: REQ-0005 appears twice; its first place is docs/spec/01-model.md:3")
	})
	t.Run("a page that is not a regular file refuses", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, "docs", "spec.md")))
		require.NoError(t, os.Symlink("spec", filepath.Join(r.Dir, "docs", "spec.md")))
		code, out := runCI(t, r, nil, nil, "reqs", "allocate")
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "ci: read docs/spec.md: not a regular file")
	})
}

func TestReqsUsage(t *testing.T) {
	r := reqFixture(t)
	r.Commit("fixture")
	for _, args := range [][]string{{"reqs", "x"}, {"reqs", "allocate", "x"}} {
		code, out := runCI(t, r, nil, nil, args...)
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "usage:")
	}
}

// TestReqsInTheCommitChecks: fast and all judge reqs with hygiene and
// workflows, from the one read of the commit's tree.
func TestReqsInTheCommitChecks(t *testing.T) {
	r := reqFixture(t)
	r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Five.\n")
	head := r.Commit("fixture")
	verdicts, err := commitChecks(context.Background(), git.Repo{Dir: r.Dir, Env: r.Env}, head)
	require.NoError(t, err)
	names := make([]string, len(verdicts))
	for i, v := range verdicts {
		names[i] = v.name
	}
	assert.Equal(t, []string{"hygiene", "workflows", "reqs"}, names)
	require.Len(t, verdicts[2].findings, 1)
	assert.Equal(t, "docs/spec/01-model.md:3: reqs: a placeholder is left; run \"go run ./tools/ci reqs allocate\"", verdicts[2].findings[0].String())
}
