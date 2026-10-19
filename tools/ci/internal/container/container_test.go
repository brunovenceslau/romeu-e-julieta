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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
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
		assert.Equal(t, []string{"/pinned/bin/go", "test", "-json", "-race", "-count=1", "-timeout=" + testTimeout, "-tags", "e2e", "./e2e/..."}, s.Argv)
		assert.Equal(t, check.WithProcessEnv(check.WithProcessEnv(check.StepEnv("/pinned/bin"), check.NetworkPassThrough), check.DockerPassThrough), s.Environ)
		assert.NotNil(t, s.Read, "the reader of the events")
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
// counts the tests that ran and fails at zero, passes a run whose tests
// passed, calls a run an infrastructure error only when every failed
// test failed on a precondition, and is a test failure otherwise.
func TestRead(t *testing.T) {
	infraLine := "    container_test.go:31: " + InfraPrefix + "the image pull: 503 Service Unavailable\n"
	runErr := errors.New("exit status 1")
	tests := []struct {
		name   string
		out    string
		runErr error
		want   string // "" passes, "infra", or a part of the failure
		text   string // a part of the text the verdict prints
	}{
		{"a test that passed", ev("run", "TestContainerVersion", "") + ev("output", "TestContainerVersion", "--- PASS: TestContainerVersion\n") + ev("pass", "TestContainerVersion", "") + ev("pass", "", ""), nil, "", "--- PASS: TestContainerVersion"},
		{"no test at all", ev("output", "", "?   \texample.invalid/e2e\t[no test files]\n") + ev("skip", "", ""), nil, "no test ran", "[no test files]"},
		{"no test, and the run failed", "# example.invalid/e2e\nx.go:1: syntax error\n" + ev("fail", "", ""), runErr, "no test ran", "syntax error"},
		{"an infrastructure error", ev("run", "TestContainerVersion", "") + ev("output", "TestContainerVersion", infraLine) + ev("fail", "TestContainerVersion", "") + ev("fail", "", ""), runErr, "infra", InfraPrefix},
		{"an infrastructure error in a subtest", ev("run", "TestContainer", "") + ev("run", "TestContainer/version", "") + ev("output", "TestContainer/version", infraLine) + ev("fail", "TestContainer/version", "") + ev("fail", "TestContainer", "") + ev("fail", "", ""), runErr, "infra", ""},
		{"an infrastructure error beside a test failure", ev("run", "TestA", "") + ev("output", "TestA", infraLine) + ev("fail", "TestA", "") + ev("run", "TestB", "") + ev("output", "TestB", "    b_test.go:9: the digest differs from the pin\n") + ev("fail", "TestB", "") + ev("fail", "", ""), runErr, "TestB", ""},
		{"the marker inside a line is no infrastructure error", ev("run", "TestA", "") + ev("output", "TestA", "    a_test.go:9: the registry said "+InfraPrefix+"x\n") + ev("fail", "TestA", "") + ev("fail", "", ""), runErr, "TestA", ""},
		{"a failed package beside an infrastructure error", ev("run", "TestA", "") + ev("output", "TestA", infraLine) + ev("fail", "TestA", "") + ev("fail", "", "") + `{"Action":"fail","Package":"example.invalid/other"}` + "\n", runErr, "example.invalid/other", ""},
		{"tests passed and the run failed", ev("run", "TestA", "") + ev("pass", "TestA", "") + ev("pass", "", ""), runErr, "exit status 1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := Read([]byte(tt.out), tt.runErr)
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

// TestTheHarnessNamesTheSamePrefix holds InfraPrefix to the constant of
// the same name in the container e2e's harness, which prints it: the two
// packages cannot import each other (e2e/internal and tools/ci/internal),
// so this test reads the harness's source.
func TestTheHarnessNamesTheSamePrefix(t *testing.T) {
	path := filepath.Join(citest.ModuleRoot(t), "e2e", "internal", "container", "container.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name == "InfraPrefix" && i < len(spec.Values) {
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok {
					value, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					got = append(got, value)
				}
			}
		}
		return true
	})
	assert.Equal(t, []string{InfraPrefix}, got)
}
