// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package egress_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// gitIn runs git in dir, without gitsafe, to build the fixture, and
// returns its standard output, trimmed.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.JulietaGit,
		Argv:  append([]string{git, "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:   dir,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// commitFiles makes a repository whose one commit holds the files, and
// returns the directory and the commit.
func commitFiles(t *testing.T, files map[string]string) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	gitIn(t, dir, "init", "-q")
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "one")
	return dir, gitIn(t, dir, "rev-parse", "HEAD")
}

// runner returns the gitsafe runner of the host.
func runner(t *testing.T) *gitsafe.Runner {
	t.Helper()
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	r, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: git, Home: t.TempDir()})
	require.NoError(t, err)
	return r
}

// TestDeriveReadsTheCommit: the lock is read with git cat-file at the
// egress commit, never from the work tree, and no mise process runs
// (07 7.1, I9): the work tree changes after the commit, and the
// result is the commit's.
func TestDeriveReadsTheCommit(t *testing.T) {
	dir, commit := commitFiles(t, map[string]string{
		"mise.toml": toml,
		"mise.lock": lockOf([3]string{"go", "core:go", "https://mirror.acme.dev/go.tar.gz"}),
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.lock"), []byte(lockOf([3]string{"go", "core:go", "https://evil.example/go.tar.gz"})), 0o600))
	require.NoError(t, os.Remove(filepath.Join(dir, "mise.toml")))
	reader := egress.GitReader(runner(t), dir, commit)
	t.Setenv("PATH", "") // a mise on the PATH could not be started either

	got, err := egress.Derive(t.Context(), egress.Input{
		Repos:   []egress.Repo{{Dir: "api", URL: "u", Read: reader}},
		Catalog: testCatalog(t),
	})
	require.NoError(t, err)
	assert.Contains(t, got.Gated, gate("mirror.acme.dev", egress.ReasonLockHost))
	for _, g := range got.Gated {
		assert.NotEqual(t, "evil.example", g.Domain)
	}
}

// TestDeriveGitStates: a commit with neither file contributes nothing, a
// commit with a mise.toml and no lock is lock-missing, and a lock over
// 1 MiB is refused by the shared file cap as lock-unparsed.
func TestDeriveGitStates(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"nothing", map[string]string{"README.md": "x"}, ""},
		{"no lock", map[string]string{"mise.toml": toml}, "RJ-340"},
		{"a lock over the cap", map[string]string{"mise.toml": toml, "mise.lock": "# " + strings.Repeat("x", egress.MaxFileSize) + "\n"}, "RJ-325"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			dir, commit := commitFiles(t, tt.files)
			_, err := egress.Derive(t.Context(), egress.Input{
				Repos:   []egress.Repo{{Dir: "api", URL: "u", Read: egress.GitReader(runner(t), dir, commit)}},
				Catalog: testCatalog(t),
			})
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			assert.Equal(t, tt.want, rowOf(t, err).ID)
		})
	}
}

// TestGitReaderFailure: a commit the repository does not hold is an
// error that is not lock-unparsed, and a refused file is ErrUnreadable.
func TestGitReaderFailure(t *testing.T) {
	dir, _ := commitFiles(t, map[string]string{"mise.lock": "x"})
	_, _, err := egress.GitReader(runner(t), dir, strings.Repeat("a", 40))(t.Context(), "mise.lock")
	require.Error(t, err)
	require.NotErrorIs(t, err, egress.ErrUnreadable)
	_, _, err = egress.GitReader(runner(t), dir, "HEAD")(t.Context(), "mise.lock")
	assert.ErrorIs(t, err, egress.ErrUnreadable, "a name of a commit is not a full object id")
}

// TestNoProcessIsStarted: no file of the package imports os/exec,
// procgroup or internal/mise, so it starts no mise (07 7.1, I9); its
// only way to a process is the gitsafe runner it is given.
func TestNoProcessIsStarted(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			assert.NotEqual(t, "os/exec", path, name)
			assert.False(t, strings.HasSuffix(path, "/internal/procgroup"), name)
			assert.False(t, strings.HasSuffix(path, "/internal/mise"), name)
		}
	}
}
