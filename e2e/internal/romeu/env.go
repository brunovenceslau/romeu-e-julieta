// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package romeu is the harness of the romeu e2e (10 10.1, REQ-0735): a
// temporary home with its XDG directories and a root that does not
// exist yet, the romeu binary built from this tree, the fake sbx
// (e2e/fakesbx) as the sbx on PATH, and origins served over TLS by git
// http-backend (Origins). A test builds an Env, runs romeu commands in
// it and reads the settings and state they write.
//
// A later task extends the harness by adding a file of its own to this
// directory and edits none of the existing ones (rule 1 of docs/plan.md);
// the pieces it builds on are Env (New, Run, WriteSettings, Settings,
// StateDir) and Origins (NewOrigins, URL, Push, Git).
package romeu

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
)

// Env is one machine for a test: every path is below one temporary
// directory, with symbolic links resolved so that a path romeu stores
// compares equal to the one the test holds.
type Env struct {
	// Home is HOME, and the base of Root.
	Home string
	// Root is $HOME/dev, the default root of init. It does not exist
	// until a command creates it.
	Root string
	// ConfigHome and StateHome are XDG_CONFIG_HOME and XDG_STATE_HOME.
	ConfigHome, StateHome string
	// Bin is the directory first on PATH: sbx (the fake) and git.
	Bin string
	// Sbx and Git are the absolute paths of the two tools in Bin.
	Sbx, Git string
	// Romeu is the absolute path of the romeu binary under test.
	Romeu string
}

// Result is the outcome of one run of romeu.
type Result struct {
	Stdout, Stderr string
	// Code is the exit status; -1 when romeu was ended without one.
	Code int
}

// New builds an Env for the test. origins is the set of origins its git
// reaches, or nil for the machine's git as it is. The binaries are built
// into the test's temporary directory, so each Env builds its own: the
// compiler's cache keeps that to a link.
func New(t testing.TB, origins *Origins) *Env {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Env{
		Home:       filepath.Join(tmp, "home"),
		ConfigHome: filepath.Join(tmp, "config"),
		StateHome:  filepath.Join(tmp, "state"),
		Bin:        filepath.Join(tmp, "bin"),
	}
	e.Root = filepath.Join(e.Home, "dev")
	for _, d := range []string{e.Home, e.ConfigHome, e.StateHome, e.Bin} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	root := repoRoot(t)
	e.Romeu = build(t, root, "./cmd/romeu", filepath.Join(tmp, "romeu"))
	e.Sbx = build(t, root, "./e2e/fakesbx", filepath.Join(e.Bin, "sbx"))
	if err := writeFakeSbxConfig(e.Sbx, tmp); err != nil {
		t.Fatal(err)
	}
	e.Git = installGit(t, e.Bin, origins)
	return e
}

// repoRoot returns the module root of this tree.
func repoRoot(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := moduleroot.Find(wd)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// build compiles the package pkg of the module at root into out.
func build(t testing.TB, root, pkg, out string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", out, pkg)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, stderr.String())
	}
	return out
}

// Settings returns the path of settings.yaml under XDG_CONFIG_HOME.
func (e *Env) Settings() string {
	return filepath.Join(e.ConfigHome, "romeu", "settings.yaml")
}

// StateDir returns the state directory under XDG_STATE_HOME.
func (e *Env) StateDir() string { return filepath.Join(e.StateHome, "romeu") }

// Environ returns the environment romeu runs in: HOME and the two XDG
// directories of the Env, PATH with Bin first, and extra after them,
// which wins. Nothing of the test process's environment is inherited,
// so a ROMEU_* variable or a XDG one of the developer's cannot change a
// path.
func (e *Env) Environ(extra ...string) []string {
	return append([]string{
		"HOME=" + e.Home,
		"XDG_CONFIG_HOME=" + e.ConfigHome,
		"XDG_STATE_HOME=" + e.StateHome,
		"PATH=" + e.Bin + ":/usr/bin:/bin",
		"TMPDIR=" + e.Home,
		"LANG=C",
	}, extra...)
}

// Run runs romeu with args in the Env's environment, and extra
// environment entries after it, and returns its outcome. A failure to
// start is a failure of the test; a non-zero exit is the Result's Code.
func (e *Env) Run(t testing.TB, args []string, extra ...string) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, e.Romeu, args...)
	cmd.Dir = e.Home
	cmd.Env = e.Environ(extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return res
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		res.Code = exit.ExitCode()
		return res
	}
	t.Fatalf("romeu %v: %v", args, err)
	return res
}
