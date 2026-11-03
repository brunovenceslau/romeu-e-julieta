// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// fakeSbx answers the reads of doctor from fixed values.
type fakeSbx struct {
	release   sbxdrv.Release
	floorErr  error
	sandboxes []sbxdrv.Sandbox
	listErr   error
	policies  string
	secrets   string
}

func (f *fakeSbx) RequireFloor(context.Context) (sbxdrv.Release, error) {
	if f.floorErr != nil {
		return f.release, f.floorErr
	}
	if err := sbxdrv.CheckFloor(f.release); err != nil {
		return f.release, err
	}
	return f.release, nil
}

func (f *fakeSbx) List(context.Context) ([]sbxdrv.Sandbox, error) { return f.sandboxes, f.listErr }
func (f *fakeSbx) PolicyList(context.Context) ([]byte, error)     { return []byte(f.policies), nil }
func (f *fakeSbx) SecretList(context.Context) ([]byte, error)     { return []byte(f.secrets), nil }

// fixture is one host for a test: HOME, the XDG directories and the
// root below one temporary directory, the host settings to write, the
// fake sbx and git, and the environment doctor reads.
type fixture struct {
	tmp, home, root, configHome, stateHome string
	settings                               spec.HostSettings
	sbx                                    *fakeSbx
	gitErr                                 error
	env                                    map[string]string
	// hostFindings is the preflight's step 5 doctor calls.
	hostFindings func(ctx context.Context, d preflight.Deps, in preflight.Input) ([]preflight.Finding, []string)
	// preSlugs are the slugs of the checks step 5 runs.
	preSlugs func() []string
	// noSettings leaves the host settings unwritten.
	noSettings bool
	// settingsMode, when set, is the mode the settings file is given.
	settingsMode os.FileMode
	// acknowledged writes toolchain.json for the toolchain doctor
	// measures, so the toolchain check passes.
	acknowledged bool
}

// newFixture returns a host on which every check passes: settings at
// the default path, a root that exists, sbx at a recorded version, git
// at its floor and the toolchain acknowledged.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	f := &fixture{
		tmp:        tmp,
		home:       filepath.Join(tmp, "home"),
		configHome: filepath.Join(tmp, "config"),
		stateHome:  filepath.Join(tmp, "state"),
		sbx: &fakeSbx{
			release:  sbxdrv.Release{Major: 0, Minor: 49, Patch: 2},
			policies: "[]\n",
			secrets:  "SCOPE SERVICE SECRET\n",
		},
		acknowledged: true,
		hostFindings: preflight.HostFindings,
		preSlugs:     preflight.HostSlugs,
	}
	f.root = filepath.Join(f.home, "Developer")
	for _, d := range []string{f.home, f.configHome, f.stateHome, f.root, filepath.Join(tmp, "bin")} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	f.env = map[string]string{"HOME": f.home, "XDG_CONFIG_HOME": f.configHome, "XDG_STATE_HOME": f.stateHome}
	f.settings = spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   f.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: f.tool(t, "sbx"), Git: f.tool(t, "git")},
	}
	// A gh at the floor of 12 12.1, so gh-floor passes on the well
	// kept host (T195).
	f.settings.Tools.Gh = stub(t, f, "gh", "printf 'gh version 2.68.0 (2025-03-11)\\nhttps://github.com/cli/cli/releases/tag/v2.68.0\\n'")
	return f
}

// tool writes an executable file named name in the fixture's bin.
func (f *fixture) tool(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(f.tmp, "bin", name)
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o700))
	return p
}

// settingsPath is where the fixture writes the host settings.
func (f *fixture) settingsPath() string { return filepath.Join(f.configHome, "romeu", "settings.yaml") }

// stateDir is the state directory of the fixture.
func (f *fixture) stateDir() string { return filepath.Join(f.stateHome, "romeu") }

// store makes the state directory as romeu makes it, if it is not there,
// and returns it open; the test closes it.
func (f *fixture) store(t *testing.T) *state.Store {
	t.Helper()
	s, err := state.Open(f.stateDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// writeProject writes p as the record of the project name.
func (f *fixture) writeProject(t *testing.T, name string, p *state.Project) {
	t.Helper()
	f.store(t)
	data, err := state.EncodeProject(p)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.stateDir(), "projects", name+".json"), data, state.FileMode))
}

// acknowledge writes toolchain.json for the live toolchain of release,
// as romeu approve --toolchain records it.
func (f *fixture) acknowledge(t *testing.T, release sbxdrv.Release) {
	t.Helper()
	live, err := approve.Measure(release, version)
	require.NoError(t, err)
	b, err := json.Marshal(live.Catalog)
	require.NoError(t, err)
	var c state.Catalog
	require.NoError(t, json.Unmarshal(b, &c))
	s := f.store(t)
	lk, err := s.Lock()
	require.NoError(t, err)
	defer lk.Unlock()
	require.NoError(t, s.UpdateToolchain(lk, func(rec *state.Toolchain) error {
		rec.Schema = state.ToolchainField
		rec.SbxVersion = live.Sbx
		rec.Romeu = live.Romeu
		rec.Julieta = live.Julieta
		rec.CatalogDigest = live.CatalogDigest
		rec.Catalog = c
		rec.AcknowledgedAt = time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
		return nil
	}))
}

// version is the version stamp of the binary under test.
var version = cli.Version{Version: "v0.1.0", Commit: "0123456", Date: "2026-10-30", Protocol: 1}

// output runs doctor --json on the fixture with the secret runner of
// the process, returns the report it printed and the error it returned.
func (f *fixture) output(t *testing.T) (Report, string, error) {
	t.Helper()
	if !f.noSettings {
		data, err := spec.EncodeHostSettings(f.settings)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(f.settingsPath()), 0o700))
		require.NoError(t, os.WriteFile(f.settingsPath(), data, 0o600))
		if f.settingsMode != 0 {
			require.NoError(t, os.Chmod(f.settingsPath(), f.settingsMode))
		}
	}
	if f.acknowledged {
		f.acknowledge(t, f.sbx.release)
	}
	var out, errOut bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&errOut), JSON: true, Version: version}
	h := newHost(env, seams{
		getenv: func(k string) string { return f.env[k] },
		goos:   "linux",
		sbx:    func(*spec.HostSettings, string) sbxReader { return f.sbx },
		gitFloor: func(*spec.HostSettings, string) func(context.Context) error {
			return func(context.Context) error { return f.gitErr }
		},
		preflight: f.hostFindings,
		preSlugs:  f.preSlugs,
		secret:    runSecretArgv,
	})
	defer h.close()
	err := run(t.Context(), env, h)
	var rep Report
	require.NoError(t, json.Unmarshal(out.Bytes(), &rep), "stdout: %s", out.String())
	return rep, out.String() + errOut.String(), err
}

// check returns the result of slug in rep.
func check(t *testing.T, rep Report, slug string) Check {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Slug == slug {
			return c
		}
	}
	t.Fatalf("no check %s in the report", slug)
	return Check{}
}

// exitOf is the exit status of a command that returned err.
func exitOf(err error) int { return cli.ExitCode(err) }

// fileHolding returns a file below dir whose content holds s, or "".
func fileHolding(t *testing.T, dir, s string) string {
	t.Helper()
	found := ""
	require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), s) {
			found = p
		}
		return nil
	}))
	return found
}
