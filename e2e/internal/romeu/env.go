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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Env is one machine for a test: every path is below one temporary
// directory, with symbolic links resolved so that a path romeu stores
// compares equal to the one the test holds.
type Env struct {
	// Home is HOME, and the base of Root.
	Home string
	// Root is $HOME/Developer, the default root of init. It does not exist
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
func New(tb testing.TB, origins *Origins) *Env {
	tb.Helper()
	tmp, err := filepath.EvalSymlinks(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	e := &Env{
		Home:       filepath.Join(tmp, "home"),
		ConfigHome: filepath.Join(tmp, "config"),
		StateHome:  filepath.Join(tmp, "state"),
		Bin:        filepath.Join(tmp, "bin"),
	}
	e.Root = filepath.Join(e.Home, "Developer")
	for _, d := range []string{e.Home, e.ConfigHome, e.StateHome, e.Bin} {
		if err := os.Mkdir(d, 0o700); err != nil {
			tb.Fatal(err)
		}
	}
	root := repoRoot(tb)
	e.Romeu = build(tb, root, "./cmd/romeu", filepath.Join(tmp, "romeu"))
	e.Sbx = build(tb, root, "./e2e/fakesbx", filepath.Join(e.Bin, "sbx"))
	if err := writeFakeSbxConfig(e.Sbx, tmp); err != nil {
		tb.Fatal(err)
	}
	e.Git = installGit(tb, e.Bin, origins)
	return e
}

// repoRoot returns the module root of this tree.
func repoRoot(tb testing.TB) string {
	tb.Helper()
	wd, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	root, err := moduleroot.Find(wd)
	if err != nil {
		tb.Fatal(err)
	}
	return root
}

// build compiles the package pkg of the module at root into out.
func build(tb testing.TB, root, pkg, out string) string {
	tb.Helper()
	b, err := procgroup.CombinedOutput(tb.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{"go", "build", "-trimpath", "-o", out, pkg},
		Dir:   root,
		Env:   os.Environ(),
	})
	if err != nil {
		tb.Fatalf("go build %s: %v\n%s", pkg, err, b)
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
func (e *Env) Run(tb testing.TB, args []string, extra ...string) Result {
	tb.Helper()
	return e.RunStdin(tb, "", args, extra...)
}

// RunStdin is Run with stdin as the standard input of romeu, for a
// command that reads an answer; an empty stdin is an empty input.
func (e *Env) RunStdin(tb testing.TB, stdin string, args []string, extra ...string) Result {
	tb.Helper()
	var in io.Reader
	if stdin != "" {
		in = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(tb.Context(), procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{e.Romeu}, args...),
		Dir:    e.Home,
		Env:    e.Environ(extra...),
		Stdin:  in,
		Stdout: &stdout, Stderr: &stderr,
	})
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return res
	}
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		res.Code = exit.Code
		return res
	}
	tb.Fatalf("romeu %v: %v", args, err)
	return res
}
