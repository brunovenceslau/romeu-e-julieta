// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
			want:    []string{"FAIL  workflows\n.github/workflows/ci.yml:17: workflows: runs-on is ${{ matrix.os }} or a pinned GitHub-hosted label such as ubuntu-26.04, never *-latest\n", "ok    coverage"},
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
// vulnerabilities, race and license. Each runs in stepEnv; the one
// that reads the Go vulnerability database keeps the variables of a
// proxy too, and the license step those of the Docker daemon, and no
// other step keeps either.
func TestAllSteps(t *testing.T) {
	root := newTree(t).Dir
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	f := newFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.bin, "git")))
	goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	vuln := f.tool(t, execTools["govulncheck"], "1.8.0", "bin", "govulncheck")
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
	require.Len(t, steps, len(fast)+3)
	assert.Equal(t, fast, steps[:len(fast)])

	vulnStep, race, license := steps[len(fast)], steps[len(fast)+1], steps[len(fast)+2]
	assert.Equal(t, step{name: "vulnerabilities", argv: []string{"mise", "exec", "--", "govulncheck", "./..."}, environ: withProcessEnv(env, networkPassThrough)}, vulnStep)
	assert.Equal(t, step{name: "race", argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=/tmp/p/cover.out", "./..."}, environ: env}, race)
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

// TestAllStepsToolOutsideInstalls checks, for each tool all starts
// through "mise exec", that a path mise resolves outside the directory
// where it installs that tool fails its own step, which starts nothing.
func TestAllStepsToolOutsideInstalls(t *testing.T) {
	steps := map[string]string{"govulncheck": "vulnerabilities"}
	require.Len(t, steps, len(execTools), "a step name for each tool of execTools")
	for tool, dir := range execTools {
		t.Run(tool, func(t *testing.T) {
			f := newFakeMise(t)
			goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
			tools := lintTools{linter: f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), goDir: filepath.Dir(goCmd)}
			elsewhere := executable(t, filepath.Join(t.TempDir(), tool))
			require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\necho '"+elsewhere+"'\n"), 0o700))
			root := t.TempDir()
			var got step
			for _, s := range allSteps(t.Context(), root, tools, "/tmp/p/cover.out") {
				if s.name == steps[tool] {
					got = s
				}
			}
			require.ErrorContains(t, got.unavailable, "is outside "+filepath.Join(evalOrSelf(f.installs()), dir)+", where mise installs it")
			_, err := got.run(t.Context(), root)
			require.ErrorIs(t, err, got.unavailable, "an unavailable tool fails its step")
		})
	}
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
		filepath.Join(filepath.Dir(goCmd), "gofmt"):                                          logged("gofmt", ""),
		filepath.Join(f.installs(), "golangci-lint", "2.14.0", "golangci-lint"):              logged("lint", ""),
		filepath.Join(f.installs(), execTools["govulncheck"], "1.8.0", "bin", "govulncheck"): logged("govulncheck", ""),
		filepath.Join(f.bin, "docker"):                                                       logged("docker", ""),
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	}
	which := "if [ \"$3\" = which ]; then\ncase \"$4\" in\n" +
		"go) echo '" + goCmd + "' ;;\n" +
		"golangci-lint) echo '" + filepath.Join(f.installs(), "golangci-lint", "2.14.0", "golangci-lint") + "' ;;\n" +
		"govulncheck) echo '" + filepath.Join(f.installs(), execTools["govulncheck"], "1.8.0", "bin", "govulncheck") + "' ;;\n" +
		"esac\nexit 0\nfi\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\n"+which+"echo \"mise $*\" >> '"+log+"'\n"), 0o700))

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
	want := []string{"gofmt -l", "go vet", "lint run", "lint run", "lint run", "lint run", "go test", "mise exec", "go test"}
	if runtime.GOOS == "linux" {
		want = append(want, "docker run")
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
	require.NoError(t, os.WriteFile(goCmd, []byte("#!/bin/sh\necho \"go $* GOPROXY=${GOPROXY-unset} GOFLAGS=$GOFLAGS HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset}\" >> '"+log+"'\n"), 0o700))
	linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	script := "#!/bin/sh\nif [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\necho \"mise $* HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset}\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid")
	t.Setenv("DOCKER_HOST", "unix:///nowhere")

	code, out := runCI(t, r, nil, nil, "setup")
	assert.Equal(t, exitOK, code, out)
	for _, w := range []string{"ok    mise trust", "ok    mise install", "ok    go mod download"} {
		assert.Contains(t, out, w)
	}
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	root, err := filepath.EvalSymlinks(r.Dir)
	require.NoError(t, err)
	assert.Equal(t, "mise trust "+filepath.Join(root, "mise.toml")+" HTTPS_PROXY=unset DOCKER_HOST=unset\n"+
		"mise install HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset\n"+
		"go mod download GOPROXY=unset GOFLAGS=-mod=readonly HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset\n", string(data))

	t.Run("a failing mise stops before the download", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\nexit 1\n"), 0o700))
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
