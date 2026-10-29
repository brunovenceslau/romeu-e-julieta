// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// pushRecorder records the commands of r as recorder does, and answers
// the read of the pushInsteadOf keys with exit 1, a config without one,
// unless rewrites is set.
func pushRecorder(r *Runner, rewrites bool) *recorder {
	var rec recorder
	rec.install(r)
	start := r.start
	r.start = func(ctx context.Context, c procgroup.Command) error {
		err := start(ctx, c)
		if slices.Contains(c.Argv, "--get-regexp") && slices.Contains(c.Argv, `^url\..*\.pushinsteadof$`) && !rewrites {
			return &procgroup.ExitError{Argv0: c.Argv[0], Code: 1}
		}
		return err
	}
	return &rec
}

// TestPushArgv pins the argv and the environment of the one push (I18,
// REQ-0502): the flags and environment of host mode, the memory remote
// as given, and refs/heads/main to refs/heads/main, never forced, never
// more than that ref.
//
//romeu:invariant I18 test
func TestPushArgv(t *testing.T) {
	home := t.TempDir()
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: home})
	require.NoError(t, err)
	rec := pushRecorder(r, false)
	const url = "https://github.com/example/shop-memory"
	require.NoError(t, r.Push(t.Context(), stubRepo, url))
	require.Len(t, rec.commands, 3, "resolve the URL, read the push rewrites, push")
	push := rec.commands[2]
	assert.Equal(t, procgroup.HostGitFetch, push.Class)
	assert.Equal(t, stubRepo, push.Dir)
	assert.Equal(t, []string{
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_ATTR_NOSYSTEM=1", "GIT_NO_LAZY_FETCH=1", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
	}, push.Env, "host mode's environment and nothing else: no key, no socket, no credential (I2)")
	want := slices.Concat([]string{"/usr/bin/git"}, hostFlags,
		[]string{"push", "--quiet", "--no-verify", "--no-signed", "--recurse-submodules=no", "--no-follow-tags", url, "refs/heads/main:refs/heads/main"})
	assert.Equal(t, want, push.Argv)
	for _, a := range push.Argv[1+len(hostFlags):] {
		assert.NotContains(t, []string{"--force", "-f", "--force-with-lease", "--delete", "-d", "--mirror", "--all", "--tags", "--prune"}, a)
		assert.False(t, strings.HasPrefix(a, "+"), "a forced refspec: %s", a)
	}
}

// TestPushRefuses refuses sandbox mode, a URL CheckURL refuses, a
// directory that is no repository and a config that rewrites push URLs,
// before git pushes.
//
//romeu:invariant I18 test
func TestPushRefuses(t *testing.T) {
	sb, err := New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	rec := pushRecorder(sb, false)
	require.ErrorIs(t, sb.Push(t.Context(), stubRepo, "https://github.com/example/m"), ErrRefused)
	assert.Empty(t, rec.commands, "julieta pushes nothing")

	host, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	rec = pushRecorder(host, false)
	for _, url := range []string{"file:///tmp/m", "/tmp/m", "ssh://git@github.com/a/b", "https://u:p@github.com/a/b"} {
		require.ErrorIs(t, host.Push(t.Context(), stubRepo, url), ErrRefused, url)
	}
	require.ErrorIs(t, host.Push(t.Context(), t.TempDir(), "https://github.com/example/m"), ErrRefused)
	assert.Empty(t, rec.commands)

	rec = pushRecorder(host, true)
	require.ErrorIs(t, host.Push(t.Context(), stubRepo, "https://github.com/example/m"), ErrRefused)
	for _, c := range rec.commands {
		assert.NotContains(t, c.Argv, "push", "no push after a pushInsteadOf")
	}
}

// TestPushToADaemon pushes a fast-forward to a remote, and is refused
// a push that is not one, which is never forced.
func TestPushToADaemon(t *testing.T) {
	src, _ := newRepo(t)
	base := t.TempDir()
	plainGit(t, base, "clone", "-q", "--bare", src, "m.git")
	url := serveReceive(t, base) + "m.git"
	r := newRunner(t, Host)

	work := t.TempDir()
	plainGit(t, work, "clone", "-q", filepath.Join(base, "m.git"), "w")
	w := filepath.Join(work, "w")
	head := commit(t, w, "two")
	plainGit(t, w, "push", "-q", filepath.Join(base, "m.git"), "HEAD:refs/heads/side")

	local := t.TempDir()
	plainGit(t, local, "clone", "-q", "--bare", filepath.Join(base, "m.git"), "l.git")
	gitDir := filepath.Join(local, "l.git")
	plainGit(t, gitDir, "update-ref", "refs/heads/main", head)
	require.NoError(t, r.Push(t.Context(), gitDir, url))
	assert.Equal(t, head, refOf(t, filepath.Join(base, "m.git"), "refs/heads/main"))

	// The remote moves on; the local main is now behind and diverges.
	other := commit(t, w, "three")
	plainGit(t, w, "push", "-q", filepath.Join(base, "m.git"), "HEAD:refs/heads/main")
	plainGit(t, gitDir, "update-ref", "refs/heads/main", plainGit(t, src, "rev-parse", "HEAD"))
	require.Error(t, r.Push(t.Context(), gitDir, url), "not a fast-forward")
	assert.Equal(t, other, refOf(t, filepath.Join(base, "m.git"), "refs/heads/main"), "nothing forced")
}

// serveReceive serves the repositories below base over git:// on
// 127.0.0.1 as serveGit does, with pushes enabled.
func serveReceive(t *testing.T, base string) string {
	t.Helper()
	git := gitPath(t)
	return listen(t, func(ctx context.Context, conn net.Conn) {
		_ = procgroup.Run(ctx, procgroup.Command{
			Class:  procgroup.JulietaGit,
			Argv:   []string{git, "daemon", "--inetd", "--export-all", "--enable=receive-pack", "--base-path=" + base},
			Env:    fixtureEnv(base),
			Stdin:  conn,
			Stdout: conn,
		})
	})
}

// pushLiterals are the places a string literal "push" or "send-pack"
// may stand in the Go files of internal/ and cmd/, tests apart: the
// argv of Push, the refusal of runInput, and the rule of julieta's
// guard that reads a git command line and starts nothing.
var pushLiterals = map[string][]string{
	"internal/gitsafe/push.go":                  {"Push"},
	"internal/gitsafe/gitsafe.go":               {""},
	"internal/cmd/julieta/hooks/guard_rules.go": {"*"},
}

// TestNoOtherPush reads every Go file of internal/ and cmd/ but the
// tests and fails a "push" or "send-pack" literal outside pushLiterals:
// no call site of procgroup but Push builds a push (I18, REQ-0502; the
// call-site scan of tools/ci keeps procgroup.Run to the packages it
// admits, and this scan covers each of them).
//
//romeu:invariant I18 test
func TestNoOtherPush(t *testing.T) {
	assert.Empty(t, pushLiteralsOutside(t, filepath.Join("..", "..")), "a push literal outside Push")

	// The positive control: a planted argv in another package is found.
	plant := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(plant, "internal", "x"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(plant, "internal", "x", "x.go"),
		[]byte("package x\n\nfunc f() []string { return []string{\"git\", \"push\", \"origin\"} }\n"), 0o600))
	assert.Len(t, pushLiteralsOutside(t, plant), 1)
}

// pushLiteralsOutside returns the place of each "push" or "send-pack"
// literal of the Go files of internal/ and cmd/ below root, tests
// apart, that pushLiterals does not admit.
func pushLiteralsOutside(t *testing.T, root string) []string {
	t.Helper()
	fset := token.NewFileSet()
	var found []string
	for _, top := range []string{"internal", "cmd"} {
		if _, err := os.Stat(filepath.Join(root, top)); err != nil {
			continue
		}
		err := fs.WalkDir(os.DirFS(root), top, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return fs.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, filepath.Join(root, p), nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				fn := ""
				if f, ok := decl.(*ast.FuncDecl); ok {
					fn = f.Name.Name
				}
				ast.Inspect(decl, func(n ast.Node) bool {
					lit, ok := n.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					v, err := strconv.Unquote(lit.Value)
					if err != nil || (v != "push" && v != "send-pack") {
						return true
					}
					allowed := pushLiterals[p]
					if !slices.Contains(allowed, fn) && !slices.Contains(allowed, "*") {
						found = append(found, fset.Position(lit.Pos()).String())
					}
					return true
				})
			}
			return nil
		})
		require.NoError(t, err)
	}
	return found
}
