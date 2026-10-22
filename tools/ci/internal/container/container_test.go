// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// TestRunnerClasses pins the runner-class table of 10 10.2 (REQ-0812):
// the four runner classes of Runners, and the runner-selected steps
// each one runs. The container e2e runs on Linux alone, native.
func TestRunnerClasses(t *testing.T) {
	assert.Equal(t, map[string][]string{
		"linux/amd64":  {"e2e"},
		"linux/arm64":  {"e2e"},
		"darwin/amd64": nil,
		"darwin/arm64": nil,
	}, Classes)
	assert.Equal(t, []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}, slices.Sorted(maps.Keys(Classes)))
}

// TestStepsSelectFromTheRunner checks that the step is selected from the
// OS and the architecture: the e2e step on each Linux class, with the
// network and the Docker daemon's variables and the reader of its
// events; a line that says why on a class that selects none; and an
// unavailable step on a platform that is no runner class.
func TestStepsSelectFromTheRunner(t *testing.T) {
	for _, key := range append(slices.Clone(check.NetworkPassThrough), check.DockerPassThrough...) {
		t.Setenv(key, "from-the-process")
	}
	p := check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}
	for _, class := range []string{"linux/amd64", "linux/arm64"} {
		goos, goarch, _ := strings.Cut(class, "/")
		steps := stepsFor(goos, goarch, p)
		require.Len(t, steps, 1, class)
		s := steps[0]
		assert.Equal(t, "e2e", s.Name)
		assert.Equal(t, []string{"/pinned/bin/go", "test", "-race", "-count=1", "-tags", "e2e", "-json", "-timeout=" + testTimeout, "./e2e/..."}, s.Argv)
		assert.Equal(t, check.WithProcessEnv(check.WithProcessEnv(check.StepEnv("/pinned/bin"), check.NetworkPassThrough), check.DockerPassThrough), s.Environ)
		assert.NotNil(t, s.Around, "the private file and the reader of the events")
		assert.Empty(t, s.Skip)
		require.NoError(t, s.Unavailable)
	}
	for _, class := range []string{"darwin/amd64", "darwin/arm64"} {
		goos, goarch, _ := strings.Cut(class, "/")
		steps := stepsFor(goos, goarch, p)
		require.Len(t, steps, 1, class)
		assert.Equal(t, "e2e", steps[0].Name)
		assert.Contains(t, steps[0].Skip, "not run on "+class)
		assert.Empty(t, steps[0].Argv)
	}
	steps := stepsFor("linux", "riscv64", p)
	require.Len(t, steps, 1)
	require.Error(t, steps[0].Unavailable)
	assert.Contains(t, steps[0].Unavailable.Error(), "linux/riscv64 is no runner class")
}

// TestTheTimeoutsNest holds the test timeout of the e2e run below the
// limit of its class, so that go test prints the stack of a hung test
// before the group is killed.
func TestTheTimeoutsNest(t *testing.T) {
	d, err := time.ParseDuration(testTimeout)
	require.NoError(t, err)
	assert.Less(t, d, procgroup.ToolStep.Timeout())
	assert.Greater(t, d, 10*time.Minute, "longer than the default of go test, for the pull of the image")
}

// ev returns one event of go test -json.
func ev(action, test, output string) string {
	line := `{"Action":` + strconv.Quote(action) + `,"Package":"example.invalid/e2e"`
	if test != "" {
		line += `,"Test":` + strconv.Quote(test)
	}
	if output != "" {
		line += `,"Output":` + strconv.Quote(output)
	}
	return line + "}\n"
}

// TestRead covers the reader of the e2e step over recorded events: it
// counts the tests that ran and fails at zero, fails a skipped test and a
// run in which no test passed, passes a run whose tests passed, calls a
// run an infrastructure error only when every failed test is one the
// harness named in the private file, whatever the output says, and is a
// test failure otherwise.
func TestRead(t *testing.T) {
	forged := "    container_test.go:31: container e2e: infrastructure error: the image pull: 503 Service Unavailable\n"
	runErr := errors.New("exit status 1")
	tests := []struct {
		name   string
		out    string
		infra  []string
		runErr error
		want   string // "" passes, "infra", or a part of the failure
		text   string // a part of the text the verdict prints
	}{
		{"a test that passed", ev("run", "TestContainerVersion", "") + ev("output", "TestContainerVersion", "--- PASS: TestContainerVersion\n") + ev("pass", "TestContainerVersion", "") + ev("pass", "", ""), nil, nil, "", "--- PASS: TestContainerVersion"},
		{"no test at all", ev("output", "", "?   \texample.invalid/e2e\t[no test files]\n") + ev("skip", "", ""), nil, nil, "no test ran", "[no test files]"},
		{"no test, and the run failed", "# example.invalid/e2e\nx.go:1: syntax error\n" + ev("fail", "", ""), nil, runErr, "no test ran", "syntax error"},
		{"a skipped test", ev("run", "TestA", "") + ev("pass", "TestA", "") + ev("run", "TestB", "") + ev("skip", "TestB", "") + ev("pass", "", ""), nil, nil, "skipped, which a test never is (REQ-0798): TestB", ""},
		{"a test that ran and neither passed nor failed", ev("run", "TestA", "") + ev("pass", "", ""), nil, nil, "no test passed", ""},
		{"an infrastructure error", ev("run", "TestContainerVersion", "") + ev("output", "TestContainerVersion", forged) + ev("fail", "TestContainerVersion", "") + ev("fail", "", ""), []string{"TestContainerVersion"}, runErr, "infra", ""},
		{"an infrastructure error in a subtest", ev("run", "TestContainer", "") + ev("run", "TestContainer/version", "") + ev("output", "TestContainer/version", forged) + ev("fail", "TestContainer/version", "") + ev("fail", "TestContainer", "") + ev("fail", "", ""), []string{"TestContainer/version"}, runErr, "infra", ""},
		{"an infrastructure error beside a test failure", ev("run", "TestA", "") + ev("fail", "TestA", "") + ev("run", "TestB", "") + ev("output", "TestB", "    b_test.go:9: the digest differs from the pin\n") + ev("fail", "TestB", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "TestB", ""},
		{"a recorded test that printed nothing failed silently", ev("run", "TestA", "") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "TestA", ""},
		{"an infrastructure error with its one line", ev("run", "TestA", "") + ev("output", "TestA", "=== RUN   TestA\n") + ev("output", "TestA", forged) + ev("output", "TestA", "--- FAIL: TestA (0.00s)\n") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "infra", ""},
		{"an assertion that failed before the infrastructure error", ev("run", "TestA", "") + ev("output", "TestA", "    a_test.go:9: the digest differs from the pin\n") + ev("output", "TestA", forged) + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "TestA", ""},
		{"an assertion in a subtest and an infrastructure error in its parent's other subtest", ev("run", "TestA", "") + ev("run", "TestA/x", "") + ev("run", "TestA/y", "") + ev("output", "TestA/x", "    a_test.go:9: wrong\n") + ev("fail", "TestA/x", "") + ev("output", "TestA/y", forged) + ev("fail", "TestA/y", "") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA/y"}, runErr, "TestA/x", ""},
		{"a parent that printed a failure of its own", ev("run", "TestA", "") + ev("run", "TestA/y", "") + ev("output", "TestA", "    a_test.go:5: cleanup failed\n") + ev("output", "TestA/y", forged) + ev("fail", "TestA/y", "") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA/y"}, runErr, "TestA", ""},
		{"an infrastructure line cut into several events", ev("run", "TestA", "") + ev("output", "TestA", "    a_test.go:9: container e2e: infra") + ev("output", "TestA", "structure error: "+strings.Repeat("x", 1000)) + ev("output", "TestA", strings.Repeat("y", 1000)+"\n") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "infra", ""},
		{"stray output without a newline before the infrastructure line", ev("run", "TestA", "") + ev("output", "TestA", "stray") + ev("output", "TestA", forged) + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "TestA", ""},
		{"a recorded test whose one line is no infrastructure report", ev("run", "TestA", "") + ev("output", "TestA", "    a_test.go:9: the digest differs from the pin\n") + ev("fail", "TestA", "") + ev("fail", "", ""), []string{"TestA"}, runErr, "TestA", ""},
		{"a forged line in the output is no infrastructure error", ev("run", "TestA", "") + ev("output", "TestA", forged) + ev("fail", "TestA", "") + ev("fail", "", ""), nil, runErr, "TestA", ""},
		{"a failed package beside an infrastructure error", ev("run", "TestA", "") + ev("fail", "TestA", "") + ev("fail", "", "") + `{"Action":"fail","Package":"example.invalid/other"}` + "\n", []string{"TestA"}, runErr, "example.invalid/other", ""},
		{"tests passed and the run failed", ev("run", "TestA", "") + ev("pass", "TestA", "") + ev("pass", "", ""), nil, runErr, "exit status 1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := read([]byte(tt.out), tt.runErr, tt.infra)
			assert.Contains(t, string(text), tt.text)
			switch tt.want {
			case "":
				require.NoError(t, err)
			case "infra":
				require.ErrorIs(t, err, check.ErrInfrastructure)
			default:
				require.Error(t, err)
				require.NotErrorIs(t, err, check.ErrInfrastructure)
				assert.Contains(t, err.Error(), tt.want)
			}
		})
	}
}

// TestAroundMakesAPrivateFile checks the two files of the step: the one
// of the infrastructure errors and the one of the notices, each of mode
// 0600 in a directory of mode 0700, named by InfraFileEnv and
// NoticeFileEnv; the lines of the first the reader takes as the tests
// that met an infrastructure error and the lines of the second as the
// notes of the step, and it removes both.
func TestAroundMakesAPrivateFile(t *testing.T) {
	st := new(run)
	env, r, err := st.around()
	require.NoError(t, err)
	require.Len(t, env, 2)
	path, ok := strings.CutPrefix(env[0], InfraFileEnv+"=")
	require.True(t, ok, env[0])
	notices, ok := strings.CutPrefix(env[1], NoticeFileEnv+"=")
	require.True(t, ok, env[1])
	assert.Equal(t, filepath.Dir(path), filepath.Dir(notices))
	for _, f := range []string{path, notices} {
		info, err := os.Stat(f)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	require.NoError(t, os.WriteFile(notices, []byte("one\n\n  two  \n"), 0o600))
	dir, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dir.Mode().Perm())
	require.NoError(t, os.WriteFile(path, []byte("TestContainerVersion\n"), 0o600))
	_, err = r([]byte(ev("run", "TestContainerVersion", "")+ev("output", "TestContainerVersion", "    c_test.go:3: "+InfraPrefix+"x\n")+ev("fail", "TestContainerVersion", "")+ev("fail", "", "")), errors.New("exit status 1"))
	require.ErrorIs(t, err, check.ErrInfrastructure)
	assert.Equal(t, []string{"one", "two"}, st.notes())
	assert.NoDirExists(t, filepath.Dir(path), "the reader removes the directory")
}

// TestCacheKey checks the key of the cache step: the runner's OS and
// architecture, then one hash that changes with the fixture lock and
// with each herdr and mise pin, and not with another pin.
func TestCacheKey(t *testing.T) {
	sum := strings.Repeat("a", 64)
	p := pins.Pins{
		Workload: &pins.Workload{Ref: "x"},
		Herdr:    &pins.Release{Version: "1.0.0", SHA256: pins.Arches{Amd64: sum, Arm64: sum}},
		Mise:     &pins.Release{Version: "2.0.0", SHA256: pins.Arches{Amd64: sum, Arm64: sum}},
	}
	key := CacheKey([]byte("lock"), p)
	assert.Regexp(t, `^container-e2e-\$\{\{ runner\.os \}\}-\$\{\{ runner\.arch \}\}-[0-9a-f]{64}$`, key)
	assert.NotEqual(t, key, CacheKey([]byte("lock2"), p), "the lock")
	other := p
	other.Mise = &pins.Release{Version: "2.0.1", SHA256: p.Mise.SHA256}
	assert.NotEqual(t, key, CacheKey([]byte("lock"), other), "the mise pin")
	other = p
	other.Herdr = &pins.Release{Version: "1.0.0", SHA256: pins.Arches{Amd64: sum, Arm64: strings.Repeat("b", 64)}}
	assert.NotEqual(t, key, CacheKey([]byte("lock"), other), "the herdr pin")
	other = p
	other.Workload = &pins.Workload{Ref: "y"}
	assert.Equal(t, key, CacheKey([]byte("lock"), other), "the workload pin is no input: the image is not cached")
}

// fixtureModule is a module of two packages, for TestReadOverRealRuns:
// one with no test and one whose test passes.
var fixtureModule = map[string]string{
	"go.mod":                  "module example.invalid/fixture\n\ngo 1.27\n",
	"notests/notests.go":      "// Package notests has no test.\npackage notests\n\n// Answer is a declaration.\nconst Answer = 42\n",
	"onetest/onetest_test.go": "package onetest\n\nimport \"testing\"\n\nfunc TestPasses(t *testing.T) {}\n",
}

// TestReadOverRealRuns runs the e2e step, its reader included, over go
// test -json of the fixture module: the package with no test turns the
// step red, and the one whose test passes keeps it green.
func TestReadOverRealRuns(t *testing.T) {
	dir := t.TempDir()
	for name, content := range fixtureModule {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	goDir := goDirOf(t)
	for pkg, want := range map[string]string{"notests": "no test ran", "onetest": ""} {
		t.Run(pkg, func(t *testing.T) {
			s := stepsFor("linux", "amd64", check.Plan{Root: dir, Tools: check.Tools{GoDir: goDir}})[0]
			s.Argv = []string{filepath.Join(goDir, "go"), "test", "-json", "-count=1", "./" + pkg}
			_, err := s.Run(t.Context(), dir)
			if want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), want)
		})
	}
}

// goDirOf returns the directory of the go command of this test's PATH,
// the pinned one in the unit step.
func goDirOf(t *testing.T) string {
	t.Helper()
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if info, err := os.Stat(filepath.Join(dir, "go")); err == nil && !info.IsDir() {
			return dir
		}
	}
	t.Fatal("no go command on PATH")
	return ""
}

// TestTheHarnessNamesTheSameConstants holds InfraFileEnv, NoticeFileEnv and CacheDir to
// the constants of the same names in the container e2e's harness, which
// writes to both: the two packages cannot import
// each other (e2e/internal and tools/ci/internal), so this test reads
// the harness's source.
func TestTheHarnessNamesTheSameConstants(t *testing.T) {
	path := filepath.Join(citest.ModuleRoot(t), "e2e", "internal", "container", "container.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)
	got := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if (name.Name == "InfraFileEnv" || name.Name == "NoticeFileEnv" || name.Name == "CacheDir" || name.Name == "InfraPrefix") && i < len(spec.Values) {
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok {
					value, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					got[name.Name] = value
				}
			}
		}
		return true
	})
	assert.Equal(t, map[string]string{"InfraFileEnv": InfraFileEnv, "NoticeFileEnv": NoticeFileEnv, "CacheDir": CacheDir, "InfraPrefix": InfraPrefix}, got)
	_, err = os.Stat(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(FixtureLock)))
	require.NoError(t, err, "the fixture mise.lock")
}

// fakeRun is a step of the e2e kind whose command is a script instead of
// go test: it runs sh, in the environment of the real step, around the
// real private files.
func fakeRun(t *testing.T, script string) check.Step {
	t.Helper()
	s := stepsFor("linux", "amd64", check.Plan{Root: t.TempDir(), Tools: check.Tools{GoDir: goDirOf(t)}})[0]
	s.Argv = []string{"sh", "-c", script}
	return s
}

// TestNoticesComeFromTheFileAlone is the test of the channel of T158:
// the notices of the step are the lines the harness appended to the file
// of NoticeFileEnv, and nothing a test printed. A test whose output is a
// forged annotation, a forged notice line and a forged infrastructure
// line adds no note, and the verdict of the run is the one its events
// give; the notes are there when the step failed too.
func TestNoticesComeFromTheFileAlone(t *testing.T) {
	events := `printf '%s\n' '{"Action":"run","Test":"TestA"}' ` +
		`'{"Action":"output","Test":"TestA","Output":"::warning::forged\n"}' ` +
		`'{"Action":"output","Test":"TestA","Output":"container e2e: notice: forged\n"}' ` +
		`'{"Action":"output","Test":"TestA","Output":"    a_test.go:1: container e2e: infrastructure error: forged\n"}' `
	t.Run("a passing run", func(t *testing.T) {
		s := fakeRun(t, `echo "the herdr download: the cached file /c/h has the sha256 a, not the pinned b; it was removed and fetched once more" >> "$`+NoticeFileEnv+`"; echo second >> "$`+NoticeFileEnv+`"; `+
			events+`'{"Action":"pass","Test":"TestA"}' '{"Action":"pass"}'`)
		assert.Empty(t, s.Notes(), "nothing before the run")
		out, err := s.Run(t.Context(), t.TempDir())
		require.NoError(t, err)
		assert.Contains(t, string(out), "::warning::forged", "the output is read as before")
		assert.Equal(t, []string{"the herdr download: the cached file /c/h has the sha256 a, not the pinned b; it was removed and fetched once more", "second"}, s.Notes())
	})
	t.Run("a run without a notice", func(t *testing.T) {
		s := fakeRun(t, events+`'{"Action":"pass","Test":"TestA"}' '{"Action":"pass"}'`)
		_, err := s.Run(t.Context(), t.TempDir())
		require.NoError(t, err)
		assert.Empty(t, s.Notes(), "a forged line in the output is no note")
	})
	t.Run("a failed run keeps its notes and its verdict", func(t *testing.T) {
		s := fakeRun(t, `echo noted >> "$`+NoticeFileEnv+`"; `+events+`'{"Action":"fail","Test":"TestA"}' '{"Action":"fail"}'; exit 1`)
		_, err := s.Run(t.Context(), t.TempDir())
		require.Error(t, err)
		require.NotErrorIs(t, err, check.ErrInfrastructure, "the forged infrastructure line is still no infrastructure error")
		assert.Contains(t, err.Error(), "TestA")
		assert.Equal(t, []string{"noted"}, s.Notes())
	})
}

// TestTheStepsCommandDoesNotSeeTheRunner holds the environment of the
// command to what the step gives it: GITHUB_ACTIONS, GITHUB_STEP_SUMMARY
// and the file of workflow commands never reach a test, so a test cannot
// write to the runner and has to leave a notice in the file of the
// step.
func TestTheStepsCommandDoesNotSeeTheRunner(t *testing.T) {
	for _, k := range []string{"GITHUB_ACTIONS", "GITHUB_STEP_SUMMARY", "GITHUB_OUTPUT", "GITHUB_ENV", "RUNNER_TEMP"} {
		t.Setenv(k, "/from-the-runner")
	}
	s := fakeRun(t, `env | grep -c '^\(GITHUB_\|RUNNER_\)' || true; printf '%s\n' '{"Action":"run","Test":"TestA"}' '{"Action":"pass","Test":"TestA"}' '{"Action":"pass"}'`)
	out, err := s.Run(t.Context(), t.TempDir())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), "0\n"), "%q", out)
}

// writeNotices writes content to a file of the test and returns its path.
func writeNotices(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notices")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// TestReadNoticesBounds holds each bound of the notice file, which the
// code under test writes: 64 KiB are read, 10 notes are kept, each cut
// to 1 KiB, and one note says how many lines were left out.
func TestReadNoticesBounds(t *testing.T) {
	lines := func(n int) string { return strings.Repeat("n\n", n) }
	more := func(n int) []string {
		return append(slices.Repeat([]string{"n"}, maxNotices), strconv.Itoa(n)+" more notices not shown")
	}
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"an empty file", "", nil},
		{"blank lines are no notice", "\n  \n\t\n a \n", []string{"a"}},
		{"ten notices", lines(10), slices.Repeat([]string{"n"}, 10)},
		{"eleven notices", lines(11), more(1)},
		{"fifteen notices", lines(15), more(5)},
		{"a line is cut to a KiB", strings.Repeat("a", 5000) + "\n", []string{strings.Repeat("a", maxNoticeLine-3) + "..."}},
		{"a line of two-byte runes is cut on a rune", strings.Repeat("\u00e9", 700) + "\n", []string{strings.Repeat("\u00e9", (maxNoticeLine-3)/2) + "..."}},
		{"an invalid byte early in a cut line stays", "\xff" + strings.Repeat("a", 5000) + "\n", []string{"\xff" + strings.Repeat("a", maxNoticeLine-4) + "..."}},
		{"a file past 64 KiB is read to 64 KiB", "a\nb\nc\n" + strings.Repeat("x", 3*maxNoticeBytes), []string{"a", "b", "c", "1 more notices not shown"}},
		{"many short lines past 64 KiB", strings.Repeat("x\n", maxNoticeBytes), append(slices.Repeat([]string{"x"}, maxNotices), strconv.Itoa(maxNoticeBytes/2-maxNotices+1)+" more notices not shown")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readNotices(writeNotices(t, tt.content))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			for _, n := range got {
				assert.LessOrEqual(t, len(n), maxNoticeLine)
				assert.True(t, utf8.ValidString(n) || strings.HasPrefix(n, "\xff"))
			}
		})
	}
}

// TestReadNoticesReadsAtMost64KiB holds the read, and not only the
// result, to the limit: a sparse file of half a gigabyte makes the
// reader allocate a few buffers of 64 KiB, not the file.
func TestReadNoticesReadsAtMost64KiB(t *testing.T) {
	path := writeNotices(t, "")
	require.NoError(t, os.Truncate(path, 512<<20))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := readNotices(path)
	runtime.ReadMemStats(&after)
	require.NoError(t, err)
	assert.Equal(t, []string{"1 more notices not shown"}, got, "NUL bytes with no line break are one line")
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(8<<20))
}

// TestReadNoticesRefusesWhatIsNotARegularFile holds the notice file to a
// regular file of the step: the code under test could replace it with a
// link to another file or a FIFO that blocks the reader. It is a plain
// error and no infrastructure error, since an infrastructure error is a
// precondition the environment could not meet and this is the code
// under test acting on the step's files.
func TestReadNoticesRefusesWhatIsNotARegularFile(t *testing.T) {
	dir := t.TempDir()
	target := writeNotices(t, "a\n")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	for name, path := range map[string]string{"a symlink to a regular file": link, "a directory": dir, "a missing file": filepath.Join(dir, "missing")} {
		t.Run(name, func(t *testing.T) {
			_, err := readNotices(path)
			require.Error(t, err)
			assert.NotErrorIs(t, err, check.ErrInfrastructure)
		})
	}
	_, err := readNotices(link)
	assert.ErrorContains(t, err, "not a regular file")
}

// TestAStepWhoseNoticeFileWasReplacedFails runs the step over a command
// that replaces the notice file with a symlink: the step fails, with
// no note.
func TestAStepWhoseNoticeFileWasReplacedFails(t *testing.T) {
	s := fakeRun(t, `rm "$`+NoticeFileEnv+`"; ln -s /dev/null "$`+NoticeFileEnv+`"; `+
		`printf '%s\n' '{"Action":"run","Test":"TestA"}' '{"Action":"pass","Test":"TestA"}' '{"Action":"pass"}'`)
	_, err := s.Run(t.Context(), t.TempDir())
	require.Error(t, err)
	assert.NotErrorIs(t, err, check.ErrInfrastructure)
	assert.Contains(t, err.Error(), "not a regular file")
	assert.Empty(t, s.Notes())
}

// TestAroundPathsAreAbsolute holds the files of the step to absolute
// paths: the tests run in the directory of their package, so a path
// made below a relative TMPDIR would name another file there.
func TestAroundPathsAreAbsolute(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("tmp", 0o700))
	t.Setenv("TMPDIR", "tmp")
	env, _, err := new(run).around()
	require.NoError(t, err)
	require.Len(t, env, 2)
	for _, kv := range env {
		_, path, _ := strings.Cut(kv, "=")
		assert.True(t, filepath.IsAbs(path), "%s", kv)
	}
}
