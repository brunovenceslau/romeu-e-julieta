// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package all

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// gitPath returns the absolute path of the git on the PATH.
func gitPath(t *testing.T) string {
	t.Helper()
	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		p := filepath.Join(dir, "git")
		if fi, err := os.Stat(p); err == nil && filepath.IsAbs(dir) && fi.Mode().IsRegular() {
			return p
		}
	}
	require.FailNow(t, "no git on the PATH of the test process")
	return ""
}

// git runs plain git in dir, with no system or global config, for the
// fixtures and the checks, and returns its standard output, trimmed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.JulietaGit,
		Argv:  append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:   dir,
		Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=f@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=f@example.invalid"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// serve serves base over git:// on 127.0.0.1, with pushes when push is
// set, and returns the URL prefix.
func serve(t *testing.T, base string, push bool) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	argv := []string{gitPath(t), "daemon", "--inetd", "--export-all", "--base-path=" + base}
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

// sshAgent serves an in-process ssh-agent holding one new ed25519 key
// and returns its socket and the key's public line.
func sshAgent(t *testing.T) (socket, public string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: priv}))
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	dir := filepath.Join(os.TempDir(), "mp"+rand.Text()[:8])
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket = filepath.Join(dir, "s")
	ln, err := new(net.ListenConfig).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()
	return socket, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}

// host is a host tree for the step: settings, the config repo's clone
// holding projects/shop.yaml, the live render of shop at that commit,
// shop's memory dirs, and a remote served with and without pushes.
type host struct {
	root, settings, envDir string
	remoteDir, url, ro     string
}

// memoryOf is the memory member of the fixture's spec: the remote's URL
// for "url", the read-only URL for "ro", the text itself otherwise
// ("none"), and no member for "".
type memoryOf string

// newHost builds the host tree; signed adds the git-ssh-sign kit with
// an agent's key and the agent's socket to host settings.
func newHost(t *testing.T, member memoryOf, signed bool) *host {
	t.Helper()
	base := t.TempDir()
	h := &host{root: filepath.Join(base, "root")}
	remotes := filepath.Join(base, "remotes")
	require.NoError(t, os.MkdirAll(remotes, 0o700))
	git(t, remotes, "init", "-q", "--bare", "m.git")
	h.remoteDir = filepath.Join(remotes, "m.git")
	h.url = serve(t, remotes, true) + "m.git"
	h.ro = serve(t, remotes, false) + "m.git"

	var socket, public string
	if signed {
		socket, public = sshAgent(t)
	}
	yaml := "schema: project/v1\nname: shop\nrepos:\n  - url: https://github.com/example/shop-api\n    primary: true\n" +
		"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: claude\n"
	if signed {
		yaml += "kits:\n  - product: git-ssh-sign\n    args: {signingKey: \"" + public + "\"}\n"
	}
	switch member {
	case "":
	case "url":
		yaml += "memory:\n  remote: " + h.url + "\n"
	case "ro":
		yaml += "memory:\n  remote: " + h.ro + "\n"
	default:
		yaml += "memory:\n  remote: " + string(member) + "\n"
	}
	_, err := spec.DecodeProject([]byte(yaml))
	require.NoError(t, err, "the fixture's spec is valid")
	clone := filepath.Join(h.root, "cfg-env", "cfg")
	require.NoError(t, os.MkdirAll(filepath.Join(clone, "projects"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(clone, "projects", "shop.yaml"), []byte(yaml), 0o600))
	git(t, clone, "init", "-q")
	git(t, clone, "add", "-A")
	git(t, clone, "commit", "-q", "-m", "specs")
	commit := git(t, clone, "rev-parse", "HEAD")

	h.envDir = filepath.Join(h.root, "shop-env")
	record, err := render.EncodeRecord(render.Record{SpecSource: render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/cfg/main", Commit: commit}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(h.envDir, ".romeu"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(h.envDir, filepath.FromSlash(render.LiveRecord)), record, 0o600))
	for name, data := range map[string]string{
		"_shared/entries/01handoff.md": "handoff\n", "_shared/stamps/01.json": "{}\n",
		"shop-api/entries/02entry.md": "entry\n", "shop-api/salvage/x": "secret\n", "shop-api/snapshot/heads.json": "{}\n",
	} {
		p := filepath.Join(h.envDir, "memory", filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
	}

	settings := spec.HostSettings{Schema: "host-settings/v1", Root: h.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}}
	settings.Tools.Sbx, settings.Tools.Git = "/usr/bin/false", gitPath(t)
	if signed {
		settings.Signing = &spec.Signing{AgentSocket: socket}
	}
	data, err := spec.EncodeHostSettings(settings)
	require.NoError(t, err)
	h.settings = filepath.Join(base, "settings.yaml")
	require.NoError(t, os.WriteFile(h.settings, data, 0o600))

	stateHome, home := t.TempDir(), t.TempDir()
	old := getenv
	getenv = func(k string) string {
		switch k {
		case "XDG_STATE_HOME":
			return stateHome
		case "HOME":
			return home
		}
		return ""
	}
	t.Cleanup(func() { getenv = old })
	return h
}

// step runs the step of line around a work that prints "work" and
// returns err, and returns the exit status and the two streams.
func (h *host) step(t *testing.T, line string, err error, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunMemoryStep(t.Context(), h.settings, &stdout, &stderr, line, args, func() error {
		stdout.WriteString("work\n")
		return err
	})
	return code, stdout.String(), stderr.String()
}

// remoteMain returns the remote's main, or "".
func (h *host) remoteMain(t *testing.T) string {
	t.Helper()
	return git(t, h.remoteDir, "for-each-ref", "--format=%(objectname)", "refs/heads/main")
}

// TestMemoryRoles names the seven commands of 08 8.1 as the ones that
// push, sync alone as the one that refuses, and sync, run and doctor as
// the ones that warn every time (REQ-1304, REQ-1305).
func TestMemoryRoles(t *testing.T) {
	var push, notices, refuse []string
	for line, r := range memoryRoles {
		if r.push {
			push = append(push, line)
		}
		if r.notices {
			notices = append(notices, line)
		}
		if r.refuse {
			refuse = append(refuse, line)
		}
		assert.NotNil(t, r.targets, line)
	}
	assert.ElementsMatch(t, []string{"sync", "run", "pull", "salvage", "rm", "recreate", "retire"}, push)
	assert.ElementsMatch(t, []string{"sync", "run", "doctor"}, notices)
	assert.Equal(t, []string{"sync"}, refuse)
}

// TestStepSyncPushes commits and pushes after the work, and warns
// memory-unsigned with no signing socket, after the work's output.
func TestStepSyncPushes(t *testing.T) {
	h := newHost(t, "url", false)
	code, stdout, stderr := h.step(t, "sync", nil, "shop")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "work\n", stdout)
	assert.Equal(t, 1, strings.Count(stderr, "warning memory-unsigned: "), stderr)
	gitDir := filepath.Join(h.envDir, "memory.git")
	assert.Equal(t, git(t, gitDir, "rev-parse", "refs/heads/main"), h.remoteMain(t))
	assert.Equal(t, "_shared/entries/01handoff.md\n_shared/stamps/01.json\nshop-api/entries/02entry.md",
		git(t, h.remoteDir, "ls-tree", "-r", "--name-only", "main"))
	assert.NoFileExists(t, filepath.Join(h.envDir, "memory", ".git"))
	assert.Equal(t, "N", git(t, h.remoteDir, "log", "-1", "--format=%G?", "main"))
}

// TestStepSigned signs with the project's key through the host's
// socket, and warns nothing.
func TestStepSigned(t *testing.T) {
	h := newHost(t, "url", true)
	code, _, stderr := h.step(t, "run", nil, "shop")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	assert.Contains(t, git(t, h.remoteDir, "cat-file", "commit", "main"), "gpgsig -----BEGIN SSH SIGNATURE-----")
	assert.Equal(t, "shop <>", git(t, h.remoteDir, "log", "-1", "--format=%an <%ae>", "main"))
}

// TestStepSyncRefusesNoMember fails sync with memory-remote-missing
// (RJ-357, exit 2) and pushes nothing; the other commands leave such a
// spec alone.
func TestStepSyncRefusesNoMember(t *testing.T) {
	h := newHost(t, "", false)
	code, _, stderr := h.step(t, "sync", nil, "shop")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "error RJ-357 memory-remote-missing: ")
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"))
	for _, line := range []string{"run", "salvage", "rm", "recreate", "doctor"} {
		code, _, stderr := h.step(t, line, nil, "shop")
		assert.Equal(t, 0, code, line)
		assert.Empty(t, stderr, line)
	}
}

// TestStepNone warns memory-local from sync, run and doctor every time,
// and pushes nothing; the other commands warn nothing.
func TestStepNone(t *testing.T) {
	h := newHost(t, "none", false)
	for _, line := range []string{"sync", "run", "doctor", "sync"} {
		code, _, stderr := h.step(t, line, nil, "shop")
		assert.Equal(t, 0, code, line)
		assert.Equal(t, 1, strings.Count(stderr, "warning memory-local: "), "%s: %s", line, stderr)
	}
	for _, line := range []string{"salvage", "rm", "recreate"} {
		_, _, stderr := h.step(t, line, nil, "shop")
		assert.Empty(t, stderr, line)
	}
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"))
}

// TestStepDoctorDoesNotPush warns and writes nothing.
func TestStepDoctorDoesNotPush(t *testing.T) {
	h := newHost(t, "url", false)
	code, _, stderr := h.step(t, "doctor", nil)
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "warning memory-unsigned: ")
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"))
}

// TestStepAfterAFailedCommand pushes nothing when the work failed, and
// reports the work's error.
func TestStepAfterAFailedCommand(t *testing.T) {
	h := newHost(t, "url", false)
	code, _, stderr := h.step(t, "rm", state.Locked.New("held"), "shop")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "RJ-101")
	assert.NotContains(t, stderr, "warning")
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"))
	assert.Empty(t, h.remoteMain(t))
}

// TestStepPushRefused warns memory-push-failed and exits 0.
func TestStepPushRefused(t *testing.T) {
	h := newHost(t, "ro", false)
	code, _, stderr := h.step(t, "salvage", nil, "shop")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "warning memory-push-failed: ")
	assert.Empty(t, h.remoteMain(t))
}

// TestStepTakesTheLock warns memory-push-failed, exit 0, when another
// romeu holds the lock once the command released it, and pushes
// nothing; the next command pushes.
func TestStepTakesTheLock(t *testing.T) {
	h := newHost(t, "url", false)
	dir, err := state.DefaultDir(getenv)
	require.NoError(t, err)
	st, err := state.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	lk, err := st.Lock()
	require.NoError(t, err)
	code, _, stderr := h.step(t, "run", nil, "shop")
	assert.Equal(t, 0, code, "the command's work stands")
	assert.Contains(t, stderr, "warning memory-push-failed: memory was not committed or pushed; the next command pushes: another romeu holds the lock")
	assert.Contains(t, stderr, "warning memory-unsigned: ", "the notices of run are printed all the same")
	assert.Empty(t, h.remoteMain(t))
	assert.NoDirExists(t, filepath.Join(h.envDir, "memory.git"), "nothing committed")
	lk.Unlock()
	code, _, _ = h.step(t, "run", nil, "shop")
	assert.Equal(t, 0, code)
	assert.NotEmpty(t, h.remoteMain(t))
}

// TestStepTargets resolves sync --all and no name to every live
// project, pull to the project of -C, and retire to the newest tree of
// the attic.
func TestStepTargets(t *testing.T) {
	h := newHost(t, "url", false)
	code, _, stderr := h.step(t, "sync", nil, "--all")
	assert.Equal(t, 0, code, stderr)
	first := h.remoteMain(t)
	assert.NotEmpty(t, first)

	require.NoError(t, os.WriteFile(filepath.Join(h.envDir, "memory", "shop-api", "entries", "03pull.md"), []byte("p\n"), 0o600))
	code, _, stderr = h.step(t, "pull", nil, "-C", filepath.Join(h.envDir, "memory", "shop-api"))
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, git(t, h.remoteDir, "ls-tree", "-r", "--name-only", "main"), "shop-api/entries/03pull.md")
	t.Chdir(h.envDir)
	require.NoError(t, os.WriteFile(filepath.Join(h.envDir, "memory", "shop-api", "entries", "03rel.md"), []byte("r\n"), 0o600))
	code, _, stderr = h.step(t, "pull", nil, "-C", "memory")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, git(t, h.remoteDir, "ls-tree", "-r", "--name-only", "main"), "shop-api/entries/03rel.md", "a relative -C")
	t.Chdir(h.root)
	code, _, stderr = h.step(t, "pull", nil, "-C", t.TempDir())
	assert.Equal(t, 0, code, "a directory outside the root")
	assert.Contains(t, stderr, "warning memory-push-failed: ")

	attic := filepath.Join(h.root, ".attic", "shop", "20261029T120000Z")
	require.NoError(t, os.MkdirAll(filepath.Dir(attic), 0o700))
	require.NoError(t, os.Rename(h.envDir, attic))
	require.NoError(t, os.WriteFile(filepath.Join(attic, "memory", "_shared", "entries", "04tomb.md"), []byte("t\n"), 0o600))
	code, _, stderr = h.step(t, "retire", nil, "shop")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, git(t, h.remoteDir, "ls-tree", "-r", "--name-only", "main"), "_shared/entries/04tomb.md")

	code, _, stderr = h.step(t, "run", nil, "../x")
	assert.Equal(t, 0, code, "a name that is no path element")
	assert.Contains(t, stderr, "warning memory-push-failed: ")
}

// probe is a romeu command of this test binary alone, with the step of
// sync around it, which shows that Tree runs the step registered by
// command line: the commands of 08 8.1 are not all linked in yet.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "memprobe"
	c.NoJourney = "a fixture of the tests of internal/cmd/romeu/all"
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{Use: "memprobe <name>", Args: cobra.ExactArgs(1), RunE: func(*cobra.Command, []string) error {
			_, err := env.Stdout.Write([]byte("work\n"))
			return err
		}}
	}
	return c
}())

var _ = cli.Wrap("romeu", "memprobe", memoryStep(memoryRoles["sync"]))

// TestTreeRunsTheStep runs the probe through cli.Main: the work's
// output first, then the step's warnings, which close the output.
func TestTreeRunsTheStep(t *testing.T) {
	h := newHost(t, "none", false)
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), SettingsPath: h.settings}
	assert.Equal(t, 0, cli.Main(t.Context(), []string{"memprobe", "shop"}, env))
	assert.Equal(t, "work\n", stdout.String())
	assert.True(t, strings.HasPrefix(stderr.String(), "warning memory-local: "), stderr.String())
}

// TestStepHostSettings reads a credential helper and gitHosts from host
// settings, finds the config repo's clone through the config project's
// live render, and fails without readable settings.
func TestStepHostSettings(t *testing.T) {
	h := newHost(t, "url", false)
	// The config repo's clone moves to the dir its own render names.
	moved := filepath.Join(h.root, "cfg-env", "config")
	require.NoError(t, os.Rename(filepath.Join(h.root, "cfg-env", "cfg"), moved))
	commit := git(t, moved, "rev-parse", "HEAD")
	record, err := render.EncodeRecord(render.Record{
		SpecSource: render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/config/main", Commit: commit},
		Repos:      []render.RecordRepo{{Dir: "config", URL: "https://github.com/owner/cfg.git", EgressRef: "refs/romeu/origin/config/main", EgressCommit: commit}},
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(h.root, "cfg-env", ".romeu"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(h.root, "cfg-env", filepath.FromSlash(render.LiveRecord)), record, 0o600))

	data, err := os.ReadFile(h.settings)
	require.NoError(t, err)
	settings, err := spec.DecodeHostSettings(data)
	require.NoError(t, err)
	settings.Tools.GitCredentialHelper = "store"
	settings.GitHosts = []spec.GitHost{{Host: "github.com"}}
	data, err = spec.EncodeHostSettings(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(h.settings, data, 0o600))
	code, _, stderr := h.step(t, "run", nil, "shop")
	assert.Equal(t, 0, code, stderr)
	assert.NotEmpty(t, h.remoteMain(t))

	// No projects/<name>.yaml at the commit: the step warns, and the
	// command's work stands.
	code, _, stderr = h.step(t, "run", nil, "cfg")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "warning memory-push-failed: the memory of cfg was not committed or pushed")

	for name, path := range map[string]string{"absent": filepath.Join(t.TempDir(), "none.yaml"), "a directory": t.TempDir()} {
		var stdout, stderr bytes.Buffer
		code := RunMemoryStep(t.Context(), path, &stdout, &stderr, "run", []string{"shop"}, func() error { return nil })
		assert.Equal(t, 0, code, name)
		assert.Contains(t, stderr.String(), "warning memory-push-failed: ", name)
	}
}

// TestRunMemoryStepRefuses refuses a command line with no step and a
// flag the probe does not take.
func TestRunMemoryStepRefuses(t *testing.T) {
	h := newHost(t, "url", false)
	code, _, _ := h.step(t, "status", nil, "shop")
	assert.Equal(t, 1, code)
	code, _, _ = h.step(t, "run", nil, "--nope")
	assert.Equal(t, 2, code)
}

// TestStepTargetsOfTheWorkingDirectory resolves pull with no -C to the
// project of the working directory, and refuses a retire with no tree
// in the attic.
func TestStepTargetsOfTheWorkingDirectory(t *testing.T) {
	h := newHost(t, "url", false)
	t.Chdir(filepath.Join(h.envDir, "memory"))
	code, _, stderr := h.step(t, "pull", nil)
	assert.Equal(t, 0, code, stderr)
	assert.NotEmpty(t, h.remoteMain(t))

	require.NoError(t, os.MkdirAll(filepath.Join(h.root, ".attic", "shop"), 0o700))
	code, _, stderr = h.step(t, "retire", nil, "shop")
	assert.Equal(t, 0, code, "an attic with no tree")
	assert.Contains(t, stderr, "warning memory-push-failed: ")
	t.Chdir(h.root)
	code, _, stderr = h.step(t, "pull", nil)
	assert.Equal(t, 0, code, "the root is in no project's tree")
	assert.Contains(t, stderr, "warning memory-push-failed: ")
}

// TestStepDoctorFailsNothing leaves to doctor's own checks host settings
// it cannot read and a live render it cannot read: the step only warns.
func TestStepDoctorFailsNothing(t *testing.T) {
	h := newHost(t, "url", false)
	require.NoError(t, os.Remove(filepath.Join(h.envDir, filepath.FromSlash(render.LiveRecord))))
	require.NoError(t, os.WriteFile(filepath.Join(h.envDir, filepath.FromSlash(render.LiveRecord)), []byte("{}"), 0o600))
	code, _, stderr := h.step(t, "doctor", nil)
	assert.Equal(t, 0, code, stderr)
	var stdout, errOut bytes.Buffer
	code = RunMemoryStep(t.Context(), filepath.Join(t.TempDir(), "none.yaml"), &stdout, &errOut, "doctor", nil, func() error { return nil })
	assert.Equal(t, 0, code, errOut.String())
}

// TestStepFailsOnTheMachine keeps as an error, after the command's
// work, a failure of the machine's own memory repository: here a layout
// violation in entries/, which needs the operator.
func TestStepFailsOnTheMachine(t *testing.T) {
	h := newHost(t, "url", false)
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(h.envDir, "memory", "shop-api", "entries", "x.md")))
	code, _, stderr := h.step(t, "run", nil, "shop")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "x.md")
	assert.Empty(t, h.remoteMain(t))
}
