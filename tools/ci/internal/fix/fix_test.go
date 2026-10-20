// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package fix

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// TestSteps checks the fix steps: one per lint target, go fix -diff of
// the pinned go over the module with the tags of the tagged suites, in
// the environment of the lint step for that target.
func TestSteps(t *testing.T) {
	steps := Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}})
	require.Len(t, steps, len(lint.Targets))
	for i, target := range lint.Targets {
		s := steps[i]
		assert.Equal(t, "fix "+target.String(), s.Name)
		// Spelled out, not read from lint.BuildTags: this test pins the list.
		assert.Equal(t, []string{"/pinned/bin/go", "fix", "-diff", "-tags", "e2e,host", "./..."}, s.Argv)
		assert.Equal(t, lint.Env(target, "/pinned/bin"), s.Environ)
		require.NotNil(t, s.Around, "%s reads the diff", s.Name)
		env, read, err := s.Around()
		require.NoError(t, err)
		assert.Nil(t, env)
		_, err = read([]byte("--- /src/a.go (old)\n+++ /src/a.go (new)\n"), nil)
		require.Error(t, err)
		assert.Equal(t, fmt.Sprintf("go fix would change the files above for %s; run \"GOOS=%s GOARCH=%s CGO_ENABLED=0 go fix -tags e2e,host ./...\" and commit what it writes",
			target, target.GOOS, target.GOARCH), err.Error())
	}
}

// TestRead holds the verdict of one run: a diff fails with the hint
// whatever the exit status, a long diff is clipped and lists its files,
// and a run without a diff keeps its output and its error.
func TestRead(t *testing.T) {
	diff := "--- /src/a.go (old)\n+++ /src/a.go (new)\n@@ -1 +1 @@\n-x\n+y\n"
	t.Run("a diff fails with the hint, even on exit status 0", func(t *testing.T) {
		out, err := Read([]byte(diff), nil, "the hint")
		require.EqualError(t, err, "the hint")
		assert.Equal(t, diff, string(out))
	})
	t.Run("no diff and no error passes", func(t *testing.T) {
		out, err := Read(nil, nil, "the hint")
		require.NoError(t, err)
		assert.Empty(t, out)
	})
	t.Run("an error without a diff keeps the output and the error", func(t *testing.T) {
		runErr := errors.New("exit status 1")
		out, err := Read([]byte("m.go:1:1: expected 'package'\n"), runErr, "the hint")
		require.ErrorIs(t, err, runErr)
		assert.Equal(t, "m.go:1:1: expected 'package'\n", string(out))
	})
	t.Run("a long diff is clipped and lists its files", func(t *testing.T) {
		var b strings.Builder
		var files []string
		for i := range MaxLines {
			name := fmt.Sprintf("/src/f%03d.go", i)
			files = append(files, name)
			fmt.Fprintf(&b, "--- %s (old)\n+++ %s (new)\n@@ -1 +1 @@\n-x\n+y\n", name, name)
		}
		out, err := Read([]byte(b.String()), errors.New("exit status 1"), "the hint")
		require.EqualError(t, err, "the hint")
		lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		require.Len(t, lines, MaxLines+1+len(files))
		assert.Equal(t, strings.Split(b.String(), "\n")[:MaxLines], lines[:MaxLines])
		assert.Equal(t, fmt.Sprintf("(clipped: %d more lines of the diff; it changes these %d files)", 5*MaxLines-MaxLines, MaxLines), lines[MaxLines])
		assert.Equal(t, files, lines[MaxLines+1:])
	})
	t.Run("a diff of MaxLines lines is not clipped", func(t *testing.T) {
		in := "--- /src/a.go (old)\n+++ /src/a.go (new)\n" + strings.Repeat("+y\n", MaxLines-2)
		out, err := Read([]byte(in), nil, "the hint")
		require.Error(t, err)
		assert.Equal(t, in, string(out))
	})
	t.Run("the files are sorted, whatever order go fix printed them in", func(t *testing.T) {
		a := "--- /src/a.go (old)\n+++ /src/a.go (new)\n@@ -1 +1 @@\n-x\n+y\n"
		b := "--- /src/b.go (old)\n+++ /src/b.go (new)\n@@ -1,2 +1 @@\n--- not a header\n-x\n+y\n"
		out, err := Read([]byte(b+a), errors.New("exit status 1"), "the hint")
		require.EqualError(t, err, "the hint")
		assert.Equal(t, a+b, string(out))
	})
	t.Run("a removed line that starts with -- starts no file", func(t *testing.T) {
		out, err := Read([]byte("--- x\n-y\n"), nil, "the hint")
		require.NoError(t, err)
		assert.Equal(t, "--- x\n-y\n", string(out))
	})
}

// fixable is a function go fix rewrites (the slicescontains fixer of
// the pinned toolchain), and clean one it leaves alone.
const (
	fixable = "func has(xs []string, x string) bool {\n\tfor _, v := range xs {\n\t\tif v == x {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n"
	clean   = "func one() int { return 1 }\n"
)

// TestStepsRunTheRealCommand runs the steps with the pinned go over
// fixture modules: a clean module passes on every target; a change go
// fix would make fails on every target with the diff and the hint, and
// in a file of one GOOS only on the targets of that GOOS; a file of the
// e2e suite is read; and a module that does not build fails with what
// go said and no hint. The last case is the positive control of the
// tags: the same e2e file passes a run without them.
func TestStepsRunTheRealCommand(t *testing.T) {
	tools := citest.PinnedLintTools(t, mise.Path, mise.Environ, mise.ResolveLintTools)
	all := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}
	tests := []struct {
		name  string
		files map[string]string
		fails []string // the targets whose step fails with a diff
		other string   // the output of a step that fails without a diff
	}{
		{"a clean module passes", map[string]string{"m.go": "package m\n\n" + clean}, nil, ""},
		{"a fixable file fails on every target", map[string]string{"m.go": "package m\n\n" + fixable}, all, ""},
		{"a darwin file fails on darwin alone", map[string]string{"m.go": "package m\n\n" + clean, "m_darwin.go": "package m\n\n" + fixable}, []string{"darwin/amd64", "darwin/arm64"}, ""},
		{"a linux file fails on linux alone", map[string]string{"m.go": "package m\n\n" + clean, "m_linux.go": "package m\n\n" + fixable}, []string{"linux/amd64", "linux/arm64"}, ""},
		{"a file of the e2e suite is read", map[string]string{"m.go": "package m\n\n" + clean, "e2e.go": "//go:build e2e\n\npackage m\n\n" + fixable}, all, ""},
		{"a module that does not build fails without the hint", map[string]string{"m.go": "package m\n\nfunc broken() int { return \"x\" }\n"}, nil, "cannot use"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.files["go.mod"] = "module example.invalid/m\n\ngo 1.27.2\n"
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}
			steps := Steps(t.Context(), check.Plan{Root: dir, Tools: tools})
			require.Len(t, steps, len(all))
			for _, s := range steps {
				target := strings.TrimPrefix(s.Name, "fix ")
				out, err := s.Run(t.Context(), dir)
				switch {
				case slices.Contains(tt.fails, target):
					require.Error(t, err, "%s\n%s", s.Name, out)
					assert.Contains(t, err.Error(), "go fix would change the files above for "+target, s.Name)
					assert.Contains(t, string(out), "+\treturn slices.Contains(xs, x)", s.Name)
				case tt.other != "":
					require.Error(t, err, "%s\n%s", s.Name, out)
					assert.NotContains(t, err.Error(), "go fix would change", s.Name)
					assert.Contains(t, string(out), tt.other, s.Name)
				default:
					require.NoError(t, err, "%s\n%s", s.Name, out)
					assert.Empty(t, strings.TrimSpace(string(out)), s.Name)
				}
			}
		})
	}

	t.Run("without the tags the e2e file is not read", func(t *testing.T) {
		dir := t.TempDir()
		for name, content := range map[string]string{
			"go.mod": "module example.invalid/m\n\ngo 1.27.2\n",
			"m.go":   "package m\n\n" + clean,
			"e2e.go": "//go:build e2e\n\npackage m\n\n" + fixable,
		} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
		}
		s := Steps(t.Context(), check.Plan{Root: dir, Tools: tools})[0]
		s.Argv = slices.DeleteFunc(slices.Clone(s.Argv), func(a string) bool { return a == "-tags" || a == lint.BuildTags })
		out, err := s.Run(t.Context(), dir)
		require.NoError(t, err, "%s", out)
	})
}
