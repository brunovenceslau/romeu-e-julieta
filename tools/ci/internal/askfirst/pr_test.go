// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package askfirst

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
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

// The tests of approvals.go, grants.go and pr.go live here, each named
// TestPR..., so the Verify line of T180, -run '^TestPR', runs them all.

// prList is the ask-first list of the fixtures: the list itself, the
// decisions and kits surfaces, a grantable one, and one whose glob tests
// that "**" matches zero segments.
const prList = `version: 1
owner: "@someone"
surfaces:
  - id: ask-first
    globs: [.github/ask-first.yaml, docs/grants.yaml]
    reason: this list
  - id: decisions
    globs: [docs/adr/**]
    reason: records
  - id: kits
    globs: [kits/**]
    reason: kits
  - id: checks
    globs: [tools/ci/*.go]
    reason: checks
  - id: zero
    globs: [z/**/f.txt]
    reason: zero segments
`

// prListWithoutDecisions is prList with the decisions surface removed,
// as a head that drops it holds.
var prListWithoutDecisions = strings.Replace(prList, "  - id: decisions\n    globs: [docs/adr/**]\n    reason: records\n", "", 1)

// prGrants is the grant file of the fixtures.
const prGrants = `version: 1
grants:
  - sitting: M0
    surface: checks
    tasks: [T013]
`

func mustList(t *testing.T, src string) *List {
	t.Helper()
	l, err := Parse([]byte(src))
	require.NoError(t, err)
	return l
}

// alert wraps lines in the approval alert.
func alert(lines ...string) string {
	return alertLine + "\n" + strings.Join(lines, "\n") + "\n"
}

const validLine = "> - [x] Approval: decisions - amend rule 8"

// TestPRApprovalLine holds the rules of 12 12.4 on whole bodies, for a
// diff that touches decisions. Its cases are the rows of the table of
// T005's acceptance, each line written as a ticked checklist line of the
// alert, then the rows T180 adds.
func TestPRApprovalLine(t *testing.T) {
	const (
		passes     = ""
		noLine     = `the diff touches "decisions" and the alert has no approval line for it`
		malformed  = "malformed approval line"
		duplicate  = `two approval lines name "decisions"`
		unknown    = "which neither ask-first list has"
		notTicked  = `the diff touches "decisions" and its approval line is not ticked`
		twoAlerts  = "the body opens 2 approval alerts"
		lineFor    = "> - [x] Approval: decisions - "
		phrase     = "amend rule 8"
		withoutDec = "without decisions"
	)
	cases := []struct {
		name, body, head, want string
	}{
		{"a valid line", alert(validLine), "", passes},
		{"a lowercase prefix", alert("> - [x] approval: decisions - " + phrase), "", noLine},
		{"an uppercase id", alert("> - [x] Approval: Decisions - " + phrase), "", malformed},
		{"an en dash separator", alert("> - [x] Approval: decisions \u2013 " + phrase), "", malformed},
		{"an empty phrase", alert(lineFor), "", malformed},
		{"a straight quote around the phrase", alert(lineFor + `"` + phrase + `"`), "", malformed},
		{"a straight quote inside the phrase", alert(lineFor + `amend" rule 8`), "", malformed},
		{"a left curly quote alone", alert(lineFor + "\u201c" + phrase), "", malformed},
		{"a right curly quote alone", alert(lineFor + phrase + "\u201d"), "", malformed},
		{"a look-alike quote", alert(lineFor + "\u00ab" + phrase + "\u00bb"), "", malformed},
		{"an apostrophe", alert(lineFor + "the record\u2019s rule 8"), "", passes},
		{"a leading no-break space", alert(lineFor + "\u00a0" + phrase), "", malformed},
		{"a zero-width space", alert(lineFor + "\u200b" + phrase), "", malformed},
		{"a byte order mark", alert(lineFor + "\ufeff" + phrase), "", malformed},
		{"a leading combining mark", alert(lineFor + "\u0301" + phrase), "", malformed},
		{"CR LF line ends", alertLine + "\r\n" + validLine + "\r\n", "", passes},
		{"a line inside an HTML comment", "<!--\n" + alert(validLine) + "-->\n", "", noLine},
		{"an unclosed comment", "<!--\n" + alert(validLine), "", noLine},
		{"a duplicate id", alert(validLine, validLine), "", duplicate},
		{"an untouched known surface", alert(validLine, "> - [ ] Approval: kits - later"), "", passes},
		{"a refused line next to a valid one", alert(validLine, "> - [x] Approval: kits - \"later\""), "", malformed},
		{"an unknown id", alert(validLine, "> - [x] Approval: nosuch - x"), "", unknown},
		{"an id with a double hyphen", alert("> - [x] Approval: a--b - x", validLine), "", unknown},
		{"a surface removed in the head", alert(validLine), withoutDec, passes},
		{"a surface removed in the head, no line", alert(), withoutDec, noLine},
		// The rows T180 adds for the checklist.
		{"an unticked line", alert("> - [ ] Approval: decisions - " + phrase), "", notTicked},
		{"an uppercase X", alert("> - [X] Approval: decisions - " + phrase), "", passes},
		{"a ticked line outside the alert", "Some text.\n\n" + validLine + "\n", "", noLine},
		{"a line after the alert ends", alert() + "\n" + validLine + "\n", "", noLine},
		{"two alerts", alert(validLine) + "\n" + alert(), "", twoAlerts},
		{"text before the prefix", alert("> - [x] see Approval: decisions - " + phrase), "", malformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			head := mustList(t, prList)
			if c.head == withoutDec {
				head = mustList(t, prListWithoutDecisions)
			}
			r := review{body: c.body, base: mustList(t, prList), head: head, changed: []string{"docs/adr/0001-x.md"}}
			got := r.findings()
			if c.want == passes {
				assert.Empty(t, got)
				return
			}
			// A malformed line also leaves its surface without a line, so
			// the outcome is the finding of the row, and maybe that one.
			require.NotEmpty(t, got)
			assert.Contains(t, got[0], c.want, "%q", got)
			for _, msg := range got[1:] {
				assert.Equal(t, noLine, msg, "%q", got)
			}
		})
	}
}

// TestPRGrantLine holds the grant phrase of REQ-1161 against the grants
// of the base: a grant that the file holds approves a line left
// unticked (decided on 2026-10-29), and a phrase names a grant that holds
// or fails, ticked or not.
func TestPRGrantLine(t *testing.T) {
	grants, err := ParseGrants([]byte(prGrants), []string{"T013"})
	require.NoError(t, err)
	cases := []struct {
		name, line, changed, want string
	}{
		{"a grant the file holds", "> - [ ] Approval: checks - granted at M0 for T013", "tools/ci/x.go", ""},
		{"a grant that does not hold on an untouched surface", "> - [ ] Approval: checks - granted at M1 for T013", "README.md", ""},
		{"a grant the file holds, ticked", "> - [x] Approval: checks - granted at M0 for T013", "tools/ci/x.go", ""},
		{"a grant the file lacks", "> - [ ] Approval: checks - granted at M1 for T013", "tools/ci/x.go", "which docs/grants.yaml at the base does not hold"},
		{"a grant the file lacks, ticked", "> - [x] Approval: checks - granted at M0 for T999", "tools/ci/x.go", "which docs/grants.yaml at the base does not hold"},
		{"a grant on decisions", "> - [x] Approval: decisions - granted at M0 for T013", "docs/adr/x.md", "a sitting grants none on that surface"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := review{body: alert(c.line), base: mustList(t, prList), changed: []string{c.changed}, grants: grants}
			got := r.findings()
			if c.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%q", got)
			assert.Contains(t, got[0], c.want)
		})
	}
}

// TestPRParseGrants checks each rule of docs/grants.yaml with one
// source that breaks it.
func TestPRParseGrants(t *testing.T) {
	tasks := []string{"T013", "T021"}
	cases := []struct {
		name, src, want string
	}{
		{"valid", prGrants, ""},
		{"no grant", "version: 1\ngrants: []\n", ""},
		{"not YAML", "version: [", "grants.yaml"},
		{"an unknown key", prGrants + "extra: 1\n", "field extra not found"},
		{"a second document", prGrants + "---\nversion: 1\n", "more than one YAML document"},
		{"version 2", strings.Replace(prGrants, "version: 1", "version: 2", 1), "version is 2"},
		{"a surface outside the grantable list", strings.Replace(prGrants, "surface: checks", "surface: decisions", 1), `no approval line on "decisions"`},
		{"a task the plan does not define", strings.Replace(prGrants, "[T013]", "[T013, T999]", 1), `names "T999", which the plan does not define`},
		{"no task", strings.Replace(prGrants, "[T013]", "[]", 1), "names no task"},
		{"a sitting of no plan", strings.Replace(prGrants, "sitting: M0", "sitting: M9", 1), `"M9" is not a sitting`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseGrants([]byte(c.src), tasks)
			if c.want == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// prRepo is a fixture repository whose main holds the list, the grants
// and two task files.
func prRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.New(t)
	r.Write(Path, prList)
	r.Write(GrantsPath, prGrants)
	r.Write("docs/plan/tasks/T013-a-task.md", "x\n")
	r.Write("docs/plan/tasks/T021-another.md", "x\n")
	r.Write("docs/adr/0001-old.md", "old\n")
	r.Write("README.md", "readme\n")
	return r, r.Commit("base")
}

// judge runs judgePR on the fixture and returns the messages.
func judge(t *testing.T, r *gittest.Repo, pr PR) []string {
	t.Helper()
	findings, err := judgePR(context.Background(), r.Repo, pr)
	require.NoError(t, err)
	var msgs []string
	for _, f := range findings {
		msgs = append(msgs, f.Msg)
	}
	return msgs
}

// TestPRReadsTheRangeFromGit holds what pr reads from git: the changed
// paths from the merge base, both names of a rename, "**" over zero
// segments, the walk that finds no commit, the head's list joined with
// the base's, and the grants of the base alone.
func TestPRReadsTheRangeFromGit(t *testing.T) {
	t.Run("a rename touches both names", func(t *testing.T) {
		r, base := prRepo(t)
		r.Git("checkout", "--quiet", "-b", "topic")
		r.Git("rm", "--quiet", "docs/adr/0001-old.md")
		r.Write("kits/moved.md", "old\n")
		head := r.Commit("move")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head})
		require.Len(t, got, 2, "%q", got)
		assert.Contains(t, got[0], `touches "decisions"`)
		assert.Contains(t, got[1], `touches "kits"`)
	})
	t.Run("a merge of main into the head touches nothing", func(t *testing.T) {
		r, _ := prRepo(t)
		r.Git("checkout", "--quiet", "-b", "topic")
		r.Write("notes.txt", "x\n")
		r.Commit("topic")
		r.Git("checkout", "--quiet", "main")
		r.Write("docs/adr/0002-new.md", "new\n")
		base := r.Commit("main moves")
		r.Git("checkout", "--quiet", "topic")
		r.Git("merge", "--quiet", "--no-edit", "main")
		head := r.Git("rev-parse", "HEAD")
		assert.Empty(t, judge(t, r, PR{BaseSHA: strings.TrimSpace(base), HeadSHA: strings.TrimSpace(head)}))
	})
	t.Run("** matches zero segments", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write("z/f.txt", "x\n")
		head := r.Commit("zero")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head})
		require.Len(t, got, 1)
		assert.Contains(t, got[0], `touches "zero"`)
	})
	t.Run("a walk that finds no commit", func(t *testing.T) {
		r, base := prRepo(t)
		assert.Equal(t, []string{"the walk from base.sha to head.sha finds no commit"}, judge(t, r, PR{BaseSHA: base, HeadSHA: base}))
	})
	t.Run("a surface the head removes keeps its line", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write(Path, prListWithoutDecisions)
		r.Write("docs/adr/0001-old.md", "changed\n")
		head := r.Commit("drop decisions")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert("> - [x] Approval: ask-first - drop decisions")})
		require.Len(t, got, 1)
		assert.Contains(t, got[0], `touches "decisions"`)
	})
	t.Run("a grant present only at the head fails", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write(GrantsPath, strings.Replace(prGrants, "[T013]", "[T013, T021]", 1))
		r.Write("tools/ci/x.go", "package main\n")
		head := r.Commit("grant at the head")
		body := alert("> - [x] Approval: ask-first - add a grant", "> - [ ] Approval: checks - granted at M0 for T021")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: body})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], "which docs/grants.yaml at the base does not hold")
	})
	t.Run("a grant present at the base passes", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write("tools/ci/x.go", "package main\n")
		head := r.Commit("checks")
		assert.Empty(t, judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert("> - [ ] Approval: checks - granted at M0 for T013")}))
	})
	t.Run("a gitlink under a glob touches its surface", func(t *testing.T) {
		r, base := prRepo(t)
		r.Git("update-index", "--add", "--cacheinfo", "160000,"+base+",kits/sub")
		r.Git("commit", "--quiet", "--message", "a gitlink")
		head := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], `touches "kits"`)
	})
	t.Run("a change main made after the fork is not the head's", func(t *testing.T) {
		r, _ := prRepo(t)
		r.Git("checkout", "--quiet", "-b", "topic")
		r.Write("notes.txt", "x\n")
		head := r.Commit("topic")
		r.Git("checkout", "--quiet", "main")
		r.Write("docs/adr/0002-new.md", "new\n")
		base := r.Commit("main moves")
		assert.Empty(t, judge(t, r, PR{BaseSHA: base, HeadSHA: head}))
	})
	t.Run("a surface the head adds needs its line", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write(Path, prList+"  - id: added\n    globs: [new/**]\n    reason: added\n")
		r.Write("new/x.txt", "x\n")
		head := r.Commit("add a surface")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert("> - [x] Approval: ask-first - add a surface")})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], `touches "added" and the alert has no approval line`)
		assert.Empty(t, judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert(
			"> - [x] Approval: ask-first - add a surface", "> - [x] Approval: added - its first file")}))
	})
	t.Run("a head that deletes the list is judged by the base's", func(t *testing.T) {
		r, base := prRepo(t)
		r.Git("rm", "--quiet", Path)
		head := r.Commit("delete the list")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], `touches "ask-first"`)
	})
	t.Run("a head whose grants do not parse fails", func(t *testing.T) {
		r, base := prRepo(t)
		r.Write(GrantsPath, strings.Replace(prGrants, "[T013]", "[T999]", 1))
		head := r.Commit("a grant of no task")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert("> - [x] Approval: ask-first - a grant")})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], `at the head: docs/grants.yaml: the grant of M0 on "checks" names "T999"`)
	})
	t.Run("a base whose grants do not parse holds none", func(t *testing.T) {
		r := gittest.New(t)
		r.Write(Path, prList)
		r.Write(GrantsPath, "version: 2\n")
		base := r.Commit("a broken base")
		r.Write(GrantsPath, prGrants)
		r.Write("docs/plan/tasks/T013-a-task.md", "x\n")
		r.Write("tools/ci/x.go", "package main\n")
		head := r.Commit("fix the grants")
		got := judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert(
			"> - [x] Approval: ask-first - fix the grants", "> - [ ] Approval: checks - granted at M0 for T013")})
		require.Len(t, got, 1, "%q", got)
		assert.Contains(t, got[0], "which docs/grants.yaml at the base does not hold")
		got = judge(t, r, PR{BaseSHA: base, HeadSHA: head, Body: alert(
			"> - [x] Approval: ask-first - fix the grants", "> - [x] Approval: checks - a check")})
		assert.Empty(t, got)
	})
}

// TestPRPayload holds the two shapes of REQ-0817 and each of the seven
// fields.
func TestPRPayload(t *testing.T) {
	h, b := strings.Repeat("a", 40), strings.Repeat("b", 64)
	obj := `{"title":"t","body":"b","head":{"ref":"topic","sha":"` + h + `"},"base":{"sha":"` + b + `","ref":"main","repo":{"default_branch":"main"}}}`
	want := PR{Title: "t", Body: "b", HeadRef: "topic", HeadSHA: h, BaseSHA: b, BaseRef: "main", DefaultBranch: "main"}
	t.Run("the pull request object", func(t *testing.T) {
		got, err := ParsePayload([]byte(obj))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("the event", func(t *testing.T) {
		got, err := ParsePayload([]byte(`{"action":"edited","pull_request":` + obj + `}`))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("a null body", func(t *testing.T) {
		got, err := ParsePayload([]byte(strings.Replace(obj, `"body":"b"`, `"body":null`, 1)))
		require.NoError(t, err)
		assert.Empty(t, got.Body)
	})
	for _, c := range []struct{ name, from, to, want string }{
		{"no title", `"title":"t",`, ``, "no title"},
		{"a null title", `"title":"t"`, `"title":null`, "title is not a string"},
		{"no body", `"body":"b",`, ``, "no body"},
		{"no head.ref", `"ref":"topic",`, ``, "no head.ref"},
		{"no head.sha", `,"sha":"` + h + `"`, ``, "no head.sha"},
		{"no base.sha", `"sha":"` + b + `",`, ``, "no base.sha"},
		{"a revision for head.sha", `"sha":"` + h + `"`, `"sha":"main"`, "head.sha is not a full commit id"},
		{"a short base.sha", `"sha":"` + b + `"`, `"sha":"` + b[:12] + `"`, "base.sha is not a full commit id"},
		{"no base.ref", `"ref":"main",`, ``, "no base.ref"},
		{"no base.repo.default_branch", `{"default_branch":"main"}`, `{}`, "no base.repo.default_branch"},
		{"a number for a string", `"title":"t"`, `"title":1`, "title is not a string"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(obj, c.from, c.to, 1)
			require.NotEqual(t, obj, src)
			_, err := ParsePayload([]byte(src))
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
	t.Run("neither shape", func(t *testing.T) {
		_, err := ParsePayload([]byte(`{"number":1}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "neither a pull request nor its event")
	})
	t.Run("not JSON", func(t *testing.T) {
		_, err := ParsePayload([]byte(`[`))
		assert.Error(t, err)
	})
}

// TestPRRun runs the subcommand as a person does, on a clone whose
// origin is unreachable, so a network read would fail it: a local run
// with --title and --body, and a run on a saved payload whose commits
// the clone holds.
func TestPRRun(t *testing.T) {
	r, base := prRepo(t)
	r.Git("remote", "add", "origin", filepath.Join(t.TempDir(), "gone"))
	r.Git("update-ref", "refs/remotes/origin/main", base)
	// Local main moves past origin/main, so a run that took main for the
	// base instead of origin/main would read a different range.
	r.Write("kits/k.txt", "k\n")
	r.Commit("local main moves")
	r.Git("checkout", "--quiet", "-b", "topic")
	r.Write("docs/adr/0002-new.md", "new\n")
	head := r.Commit("a record")
	dir := t.TempDir()
	bodyFile := filepath.Join(dir, "body.md")
	run := func(t *testing.T, body string, args ...string) (bool, string) {
		t.Helper()
		require.NoError(t, os.WriteFile(bodyFile, []byte(body), 0o600))
		var out bytes.Buffer
		ok, err := RunPR(context.Background(), check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &out}, args)
		require.NoError(t, err)
		return ok, out.String()
	}
	t.Run("a local run that passes", func(t *testing.T) {
		ok, out := run(t, alert(validLine), "--title", "docs: a record", "--body", bodyFile, "--base", "main")
		assert.True(t, ok, out)
		assert.Equal(t, "pr: ok\n", out)
	})
	t.Run("the default base is origin/main", func(t *testing.T) {
		ok, out := run(t, alert(validLine), "--title", "docs: a record", "--body", bodyFile)
		assert.False(t, ok)
		assert.Contains(t, out, `the diff touches "kits"`)
	})
	t.Run("a local run that fails", func(t *testing.T) {
		ok, out := run(t, alert("> - [ ] Approval: decisions - "+"amend rule 8"), "--title", "docs: a record", "--body", bodyFile, "--base", "main")
		assert.False(t, ok)
		assert.Contains(t, out, "pull request body: pr: the diff touches \"decisions\" and its approval line is not ticked")
		assert.Contains(t, out, "pr: 1 finding(s)")
	})
	t.Run("a saved payload", func(t *testing.T) {
		payload := filepath.Join(dir, "event.json")
		src := `{"pull_request":{"title":"t","body":null,"head":{"ref":"topic","sha":"` + head + `"},"base":{"sha":"` + base + `","ref":"main","repo":{"default_branch":"main"}}}}`
		require.NoError(t, os.WriteFile(payload, []byte(src), 0o600))
		ok, out := run(t, "", payload)
		assert.False(t, ok)
		assert.Contains(t, out, "has no approval line for it")
	})
	t.Run("a commit the clone lacks", func(t *testing.T) {
		var out bytes.Buffer
		_, err := RunPR(context.Background(), check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &out},
			[]string{"--title", "t", "--body", bodyFile, "--base", strings.Repeat("e", 40)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "names no commit of this clone")
	})
	for _, args := range [][]string{{}, {"a", "b"}, {"--title", "t"}, {"--title", "t", "--body", "b", "extra"}, {"--nosuch"}} {
		t.Run("usage "+strings.Join(args, " "), func(t *testing.T) {
			_, err := RunPR(context.Background(), check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &bytes.Buffer{}}, args)
			assert.Error(t, err)
		})
	}
}

// TestPRTemplateHasNoApprovalLine reads the pull request template as a
// body: a body that keeps the hint holds no approval line and no
// malformed one (REQ-1158).
func TestPRTemplateHasNoApprovalLine(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), ".github", "pull_request_template.md"))
	require.NoError(t, err)
	assert.Equal(t, alertBody{}, readAlert(string(b)))
}

// TestPROneReader holds invariant (e) of T005's acceptance, which T180
// takes: pr and the CODEOWNERS generator read .github/ask-first.yaml with
// the one reader, Parse, and pr reads docs/grants.yaml with ParseGrants
// (no generator reads the grants); no other function of the package
// decodes YAML. A reader inlined into
// either caller turns it red.
func TestPROneReader(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	calls := map[string]map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		require.NoError(t, err)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			called := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					switch fun := c.Fun.(type) {
					case *ast.Ident:
						called[fun.Name] = true
					case *ast.SelectorExpr:
						if x, ok := fun.X.(*ast.Ident); ok {
							called[x.Name+"."+fun.Sel.Name] = true
						}
					}
				}
				return true
			})
			calls[fn.Name.Name] = called
		}
	}
	assert.True(t, calls["CodeownersOutput"]["Parse"], "the generator reads the list with Parse")
	assert.True(t, calls["readList"]["Parse"], "pr reads the list with Parse")
	assert.True(t, calls["readGrants"]["ParseGrants"], "pr reads the grants with ParseGrants")
	for name, called := range calls {
		for c := range called {
			if strings.HasPrefix(c, "yaml.") && name != "Parse" && name != "ParseGrants" {
				t.Errorf("%s calls %s: only Parse and ParseGrants decode YAML", name, c)
			}
		}
	}
}
