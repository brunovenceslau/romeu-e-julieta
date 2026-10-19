// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// commitTime is the committer date of the first commit of a fixture:
// far from now, so a build date taken from the clock cannot match it.
var commitTime = time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)

// markerJulieta is a string only the fixture julieta holds, which the
// fixture romeu must carry after the build embeds it.
const markerJulieta = "julieta-fixture-marker-7f3a91"

// write writes the files, name to content, under dir.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

// commit stages everything under dir and commits it, unsigned, with the
// committer date at when. The repository is a throwaway fixture; the
// signing rule is of the real repository.
func commit(t *testing.T, dir, message string, when time.Time) string {
	t.Helper()
	ctx := t.Context()
	date := when.Format(time.RFC3339)
	env := append(gitEnv(), "GIT_COMMITTER_DATE="+date, "GIT_AUTHOR_DATE="+date,
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	for _, args := range [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-q", "--no-verify", "--allow-empty", "-m", message},
	} {
		_, err := output(ctx, procgroup.ToolGit, dir, env, args)
		require.NoError(t, err)
	}
	out, err := git(ctx, dir, "rev-parse", "HEAD")
	require.NoError(t, err)
	return string(out[:len(out)-1])
}

// initRepo makes a repository on main under a new directory.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := output(t.Context(), procgroup.ToolGit, dir, append(gitEnv(), "GIT_CONFIG_GLOBAL=/dev/null"),
		[]string{"git", "init", "-q", "-b", "main"})
	require.NoError(t, err)
	return dir
}

// fixtureModule is a module with the two commands of the repository, one
// dependency with a license by a replace directive, and the files a
// build reads. The fixture romeu embeds internal/embedded/julieta, which
// no file of the tree holds: only the overlay of a build does.
func fixtureModule(t *testing.T) string {
	t.Helper()
	dep := t.TempDir()
	write(t, dep, map[string]string{
		"go.mod":  "module example.com/dep\n\ngo 1.27\n",
		"dep.go":  "package dep\n\n// Name is the dependency's name.\nconst Name = \"dep\"\n",
		"LICENSE": "The dep license: use it freely.\n",
	})
	root := initRepo(t)
	write(t, root, map[string]string{
		"go.mod":              "module example.com/fixture\n\ngo 1.27\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => " + dep + "\n",
		"VERSION":             "1.0.0\n",
		"COPYING":             "The fixture's own license.\n",
		"cmd/julieta/main.go": "package main\n\nimport _ \"example.com/dep\"\n\nvar version = \"devel\"\n\nfunc main() { println(version, \"" + markerJulieta + "\") }\n",
		"cmd/romeu/main.go": "package main\n\nimport (\n\t\"example.com/dep\"\n\t\"example.com/fixture/internal/embedded\"\n)\n\n" +
			"var version, commit, date = \"devel\", \"unknown\", \"unknown\"\n\n" +
			"func main() { println(dep.Name, version, commit, date, len(embedded.Julieta)) }\n",
		"internal/embedded/embedded.go": "// Package embedded holds the julieta binary.\npackage embedded\n\nimport _ \"embed\"\n\n// Julieta is the embedded binary.\n//\n//go:embed julieta\nvar Julieta []byte\n",
	})
	commit(t, root, "fixture", commitTime)
	return root
}

// members reads an archive back: the headers in order and the content of
// each file.
func members(t *testing.T, data []byte) ([]*tar.Header, map[string][]byte) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	var headers []*tar.Header
	content := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		headers = append(headers, h)
		content[h.Name] = body
	}
	return headers, content
}
