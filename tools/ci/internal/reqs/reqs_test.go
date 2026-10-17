// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package reqs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// reqRetiredRule is the paragraph of 12 12.11 that the retired table
// of a fixture sits under.
const reqRetiredRule = "**Retired requirements.**"

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

// reqTaskFile is a task file of the given id and fate that cites ids.
func reqTaskFile(id, fate string, ids ...string) string {
	return "---\nid: " + id + "\nfate: " + fate + "\n---\n\n# " + id + "\n\n- Implements: " + strings.Join(ids, ", ") + "\n- Verify: go test\n"
}

func TestReqsPasses(t *testing.T) {
	r := reqFixture(t)
	r.Commit("fixture")
	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitOK, code, out)
	assert.Equal(t, "reqs: ok\n", out)
}

// TestReqsRules has one fixture for each failure of 12 12.11: each case
// changes a tree that passes and names the finding it expects.
func TestReqsRules(t *testing.T) {
	tests := []struct {
		name   string
		change func(r *gittest.Repo)
		want   []string // every finding, one entry per line of the output
		absent []string
	}{
		{
			name:   "a placeholder left in a page",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Five.\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: a placeholder is left; run \"go run ./tools/ci reqs allocate\""},
		},
		{
			name: "a token after the prefix that is neither four digits nor new",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-12` Five; `REQ-abcd` six.\n`REQ-10000` Seven.\n")
			},
			want: []string{
				"docs/spec/01-model.md:3: reqs: the token after REQ- is neither four digits nor new",
				"docs/spec/01-model.md:3: reqs: the token after REQ- is neither four digits nor new",
				"docs/spec/01-model.md:4: reqs: the token after REQ- is neither four digits nor new",
			},
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
			want: []string{
				"docs/spec/00-scope.md:11: reqs: REQ-0005 sits in a page or section that carries no id (12 12.11)",
				"docs/spec.md: reqs: REQ-0005 is neither live nor retired",
			},
			absent: []string{"docs/spec/00-scope.md:15:"},
		},
		{
			name:   "an id in a decision record",
			change: func(r *gittest.Repo) { r.Write("docs/adr/0001-first.md", "# 1. First\n\n`REQ-0001` is cited here.\n") },
			want:   []string{"docs/adr/0001-first.md:3: reqs: REQ-0001 sits in a decision record, which carries no id (12 12.11)"},
		},
		{
			name: "an id before a list marker",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-0005` 1. Five.\n  `REQ-0006` - Six.\n")
			},
			want: []string{
				"docs/spec/01-model.md:3: reqs: REQ-0005 precedes the marker of a list item (12 12.11)",
				"docs/spec/01-model.md:4: reqs: REQ-0006 precedes the marker of a list item (12 12.11)",
			},
		},
		{
			name: "an id inside a fenced block",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n```\ntree/   `REQ-0005` a file\n```\n")
			},
			want: []string{"docs/spec/01-model.md:4: reqs: REQ-0005 sits inside a fenced block (12 12.11)"},
		},
		{
			name: "an id inside a tilde fence or a fence indented by three spaces",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n~~~\n`REQ-0005` a\n~~~\n\n   ```\n`REQ-0006` b\n   ```\n")
			},
			want: []string{
				"docs/spec/01-model.md:4: reqs: REQ-0005 sits inside a fenced block (12 12.11)",
				"docs/spec/01-model.md:8: reqs: REQ-0006 sits inside a fenced block (12 12.11)",
			},
		},
		{
			name: "only a run of the opener's character as long as the opener closes a fence",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n````\n```\n`REQ-0005` a\n```\n````\n\n~~~\n```\n`REQ-0006` b\n~~~\n")
			},
			want: []string{
				"docs/spec/01-model.md:5: reqs: REQ-0005 sits inside a fenced block (12 12.11)",
				"docs/spec/01-model.md:11: reqs: REQ-0006 sits inside a fenced block (12 12.11)",
			},
		},
		{
			name: "a closer with text after it does not close a fence",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n```\n`REQ-0005` a\n``` x\n`REQ-0006` b\n```\n")
			},
			want: []string{
				"docs/spec/01-model.md:4: reqs: REQ-0005 sits inside a fenced block (12 12.11)",
				"docs/spec/01-model.md:6: reqs: REQ-0006 sits inside a fenced block (12 12.11)",
			},
		},
		{
			name: "an id in a table header",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/01-model.md", "# 01\n\n| A | `REQ-0005` B |\n|---|---|\n| a | b |\n")
			},
			want: []string{"docs/spec/01-model.md:3: reqs: REQ-0005 sits in a table header row (12 12.11)"},
		},
		{
			name:   "an id inside a code span",
			change: func(r *gittest.Repo) { r.Write("docs/spec/01-model.md", "# 01\n\n`{\"a\": `REQ-0005` 1}`\n") },
			want:   []string{"docs/spec/01-model.md:3: reqs: REQ-0005 sits inside another code span (12 12.11)"},
		},
		{
			name: "a placeholder inside a retired row",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-0004` | 2026-10-12 | Four, `REQ-new`. | none |"))
			},
			want: []string{"docs/spec/12-engineering.md:9: reqs: a placeholder is left"},
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
			name: "a retired id listed twice",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable(
					"| `REQ-0004` | 2026-10-12 | Four. | none |",
					"| `REQ-0004` | 2026-10-13 | Four again. | none |"))
			},
			want: []string{"docs/spec/12-engineering.md:10: reqs: REQ-0004 is retired twice"},
		},
		{
			name: "a retired row whose first cell lacks the prefix",
			change: func(r *gittest.Repo) {
				r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| 0004 | 2026-10-12 | Four. | none |"))
			},
			want: []string{"docs/spec/12-engineering.md:9: reqs: a retired row's first cell is not an id"},
		},
		{
			name: "an Implements entry whose id is retired or unknown",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)", "REQ-0004 (12 12.11)", "REQ-0099 (none)"))
			},
			want: []string{
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0004, which is retired",
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0099, which is unknown",
			},
		},
		{
			name: "an Implements entry with more digits than an id",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-00031 (00 0.1)"))
			},
			want: []string{
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-00031, which is no id",
				"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task",
			},
		},
		{
			name: "an id inside a longer token is no citation",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "XREQ-0001 (index)", "X-REQ-0001 (index)", "1REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
			},
			want: []string{"docs/spec.md:3: reqs: REQ-0001 is cited by no task of fate kept, new, deferred, merged"},
		},
		{
			name: "a reading aid that names another section",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (00 0.1)", "REQ-0003 (00 (opening))"))
			},
			want: []string{
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0002 as (00 0.1); it sits in index",
				"docs/plan/tasks/T001-a.md:8: reqs: Implements cites REQ-0003 as (00 (opening)); it sits in 00 0.1",
			},
		},
		{
			name: "a live id that no task cites",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
			},
			want: []string{"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task of fate kept, new, deferred, merged"},
		},
		{
			name: "an id cited by more tasks than the cap",
			change: func(r *gittest.Repo) {
				for i, fate := range []string{"kept", "new", "merged", "kept", "kept"} {
					id := "T00" + string(rune('1'+i))
					r.Write("docs/plan/tasks/"+id+"-a.md", reqTaskFile(id, fate, "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
				}
				r.Write("docs/plan/tasks/T006-a.md", reqTaskFile("T006", "new", "REQ-0001 (index)"))
			},
			want: []string{"docs/spec.md:3: reqs: REQ-0001 is cited by 6 tasks, more than the 5 of 12 12.11; one obligation has few implementers"},
		},
		{
			name: "a folded task is not read",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
				r.Write("docs/plan/tasks/T002-b.md", reqTaskFile("T002", "folded", "REQ-0003 (00 0.1)", "REQ-0099 (none)"))
			},
			want: []string{"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task"},
		},
		{
			name: "a fate outside the plan's set",
			change: func(r *gittest.Repo) {
				r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
				r.Write("docs/plan/tasks/T002-b.md", reqTaskFile("T002", "Kept", "REQ-0003 (00 0.1)"))
			},
			want: []string{
				"docs/plan/tasks/T002-b.md:3: reqs: the fate Kept is none of kept, new, deferred, merged, folded, removed, left-v1",
				"docs/spec/00-scope.md:5: reqs: REQ-0003 is cited by no task",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := reqFixture(t)
			tt.change(r)
			r.Commit("fixture")
			code, out := citest.Run(t, r, nil, Run)
			assert.Equal(t, check.ExitFail, code, out)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			for _, a := range tt.absent {
				assert.NotContains(t, out, a)
			}
			assert.Contains(t, out, check.Summary("reqs", len(tt.want)), "the findings are exactly the ones named")
		})
	}
}

// TestReqsReads holds what the check reads without a finding, and
// what it cannot read at all.
func TestReqsReads(t *testing.T) {
	t.Run("a bare mention of the prefix is no finding", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\nAn id is `REQ-` and four digits.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("an id after a list marker is well placed", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\n1. `REQ-0005` Five.\n   - `REQ-0006` Six, `REQ-0007` seven.\n\n| A | B |\n|---|---|\n| `REQ-0008` a | `b` `REQ-0009` c |\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a code span that wraps a line hides no id after it", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\nThe hint is `romeu run\n<name>`; `REQ-0005` five.\n\n`REQ-0006` Six.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("an unbalanced backtick does not leak past a blank line", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\nAn odd ` tick.\n\n`REQ-0005` Five.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a heading inside a fence does not change the section", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\n## 1.1 First\n\n```\n## 1.2 Fake\n```\n\n`REQ-0005` Five.\n")
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)", "REQ-0005 (01 1.1)"))
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a run of backticks with a backtick after it opens no fence", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\n``` a ` b\n\n`REQ-0005` Five.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a run indented by four spaces opens no fence", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\n    ```\n\n`REQ-0005` Five.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a run of two backticks opens no fence", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\n``\n\n`REQ-0005` Five.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a fence ends the backtick count of the paragraph before it", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\n\nAn odd ` tick.\n```\ncode\n```\n`REQ-0005` Five.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("a long line of ids is read in time linear in its length", func(t *testing.T) {
		// One MiB of text, then the ids. The backticks are counted span
		// by span, so the bytes counted are the line once for the spans
		// and once for its parity: two lengths of the line. A count from
		// the line's start for each span would read the text once per
		// id, ten thousand lengths. Counting bytes, not seconds, holds
		// on a runner of any speed.
		const n = 10_000
		line := strings.Repeat("a", 1<<20) + strings.Repeat(" `REQ-0001` a", n)
		read := 0
		prev := countTicks
		t.Cleanup(func() { countTicks = prev })
		countTicks = func(s string) int {
			read += len(s)
			return strings.Count(s, "`")
		}
		marks := reqMarks(reqPage{path: "docs/spec/01-model.md", lines: []string{line}})
		require.Len(t, marks, n)
		assert.Empty(t, marks[n-1].misplaced)
		assert.LessOrEqual(t, read, 2*len(line), "the backticks are counted in one pass")
	})
	t.Run("a front matter outside the task directory or not in a .md file, and a page that is not .md, are not read", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/T009-a.md", reqTaskFile("T009", "kept", "REQ-0099 (none)"))
		r.Write("docs/plan/tasks/sub/T010-a.md", reqTaskFile("T010", "kept", "REQ-0099 (none)"))
		r.Write("docs/plan/tasks/T011-a.txt", reqTaskFile("T011", "kept", "REQ-0099 (none)"))
		r.Write("docs/spec/05-x.mdx", "# 05\n\n`REQ-0001` Again; `REQ-new`.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a chapter-named file outside docs/spec is no page", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/01-model.md", "# 01\n\n`REQ-0001` Again; `REQ-new`.\n")
		r.Write("docs/spec/sub/01-model.md", "# 01\n\n`REQ-0001` Again; `REQ-new`.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a page that is a submodule entry is an error", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.Mkdir(filepath.Join(r.Dir, "docs", "spec", "01-model.md"), 0o755))
		r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,docs/spec/01-model.md")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: read docs/spec/01-model.md: not a file")
	})
	t.Run("a page that is a submodule entry is an error, its path quoted", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, "docs", "spec", "01-a\tb.md"), 0o755))
		r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,docs/spec/01-a\tb.md")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, `ci: read "docs/spec/01-a\tb.md": not a file`)
	})
	t.Run("a task file that is a submodule entry is an error, its path quoted", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, "docs", "plan", "tasks", "T001\ta.md"), 0o755))
		r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,docs/plan/tasks/T001\ta.md")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, `ci: read "docs/plan/tasks/T001\ta.md": not a file`)
	})
	t.Run("a decision record that is a submodule entry is skipped", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, "docs", "adr", "0002-sub.md"), 0o755))
		r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,docs/adr/0002-sub.md")
		r.Commit("fixture")
		require.Contains(t, r.Git("ls-tree", "-r", "HEAD", "docs/adr"), "160000 commit", "the record is a submodule entry")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
}

func TestReqsCoverage(t *testing.T) {
	t.Run("the coverage half is silent without task files", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan.md", "# Plan\n\nNo task file yet.\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a deferred-only id is reported and passes", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
		r.Write("docs/plan/tasks/T002-b.md", reqTaskFile("T002", "deferred", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\nreqs: 1 id(s) cited only by deferred tasks:\n  REQ-0003\n", out)
	})
	t.Run("the deferred-only ids are listed eight to a line", func(t *testing.T) {
		ids := make([]string, 9)
		for i := range ids {
			ids[i] = reqPrefix + reqBody(i+1)
		}
		assert.Equal(t, "reqs: 9 id(s) cited only by deferred tasks:\n  REQ-0001 REQ-0002 REQ-0003 REQ-0004 REQ-0005 REQ-0006 REQ-0007 REQ-0008\n  REQ-0009\n", reqDeferredLines(ids))
	})
	t.Run("an Implements line that wraps is read whole", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", "---\nid: T001\nfate: new\n---\n\n- Implements: `REQ-0001` (index), REQ-0002\n  (index),\n  REQ-0003 (00\n  0.1)\n- Verify: go test\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out, "a reading aid that wraps is read whole")
	})
	t.Run("a wrapped entry's finding names its line", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", "---\nid: T001\nfate: new\n---\n\n- Implements: `REQ-0001` (index), REQ-0002\n  (index),\n  REQ-0003 (00 0.1),\n  REQ-0099 (00 0.1)\n- Verify: go test\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "docs/plan/tasks/T001-a.md:9: reqs: Implements cites REQ-0099, which is unknown")
	})
	t.Run("an untracked task file does not satisfy the coverage half", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
		r.Commit("fixture")
		r.Write("docs/plan/tasks/T002-b.md", reqTaskFile("T002", "new", "REQ-0003 (00 0.1)"))
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "REQ-0003 is cited by no task")
	})
	t.Run("a task file without a fate is left to the plan check", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
		r.Write("docs/plan/tasks/T002-b.md", "# T002\n\n- Implements: REQ-0099 (none)\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("an Implements entry without a reading aid is accepted", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001", "`REQ-0002`", "REQ-0003"))
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("an id cited by exactly the cap passes, and a deferred citer does not count", func(t *testing.T) {
		r := reqFixture(t)
		for i, fate := range []string{"kept", "new", "merged", "kept", "kept", "deferred"} {
			id := "T00" + string(rune('1'+i))
			r.Write("docs/plan/tasks/"+id+"-a.md", reqTaskFile(id, fate, "REQ-0001 (index)", "REQ-0002 (index)", "REQ-0003 (00 0.1)"))
		}
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a task file with CR LF line ends is read", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
		r.Write("docs/plan/tasks/T002-b.md", strings.ReplaceAll(reqTaskFile("T002", "new", "REQ-0003 (00 0.1)"), "\n", "\r\n"))
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
	})
	t.Run("a shared id passes the cap", func(t *testing.T) {
		live := map[string]reqMark{
			"0001": {page: "docs/spec.md", line: 3, section: "index"},
			"1169": {page: "docs/spec/12-engineering.md", line: 5, section: "12 12.5"},
		}
		var tasks []reqTask
		for i := range reqCiteCap + 1 {
			tasks = append(tasks, reqTask{path: "docs/plan/tasks/T00" + string(rune('1'+i)) + "-a.md", fate: "kept", cites: []reqMark{
				{body: "0001", section: "index"},
				{body: "1169", section: "12 12.5"},
			}})
		}
		findings, deferred := reqCoverage(live, nil, tasks)
		require.Len(t, findings, 1)
		assert.Equal(t, "docs/spec.md:3: reqs: REQ-0001 is cited by 6 tasks, more than the 5 of 12 12.11; one obligation has few implementers", findings[0].String())
		assert.Empty(t, deferred)
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
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "docs/spec.md:3: REQ-0005\ndocs/spec.md:3: REQ-0006\ndocs/spec/00-scope.md:3: REQ-0007\ndocs/spec/01-model.md:3: REQ-0008\nreqs: 4 placeholder(s) numbered\n", out)
		assert.Equal(t, "# Index\n\n- `REQ-0005` Five, then `REQ-0006` six.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n", read(t, r, "docs/spec.md"))
		assert.Equal(t, "# 00\n\n`REQ-0003` Three; `REQ-0007` seven.\n", read(t, r, "docs/spec/00-scope.md"))
		assert.Equal(t, "# 01\n\n`REQ-0008` Eight.\n", read(t, r, "docs/spec/01-model.md"))
		assert.Equal(t, "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-0004` | 2026-10-12 | Four. | none |"), read(t, r, "docs/spec/12-engineering.md"), "a page without a placeholder is not rewritten")
		r.Commit("allocated")
		code, out = citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
	})
	t.Run("allocate is idempotent", func(t *testing.T) {
		r := reqFixture(t)
		before := read(t, r, "docs/spec.md")
		info, err := os.Stat(filepath.Join(r.Dir, "docs", "spec.md"))
		require.NoError(t, err)
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "reqs: ok\n", out)
		assert.Equal(t, before, read(t, r, "docs/spec.md"))
		after, err := os.Stat(filepath.Join(r.Dir, "docs", "spec.md"))
		require.NoError(t, err)
		// A write renames a new file over the page, so a page that was
		// written is another file; a time stamp could tie.
		assert.True(t, os.SameFile(info, after), "no byte changed, so no write")
	})
	t.Run("allocate may take REQ-9999 itself", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-9998` | 2026-10-12 | Next to last. | none |"))
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` The last.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "docs/spec/01-model.md:3: REQ-9999\nreqs: 1 placeholder(s) numbered\n", out)
		assert.Equal(t, "# 01\n\n`REQ-9999` The last.\n", read(t, r, "docs/spec/01-model.md"))
	})
	t.Run("a page with CR LF line ends keeps them", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-model.md", "# 01\r\n\r\n`REQ-new` Five.\r\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, "# 01\r\n\r\n`REQ-0005` Five.\r\n", read(t, r, "docs/spec/01-model.md"))
	})
	t.Run("a placeholder in an excluded section refuses, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec.md", "# Index\n\n- `REQ-new` Five.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n\n## Open questions\n\n`REQ-new` A question.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: docs/spec.md:9: reqs: a placeholder sits in a page or section that carries no id (12 12.11)\nnothing was written")
		assert.Contains(t, read(t, r, "docs/spec.md"), "- `REQ-new` Five.")
	})
	t.Run("a misplaced placeholder refuses, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec.md", "# Index\n\n- `REQ-new` Five.\n- `REQ-0001` One.\n- `REQ-0002` Two.\n")
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` 1. Six.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: docs/spec/01-model.md:3: reqs: REQ-new precedes the marker of a list item (12 12.11)\nnothing was written")
		assert.Contains(t, read(t, r, "docs/spec.md"), "- `REQ-new` Five.", "the index, numbered before the refusal, is not written")
	})
	t.Run("allocate refuses when the retired table does not parse, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-0004` | 2026-11-3 | Four. | none |"))
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Five.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: docs/spec/12-engineering.md:9: reqs: a retired row's second cell is not an ISO date\nnothing was written")
		assert.Contains(t, read(t, r, "docs/spec/01-model.md"), "`REQ-new`")
	})
	t.Run("a number past the last refuses, and nothing is written", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-9999` | 2026-10-12 | Last. | none |"))
		r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Too many.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
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
			code, out := citest.Run(t, r, nil, Run, "allocate")
			require.Equal(t, check.ExitOK, code, out)
			return read(t, r, page)
		}
		a := branch("docs/spec/01-model.md", "# 01\n\n`REQ-new` From branch a.\n")
		b := branch("docs/spec/02-layouts.md", "# 02\n\n`REQ-new` From branch b.\n")
		base.Write("docs/spec/01-model.md", a)
		base.Write("docs/spec/02-layouts.md", b)
		base.Commit("merged")
		code, out := citest.Run(t, base, nil, Run)
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "docs/spec/02-layouts.md:3: reqs: REQ-0005 appears twice; its first place is docs/spec/01-model.md:3")
	})
	t.Run("a page that is not a regular file refuses", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, "docs", "spec.md")))
		require.NoError(t, os.Symlink("spec", filepath.Join(r.Dir, "docs", "spec.md")))
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: read docs/spec.md: not a regular file")
	})
	t.Run("a page that is not a regular file refuses, its path quoted", func(t *testing.T) {
		r := reqFixture(t)
		require.NoError(t, os.Mkdir(filepath.Join(r.Dir, "docs", "spec", "01-a\tb.md"), 0o755))
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, `ci: read "docs/spec/01-a\tb.md": not a regular file`)
	})
	t.Run("a number past the last refuses, its path quoted", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/12-engineering.md", "# 12\n\n## 12.11 Requirement ids\n\n"+reqTable("| `REQ-9999` | 2026-10-12 | Last. | none |"))
		r.Write("docs/spec/01-a\tb.md", "# 01\n\n`REQ-new` Too many.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, `ci: "docs/spec/01-a\tb.md:3": the next id would pass REQ-9999; nothing was written`)
	})
	t.Run("a numbered page is named with its control characters escaped", func(t *testing.T) {
		r := reqFixture(t)
		r.Write("docs/spec/01-a\x1bb.md", "# 01\n\n`REQ-new` Five.\n")
		code, out := citest.Run(t, r, nil, Run, "allocate")
		assert.Equal(t, check.ExitOK, code, out)
		assert.Equal(t, `docs/spec/01-a\x1bb.md:3: REQ-0005`+"\nreqs: 1 placeholder(s) numbered\n", out)
		assert.Equal(t, "# 01\n\n`REQ-0005` Five.\n", read(t, r, "docs/spec/01-a\x1bb.md"))
	})
}

func TestReqsUsage(t *testing.T) {
	r := reqFixture(t)
	r.Commit("fixture")
	for _, args := range [][]string{{"reqs", "x"}, {"reqs", "allocate", "x"}} {
		code, out := citest.Run(t, r, nil, Run, args[1:]...)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "usage:")
	}
}

// TestReqsInTheCommitChecks: fast and all judge reqs with hygiene and
// workflows, from the one read of the commit's tree, and carry the
// count of the ids that rest on deferred tasks alone.
func TestReqsInTheCommitChecks(t *testing.T) {
	r := reqFixture(t)
	r.Write("docs/spec/01-model.md", "# 01\n\n`REQ-new` Five.\n")
	r.Write("docs/plan/tasks/T001-a.md", reqTaskFile("T001", "kept", "REQ-0001 (index)", "REQ-0002 (index)"))
	r.Write("docs/plan/tasks/T002-b.md", reqTaskFile("T002", "deferred", "REQ-0003 (00 0.1)"))
	head := r.Commit("fixture")
	files, err := check.HeadTree(t.Context(), r.Repo, head)
	require.NoError(t, err)
	verdicts, err := Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: files})
	require.NoError(t, err)
	require.Len(t, verdicts, 1)
	assert.Equal(t, "reqs", verdicts[0].Name)
	require.Len(t, verdicts[0].Findings, 1)
	assert.Equal(t, "docs/spec/01-model.md:3: reqs: a placeholder is left; run \"go run ./tools/ci reqs allocate\"", verdicts[0].Findings[0].String())
	assert.Equal(t, "      1 id(s) cited only by deferred tasks; go run ./tools/ci reqs lists them\n", verdicts[0].Info)
}
