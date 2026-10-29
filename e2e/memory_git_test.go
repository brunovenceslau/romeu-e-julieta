// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The durable copy of memory (08 8.1, REQ-1303, REQ-1304, REQ-1305;
// T175), driven through the step all wires around the commands of 08
// 8.1, around a probe in place of each command's own work: sync, run,
// pull, salvage, rm, recreate and retire are not all linked into romeu
// yet (T043, T066, T067, T068, T069), so the acceptance items that need
// their real work wait on those tasks. Git is the host's, the remote a
// bare repository served over git:// on 127.0.0.1.

// memGit runs plain git in dir, with no system or global config, and
// returns its standard output, trimmed.
func memGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	git, err := lookGit()
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.JulietaGit,
		Argv:  append([]string{git, "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:   dir,
		Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=f@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=f@example.invalid"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// lookGit returns the absolute path of the git on the PATH.
func lookGit() (string, error) {
	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		p := filepath.Join(dir, "git")
		if fi, err := os.Stat(p); err == nil && filepath.IsAbs(dir) && fi.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

// memServe serves base over git:// on 127.0.0.1, with pushes when push
// is set, and returns the URL prefix.
func memServe(t *testing.T, base string, push bool) string {
	t.Helper()
	git, err := lookGit()
	require.NoError(t, err)
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	argv := []string{git, "daemon", "--inetd", "--export-all", "--base-path=" + base}
	if push {
		argv = append(argv, "--enable=receive-pack")
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				_ = procgroup.Run(ctx, procgroup.Command{Class: procgroup.JulietaGit, Argv: argv,
					Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + base}, Stdin: conn, Stdout: conn})
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

// memHost is one machine's host tree: settings, the config repo's
// clone, and the live render of the project shop.
type memHost struct {
	root, settings, envDir, clone string
}

// newMemHost makes a machine whose spec of shop names remote, with
// shop's memory dirs made, and points XDG_STATE_HOME at a new
// directory.
func newMemHost(t *testing.T, remote string) *memHost {
	t.Helper()
	git, err := lookGit()
	require.NoError(t, err)
	base := t.TempDir()
	h := &memHost{root: filepath.Join(base, "root")}
	h.envDir = filepath.Join(h.root, "shop-env")
	h.clone = filepath.Join(h.root, "cfg-env", "cfg")
	require.NoError(t, os.MkdirAll(filepath.Join(h.clone, "projects"), 0o700))
	memGit(t, h.clone, "init", "-q")
	h.setRemote(t, remote)
	for _, d := range []string{"_shared", "shop-api"} {
		require.NoError(t, os.MkdirAll(filepath.Join(h.envDir, "memory", d), 0o700))
	}
	data := []byte("schema: host-settings/v1\nroot: " + h.root + "\nconfig:\n  project: cfg\n  url: https://github.com/owner/cfg\n  ref: main\n" +
		"tools:\n  sbx: /usr/bin/false\n  git: " + git + "\n")
	_, err = all.DecodeHostSettings(data)
	require.NoError(t, err, "the fixture's settings are valid")
	h.settings = filepath.Join(base, "settings.yaml")
	require.NoError(t, os.WriteFile(h.settings, data, 0o600))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return h
}

// setRemote commits a spec of shop whose memory.remote is remote ("" for
// no member) and makes the live render read it.
func (h *memHost) setRemote(t *testing.T, remote string) {
	t.Helper()
	yaml := "schema: project/v1\nname: shop\nrepos:\n  - url: https://github.com/example/shop-api\n    primary: true\n" +
		"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: claude\n"
	if remote != "" {
		yaml += "memory:\n  remote: " + remote + "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(h.clone, "projects", "shop.yaml"), []byte(yaml), 0o600))
	memGit(t, h.clone, "add", "-A")
	memGit(t, h.clone, "commit", "-q", "--allow-empty", "-m", "spec")
	commit := memGit(t, h.clone, "rev-parse", "HEAD")
	record := `{"schema":"render/v1","specSource":{"kind":"origin","ref":"refs/romeu/origin/cfg/main","commit":"` + commit + `"}}` + "\n"
	require.NoError(t, os.MkdirAll(filepath.Join(h.envDir, ".romeu"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(h.envDir, ".romeu", "render.json"), []byte(record), 0o600))
}

// write writes data at the slash path name below the memory dir.
func (h *memHost) write(t *testing.T, name, data string) {
	t.Helper()
	p := filepath.Join(h.envDir, "memory", filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
}

// step runs the memory step of line for shop around a probe that
// prints "work", and returns the exit status and the error stream.
func (h *memHost) step(t *testing.T, line string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := all.RunMemoryStep(t.Context(), h.settings, &stdout, &stderr, line, []string{"shop"}, func() error {
		stdout.WriteString("work\n")
		return nil
	})
	require.Equal(t, "work\n", stdout.String())
	return code, stderr.String()
}

// mainOf returns refs/heads/main of the repository in dir, or "".
func mainOf(t *testing.T, dir string) string {
	t.Helper()
	return memGit(t, dir, "for-each-ref", "--format=%(objectname)", "refs/heads/main")
}

// TestMemoryGitSync: a spec without memory.remote fails the step of
// sync with memory-remote-missing; with none, sync, run and doctor warn
// memory-local; with a URL, the step makes memory.git and no .git under
// memory/, and the commit holds exactly the entries and stamps (a
// planted salvage/x and snapshot/heads.json are absent), is on the
// remote's main, and a second run with no change commits and pushes
// nothing (REQ-0277).
func TestMemoryGitSync(t *testing.T) {
	remotes := t.TempDir()
	memGit(t, remotes, "init", "-q", "--bare", "m.git")
	rw, ro := memServe(t, remotes, true)+"m.git", memServe(t, remotes, false)+"m.git"
	remote := filepath.Join(remotes, "m.git")

	h := newMemHost(t, "")
	code, stderr := h.step(t, "sync")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "error RJ-357 memory-remote-missing: ")

	h.setRemote(t, "none")
	for _, line := range []string{"sync", "run", "doctor"} {
		code, stderr := h.step(t, line)
		assert.Equal(t, 0, code, line)
		assert.Contains(t, stderr, "warning memory-local: ", line)
	}
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"))

	h.setRemote(t, rw)
	h.write(t, "_shared/entries/01handoff.md", "handoff\n")
	h.write(t, "_shared/stamps/01.json", "{}\n")
	h.write(t, "shop-api/salvage/x", "secret\n")
	h.write(t, "shop-api/snapshot/heads.json", "{}\n")
	code, stderr = h.step(t, "sync")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "warning memory-unsigned: ", "no signing socket in host settings")
	gitDir := filepath.Join(h.envDir, "memory.git")
	assert.DirExists(t, gitDir)
	assert.NoFileExists(t, filepath.Join(h.envDir, "memory", ".git"))
	assert.Equal(t, "_shared/entries/01handoff.md\n_shared/stamps/01.json", memGit(t, remote, "ls-tree", "-r", "--name-only", "main"))
	assert.Equal(t, "shop <>", memGit(t, remote, "log", "-1", "--format=%an <%ae>", "main"))
	head := mainOf(t, remote)
	assert.Equal(t, head, mainOf(t, gitDir))

	h.setRemote(t, ro) // a push would now be refused, and warned
	code, stderr = h.step(t, "run")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stderr, "memory-push-failed", "nothing to push")
	assert.Equal(t, head, mainOf(t, gitDir), "nothing committed")
}

// TestMemoryGitRemote: with the remote moved on by one commit, the next
// command fast-forwards and then pushes; with a conflicting history,
// memory-diverged is warned and nothing is pushed or rewritten; with the
// remote refusing the push, memory-push-failed is warned and the
// command exits 0; and a second machine with an empty memory dir takes
// the remote's history with the separate git dir.
func TestMemoryGitRemote(t *testing.T) {
	remotes := t.TempDir()
	memGit(t, remotes, "init", "-q", "--bare", "m.git")
	rw, ro := memServe(t, remotes, true)+"m.git", memServe(t, remotes, false)+"m.git"
	remote := filepath.Join(remotes, "m.git")

	one := newMemHost(t, rw)
	one.write(t, "_shared/entries/01one.md", "one\n")
	code, stderr := one.step(t, "salvage")
	require.Equal(t, 0, code, stderr)

	two := newMemHost(t, rw) // the second machine: an empty memory dir
	code, stderr = two.step(t, "sync")
	require.Equal(t, 0, code, stderr)
	assert.FileExists(t, filepath.Join(two.envDir, "memory", "_shared", "entries", "01one.md"))
	assert.NoFileExists(t, filepath.Join(two.envDir, "memory", ".git"))
	two.write(t, "shop-api/entries/02two.md", "two\n")
	code, _ = two.step(t, "rm")
	require.Equal(t, 0, code)
	moved := mainOf(t, remote)

	one.write(t, "shop-api/entries/03one.md", "three\n")
	code, stderr = one.step(t, "recreate")
	require.Equal(t, 0, code, stderr)
	assert.FileExists(t, filepath.Join(one.envDir, "memory", "shop-api", "entries", "02two.md"), "fast-forwarded")
	assert.Equal(t, moved, memGit(t, remote, "rev-parse", "main~1"), "pushed on top of the remote's")

	// A conflicting history: the second machine commits beside the
	// first's last push without fetching it.
	twoGit := filepath.Join(two.envDir, "memory.git")
	two.write(t, "shop-api/entries/04two.md", "four\n")
	memGit(t, two.envDir, "--git-dir="+twoGit, "--work-tree="+filepath.Join(two.envDir, "memory"), "-c", "core.bare=false", "add", "-A")
	memGit(t, two.envDir, "--git-dir="+twoGit, "--work-tree="+filepath.Join(two.envDir, "memory"), "-c", "core.bare=false", "commit", "-q", "-m", "beside")
	ours, theirs := mainOf(t, twoGit), mainOf(t, remote)
	code, stderr = two.step(t, "salvage")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "warning memory-diverged: ")
	assert.Equal(t, theirs, mainOf(t, remote), "nothing pushed")
	assert.Equal(t, ours, mainOf(t, twoGit), "nothing rewritten")

	one.setRemote(t, ro)
	one.write(t, "shop-api/entries/05one.md", "five\n")
	code, stderr = one.step(t, "run")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "warning memory-push-failed: ")
}
