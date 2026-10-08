// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// runAllIn runs "all" in the fixture with stub steps and the cover
// profile at profile.
func runAllIn(t *testing.T, r *gittest.Repo, steps []step, profile string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	e := env{dir: r.Dir, gitEnv: r.Env, stdin: strings.NewReader(""), stdout: &out, stderr: &out, steps: steps, profile: profile}
	return run(t.Context(), e, append([]string{"all"}, args...)), out.String()
}

// TestAll covers all with stub steps: every check runs and reports,
// whatever an earlier one found, and the checks in this process read
// the tree at HEAD and the cover profile.
func TestAll(t *testing.T) {
	tests := []struct {
		name    string
		steps   []step
		change  func(t *testing.T, r *gittest.Repo)
		profile string // the profile's blocks for profileOf; "missing" for none
		code    int
		want    []string
	}{
		{
			name:    "every check passing",
			steps:   []step{passing},
			profile: "tools/ci 1 1",
			code:    exitOK,
			want:    []string{"platform  ", "ok    passes", "ok    hygiene", "ok    workflows", "ok    coverage"},
		},
		{
			name:    "a failing step does not hide the checks after it",
			steps:   []step{failing, passing},
			profile: "tools/ci 1 1",
			code:    exitFail,
			want:    []string{"FAIL  fails", "ok    passes", "ok    hygiene", "ok    workflows", "ok    coverage"},
		},
		{
			name:    "a package below its threshold",
			steps:   []step{passing},
			profile: "tools/ci 1 0",
			code:    exitFail,
			want:    []string{"ok    workflows", "FAIL  coverage\ntools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\n"},
		},
		{
			name:    "no cover profile, as when the race step could not start",
			steps:   []step{passing},
			profile: "missing",
			code:    exitFail,
			want:    []string{"FAIL  coverage\nthe cover profile of the race step: "},
		},
		{
			name:  "a workflow outside the grammar",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".github/workflows/ci.yml", strings.Replace(goodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1))
			},
			profile: "tools/ci 1 1",
			code:    exitFail,
			want:    []string{"FAIL  workflows\n.github/workflows/ci.yml:17: workflows: a runner label is pinned, never *-latest\n", "ok    coverage"},
		},
		{
			name:    "a hygiene finding",
			steps:   []step{passing},
			change:  func(t *testing.T, r *gittest.Repo) { r.Write("docs/x.md", "a line with "+emDash+"\n") },
			profile: "tools/ci 1 1",
			code:    exitFail,
			want:    []string{"FAIL  hygiene", "em-dash", "ok    workflows"},
		},
		{
			name:  "a step that needs a module the cache lacks names the setup",
			steps: []step{{name: "unit", argv: []string{"sh", "-c", "echo 'x.go:1:2: module lookup disabled by GOPROXY=off'; exit 1"}, environ: []string{"PATH=" + os.Getenv("PATH")}}},
			code:  exitFail,
			want:  []string{"FAIL  unit\nx.go:1:2: module lookup disabled by GOPROXY=off\n", `run "go run ./tools/ci setup", which runs "go mod download"`},
		},
		{
			name:  "a nested module stops all before any check",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/inner/go.mod", "module example.invalid/inner\n")
			},
			code: exitError,
			want: []string{"ci: the checks read one module"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
			if tt.change != nil {
				tt.change(t, r)
			}
			r.Commit("fixture")
			profile := filepath.Join(t.TempDir(), "cover.out")
			if tt.profile != "missing" && tt.profile != "" {
				require.NoError(t, os.WriteFile(profile, profileOf(tt.profile), 0o600))
			}
			code, out := runAllIn(t, r, tt.steps, profile)
			assert.Equal(t, tt.code, code, "exit status\n%s", out)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
		})
	}
}

// TestAllUsage refuses an argument.
func TestAllUsage(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	for _, args := range [][]string{{"all", "x"}, {"setup", "x"}, {"workflows", "x"}} {
		code, out := runCI(t, r, nil, nil, args...)
		assert.Equal(t, exitError, code, "%v\n%s", args, out)
	}
}

// TestWorkflowsCommand runs "tools/ci workflows" on a fixture
// repository: exit 0 with a workflow inside the grammar, 1 outside it.
func TestWorkflowsCommand(t *testing.T) {
	r := newTree(t)
	r.Write(".github/workflows/ci.yml", goodWorkflow)
	r.Commit("fixture")
	code, out := runCI(t, r, nil, nil, "workflows")
	assert.Equal(t, exitOK, code, out)
	assert.Equal(t, "workflows: ok\n", out)

	r.Write(".github/workflows/extra.yaml", goodWorkflow)
	r.Commit("a second ending")
	code, out = runCI(t, r, nil, nil, "workflows")
	assert.Equal(t, exitFail, code, out)
	assert.Equal(t, ".github/workflows/extra.yaml: workflows: a file here ends in .yml\nworkflows: 1 finding(s)\n", out)
}

// TestAllSteps pins the command steps of all: the steps of fast, then
// vulnerabilities, race and license, each in stepEnv. govulncheck and
// reuse run through "mise exec" once mise has found each below the
// directory where it installs it; a tool it does not find there fails
// its own step, and starts nothing.
func TestAllSteps(t *testing.T) {
	f := newFakeMise(t)
	goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	vuln := f.tool(t, execTools["govulncheck"], "1.8.0", "bin", "govulncheck")
	script := "#!/bin/sh\ncase \"$4\" in\ngovulncheck) echo '" + vuln + "' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))

	root := t.TempDir()
	tools := lintTools{linter: linter, goDir: filepath.Dir(goCmd)}
	env := stepEnv(tools.goDir)
	steps := allSteps(t.Context(), root, tools, "/tmp/p/cover.out")
	fast := fastSteps(root, tools)
	require.Len(t, steps, len(fast)+3)
	assert.Equal(t, fast, steps[:len(fast)])

	vulnStep, race, license := steps[len(fast)], steps[len(fast)+1], steps[len(fast)+2]
	assert.Equal(t, step{name: "vulnerabilities", argv: []string{"mise", "exec", "--", "govulncheck", "./..."}, environ: env}, vulnStep)
	assert.Equal(t, step{name: "race", argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=/tmp/p/cover.out", "./..."}, environ: env}, race)
	assert.Equal(t, licenseStep(root, runtime.GOOS, env), license)

	t.Run("a tool outside the installs directory fails its own step", func(t *testing.T) {
		elsewhere := executable(t, filepath.Join(t.TempDir(), "govulncheck"))
		require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\necho '"+elsewhere+"'\n"), 0o700))
		vulnStep := allSteps(t.Context(), root, tools, "/tmp/p/cover.out")[len(fast)]
		require.ErrorContains(t, vulnStep.unavailable, "is outside "+filepath.Join(evalOrSelf(f.installs()), execTools["govulncheck"])+", where mise installs it")
		_, err := vulnStep.run(t.Context(), root)
		require.ErrorIs(t, err, vulnStep.unavailable, "an unavailable tool fails its step")
	})
}

// TestLicenseStep pins the license step: reuse from the image pinned
// by digest, on linux, with the tree mounted read-only and no network;
// on another platform a step that is not run, which all reports with a
// "--" line, and neither "ok" nor "FAIL".
func TestLicenseStep(t *testing.T) {
	env := []string{"PATH=/x"}
	assert.Equal(t, step{
		name: "license",
		argv: []string{"docker", "run", "--rm", "--network", "none",
			"--mount", "type=bind,source=/src/repo,target=/data,readonly",
			"fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e", "lint"},
		environ: env,
	}, licenseStep("/src/repo", "linux", env))
	assert.Regexp(t, `@sha256:[0-9a-f]{64}$`, reuseImage)

	darwin := licenseStep("/src/repo", "darwin", env)
	assert.Equal(t, "not run on darwin: reuse runs from a Linux container image, and the Linux runners run it on the same commit", darwin.skip)
	assert.Empty(t, darwin.argv, "nothing is started")

	require.Error(t, licenseStep("/src/a,b", "linux", env).unavailable, "a path the mount option cannot name")

	var out bytes.Buffer
	c := newChecks(&out)
	c.run(t.Context(), t.TempDir(), []step{darwin, passing})
	assert.True(t, c.ok, "a step not run fails nothing")
	assert.Equal(t, "--    license: "+darwin.skip+"\n", strings.Split(out.String(), "ok    passes")[0])
}

// evalOrSelf resolves the symbolic links of a path that exists.
func evalOrSelf(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// TestSetup runs setup against a fake mise and a fake go: it trusts
// mise.toml by its path, installs, and downloads the modules with the
// pinned go command, the one step whose environment keeps the go
// command's default proxy.
func TestSetup(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	f := newFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
	log := filepath.Join(t.TempDir(), "log")
	goCmd := filepath.Join(f.installs(), "go", "1.27.0", "bin", "go")
	require.NoError(t, os.MkdirAll(filepath.Dir(goCmd), 0o755))
	require.NoError(t, os.WriteFile(goCmd, []byte("#!/bin/sh\necho \"go $* GOPROXY=${GOPROXY-unset} GOFLAGS=$GOFLAGS\" >> '"+log+"'\n"), 0o700))
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	script := "#!/bin/sh\nif [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\necho \"mise $*\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))

	code, out := runCI(t, r, nil, nil, "setup")
	assert.Equal(t, exitOK, code, out)
	for _, w := range []string{"ok    mise trust", "ok    mise install", "ok    go mod download"} {
		assert.Contains(t, out, w)
	}
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	root, err := filepath.EvalSymlinks(r.Dir)
	require.NoError(t, err)
	assert.Equal(t, "mise trust "+filepath.Join(root, "mise.toml")+"\nmise install\ngo mod download GOPROXY=unset GOFLAGS=-mod=readonly\n", string(data))

	t.Run("a failing mise stops before the download", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\nexit 1\n"), 0o700))
		code, out := runCI(t, r, nil, nil, "setup")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "FAIL  mise trust")
		assert.NotContains(t, out, "go mod download")
	})
}
