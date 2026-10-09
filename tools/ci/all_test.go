// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
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
			want:    []string{"FAIL  workflows\n.github/workflows/ci.yml:25: workflows: runs-on is ${{ matrix.os }} or one of ubuntu-26.04, ubuntu-26.04-arm, macos-26, macos-26-intel, never *-latest\n", "ok    coverage"},
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

// TestAllSteps pins the command steps of all: the steps of fast, with
// vulnerabilities and license before unit, since neither runs code of
// the change, then race. Each runs in stepEnv; the one
// that reads the Go vulnerability database keeps the variables of a
// proxy too, and the license step those of the Docker daemon, and no
// other step keeps either.
func TestAllSteps(t *testing.T) {
	root := newTree(t).Dir
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.lock"), []byte(lockFixture), 0o600))
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	f := newFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
	goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	vuln := f.tool(t, govulncheckDir, "1.8.0", "bin", "govulncheck")
	script := "#!/bin/sh\ncase \"$4\" in\ngovulncheck) echo '" + vuln + "' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
	for _, key := range slices.Concat(networkPassThrough, dockerPassThrough) {
		t.Setenv(key, "from-the-process")
	}

	common, err := gitCommonDir(t.Context(), root)
	require.NoError(t, err)
	tools := lintTools{linter: linter, goDir: filepath.Dir(goCmd)}
	env := stepEnv(tools.goDir)
	steps := allSteps(t.Context(), root, tools, "/tmp/p/cover.out")
	fast := fastSteps(root, tools)
	unit := len(fast) - 1
	require.Equal(t, "unit", fast[unit].name)
	require.Len(t, steps, len(fast)+3)
	assert.Equal(t, fast[:unit], steps[:unit])
	assert.Equal(t, fast[unit], steps[unit+2])

	vulnStep, license, race := steps[unit], steps[unit+1], steps[unit+3]
	assert.Equal(t, step{name: "vulnerabilities", argv: []string{evalOrSelf(vuln), "./..."}, environ: withProcessEnv(env, networkPassThrough)}, vulnStep, "govulncheck runs by the path mise resolves, and not through mise exec")
	assert.Equal(t, 40*time.Minute, raceTimeout)
	assert.Less(t, raceTimeout, ciJobTimeout(t), "the race run ends before the job of ci.yml does")
	assert.Equal(t, step{name: "race", argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=/tmp/p/cover.out", "./..."}, environ: env, timeout: raceTimeout}, race)
	for _, s := range steps {
		assert.Equal(t, s.name == "race", s.timeout != 0, "%s has a budget of its own only if it is the race run", s.name)
	}
	assert.Equal(t, licenseStep(root, common, runtime.GOOS, withProcessEnv(env, dockerPassThrough)), license)

	keys := func(s step) []string {
		var out []string
		for _, kv := range s.environ {
			key, _, _ := strings.Cut(kv, "=")
			out = append(out, key)
		}
		return out
	}
	for _, s := range steps {
		for _, key := range networkPassThrough {
			assert.Equal(t, s.name == "vulnerabilities", slices.Contains(keys(s), key), "%s keeps %s", s.name, key)
		}
		for _, key := range dockerPassThrough {
			assert.Equal(t, s.name == "license" && runtime.GOOS == "linux", slices.Contains(keys(s), key), "%s keeps %s", s.name, key)
		}
		assert.NotContains(t, keys(s), "MISE_DATA_DIR")
		assert.NotContains(t, keys(s), "XDG_DATA_HOME")
	}
	for _, key := range slices.Concat(networkPassThrough, dockerPassThrough) {
		assert.NotContains(t, []string{"MISE_DATA_DIR", "XDG_DATA_HOME"}, key)
	}
}

// ciJobTimeout returns the timeout-minutes of the job of ci.yml.
func ciJobTimeout(t *testing.T) time.Duration {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(moduleRoot(t), filepath.FromSlash(workflowsDir), "ci.yml"))
	require.NoError(t, err)
	var wf struct {
		Jobs map[string]struct {
			Minutes int `yaml:"timeout-minutes"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(src, &wf))
	job, ok := wf.Jobs["all"]
	require.True(t, ok)
	require.Positive(t, job.Minutes)
	return time.Duration(job.Minutes) * time.Minute
}

// TestAllStepsVulnFailsClosed checks that govulncheck, which all starts
// by path, fails its own step and starts nothing when mise resolves it
// outside the directory where it installs the tool, at a version
// beside the locked one, or not at all. The last case is the one
// "mise exec" ran anyway, with a fake of the search path.
func TestAllStepsVulnFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		// resolve returns what the fake mise prints for govulncheck; ""
		// makes it fail, as for a tool that is not installed.
		resolve func(t *testing.T, f fakeMise) string
		want    string
	}{
		{
			name:    "outside the installs directory",
			resolve: func(t *testing.T, _ fakeMise) string { return executable(t, filepath.Join(t.TempDir(), "govulncheck")) },
			want:    "is outside " + filepath.Join("{installs}", govulncheckDir) + ", where mise installs it",
		},
		{
			name:    "at a version beside the locked one",
			resolve: func(t *testing.T, f fakeMise) string { return f.tool(t, govulncheckDir, "1.7.0", "bin", "govulncheck") },
			want:    "the install is not the version 1.8.0 that mise.lock locks",
		},
		{
			name:    "not installed, with a fake on the search path",
			resolve: func(t *testing.T, _ fakeMise) string { return "" },
			want:    "mise which govulncheck: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeMise(t)
			goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
			tools := lintTools{linter: f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), goDir: filepath.Dir(goCmd)}
			marker := filepath.Join(t.TempDir(), "ran")
			require.NoError(t, os.WriteFile(filepath.Join(f.bin, "govulncheck"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700))
			script := "#!/bin/sh\nexit 1\n"
			if path := tt.resolve(t, f); path != "" {
				script = "#!/bin/sh\necho '" + path + "'\n"
			}
			require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
			root := lockedRoot(t, lockFixture)
			var got step
			for _, s := range allSteps(t.Context(), root, tools, "/tmp/p/cover.out") {
				if s.name == "vulnerabilities" {
					got = s
				}
			}
			require.ErrorContains(t, got.unavailable, strings.ReplaceAll(tt.want, "{installs}", evalOrSelf(f.installs())))
			_, err := got.run(t.Context(), root)
			require.ErrorIs(t, err, got.unavailable, "an unavailable tool fails its step")
			assert.NoFileExists(t, marker, "no govulncheck of the search path ran")
		})
	}
}

// TestStepRunDeadline checks that a step that outlives its limit fails
// with context.DeadlineExceeded and a message that names the limit,
// where the bare error of the killed command says only "signal: killed".
func TestStepRunDeadline(t *testing.T) {
	sleep := []string{"sleep", "30"}
	t.Run("the limit of the step", func(t *testing.T) {
		_, err := step{name: "slow", argv: sleep, timeout: 50 * time.Millisecond, environ: []string{"PATH=" + os.Getenv("PATH")}}.run(t.Context(), t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.ErrorContains(t, err, "context deadline exceeded after 50ms: ")
	})
	t.Run("the deadline of the run, before the limit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		_, err := step{name: "slow", argv: sleep, environ: []string{"PATH=" + os.Getenv("PATH")}}.run(ctx, t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "at the deadline of the run: ")
		assert.NotContains(t, err.Error(), "after 15m")
	})
	t.Run("a grandchild that holds the output pipe", func(t *testing.T) {
		// sh forks sleep, which inherits the pipe and outlives sh: only
		// killing the process group, or WaitDelay, ends the wait.
		start := time.Now()
		_, err := step{name: "slow", argv: []string{"sh", "-c", "sleep 30; true"}, timeout: 200 * time.Millisecond, environ: []string{"PATH=" + os.Getenv("PATH")}}.run(t.Context(), t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second, "the step returns at its deadline, not when the grandchild ends")
	})
	t.Run("a failure that is not a deadline", func(t *testing.T) {
		_, err := step{name: "fails", argv: []string{"false"}, timeout: time.Minute, environ: []string{"PATH=" + os.Getenv("PATH")}}.run(t.Context(), t.TempDir())
		require.Error(t, err)
		assert.NotErrorIs(t, err, context.DeadlineExceeded)
	})
}

// TestArchMatches maps uname -m to the architecture of the Go binary.
func TestArchMatches(t *testing.T) {
	for _, tt := range []struct {
		uname, goarch string
		want          bool
	}{
		{"x86_64", "amd64", true}, {"x86_64", "arm64", false},
		{"aarch64", "arm64", true}, {"arm64", "arm64", true}, {"aarch64", "amd64", false},
		{"unknown", "amd64", false}, {"", "arm64", false}, {"i686", "amd64", false},
	} {
		assert.Equal(t, tt.want, archMatches(tt.uname, tt.goarch), "%s with %s", tt.uname, tt.goarch)
	}
}

// TestAllArchitecture reports the verdict of the architecture check: ok
// when uname -m names the architecture of the binary, FAIL, with both
// named, when it does not, and the other checks still run.
func TestAllArchitecture(t *testing.T) {
	other := map[string]string{"amd64": "aarch64", "arm64": "x86_64"}[runtime.GOARCH]
	if other == "" {
		t.Skip("no uname -m to compare with on " + runtime.GOARCH)
	}
	same := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	for _, tt := range []struct {
		uname, want string
		code        int
	}{
		{same, "ok    architecture\n", exitOK},
		{other, "FAIL  architecture\nuname -m reports " + other + " and this binary runs as " + runtime.GOARCH + ": ", exitFail},
		{"unknown", "FAIL  architecture\nuname -m reports unknown", exitFail},
	} {
		t.Run(tt.uname, func(t *testing.T) {
			r := newTree(t)
			r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
			r.Commit("fixture")
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, profileOf("tools/ci 1 1"), 0o600))
			var out bytes.Buffer
			e := env{dir: r.Dir, gitEnv: r.Env, stdin: strings.NewReader(""), stdout: &out, stderr: &out, steps: []step{passing}, profile: profile,
				machine: func(context.Context) string { return tt.uname }}
			assert.Equal(t, tt.code, run(t.Context(), e, []string{"all"}), "%s", out.String())
			assert.Contains(t, out.String(), tt.want)
			assert.Contains(t, out.String(), "ok    passes", "the other checks run")
			assert.Contains(t, out.String(), "ok    coverage")
		})
	}
}

// TestAllArchitectureTranslated fails the verdict of a binary that runs
// under Rosetta, where uname -m prints the architecture it emulates and
// so matches.
func TestAllArchitectureTranslated(t *testing.T) {
	same := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if same == "" {
		t.Skip("no uname -m to compare with on " + runtime.GOARCH)
	}
	for _, translated := range []bool{false, true} {
		r := newTree(t)
		r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
		r.Commit("fixture")
		profile := filepath.Join(t.TempDir(), "cover.out")
		require.NoError(t, os.WriteFile(profile, profileOf("tools/ci 1 1"), 0o600))
		var out bytes.Buffer
		e := env{dir: r.Dir, gitEnv: r.Env, stdin: strings.NewReader(""), stdout: &out, stderr: &out, steps: []step{passing}, profile: profile,
			machine:    func(context.Context) string { return same },
			translated: func(context.Context) bool { return translated }}
		code := run(t.Context(), e, []string{"all"})
		if translated {
			assert.Equal(t, exitFail, code, "%s", out.String())
			assert.Contains(t, out.String(), "FAIL  architecture\nsysctl.proc_translated is 1: ")
			assert.Contains(t, out.String(), "ok    passes", "the other checks run")
		} else {
			assert.Equal(t, exitOK, code, "%s", out.String())
			assert.Contains(t, out.String(), "ok    architecture\n")
		}
	}
}

// TestProcTranslated is false off macOS, where there is no such key,
// and on macOS it is what sysctl says. An Intel Mac has no such key
// either (measured on the macos-26-intel runner), so a sysctl error
// means "not translated".
func TestProcTranslated(t *testing.T) {
	if runtime.GOOS != "darwin" {
		assert.False(t, procTranslated(t.Context()))
		return
	}
	out, err := exec.CommandContext(t.Context(), "sysctl", "-n", "sysctl.proc_translated").Output()
	want := err == nil && strings.TrimSpace(string(out)) == "1"
	assert.Equal(t, want, procTranslated(t.Context()))
}

// TestLicenseStep pins the license step: reuse from the image pinned
// by digest, on linux, with the tree mounted read-only, no network and
// no privilege; on another platform a step that is not run, which all
// reports with a "--" line, and neither "ok" nor "FAIL".
func TestLicenseStep(t *testing.T) {
	env := []string{"PATH=/x"}
	assert.Equal(t, step{
		name: "license",
		argv: []string{"docker", "run", "--rm", "--network", "none",
			"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534",
			"--env", "GIT_CONFIG_COUNT=2",
			"--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=/src/repo",
			"--env", "GIT_CONFIG_KEY_1=safe.directory", "--env", "GIT_CONFIG_VALUE_1=/src/main/.git",
			"--mount", "type=bind,source=/src/repo,target=/src/repo,readonly",
			"--mount", "type=bind,source=/src/main/.git,target=/src/main/.git,readonly",
			"--workdir", "/src/repo",
			"fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e", "lint"},
		environ: env,
	}, licenseStep("/src/repo", "/src/main/.git", "linux", env))
	assert.Regexp(t, `@sha256:[0-9a-f]{64}$`, reuseImage)

	darwin := licenseStep("/src/repo", "/src/repo/.git", "darwin", env)
	assert.Equal(t, "not run on darwin: reuse runs from a Linux container image; the hosted Linux runners run it", darwin.skip)
	assert.Empty(t, darwin.argv, "nothing is started")

	require.Error(t, licenseStep("/src/a,b", "/src/a,b/.git", "linux", env).unavailable, "a path the mount option cannot name")
	require.Error(t, licenseStep("/src/a", "/src/b=c/.git", "linux", env).unavailable, "a git directory the mount option cannot name")

	var out bytes.Buffer
	c := newChecks(&out)
	c.run(t.Context(), t.TempDir(), []step{passing, darwin})
	assert.True(t, c.ok, "a step not run fails nothing")
	assert.Contains(t, out.String(), "--    license: "+darwin.skip+"\n")
	assert.NotContains(t, out.String(), "ok    license")
	assert.NotContains(t, out.String(), "FAIL  license")
}

// TestAllWiring runs all with its real steps against fake tools: a
// fake mise, go, gofmt, golangci-lint and docker that log how they are
// started. The steps run in the order of 10 10.2, the race step writes
// its profile where coverage reads it, and the profile's directory is
// gone after the run.
func TestAllWiring(t *testing.T) {
	r := newTree(t)
	r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
	r.Write("mise.lock", lockFixture)
	r.Write(pinnedMiseWorkflow, miseWorkflow(t, fakeMiseVersion))
	r.Commit("fixture")
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	f := newFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
	log := filepath.Join(t.TempDir(), "log")
	logged := func(name string, extra string) string {
		return "#!/bin/sh\necho \"" + name + " $*\" >> '" + log + "'\n" + extra
	}
	goCmd := filepath.Join(f.installs(), "go", "1.27.0", "bin", "go")
	writeProfile := "for a in \"$@\"; do case \"$a\" in -coverprofile=*) printf 'mode: set\\n" + fixtureModule + "/tools/ci/f.go:1.2,3.4 1 0\\n' > \"${a#-coverprofile=}\" ;; esac; done\n"
	for path, script := range map[string]string{
		goCmd: logged("go", writeProfile),
		filepath.Join(filepath.Dir(goCmd), "gofmt"):                                logged("gofmt", ""),
		filepath.Join(f.installs(), "golangci-lint", "2.14.0", "golangci-lint"):    logged("lint", ""),
		filepath.Join(f.installs(), govulncheckDir, "1.8.0", "bin", "govulncheck"): logged("govulncheck", ""),
		filepath.Join(f.bin, "docker"):                                             logged("docker", ""),
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	}
	which := "if [ \"$3\" = which ]; then\ncase \"$4\" in\n" +
		"go) echo '" + goCmd + "' ;;\n" +
		"golangci-lint) echo '" + filepath.Join(f.installs(), "golangci-lint", "2.14.0", "golangci-lint") + "' ;;\n" +
		"govulncheck) echo '" + filepath.Join(f.installs(), govulncheckDir, "1.8.0", "bin", "govulncheck") + "' ;;\n" +
		"esac\nexit 0\nfi\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\n"+answerVersion+which+"echo \"mise $*\" >> '"+log+"'\n"), 0o700))

	code, out := runCI(t, r, nil, nil, "all")
	assert.Equal(t, exitFail, code, out)
	assert.Contains(t, out, "FAIL  coverage\ntools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\n", "coverage read the profile the race step wrote")

	data, err := os.ReadFile(log)
	require.NoError(t, err)
	var firstWords []string
	var profile string
	for line := range strings.Lines(string(data)) {
		words := strings.Fields(line)
		firstWords = append(firstWords, strings.Join(words[:min(2, len(words))], " "))
		for _, w := range words {
			if p, ok := strings.CutPrefix(w, "-coverprofile="); ok {
				profile = p
			}
		}
	}
	want := []string{"gofmt -l", "go vet", "go run", "lint run", "lint run", "lint run", "lint run", "govulncheck ./..."}
	if runtime.GOOS == "linux" {
		want = append(want, "docker run")
	}
	want = append(want, "go test", "go test")
	assert.Equal(t, want, firstWords, "the order of the steps")
	require.NotEmpty(t, profile, "the race step names its profile")
	assert.Equal(t, coverFile, filepath.Base(profile))
	assert.NoFileExists(t, profile, "the profile's directory is removed after the run")
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
	r.Write("mise.lock", lockFixture)
	r.Write(pinnedMiseWorkflow, miseWorkflow(t, fakeMiseVersion))
	r.Commit("fixture")
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	f := newFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
	log := filepath.Join(t.TempDir(), "log")
	goCmd := filepath.Join(f.installs(), "go", "1.27.0", "bin", "go")
	require.NoError(t, os.MkdirAll(filepath.Dir(goCmd), 0o755))
	require.NoError(t, os.WriteFile(goCmd, []byte("#!/bin/sh\necho \"go $* GOPROXY=${GOPROXY-unset} GOFLAGS=$GOFLAGS HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset}\" >> '"+log+"'\n"), 0o700))
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	script := "#!/bin/sh\n" + answerVersion + "if [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\necho \"mise $* HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset} MISE=${MISE_OVERRIDE_CONFIG_FILENAMES-unset},${MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES-unset},${MISE_ENV-unset},${MISE_AUTO_ENV-unset} GO=${GOENV-unset},${GOTOOLCHAIN-unset}\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid")
	t.Setenv("DOCKER_HOST", "unix:///nowhere")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MISE_ENV", "ci")
	// The install builds govulncheck with the go command, so it gets
	// GOENV=off and GOTOOLCHAIN=local even when the process holds others.
	// passThroughEnv already drops these two, so the override itself is
	// held by TestMiseEnvList.
	t.Setenv("GOENV", "/from/the/process")
	t.Setenv("GOTOOLCHAIN", "go1.99.0")

	code, out := runCI(t, r, nil, nil, "setup")
	assert.Equal(t, exitOK, code, out)
	for _, w := range []string{"ok    mise trust", "ok    mise install", "ok    go mod download"} {
		assert.Contains(t, out, w)
	}
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	root, err := filepath.EvalSymlinks(r.Dir)
	require.NoError(t, err)
	// Both mise steps run with miseEnv, whatever the process holds.
	assert.Equal(t, "mise trust "+filepath.Join(root, "mise.toml")+" HTTPS_PROXY=unset DOCKER_HOST=unset MISE=mise.toml,none,,false GO=off,local\n"+
		"mise install HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset MISE=mise.toml,none,,false GO=off,local\n"+
		"go mod download GOPROXY=unset GOFLAGS=-mod=readonly HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset\n", string(data))

	t.Run("a hosted run without the mise environment stops before mise", func(t *testing.T) {
		require.NoError(t, os.Remove(log))
		t.Setenv("GITHUB_ACTIONS", "true")
		for _, kv := range miseEnv { // whatever the caller exports
			key, _, _ := strings.Cut(kv, "=")
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
		code, out := runCI(t, r, nil, nil, "setup")
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "the workflow sets MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml for every mise run")
		assert.NoFileExists(t, log, "no mise command ran")
	})

	t.Run("a mise of another version stops before mise runs anything else", func(t *testing.T) {
		require.NoError(t, os.WriteFile(log, nil, 0o600))
		r.Write(pinnedMiseWorkflow, miseWorkflow(t, "2026.10.4"))
		r.Commit("pins another mise")
		t.Cleanup(func() {
			r.Write(pinnedMiseWorkflow, miseWorkflow(t, fakeMiseVersion))
			r.Commit("pins the fake's mise again")
		})
		code, out := runCI(t, r, nil, nil, "setup")
		assert.Equal(t, exitError, code, out)
		assert.Contains(t, out, "ci: mise is at version "+fakeMiseVersion+", and .github/workflows/ci.yml pins 2026.10.4")
		data, err := os.ReadFile(log)
		require.NoError(t, err)
		assert.Empty(t, string(data), "no mise trust, no mise install")
	})

	t.Run("a failing mise stops before the download", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\n"+answerVersion+"exit 1\n"), 0o700))
		code, out := runCI(t, r, nil, nil, "setup")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "FAIL  mise trust")
		assert.NotContains(t, out, "go mod download")
	})
}

// TestPassThroughLists pins the two lists of variables a step may keep
// from the process, and the way withProcessEnv adds them: in the order
// of the list, a variable that is empty or unset left out.
func TestPassThroughLists(t *testing.T) {
	assert.Equal(t, []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}, networkPassThrough)
	assert.Equal(t, []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG"}, dockerPassThrough)

	t.Setenv("DOCKER_HOST", "unix:///a")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_CONFIG", "/c")
	base := []string{"PATH=/x"}
	assert.Equal(t, []string{"PATH=/x", "DOCKER_HOST=unix:///a", "DOCKER_CONFIG=/c"}, withProcessEnv(base, dockerPassThrough), "an empty variable is left out")
	assert.Equal(t, []string{"PATH=/x"}, base, "the base is not changed")
}

// overwriteObject returns a step that overwrites the loose object of
// the blob that rev names with a compressed blob of other bytes, under
// the same id: git cat-file does not hash what it reads, so a check
// that reads the blob after the step reads the other bytes. It is what
// a test of a change could do while a step runs it.
func overwriteObject(t *testing.T, r *gittest.Repo, rev, other string) step {
	t.Helper()
	id := strings.TrimSpace(r.Git("rev-parse", "--verify", rev))
	object := filepath.Join(r.Dir, ".git", "objects", id[:2], id[2:])
	require.FileExists(t, object, "the blob is a loose object")
	var b bytes.Buffer
	z := zlib.NewWriter(&b)
	_, err := fmt.Fprintf(z, "blob %d\x00%s", len(other), other)
	require.NoError(t, err)
	require.NoError(t, z.Close())
	src := filepath.Join(t.TempDir(), "object")
	require.NoError(t, os.WriteFile(src, b.Bytes(), 0o600))
	return step{name: "rewrites", argv: []string{"sh", "-c", `chmod u+w "$1" && cat "$2" > "$1"`, "sh", object, src}, environ: r.Env}
}

// TestCommitChecksReadBeforeTheSteps shows that hygiene, workflows and
// the pushed range judge the commit as it was before any step ran: a
// step that rewrites the objects of the commit, as a test of the
// change could, changes no verdict, in all, in fast by hand and in a
// pre-push run.
func TestCommitChecksReadBeforeTheSteps(t *testing.T) {
	dash := "a line with " + emDash + "\n"
	floating := strings.Replace(goodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1)
	fixture := func(t *testing.T) (*gittest.Repo, []step) {
		t.Helper()
		r := newTree(t)
		r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
		r.Write("docs/x.md", dash)
		r.Write(".github/workflows/ci.yml", floating)
		r.Commit("fixture")
		return r, []step{overwriteObject(t, r, "HEAD:docs/x.md", "a plain line\n"), overwriteObject(t, r, "HEAD:.github/workflows/ci.yml", goodWorkflow)}
	}
	t.Run("all", func(t *testing.T) {
		r, steps := fixture(t)
		profile := filepath.Join(t.TempDir(), "cover.out")
		require.NoError(t, os.WriteFile(profile, profileOf("tools/ci 1 1"), 0o600))
		code, out := runAllIn(t, r, append(steps, passing), profile)
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "ok    rewrites")
		assert.Contains(t, out, "FAIL  hygiene\ndocs/x.md:1: em-dash")
		assert.Contains(t, out, "FAIL  workflows\n.github/workflows/ci.yml:25: workflows: runs-on")
	})
	t.Run("fast by hand", func(t *testing.T) {
		r, steps := fixture(t)
		code, out := runCI(t, r, nil, steps, "fast")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "FAIL  hygiene\ndocs/x.md:1: em-dash")
		assert.Contains(t, out, "FAIL  workflows\n.github/workflows/ci.yml:25: workflows: runs-on")
	})
	t.Run("the pushed range of a pre-push run", func(t *testing.T) {
		const zero = "0000000000000000000000000000000000000000"
		r := newTree(t)
		r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
		r.Commit("fixture")
		r.Git("push", "--quiet", "origin", "main")
		r.Git("fetch", "--quiet", "origin")
		named := "see zorvex-quimby\n"
		r.Write("docs/page.md", named)
		r.Commit("docs: a page")
		require.NoError(t, os.Remove(filepath.Join(r.Dir, "docs", "page.md")))
		tip := strings.TrimSpace(r.Commit("docs: no page"))
		stdin := strings.NewReader("refs/heads/main " + tip + " refs/heads/main " + zero + "\n")
		code, out := runCI(t, r, stdin, []step{overwriteObject(t, r, "HEAD~1:docs/page.md", "plain\n")}, "fast", "origin", "url")
		assert.Equal(t, exitFail, code, out)
		assert.Contains(t, out, "ok    hygiene")
		assert.Contains(t, out, "FAIL  pushed range")
		assert.Contains(t, out, "docs/page.md:1: name:")
	})
}

// TestCommitCheckThatCannotJudgeFails holds the rule of commitChecks
// in fast by hand, in all and in a pre-push run: a check of the commit
// that cannot judge what it read fails, with exit status 1 and its
// reason on the FAIL line, and the other checks still report. Either
// mutation of that rule, back to an error that stops the run (exit 2)
// or to a verdict that drops the error (a broken denylist read as
// "ok"), fails this test.
func TestCommitCheckThatCannotJudgeFails(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	// pushedFails is whether the pushed range check reads the file too,
	// so that a pre-push run fails it as well as hygiene.
	broken := []struct {
		name, path, content string
		pushedFails         bool
	}{
		{"a denylist that does not parse", names.Path, "entries: [\n", true},
		{"word lists whose self-test fails", prose.Path, "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [plain]\n    passes: [plain]\n", false},
	}
	modes := []struct {
		name    string
		prePush bool
		run     func(t *testing.T, r *gittest.Repo, origin string) (int, string)
	}{
		{"fast by hand", false, func(t *testing.T, r *gittest.Repo, _ string) (int, string) {
			return runCI(t, r, nil, []step{passing}, "fast")
		}},
		{"all", false, func(t *testing.T, r *gittest.Repo, _ string) (int, string) {
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, profileOf("tools/ci 1 1"), 0o600))
			return runAllIn(t, r, []step{passing}, profile)
		}},
		{"fast as the pre-push hook", true, func(t *testing.T, r *gittest.Repo, origin string) (int, string) {
			tip := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
			stdin := strings.NewReader("refs/heads/main " + tip + " refs/heads/main " + zero + "\n")
			return runCI(t, r, stdin, []step{passing}, "fast", "origin", origin)
		}},
	}
	for _, b := range broken {
		for _, m := range modes {
			t.Run(b.name+", "+m.name, func(t *testing.T) {
				origin := gittest.NewBare(t)
				r := newTree(t)
				r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
				r.Git("remote", "add", "origin", origin.Dir)
				r.Commit("a good base")
				r.Git("push", "--quiet", "origin", "main")
				r.Git("fetch", "--quiet", "origin")
				r.Write(b.path, b.content)
				r.Commit("breaks " + b.path)
				code, out := m.run(t, r, origin.Dir)
				assert.Equal(t, exitFail, code, "exit status\n%s", out)
				assert.Contains(t, out, "FAIL  hygiene\n"+b.path+": ", "the reason is on the FAIL line")
				if m.prePush && b.pushedFails {
					assert.Contains(t, out, "FAIL  pushed range", "the pushed range cannot be judged either")
				}
				if m.prePush && !b.pushedFails {
					assert.NotContains(t, out, "FAIL  pushed range", "the file is not one the pushed range reads")
				}
				assert.Contains(t, out, "ok    passes")
				assert.Contains(t, out, "ok    workflows", "the other checks still report")
			})
		}
	}
}

// TestAllStepsLeaveTheTree holds the check that all runs after its
// steps: the steps leave HEAD, the index and the working tree as they
// found them, or all fails. The first case is the attack the check
// closes: a test of the change, run by a step, prepends an SPDX header
// to a file that has none, so that a check after it would pass a
// commit whose file still lacks one. A file that already differed from
// HEAD is compared by its content too, and a change that is there
// before the steps and stays is no finding.
func TestAllStepsLeaveTheTree(t *testing.T) {
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	// The tag is put together at run time, so that reuse does not read
	// this file as one that declares a license.
	header := "<!-- SPDX-" + "License-Identifier: GPL-3.0-only -->"
	tests := []struct {
		name   string
		before func(t *testing.T, r *gittest.Repo) // after the commit, before all
		script string                              // what the step runs, at the top of the tree
		want   []string
	}{
		{name: "a test prepends an SPDX header to a file that has none",
			script: `{ printf '%s\n' '` + header + `'; cat README.md; } > x && mv x README.md`,
			want:   []string{"FAIL  working tree\n", "\nREADME.md\n"}},
		{name: "a file appears", script: "echo x > late.txt", want: []string{"FAIL  working tree\n", "\nlate.txt\n"}},
		{name: "a file is staged", script: "echo more >> README.md && git add README.md", want: []string{"FAIL  working tree\n", "the index changed"}},
		{name: "HEAD moves", script: "git commit --quiet --allow-empty --message late", want: []string{"FAIL  working tree\n", "HEAD is now "}},
		{name: "a file that differed before is changed again",
			before: func(t *testing.T, r *gittest.Repo) { r.Write("README.md", "# A fixture\n\nWork in progress.\n") },
			script: "echo more >> README.md", want: []string{"FAIL  working tree\n", "\nREADME.md\n"}},
		{name: "work in progress that the steps leave as it was",
			before: func(t *testing.T, r *gittest.Repo) {
				r.Write("README.md", "# A fixture\n\nWork in progress.\n")
				r.Write("notes.txt", "untracked\n")
			},
			script: "true", want: []string{"ok    working tree\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
			r.Commit("fixture")
			if tt.before != nil {
				tt.before(t, r)
			}
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, profileOf("tools/ci 1 1"), 0o600))
			mutate := step{name: "mutates", argv: []string{sh, "-c", tt.script}, environ: r.Env}
			code, out := runAllIn(t, r, []step{mutate, passing}, profile)
			want := exitFail
			if strings.HasPrefix(tt.want[0], "ok") {
				want = exitOK
			}
			assert.Equal(t, want, code, "exit status\n%s", out)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			assert.Contains(t, out, "ok    hygiene", "the checks of the commit still report")
		})
	}
}

// TestCommitChecksReadBeforeMise runs fast and all with their real
// steps (no stubs, so resolveLintTools starts mise) against a fake
// mise that rewrites the loose object of a judged file whenever it
// runs: hygiene still reports the finding of the commit as it was, so
// the commit is read before "mise --version" and "mise which" run.
func TestCommitChecksReadBeforeMise(t *testing.T) {
	for _, mode := range []string{"fast", "all"} {
		t.Run(mode, func(t *testing.T) {
			chmod, err := exec.LookPath("chmod")
			require.NoError(t, err)
			cat, err := exec.LookPath("cat")
			require.NoError(t, err)
			gitPath, err := exec.LookPath("git")
			require.NoError(t, err)
			r := newTree(t)
			r.Write("go.mod", "module "+fixtureModule+"\n\ngo 1.27.0\n")
			r.Write("mise.lock", lockFixture)
			r.Write(pinnedMiseWorkflow, miseWorkflow(t, fakeMiseVersion))
			r.Write("docs/x.md", "a line with "+emDash+"\n")
			r.Commit("fixture")
			rewrite := overwriteObject(t, r, "HEAD:docs/x.md", "a plain line\n").argv
			object, src := rewrite[4], rewrite[5]

			f := newFakeMise(t)
			require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
			require.NoError(t, os.WriteFile(filepath.Join(f.bin, "docker"), []byte("#!/bin/sh\n"), 0o700))
			goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
			f.tool(t, "go", "1.27.0", "bin", "gofmt")
			f.tool(t, govulncheckDir, "1.8.0", "bin", "govulncheck")
			prelude := "'" + chmod + "' u+w '" + object + "' && '" + cat + "' '" + src + "' > '" + object + "'\n" + answerVersion +
				"if [ \"$4\" = govulncheck ]; then echo '" + filepath.Join(f.installs(), govulncheckDir, "1.8.0", "bin", "govulncheck") + "'; exit 0; fi\n"
			f.script(t, prelude, f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), goCmd)

			code, out := runCI(t, r, nil, nil, mode)
			assert.Equal(t, exitFail, code, out)
			assert.Contains(t, out, "ok    unit", "the real steps ran, after mise")
			assert.Contains(t, out, "FAIL  hygiene\ndocs/x.md:1: em-dash")
			assert.Equal(t, "a plain line", strings.TrimSpace(r.Git("cat-file", "blob", "HEAD:docs/x.md")), "mise did rewrite the object")
		})
	}
}
