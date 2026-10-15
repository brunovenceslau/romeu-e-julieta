// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// gitPath returns the absolute path of the git the tests run, found on
// the PATH of the test process. The product never looks git up: host
// settings name it (04 4.1, Subprocesses, item 1).
func gitPath(t *testing.T) string {
	t.Helper()
	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if !filepath.IsAbs(dir) {
			continue
		}
		p := filepath.Join(dir, "git")
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return p
		}
	}
	require.FailNow(t, "no git on the PATH of the test process")
	return ""
}

// fixtureEnv is the environment of the plain git that builds fixtures:
// no system or global config, so the host's own config (signing, for
// one) cannot reach a fixture, and a fixed identity and date.
func fixtureEnv(home string) []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"PATH=/usr/bin:/bin",
		"HOME=" + home,
		"LANG=C",
		"GIT_AUTHOR_NAME=fixture",
		"GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=fixture",
		"GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	}
}

// plainGit runs git without gitsafe, to build fixtures and to run the
// positive controls, and returns its standard output, trimmed.
func plainGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := plainGitErr(t, dir, nil, args...)
	require.NoError(t, err, "git %s", strings.Join(args, " "))
	return out
}

// plainGitErr runs git as plainGit does, with extra variables appended
// to the environment, and returns its error.
func plainGitErr(t *testing.T, dir string, extraEnv []string, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.JulietaGit,
		Argv:   argv,
		Dir:    dir,
		Env:    append(fixtureEnv(t.TempDir()), extraEnv...),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", errors.Join(err, errors.New(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// newRepo makes a repository with one commit on main and returns its
// directory and the commit.
func newRepo(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	plainGit(t, dir, "init", "-q")
	return dir, commit(t, dir, "one")
}

// commit adds an empty commit to the branch checked out in dir and
// returns it.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	plainGit(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return plainGit(t, dir, "rev-parse", "HEAD")
}

// newRunner returns a runner of the mode over the git of the test
// process, with a temporary home.
func newRunner(t *testing.T, mode Mode) *Runner {
	t.Helper()
	r, err := New(Config{Mode: mode, Git: gitPath(t), Home: t.TempDir()})
	require.NoError(t, err)
	return r
}

// serveGit serves the repositories below base over git:// on
// 127.0.0.1, one git daemon --inetd per connection, and returns the
// URL prefix of base, ending in a slash.
func serveGit(t *testing.T, base string) string {
	t.Helper()
	git := gitPath(t)
	return listen(t, func(ctx context.Context, conn net.Conn) {
		// A failed session shows as a failed fetch in the test that
		// made it; the daemon's own error adds nothing.
		_ = procgroup.Run(ctx, procgroup.Command{
			Class:  procgroup.JulietaGit,
			Argv:   []string{git, "daemon", "--inetd", "--export-all", "--base-path=" + base},
			Env:    fixtureEnv(base),
			Stdin:  conn,
			Stdout: conn,
		})
	})
}

// serveSilence accepts git:// connections on 127.0.0.1 and never
// answers, so a fetch from it runs until its limit.
func serveSilence(t *testing.T) string {
	t.Helper()
	return listen(t, func(ctx context.Context, _ net.Conn) { <-ctx.Done() })
}

// listen accepts connections on 127.0.0.1 and hands each to serve,
// which returns when it is done with the connection or ctx ends. It
// returns the git:// URL prefix of the listener; the cleanup stops the
// listener and waits for every session.
func listen(t *testing.T, serve func(ctx context.Context, conn net.Conn)) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				serve(ctx, conn)
			})
		}
	})
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	})
	return "git://" + ln.Addr().String() + "/"
}

// serveHTTPS serves the files below base over HTTPS on 127.0.0.1, which
// git reads as the dumb HTTP protocol, and returns the host and port of
// the server and the path of a PEM file that holds its certificate.
func serveHTTPS(t *testing.T, base string) (host, caFile string) {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.FileServer(http.Dir(base)))
	// A handshake a test makes fail on purpose is not news.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	caFile = filepath.Join(t.TempDir(), "ca.pem")
	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
	require.NoError(t, os.WriteFile(caFile, buf.Bytes(), 0o600))
	return srv.Listener.Addr().String(), caFile
}

// bareCopy makes a bare copy of the repository src named name below a
// new base directory, ready to serve over git:// and dumb HTTP, and
// returns the base.
func bareCopy(t *testing.T, src, name string) string {
	t.Helper()
	base := t.TempDir()
	plainGit(t, base, "clone", "-q", "--bare", src, name)
	plainGit(t, filepath.Join(base, name), "update-server-info")
	return base
}

// refOf returns the commit ref names in the repository in dir, or ""
// when it does not exist.
func refOf(t *testing.T, dir, ref string) string {
	t.Helper()
	out, err := plainGitErr(t, dir, nil, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return ""
	}
	return out
}

// recorder replaces the runner's start function and records each
// command, answering ls-remote --get-url with its URL unchanged.
type recorder struct {
	mu       sync.Mutex
	commands []procgroup.Command
}

// install makes r record the commands of the runner.
func (rec *recorder) install(r *Runner) {
	r.start = func(_ context.Context, c procgroup.Command) error {
		rec.mu.Lock()
		rec.commands = append(rec.commands, c)
		rec.mu.Unlock()
		if i := slices.Index(c.Argv, "--get-url"); i >= 0 && c.Stdout != nil {
			_, err := io.WriteString(c.Stdout, c.Argv[i+1]+"\n")
			return err
		}
		if slices.Index(c.Argv, "version") >= 0 && c.Stdout != nil {
			_, err := io.WriteString(c.Stdout, "git version 2.53.0\n")
			return err
		}
		return nil
	}
}
