// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package gittest builds fixture repositories for tests. A fixture runs
// git with an environment of its own, so it reads no configuration of
// the machine and is not steered by the variables git sets for a hook:
// the unit tests run inside the pre-push hook too.
package gittest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Repo is a fixture repository in a temporary directory.
type Repo struct {
	git.Repo
	t testing.TB
}

// Env returns the environment a fixture runs git with. The identities
// and dates are fixed, so the same steps give the same commit ids.
func Env(t testing.TB) []string {
	t.Helper()
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=author@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_NAME=Test Committer",
		"GIT_COMMITTER_EMAIL=committer@example.invalid",
		"GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
	}
}

// New creates an empty repository whose first branch is main.
func New(t testing.TB) *Repo {
	t.Helper()
	r := &Repo{Dir: t.TempDir(), Env: Env(t), t: t}
	r.Git("init", "--quiet", "--initial-branch=main")
	return r
}

// NewBare creates an empty bare repository, to push to.
func NewBare(t testing.TB) *Repo {
	t.Helper()
	r := &Repo{Dir: t.TempDir(), Env: Env(t), t: t}
	r.Git("init", "--quiet", "--bare", "--initial-branch=main")
	return r
}

// WithEnv returns the same repository with more variables set, for a
// commit under another identity.
func (r *Repo) WithEnv(vars ...string) *Repo {
	env := append(append([]string{}, r.Env...), vars...)
	return &Repo{Dir: r.Dir, Env: env, t: r.t}
}

// For returns the same repository bound to t, so that a subtest's
// failure stops the subtest and not the test that made the repository.
func (r *Repo) For(t testing.TB) *Repo {
	return &Repo{Dir: r.Dir, Env: r.Env, t: t}
}

// Git runs one git command, fails the test when it fails, and returns
// its output without the final newline.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	// Not t.Context(): that context ends before the test's cleanups
	// run, and a cleanup may still call git.
	out, err := r.Run(context.Background(), nil, args...)
	require.NoError(r.t, err, "fixture")
	return strings.TrimSuffix(string(out), "\n")
}

// WriteObject writes a raw object of the given type and returns its id.
// Git checks it as it checks what it writes itself. It makes a fixture
// that git's porcelain would need a signing key or a crafted history to
// make, such as a merge commit that embeds a signed tag.
func (r *Repo) WriteObject(kind, raw string) string {
	r.t.Helper()
	// Not t.Context(): that context ends before the test's cleanups
	// run, and a cleanup may still call git.
	out, err := r.Run(context.Background(), []byte(raw), "hash-object", "-t", kind, "-w", "--stdin")
	require.NoError(r.t, err, "fixture")
	return strings.TrimSpace(string(out))
}

// Write writes one file of the working tree, with its directories.
func (r *Repo) Write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.Dir, filepath.FromSlash(path))
	require.NoError(r.t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(r.t, os.WriteFile(full, []byte(content), 0o644))
}

// Commit stages every change, commits it, and returns the commit id. A
// commit with no change is allowed, for a message-only fixture.
func (r *Repo) Commit(message string) string {
	r.t.Helper()
	r.Git("add", "--all")
	r.Git("commit", "--quiet", "--allow-empty", "--message", message)
	return r.Git("rev-parse", "HEAD")
}
