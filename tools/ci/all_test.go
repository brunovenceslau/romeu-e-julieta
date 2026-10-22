// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/container"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/crossbuild"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/hygiene"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/imports"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/license"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/workflows"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
)

// runAllIn runs "all" in the fixture with stub steps and the cover
// profile at profile.
func runAllIn(t *testing.T, r *gittest.Repo, steps []check.Step, profile string, args ...string) (int, string) {
	t.Helper()
	return runJobIn(t, r, "all", steps, profile, args...)
}

// runJobIn runs the subcommand command, all or one of check.Jobs, in
// the fixture, as runAllIn does.
func runJobIn(t *testing.T, r *gittest.Repo, command string, steps []check.Step, profile string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, Steps: steps, Profile: profile, Now: frozen}
	return run(t.Context(), e, append([]string{command}, args...)), out.String()
}

// infraStep is a step whose Around reads an infrastructure error, as
// the e2e step does for a pull that failed on transport.
var infraStep = check.Step{Name: "e2e", Argv: []string{"false"}, Environ: []string{"PATH=" + os.Getenv("PATH")},
	Around: func() ([]string, check.Read, error) {
		return nil, func([]byte, error) ([]byte, error) {
			return nil, fmt.Errorf("the image pull: %w", check.ErrInfrastructure)
		}, nil
	}}

// TestAll covers all with stub steps: every check runs and reports,
// whatever an earlier one found, and the checks in this process read
// the tree at HEAD and the cover profile.
func TestAll(t *testing.T) {
	tests := []struct {
		name    string
		steps   []check.Step
		change  func(t *testing.T, r *gittest.Repo)
		profile string // the profile's blocks for profileOf; "missing" for none
		code    int
		want    []string
	}{
		{
			name:    "every check passing",
			steps:   []check.Step{citest.Passing},
			profile: "tools/ci 1 1",
			code:    check.ExitOK,
			want:    []string{"platform  ", "ok    passes", "ok    hygiene", "ok    workflows", "ok    coverage"},
		},
		{
			name:    "a failing step does not hide the checks after it",
			steps:   []check.Step{citest.Failing, citest.Passing},
			profile: "tools/ci 1 1",
			code:    check.ExitFail,
			want:    []string{"FAIL  fails", "ok    passes", "ok    hygiene", "ok    workflows", "ok    coverage"},
		},
		{
			name:    "a package below its threshold",
			steps:   []check.Step{citest.Passing},
			profile: "tools/ci 1 0",
			code:    check.ExitFail,
			want:    []string{"ok    workflows", "FAIL  coverage (0s)\ntools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\n"},
		},
		{
			name:    "no cover profile, as when the race step could not start",
			steps:   []check.Step{citest.Passing},
			profile: "missing",
			code:    check.ExitFail,
			want:    []string{"FAIL  coverage (0s)\nthe cover profile of the race step: "},
		},
		{
			name:  "a workflow outside the grammar",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				t.Helper()
				r.Write(".github/workflows/ci.yml", strings.Replace(citest.GoodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1))
			},
			profile: "tools/ci 1 1",
			code:    check.ExitFail,
			want:    []string{"FAIL  workflows (0s)\n.github/workflows/ci.yml:29: workflows: runs-on is ${{ matrix.os }} or one of ubuntu-26.04, ubuntu-26.04-arm, macos-26, macos-26-intel, never *-latest\n", "ok    coverage"},
		},
		{
			name:  "a hygiene finding",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				t.Helper()
				r.Write("docs/x.md", "a line with "+hygiene.EmDash+"\n")
			},
			profile: "tools/ci 1 1",
			code:    check.ExitFail,
			want:    []string{"FAIL  hygiene", "em-dash", "ok    workflows"},
		},
		{
			name:  "a step that needs a module the cache lacks names the setup",
			steps: []check.Step{{Name: "race", Argv: []string{"sh", "-c", "echo 'x.go:1:2: module lookup disabled by GOPROXY=off'; exit 1"}, Environ: []string{"PATH=" + os.Getenv("PATH")}}},
			code:  check.ExitFail,
			want:  []string{"FAIL  race (0s)\nx.go:1:2: module lookup disabled by GOPROXY=off\n", `run "go run ./tools/ci setup", which runs "go mod download"`},
		},
		{
			name:    "an infrastructure error alone exits with its own status",
			steps:   []check.Step{infraStep, citest.Passing},
			profile: "tools/ci 1 1",
			code:    check.ExitInfra,
			want:    []string{"FAIL  e2e", "ok    passes", "all: failed: e2e\n", "ci: all: e2e: an infrastructure error"},
		},
		{
			name:    "an infrastructure error beside a failed check exits as a failure",
			steps:   []check.Step{infraStep, citest.Failing},
			profile: "tools/ci 1 1",
			code:    check.ExitFail,
			want:    []string{"FAIL  e2e", "FAIL  fails", "all: failed: e2e, fails\n"},
		},
		{
			name:  "a nested module stops all before any check",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				t.Helper()
				r.Write("tools/inner/go.mod", "module example.invalid/inner\n")
			},
			code: check.ExitError,
			want: []string{"ci: the checks read one module"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newSchemaTree(t)
			r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
			if tt.change != nil {
				tt.change(t, r)
			}
			r.Commit("fixture")
			profile := filepath.Join(t.TempDir(), "cover.out")
			if tt.profile != "missing" && tt.profile != "" {
				require.NoError(t, os.WriteFile(profile, citest.ProfileOf(tt.profile), 0o600))
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
	r := citest.NewTree(t)
	r.Commit("fixture")
	for _, args := range [][]string{{"all", "x"}, {"setup", "x"}, {"workflows", "x"}} {
		code, out := runCI(t, r, nil, nil, args...)
		assert.Equal(t, check.ExitError, code, "%v\n%s", args, out)
	}
}

// TestAllSteps pins the command steps of all: the steps of fast but its
// last, race, with vulnerabilities, the two steps of license, the four
// cross builds and imports before race, since none runs code of the
// change, then race, which writes the cover profile, then e2e.
// Each runs in check.StepEnv; the one that reads the Go vulnerability
// database keeps the variables of a proxy too, and the reuse step of
// license those of the Docker daemon, and no other step keeps either.
func TestAllSteps(t *testing.T) {
	root := citest.NewTree(t).Dir
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.lock"), []byte(citest.LockFixture), 0o600))
	gitPath, err := procgroup.LookPath("git")
	require.NoError(t, err)
	f := citest.NewFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.Bin, "git")))
	goCmd := f.Tool(t, "go", "1.27.0", "bin", "go")
	linter := f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	vuln := f.Tool(t, mise.GovulncheckDir, "1.8.0", "bin", "govulncheck")
	script := "#!/bin/sh\ncase \"$4\" in\ngovulncheck) echo '" + vuln + "' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(script), 0o700))
	for _, key := range slices.Concat(check.NetworkPassThrough, check.DockerPassThrough) {
		t.Setenv(key, "from-the-process")
	}

	common, err := license.GitCommonDir(t.Context(), root)
	require.NoError(t, err)
	tools := check.Tools{Linter: linter, GoDir: filepath.Dir(goCmd)}
	env := check.StepEnv(tools.GoDir)
	steps := allSteps(t.Context(), root, tools, "/tmp/p/cover.out")
	fast := fastSteps(t.Context(), root, tools)
	race := len(fast) - 1
	require.Equal(t, "race", fast[race].Name, "the one test run is the last step of fast")
	assert.Equal(t, []string{goCmd, "test", "-race", "-count=1", "./..."}, fast[race].Argv, "fast has no step after the run to read a cover profile")
	require.Len(t, steps, len(fast)+13, "no e2e directory, so no tagged suite to vet")
	assert.Equal(t, withoutAround(t, fast[:race]), withoutAround(t, steps[:race]))
	assert.Equal(t, crossbuild.Steps(t.Context(), check.Plan{Root: root, Tools: tools}), steps[race+6:race+10])
	assert.Equal(t, imports.Steps(t.Context(), check.Plan{Root: root, Tools: tools}), steps[race+10:race+11])

	assert.Equal(t, mise.Steps(t.Context(), check.Plan{Root: root, Tools: tools}), steps[race+11:race+12])
	vulnSteps, licenseStep, modulesStep, raceStep, e2e := steps[race:race+4], steps[race+4], steps[race+5], steps[race+12], steps[race+13]
	wantE2E := container.Steps(t.Context(), check.Plan{Root: root, Tools: tools})[0]
	assert.Equal(t, wantE2E.Around == nil, e2e.Around == nil)
	assert.Equal(t, wantE2E.Notes == nil, e2e.Notes == nil)
	wantE2E.Around, e2e.Around, wantE2E.Notes, e2e.Notes = nil, nil, nil, nil
	assert.Equal(t, wantE2E, e2e, "the e2e step comes last, after race")
	for i, target := range lint.Targets {
		assert.Equal(t, check.Step{Name: "vulnerabilities " + target.String(), Argv: []string{evalOrSelf(vuln), "./..."}, Environ: check.WithProcessEnv(lint.Env(target, tools.GoDir), check.NetworkPassThrough),
			NoticeOn: "govulncheck: fetching vulnerabilities:", Notice: "vulnerabilities: could not fetch the vulnerability database, so this all is not the equivalent run (10 10.2)"}, vulnSteps[i], "govulncheck runs by the path mise resolves, and not through mise exec, once per target")
	}
	assert.Equal(t, 40*time.Minute, procgroup.ToolRace.Timeout())
	assert.Less(t, procgroup.ToolRace.Timeout(), ciJobTimeout(t), "the race run ends before the race job of ci.yml does")
	assert.Equal(t, check.Step{Name: "race", Argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=/tmp/p/cover.out", "./..."}, Environ: env, Class: procgroup.ToolRace}, raceStep)
	for _, s := range steps {
		assert.Equal(t, s.Name == "race", s.Class != 0, "%s has a class of its own only if it is the race run", s.Name)
	}
	assert.Equal(t, license.Step(root, common, runtime.GOOS, check.WithProcessEnv(env, check.DockerPassThrough)), licenseStep)
	assert.Equal(t, license.ModulesStep(check.Plan{Root: root, Tools: tools}), modulesStep)

	keys := func(s check.Step) []string {
		var out []string
		for _, kv := range s.Environ {
			key, _, _ := strings.Cut(kv, "=")
			out = append(out, key)
		}
		return out
	}
	e2eRuns := e2e.Skip == "" && e2e.Unavailable == nil
	for _, s := range steps {
		for _, key := range check.NetworkPassThrough {
			assert.Equal(t, strings.HasPrefix(s.Name, "vulnerabilities ") || (s.Name == "e2e" && e2eRuns), slices.Contains(keys(s), key), "%s keeps %s", s.Name, key)
		}
		for _, key := range check.DockerPassThrough {
			assert.Equal(t, (s.Name == "license" && runtime.GOOS == "linux") || (s.Name == "e2e" && e2eRuns), slices.Contains(keys(s), key), "%s keeps %s", s.Name, key)
		}
	}
	for _, key := range slices.Concat(check.NetworkPassThrough, check.DockerPassThrough, check.GoModulePassThrough) {
		assert.NotContains(t, goenv.PassThrough, key)
	}
}

// ciJobTimeout returns the timeout-minutes of the race job of ci.yml,
// the job that runs the race step.
func ciJobTimeout(t *testing.T) time.Duration {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(workflows.Dir), "ci.yml"))
	require.NoError(t, err)
	var wf struct {
		Jobs map[string]struct {
			Minutes int `yaml:"timeout-minutes"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(src, &wf))
	job, ok := wf.Jobs["race"]
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
		resolve func(t *testing.T, f citest.FakeMise) string
		want    string
	}{
		{
			name: "outside the installs directory",
			resolve: func(t *testing.T, _ citest.FakeMise) string {
				t.Helper()
				return citest.Executable(t, filepath.Join(t.TempDir(), "govulncheck"))
			},
			want: "is outside " + filepath.Join("{installs}", mise.GovulncheckDir) + ", where mise installs it",
		},
		{
			name: "at a version beside the locked one",
			resolve: func(t *testing.T, f citest.FakeMise) string {
				t.Helper()
				return f.Tool(t, mise.GovulncheckDir, "1.7.0", "bin", "govulncheck")
			},
			want: "the install is not the version 1.8.0 that mise.lock locks",
		},
		{
			name: "not installed, with a fake on the search path",
			resolve: func(t *testing.T, _ citest.FakeMise) string {
				t.Helper()
				return ""
			},
			want: "mise which govulncheck: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := citest.NewFakeMise(t)
			goCmd := f.Tool(t, "go", "1.27.0", "bin", "go")
			tools := check.Tools{Linter: f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), GoDir: filepath.Dir(goCmd)}
			marker := filepath.Join(t.TempDir(), "ran")
			require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "govulncheck"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700))
			script := "#!/bin/sh\nexit 1\n"
			if path := tt.resolve(t, f); path != "" {
				script = "#!/bin/sh\necho '" + path + "'\n"
			}
			require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(script), 0o700))
			root := citest.LockedRoot(t, citest.LockFixture)
			var got check.Step
			for _, s := range allSteps(t.Context(), root, tools, "/tmp/p/cover.out") {
				if strings.HasPrefix(s.Name, "vulnerabilities ") {
					got = s
				}
			}
			require.ErrorContains(t, got.Unavailable, strings.ReplaceAll(tt.want, "{installs}", evalOrSelf(f.Installs())))
			_, err := got.Run(t.Context(), root)
			require.ErrorIs(t, err, got.Unavailable, "an unavailable tool fails its step")
			assert.NoFileExists(t, marker, "no govulncheck of the search path ran")
		})
	}
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

// unames are what uname -m prints, by runtime.GOARCH, on a machine of
// that architecture (same) and on one of the other (other).
var unames = map[string]struct{ same, other string }{
	"amd64": {same: "x86_64", other: "aarch64"},
	"arm64": {same: "aarch64", other: "x86_64"},
}

// TestAllArchitecture reports the verdict of the architecture check: ok
// when uname -m names the architecture of the binary, FAIL, with both
// named, when it does not, and the other checks still run.
func TestAllArchitecture(t *testing.T) {
	type unameCase struct {
		uname, want string
		code        int
	}
	cases := []unameCase{{"unknown", "FAIL  architecture (0s)\nuname -m reports unknown", check.ExitFail}}
	// On an architecture with no entry, the FAIL case runs alone.
	if a, ok := unames[runtime.GOARCH]; ok {
		cases = append(cases,
			unameCase{a.same, "ok    architecture (0s)\n", check.ExitOK},
			unameCase{a.other, "FAIL  architecture (0s)\nuname -m reports " + a.other + " and this binary runs as " + runtime.GOARCH + ": ", check.ExitFail})
	}
	for _, tt := range cases {
		t.Run(tt.uname, func(t *testing.T) {
			r := newSchemaTree(t)
			r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
			r.Commit("fixture")
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))
			var out bytes.Buffer
			e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, Steps: []check.Step{citest.Passing}, Profile: profile, Now: frozen,
				Machine: func(context.Context) string { return tt.uname }}
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
	a, ok := unames[runtime.GOARCH]
	same := a.same
	translatedCases := []bool{false, true}
	if !ok {
		// With no uname -m of its own, the machine fails on that alone,
		// so only the FAIL case runs.
		same, translatedCases = "unknown", []bool{true}
	}
	for _, translated := range translatedCases {
		r := newSchemaTree(t)
		r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
		r.Commit("fixture")
		profile := filepath.Join(t.TempDir(), "cover.out")
		require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))
		var out bytes.Buffer
		e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, Steps: []check.Step{citest.Passing}, Profile: profile, Now: frozen,
			Machine:    func(context.Context) string { return same },
			Translated: func(context.Context) bool { return translated }}
		code := run(t.Context(), e, []string{"all"})
		if translated {
			assert.Equal(t, check.ExitFail, code, "%s", out.String())
			if ok {
				assert.Contains(t, out.String(), "FAIL  architecture (0s)\nsysctl.proc_translated is 1: ")
			}
			assert.Contains(t, out.String(), "ok    passes", "the other checks run")
		} else {
			assert.Equal(t, check.ExitOK, code, "%s", out.String())
			assert.Contains(t, out.String(), "ok    architecture (0s)\n")
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
	var out strings.Builder
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolProbe,
		Argv:   []string{"sysctl", "-n", "sysctl.proc_translated"},
		Env:    os.Environ(),
		Stdout: &out,
	})
	want := err == nil && strings.TrimSpace(out.String()) == "1"
	assert.Equal(t, want, procTranslated(t.Context()))
}

// TestProbe checks how a probe of the machine runs: in this process's
// environment, with what it prints trimmed and its standard error
// dropped, under the class procgroup.ToolProbe, past whose limit it is
// subprocess-timeout.
func TestProbe(t *testing.T) {
	t.Setenv("CI_TEST_PROBE", "from the process")
	out, err := probe(t.Context(), "sh", "-c", `printf ' %s \n' "$CI_TEST_PROBE"; echo noise >&2`)
	require.NoError(t, err)
	assert.Equal(t, "from the process", out)

	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolProbe, 50*time.Millisecond)
	_, err = probe(ctx, "sleep", "30")
	timeout, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "the error is subprocess-timeout: %v", err)
	assert.Equal(t, procgroup.ToolProbe, timeout.Class)

	t.Setenv("PATH", t.TempDir())
	assert.Contains(t, unameMachine(t.Context()), "unknown (", "a probe that fails says why")
}

// TestAllWiring runs all with its real steps against fake tools: a
// fake mise, go, gofmt, golangci-lint and docker that log how they are
// started. The steps run in the order of 10 10.2, the race step writes
// its profile where coverage reads it, and the profile's directory is
// gone after the run.
func TestAllWiring(t *testing.T) {
	r := citest.NewTree(t)
	r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
	r.Write("mise.lock", citest.LockFixture)
	r.Write(mise.PinnedWorkflow, citest.MiseWorkflow(t, citest.FakeMiseVersion))
	r.Commit("fixture")
	gitPath, err := procgroup.LookPath("git")
	require.NoError(t, err)
	f := citest.NewFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.Bin, "git")))
	log := filepath.Join(t.TempDir(), "log")
	logged := func(name string, extra string) string {
		return "#!/bin/sh\necho \"" + name + " $*\" >> '" + log + "'\n" + extra
	}
	goCmd := filepath.Join(f.Installs(), "go", "1.27.0", "bin", "go")
	writeProfile := "for a in \"$@\"; do case \"$a\" in -coverprofile=*) printf 'mode: set\\n" + citest.FixtureModule + "/tools/ci/f.go:1.2,3.4 1 0\\n' > \"${a#-coverprofile=}\" ;; esac; done\n"
	for path, script := range map[string]string{
		goCmd: logged("go", writeProfile),
		filepath.Join(filepath.Dir(goCmd), "gofmt"):                                     logged("gofmt", ""),
		filepath.Join(f.Installs(), "golangci-lint", "2.14.0", "golangci-lint"):         logged("lint", ""),
		filepath.Join(f.Installs(), mise.GovulncheckDir, "1.8.0", "bin", "govulncheck"): logged("govulncheck", ""),
		filepath.Join(f.Bin, "docker"):                                                  logged("docker", ""),
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	}
	which := "if [ \"$3\" = which ]; then\ncase \"$4\" in\n" +
		"go) echo '" + goCmd + "' ;;\n" +
		"golangci-lint) echo '" + filepath.Join(f.Installs(), "golangci-lint", "2.14.0", "golangci-lint") + "' ;;\n" +
		"govulncheck) echo '" + filepath.Join(f.Installs(), mise.GovulncheckDir, "1.8.0", "bin", "govulncheck") + "' ;;\n" +
		"esac\nexit 0\nfi\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte("#!/bin/sh\n"+citest.AnswerVersion+which+"echo \"mise $*\" >> '"+log+"'\n"), 0o700))

	code, out := runCI(t, r, nil, nil, "all")
	assert.Equal(t, check.ExitFail, code, out)
	assert.Contains(t, out, "FAIL  coverage (0s)\ntools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\n", "coverage read the profile the race step wrote")

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
	want := []string{"gofmt -l", "go vet", "go vet", "go vet", "go vet", "go fix", "go fix", "go fix", "go fix", "go run", "lint run", "lint run", "lint run", "lint run", "govulncheck ./...", "govulncheck ./...", "govulncheck ./...", "govulncheck ./..."}
	if runtime.GOOS == "linux" {
		want = append(want, "docker run")
	}
	want = append(want, "go run", "go build", "go build", "go build", "go build", "go run", "go run", "go test")
	if slices.Contains(container.Classes[runtime.GOOS+"/"+runtime.GOARCH], "e2e") {
		want = append(want, "go test")
	}
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

// overwriteObject returns a step that overwrites the loose object of
// the blob that rev names with a compressed blob of other bytes, under
// the same id: git cat-file does not hash what it reads, so a check
// that reads the blob after the step reads the other bytes. It is what
// a test of a change could do while a step runs it.
func overwriteObject(t *testing.T, r *gittest.Repo, rev, other string) check.Step {
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
	return check.Step{Name: "rewrites", Argv: []string{"sh", "-c", `chmod u+w "$1" && cat "$2" > "$1"`, "sh", object, src}, Environ: r.Env}
}

// TestCommitChecksReadBeforeTheSteps shows that hygiene, workflows and
// the pushed range judge the commit as it was before any step ran: a
// step that rewrites the objects of the commit, as a test of the
// change could, changes no verdict, in all, in fast by hand and in a
// pre-push run.
func TestCommitChecksReadBeforeTheSteps(t *testing.T) {
	dash := "a line with " + hygiene.EmDash + "\n"
	floating := strings.Replace(citest.GoodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1)
	fixture := func(t *testing.T) (*gittest.Repo, []check.Step) {
		t.Helper()
		r := citest.NewTree(t)
		r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
		r.Write("docs/x.md", dash)
		r.Write(".github/workflows/ci.yml", floating)
		r.Commit("fixture")
		return r, []check.Step{overwriteObject(t, r, "HEAD:docs/x.md", "a plain line\n"), overwriteObject(t, r, "HEAD:.github/workflows/ci.yml", citest.GoodWorkflow)}
	}
	t.Run("all", func(t *testing.T) {
		r, steps := fixture(t)
		profile := filepath.Join(t.TempDir(), "cover.out")
		require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))
		code, out := runAllIn(t, r, append(steps, citest.Passing), profile)
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "ok    rewrites")
		assert.Contains(t, out, "FAIL  hygiene (0s)\ndocs/x.md:1: em-dash")
		assert.Contains(t, out, "FAIL  workflows (0s)\n.github/workflows/ci.yml:29: workflows: runs-on")
	})
	t.Run("fast by hand", func(t *testing.T) {
		r, steps := fixture(t)
		code, out := runCI(t, r, nil, steps, "fast")
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "FAIL  hygiene (0s)\ndocs/x.md:1: em-dash")
		assert.Contains(t, out, "FAIL  workflows (0s)\n.github/workflows/ci.yml:29: workflows: runs-on")
	})
	t.Run("the pushed range of a pre-push run", func(t *testing.T) {
		const zero = "0000000000000000000000000000000000000000"
		r := citest.NewTree(t)
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
		code, out := runCI(t, r, stdin, []check.Step{overwriteObject(t, r, "HEAD~1:docs/page.md", "plain\n")}, "fast", "origin", "url")
		assert.Equal(t, check.ExitFail, code, out)
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
			t.Helper()
			return runCI(t, r, nil, []check.Step{citest.Passing}, "fast")
		}},
		{"all", false, func(t *testing.T, r *gittest.Repo, _ string) (int, string) {
			t.Helper()
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))
			return runAllIn(t, r, []check.Step{citest.Passing}, profile)
		}},
		{"fast as the pre-push hook", true, func(t *testing.T, r *gittest.Repo, origin string) (int, string) {
			t.Helper()
			tip := strings.TrimSpace(r.Git("rev-parse", "HEAD"))
			stdin := strings.NewReader("refs/heads/main " + tip + " refs/heads/main " + zero + "\n")
			return runCI(t, r, stdin, []check.Step{citest.Passing}, "fast", "origin", origin)
		}},
	}
	for _, b := range broken {
		for _, m := range modes {
			t.Run(b.name+", "+m.name, func(t *testing.T) {
				origin := gittest.NewBare(t)
				r := citest.NewTree(t)
				r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
				r.Git("remote", "add", "origin", origin.Dir)
				r.Commit("a good base")
				r.Git("push", "--quiet", "origin", "main")
				r.Git("fetch", "--quiet", "origin")
				r.Write(b.path, b.content)
				r.Commit("breaks " + b.path)
				code, out := m.run(t, r, origin.Dir)
				assert.Equal(t, check.ExitFail, code, "exit status\n%s", out)
				assert.Contains(t, out, "FAIL  hygiene (0s)\n"+b.path+": ", "the reason is on the FAIL line")
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
	sh, err := procgroup.LookPath("sh")
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
			want:   []string{"FAIL  working tree (0s)\n", "\nREADME.md\n"}},
		{name: "a file appears", script: "echo x > late.txt", want: []string{"FAIL  working tree (0s)\n", "\nlate.txt\n"}},
		{name: "a file is staged", script: "echo more >> README.md && git add README.md", want: []string{"FAIL  working tree (0s)\n", "the index changed"}},
		{name: "HEAD moves", script: "git commit --quiet --allow-empty --message late", want: []string{"FAIL  working tree (0s)\n", "HEAD is now "}},
		{name: "a file that differed before is changed again",
			before: func(t *testing.T, r *gittest.Repo) {
				t.Helper()
				r.Write("README.md", "# A fixture\n\nWork in progress.\n")
			},
			script: "echo more >> README.md", want: []string{"FAIL  working tree (0s)\n", "\nREADME.md\n"}},
		{name: "work in progress that the steps leave as it was",
			before: func(t *testing.T, r *gittest.Repo) {
				t.Helper()
				r.Write("README.md", "# A fixture\n\nWork in progress.\n")
				r.Write("notes.txt", "untracked\n")
			},
			script: "true", want: []string{"ok    working tree (0s)\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newSchemaTree(t)
			r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
			r.Commit("fixture")
			if tt.before != nil {
				tt.before(t, r)
			}
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))
			mutate := check.Step{Name: "mutates", Argv: []string{sh, "-c", tt.script}, Environ: r.Env}
			code, out := runAllIn(t, r, []check.Step{mutate, citest.Passing}, profile)
			want := check.ExitFail
			if strings.HasPrefix(tt.want[0], "ok") {
				want = check.ExitOK
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
// steps (no stubs, so mise.ResolveLintTools starts mise) against a fake
// mise that rewrites the loose object of a judged file whenever it
// runs: hygiene still reports the finding of the commit as it was, so
// the commit is read before "mise --version" and "mise which" run.
func TestCommitChecksReadBeforeMise(t *testing.T) {
	for _, mode := range []string{"fast", "all"} {
		t.Run(mode, func(t *testing.T) {
			chmod, err := procgroup.LookPath("chmod")
			require.NoError(t, err)
			cat, err := procgroup.LookPath("cat")
			require.NoError(t, err)
			gitPath, err := procgroup.LookPath("git")
			require.NoError(t, err)
			r := citest.NewTree(t)
			r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
			r.Write("mise.lock", citest.LockFixture)
			r.Write(mise.PinnedWorkflow, citest.MiseWorkflow(t, citest.FakeMiseVersion))
			r.Write("docs/x.md", "a line with "+hygiene.EmDash+"\n")
			r.Commit("fixture")
			rewrite := overwriteObject(t, r, "HEAD:docs/x.md", "a plain line\n").Argv
			object, src := rewrite[4], rewrite[5]

			f := citest.NewFakeMise(t)
			require.NoError(t, os.Symlink(gitPath, filepath.Join(f.Bin, "git")))
			require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "docker"), []byte("#!/bin/sh\n"), 0o700))
			goCmd := f.Tool(t, "go", "1.27.0", "bin", "go")
			f.Tool(t, "go", "1.27.0", "bin", "gofmt")
			f.Tool(t, mise.GovulncheckDir, "1.8.0", "bin", "govulncheck")
			prelude := "'" + chmod + "' u+w '" + object + "' && '" + cat + "' '" + src + "' > '" + object + "'\n" + citest.AnswerVersion +
				"if [ \"$4\" = govulncheck ]; then echo '" + filepath.Join(f.Installs(), mise.GovulncheckDir, "1.8.0", "bin", "govulncheck") + "'; exit 0; fi\n"
			f.Script(t, prelude, f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), goCmd)

			code, out := runCI(t, r, nil, nil, mode)
			assert.Equal(t, check.ExitFail, code, out)
			assert.Contains(t, out, "ok    race", "the real steps ran, after mise")
			assert.Contains(t, out, "FAIL  hygiene (0s)\ndocs/x.md:1: em-dash")
			assert.Equal(t, "a plain line", strings.TrimSpace(r.Git("cat-file", "blob", "HEAD:docs/x.md")), "mise did rewrite the object")
		})
	}
}

// jobsOf returns the names of the checks of all by job, in the order of
// the registry.
func jobsOf() map[string][]string {
	jobs := map[string][]string{}
	for _, c := range check.Registered() {
		if c.All {
			jobs[c.JobName()] = append(jobs[c.JobName()], c.Name)
		}
	}
	return jobs
}

// TestChecksNameTheirJob holds the jobs of 10 10.2: static, race and e2e
// are the only ones, a check that names none is static, the race job
// holds the race step and the coverage check after it, the e2e job the
// container e2e, and every check of all is in exactly one job, so that
// all, which runs their union, misses none.
func TestChecksNameTheirJob(t *testing.T) {
	assert.Equal(t, []string{"static", "race", "e2e"}, check.Jobs)
	jobs := jobsOf()
	assert.Equal(t, []string{"race", "coverage"}, jobs["race"])
	assert.Equal(t, []string{"container"}, jobs["e2e"])
	assert.NotContains(t, jobs["static"], "race")
	assert.NotContains(t, jobs["static"], "coverage")
	assert.NotContains(t, jobs["static"], "container")
	assert.Len(t, jobs, 3)
	for job := range jobs {
		assert.Contains(t, check.Jobs, job)
	}
	var union []string
	for _, names := range jobs {
		union = append(union, names...)
	}
	var all []string
	for _, c := range registered(inAll) {
		all = append(all, c.Name)
	}
	assert.ElementsMatch(t, all, union, "the jobs hold every check of all, each once")
	for _, c := range check.Registered() {
		if c.Name == "pushed range" {
			assert.False(t, c.All, "a check of the pre-push hook alone is in no job")
		}
	}
}

// TestAllRunsTheUnionOfTheJobs holds the order of all: the steps of the
// static job, then those of the race job, then those of the e2e job,
// which is the order of today, and no step is in two jobs.
func TestAllRunsTheUnionOfTheJobs(t *testing.T) {
	tools := check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/go/bin"}
	root := citest.ModuleRoot(t)
	profile := filepath.Join(t.TempDir(), coverFile)
	names := func(steps []check.Step) []string {
		var out []string
		for _, s := range steps {
			out = append(out, s.Name)
		}
		return out
	}
	var union []string
	for _, job := range check.Jobs {
		got := names(stepsOf(t.Context(), inJob(job), check.Plan{Root: root, Tools: tools, Profile: profile}))
		require.NotEmpty(t, got, job)
		for _, name := range got {
			assert.NotContains(t, union, name, "%s is in one job", name)
		}
		union = append(union, got...)
	}
	assert.Equal(t, names(allSteps(t.Context(), root, tools, profile)), union)
	assert.Equal(t, []string{"race"}, names(stepsOf(t.Context(), inJob(check.JobRace), check.Plan{Root: root, Tools: tools, Profile: profile})))
}

// TestOnlyRaceAndE2ERunGoTest holds that go test runs once in all, with
// the race detector: no step but race and e2e starts it, and none runs
// the tests without -race (fast and all have no unit step).
func TestOnlyRaceAndE2ERunGoTest(t *testing.T) {
	tools := check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/go/bin"}
	root := citest.ModuleRoot(t)
	profile := filepath.Join(t.TempDir(), coverFile)
	for name, steps := range map[string][]check.Step{
		"fast": fastSteps(t.Context(), root, tools),
		"all":  allSteps(t.Context(), root, tools, profile),
	} {
		var tests []string
		for _, s := range steps {
			if len(s.Argv) > 1 && filepath.Base(s.Argv[0]) == "go" && s.Argv[1] == "test" {
				tests = append(tests, s.Name)
				assert.Contains(t, s.Argv, "-race", "%s: %s runs the tests with the race detector", name, s.Name)
			}
			assert.NotEqual(t, "unit", s.Name, "%s has no unit step", name)
		}
		want := []string{"race"}
		// The e2e step runs go test only on a class of the runner-class
		// table that runs it; elsewhere it is a step not run (10 10.2).
		if name == "all" && slices.Contains(container.Classes[runtime.GOOS+"/"+runtime.GOARCH], "e2e") {
			want = append(want, "e2e")
		}
		assert.Equal(t, want, tests, name)
	}
	_, err := os.Stat(filepath.Join(root, "tools", "ci", "internal", "unit"))
	assert.ErrorIs(t, err, os.ErrNotExist, "the package of the unit step is gone")
}

// TestJobSubcommands runs static, race and e2e in a fixture with stub
// steps. Each prints the platform and the working-tree report that all
// prints, runs the checks of its own job alone, and exits as all does:
// 0, 1 for a failed step, 3 for an infrastructure error alone, 2 for
// an argument.
func TestJobSubcommands(t *testing.T) {
	// ran and absent are the checks a job reports in this fixture: the
	// ones of the commit and the one after the steps.
	jobs := []struct {
		job         string
		ran, absent []string
	}{
		{"static", []string{"ok    hygiene", "ok    workflows", "ok    kits", "ok    schema", "ok    catalog", "ok    reqs", "ok    plan"}, []string{"coverage"}},
		{"race", []string{"ok    coverage"}, []string{"hygiene", "workflows", "kits", "schema", "catalog", "reqs", "plan"}},
		{"e2e", nil, []string{"hygiene", "workflows", "kits", "schema", "catalog", "reqs", "plan", "coverage"}},
	}
	skipped := check.Step{Name: "e2e", Argv: []string{"false"}, Skip: "not run on darwin/arm64: the container e2e runs on the Linux runners"}
	for _, tt := range jobs {
		t.Run(tt.job, func(t *testing.T) {
			r := newSchemaTree(t)
			r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
			r.Commit("fixture")
			profile := filepath.Join(t.TempDir(), "cover.out")
			require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 1"), 0o600))

			code, out := runAllIn(t, r, []check.Step{citest.Passing}, profile)
			require.Equal(t, check.ExitOK, code, out)

			code, out = runJobIn(t, r, tt.job, []check.Step{citest.Passing}, profile)
			assert.Equal(t, check.ExitOK, code, out)
			assert.Contains(t, out, "platform  ")
			assert.Contains(t, out, "ok    working tree")
			assert.Contains(t, out, "ok    architecture")
			assert.Contains(t, out, "ok    passes")
			for _, want := range tt.ran {
				assert.Contains(t, out, want)
			}
			for _, name := range tt.absent {
				assert.NotContains(t, out, name, "%s does not run in the %s job", name, tt.job)
			}

			code, out = runJobIn(t, r, tt.job, []check.Step{citest.Failing, citest.Passing}, profile)
			assert.Equal(t, check.ExitFail, code, out)
			assert.Contains(t, out, "FAIL  fails")
			assert.Contains(t, out, "ok    passes", "a failed step does not hide the next one")
			assert.Contains(t, out, tt.job+": failed: fails\n")

			code, out = runJobIn(t, r, tt.job, []check.Step{infraStep, citest.Passing}, profile)
			assert.Equal(t, check.ExitInfra, code, "an infrastructure error alone exits 3\n%s", out)
			assert.Contains(t, out, "ci: "+tt.job+": e2e: an infrastructure error")

			code, out = runJobIn(t, r, tt.job, []check.Step{citest.Passing}, profile, "extra")
			assert.Equal(t, check.ExitError, code, out)
			assert.Contains(t, out, tt.job+" takes no argument")

			code, out = runJobIn(t, r, tt.job, []check.Step{skipped}, profile)
			assert.Equal(t, check.ExitOK, code, out)
			assert.Contains(t, out, "--    e2e: not run on darwin/arm64", "a step not run on the class prints its line")
		})
	}
}

// TestJobSubcommandCoverageIsTheRaceJobs shows that the race job fails
// on the coverage after its step, and that the static job needs no
// profile.
func TestJobSubcommandCoverageIsTheRaceJobs(t *testing.T) {
	r := newSchemaTree(t)
	r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
	r.Commit("fixture")
	profile := filepath.Join(t.TempDir(), "cover.out")
	require.NoError(t, os.WriteFile(profile, citest.ProfileOf("tools/ci 1 0"), 0o600))

	code, out := runJobIn(t, r, "race", []check.Step{citest.Passing}, profile)
	assert.Equal(t, check.ExitFail, code, out)
	assert.Contains(t, out, "FAIL  coverage (0s)\ntools/ci: coverage: 0.0% of 1 statements covered, below 80% (S9)\n")
	assert.Contains(t, out, "race: failed: coverage\n")

	code, out = runJobIn(t, r, "static", []check.Step{citest.Passing}, filepath.Join(t.TempDir(), "none"))
	assert.Equal(t, check.ExitOK, code, out)
}

// TestJobSubcommandsAreRegistered holds the usage text: all and the
// three jobs, each a subcommand.
func TestJobSubcommandsAreRegistered(t *testing.T) {
	for _, name := range slices.Concat([]string{"all"}, check.Jobs) {
		_, ok := check.Lookup(name)
		assert.True(t, ok, name)
		assert.Contains(t, check.Usage(), "go run ./tools/ci "+name+"\n")
	}
}

// TestRunChecksRefusesAJobWithNoCheck holds that a job that selects no
// check is an error and not a green run: a typo in a Job field, or a
// check moved out of a job, would otherwise leave a job that passes
// having judged nothing.
func TestRunChecksRefusesAJobWithNoCheck(t *testing.T) {
	r := newSchemaTree(t)
	r.Write("go.mod", "module "+citest.FixtureModule+"\n\ngo 1.27.0\n")
	r.Commit("fixture")
	var out bytes.Buffer
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out, Steps: []check.Step{citest.Passing}, Now: frozen}
	ok, err := runChecks(t.Context(), e, nil, "nothing", inJob("nothing"))
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "nothing: no check runs in this job")
	assert.Empty(t, out.String(), "nothing ran")
	for _, job := range check.Jobs {
		assert.NotEmpty(t, registered(inJob(job)), "the %s job has checks", job)
	}
}
