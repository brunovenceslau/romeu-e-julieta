// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// headWorkflow is a workflow of the head with one job, whose name line
// is name (empty for none) and whose id is id.
func headWorkflow(id, name string) string {
	src := "name: x\non: pull_request\njobs:\n  " + id + ":\n"
	if name != "" {
		src += "    name: " + name + "\n"
	}
	return src + "    runs-on: ubuntu-26.04\n    steps:\n      - run: echo\n"
}

// TestPRHeadJobs holds REQ-1154 and REQ-1155 on single workflow files of
// the head: a job outside ask-first.yml whose check name equals the ask-first job's
// fails, one whose name holds "${{" fails, with a literal expression and
// with a matrix value, unless a literal prefix keeps it from the pr
// job's name, and an ask-first.yml of the head that names an event other than
// pull_request_target fails.
func TestPRHeadJobs(t *testing.T) {
	const other = ".github/workflows/x.yml"
	cases := []struct {
		name, path, src, want string
	}{
		{"another job name", other, headWorkflow("build", "build"), ""},
		{"the ask-first job's name", other, headWorkflow("build", "ask-first"), "could post the check ask-first"},
		{"the ask-first job's name in another case and with spaces", other, headWorkflow("build", `" ASK-FIRST "`), "could post the check ask-first"},
		{"the ask-first job's id and no name", other, headWorkflow("ask-first", ""), "could post the check ask-first"},
		{"the ask-first job's id with another name", other, headWorkflow("ask-first", "build"), ""},
		{"the ask-first job's id and a null name", other, headWorkflow("ask-first", "~"), "could post the check ask-first"},
		{"the ask-first job's id and an empty name", other, headWorkflow("ask-first", `""`), "could post the check ask-first"},
		{"the ask-first job's id and a name with no value", other, "name: x\non: push\njobs:\n  ask-first:\n    name:\n    runs-on: ubuntu-26.04\n", "could post the check ask-first"},
		{"a literal expression", other, headWorkflow("build", `"${{ 'ask-first' }}"`), "could post the check ask-first"},
		{"a matrix value", other, "name: x\non: push\njobs:\n  build:\n    name: ${{ matrix.n }}\n    strategy:\n      matrix:\n        n: [ask-first]\n    runs-on: ubuntu-26.04\n    steps:\n      - run: echo\n", "could post the check ask-first"},
		{"an expression after a prefix of the name", other, headWorkflow("build", `"ask-firs${{ matrix.r }}"`), "could post the check ask-first"},
		{"an expression after a prefix in another case", other, headWorkflow("build", `"ASK-FIRS${{ matrix.r }}"`), "could post the check ask-first"},
		{"an expression after spaces", other, headWorkflow("build", `" ${{ matrix.r }}"`), "could post the check ask-first"},
		{"an expression after a prefix that is not the name's", other, headWorkflow("build", `"static on ${{ matrix.os }}"`), ""},
		{"an expression after the whole name and more", other, headWorkflow("build", `"ask-first2 ${{ matrix.os }}"`), ""},
		{"a name that is not a scalar", other, headWorkflow("build", "[ask-first]"), "a name ask-first cannot read"},
		{"a file that is not YAML", other, "jobs: [\n", "cannot read"},
		{"two documents", other, headWorkflow("build", "build") + "---\nname: y\n", "cannot read"},
		{"an alias that names the job", other, "name: x\non: push\nx: &n pr\njobs:\n  build:\n    name: *n\n    runs-on: ubuntu-26.04\n", "cannot read"},
		{"a name written twice", other, "name: x\non: push\njobs:\n  build:\n    name: ask-first\n    name: build\n    runs-on: ubuntu-26.04\n", "cannot read"},
		{"a job written twice", other, "name: x\non: push\njobs:\n  build:\n    name: build\n  build:\n    name: ask-first\n", "cannot read"},
		{"jobs in another case", other, "name: x\non: push\nJobs:\n  ask-first:\n    runs-on: ubuntu-26.04\n", "could post the check ask-first"},
		{"a name in another case", other, "name: x\non: push\njobs:\n  build:\n    Name: ask-first\n", "could post the check ask-first"},
		{"a name written twice in two cases", other, "name: x\non: push\njobs:\n  build:\n    name: build\n    NAME: pr\n", "cannot read"},
		{"jobs that are not a mapping", other, "name: x\non: push\njobs: [ask-first]\n", "not a mapping"},
		{"the ask-first job's name in a file ending .yaml", ".github/workflows/x.yaml", headWorkflow("build", "ask-first"), "could post the check ask-first"},
		{"ask-first.yml on pull_request_target", PRWorkflow, "on:\n  pull_request_target:\n    branches: [main]\njobs:\n  ask-first:\n    name: ask-first\n", ""},
		{"ask-first.yml on pull_request_target as one name", PRWorkflow, "on: pull_request_target\njobs: {}\n", ""},
		{"ask-first.yml on pull_request", PRWorkflow, "on:\n  pull_request:\njobs:\n  ask-first:\n    name: ask-first\n", "names an event other than pull_request_target"},
		{"ask-first.yml on pull_request_target and push", PRWorkflow, "on: [pull_request_target, push]\njobs: {}\n", "names an event other than pull_request_target"},
		{"ask-first.yml on pull_request_target and push, as a mapping", PRWorkflow, "on:\n  pull_request_target:\n  push:\njobs: {}\n", "names an event other than pull_request_target"},
		{"ask-first.yml on push and pull_request_target, as a mapping", PRWorkflow, "on:\n  push:\n  pull_request_target:\njobs: {}\n", "names an event other than pull_request_target"},
		{"ask-first.yml with no event", PRWorkflow, "jobs: {}\n", "names an event other than pull_request_target"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := workflowFindings(c.path, []byte(c.src))
			if c.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, c.want)
			assert.Equal(t, c.path, got[0].Where)
		})
	}
}

// TestPRHeadJobsOfThisRepository reads the committed workflows as a head
// would hold them: none of them could post the check of the ask-first job.
func TestPRHeadJobsOfThisRepository(t *testing.T) {
	dir := filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(workflowsDir))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		assert.Empty(t, workflowFindings(workflowsDir+"/"+e.Name(), src), e.Name())
	}
}

// TestPRHeadFromGit runs the rules of the head through Judge on a
// fixture repository: a workflow file of the head is read as an object,
// one in a subdirectory or with another ending is not, and a go.work,
// go.work.sum or vendor at the top of the head fails, one in a
// subdirectory does not, and so does a go.work the base already held.
func TestPRHeadFromGit(t *testing.T) {
	cases := []struct {
		name  string
		base  map[string]string
		head  map[string]string
		wants []string
	}{
		{"a workflow that posts the ask-first check", nil, map[string]string{".github/workflows/x.yml": headWorkflow("ask-first", "")}, []string{"could post the check ask-first"}},
		{"a workflow in a subdirectory", nil, map[string]string{".github/workflows/old/x.yml": headWorkflow("ask-first", "")}, nil},
		{"a directory named as a workflow", nil, map[string]string{".github/workflows/x.yml/a.txt": "x\n"}, []string{"is not a file"}},
		{"a workflow that is a symbolic link", nil, map[string]string{"link:.github/workflows/x.yml": "../../README.md"}, []string{"cannot read"}},
		{"a workflow with another ending", nil, map[string]string{".github/workflows/x.txt": headWorkflow("ask-first", "")}, nil},
		{"a go.work", nil, map[string]string{"go.work": "go 1.24\n"}, []string{"the head holds go.work"}},
		{"a go.work.sum", nil, map[string]string{"go.work.sum": "\n"}, []string{"the head holds go.work.sum"}},
		{"a vendor directory", nil, map[string]string{"vendor/modules.txt": "\n"}, []string{"the head holds vendor"}},
		{"a vendor file", nil, map[string]string{"vendor": "\n"}, []string{"the head holds vendor"}},
		{"a go.work the base holds, edited", map[string]string{"go.work": "go 1.24\n"}, map[string]string{"go.work": "go 1.24\n\nuse .\n"}, []string{"the head holds go.work"}},
		{"a go.work in a subdirectory", nil, map[string]string{"sub/go.work": "go 1.24\n"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, _ := prRepo(t)
			for name, src := range c.base {
				r.Write(name, src)
			}
			base := r.Commit("base files")
			r.Git("checkout", "--quiet", "-b", "topic")
			for name, src := range c.head {
				if link, ok := strings.CutPrefix(name, "link:"); ok {
					full := filepath.Join(r.Dir, filepath.FromSlash(link))
					require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
					require.NoError(t, os.Symlink(src, full))
					continue
				}
				r.Write(name, src)
			}
			head := r.Commit("head files")
			findings, err := Judge(t.Context(), r.Repo, PR{BaseSHA: base, HeadSHA: head}, false)
			require.NoError(t, err)
			var got []string
			for _, f := range findings {
				if !strings.Contains(f.Msg, "approval line") {
					got = append(got, f.Msg)
				}
			}
			require.Len(t, got, len(c.wants), "%q", got)
			for i, w := range c.wants {
				assert.Contains(t, got[i], w)
			}
		})
	}
}
