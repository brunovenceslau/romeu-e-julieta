// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// host is a machine for step 1: HOME, the state directory, host
// settings whose root is under HOME and whose sbx is a script that
// prints a version.
type host struct {
	home, state, settings, root string
	env                         map[string]string
	stderr                      bytes.Buffer
}

// version is the version stamp of the romeu under test.
var version = cli.Version{Version: "v1.0.0", Commit: "abc", Protocol: 1}

// newHost writes the settings and the sbx script, points getenv at the
// host and makes the embedded julieta a few fixture bytes.
func newHost(t *testing.T) *host {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	h := &host{
		home:     filepath.Join(dir, "home"),
		state:    filepath.Join(dir, "xdg-state"),
		settings: filepath.Join(dir, "settings.yaml"),
	}
	h.root = filepath.Join(h.home, "Developer")
	h.env = map[string]string{"HOME": h.home, "XDG_STATE_HOME": h.state}
	require.NoError(t, os.MkdirAll(h.root, 0o700))
	sbx := filepath.Join(dir, "sbx")
	require.NoError(t, os.WriteFile(sbx, []byte("#!/bin/sh\necho 'sbx version v0.49.2 0123456789abcdef'\n"), 0o700))
	data, err := spec.EncodeHostSettings(spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   h.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: sbx, Git: "/usr/bin/git"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(h.settings, data, 0o600))

	oldGetenv, oldJulieta := getenv, embeddedJulieta
	getenv = func(k string) string { return h.env[k] }
	embeddedJulieta = func() []byte { return []byte("julieta") }
	t.Cleanup(func() { getenv, embeddedJulieta = oldGetenv, oldJulieta })
	return h
}

// run returns a run of h with the flags f.
func (h *host) run(f flags) *run {
	return newRun(&cli.Env{
		Binary: "romeu", Stdout: cli.NewWriter(&bytes.Buffer{}), Stderr: cli.NewWriter(&h.stderr),
		Now: time.Now, Version: version, SettingsPath: h.settings,
	}, f)
}

// acknowledge records the live toolchain as gate 1 does.
func (h *host) acknowledge(t *testing.T) {
	t.Helper()
	settings, err := spec.DecodeHostSettings(mustRead(t, h.settings))
	require.NoError(t, err)
	release, err := (&sbxdrv.Runner{Sbx: settings.Tools.Sbx, Home: h.home}).RequireFloor(t.Context(), sbxdrv.Opts{Step: "test"})
	require.NoError(t, err)
	live, err := approve.Measure(release, version)
	require.NoError(t, err)
	st, err := state.Open(filepath.Join(h.state, "romeu"))
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	lk, err := st.Lock()
	require.NoError(t, err)
	defer lk.Unlock()
	require.NoError(t, st.UpdateToolchain(lk, func(rec *state.Toolchain) error {
		rec.Schema, rec.SbxVersion, rec.Romeu, rec.Julieta = state.ToolchainField, live.Sbx, live.Romeu, live.Julieta
		rec.CatalogDigest, rec.Catalog, rec.AcknowledgedAt = live.CatalogDigest, state.Catalog{}, time.Now().UTC()
		return nil
	}))
}

// interrupt writes into <root>/<name>-env/ the render.json of a first
// promotion that was killed after its step 1 and whose candidate dir is
// gone, which recovery abandons.
func (h *host) interrupt(t *testing.T, name string) string {
	t.Helper()
	envDir := filepath.Join(h.root, name+"-env")
	require.NoError(t, os.MkdirAll(filepath.Join(envDir, ".romeu"), 0o700))
	id, err := canon.ParseULID("01m5mre3vz48a3n74a11ataacn")
	require.NoError(t, err)
	data, err := render.EncodeRecord(render.Record{
		Schema:     render.RecordSchema,
		SpecSource: render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/cfg/main", Commit: strings.Repeat("c", 40)},
		Files:      map[string]string{},
		Promoting:  &render.Promoting{CandidateID: id, PreviousFiles: map[string]string{}},
	})
	require.NoError(t, err)
	record := filepath.Join(envDir, ".romeu", "render.json")
	require.NoError(t, os.WriteFile(record, data, 0o600))
	return record
}

// mustRead returns the content of the file at p.
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	return data
}

// TestLockAndCheck is step 1 of 04 4.2: the host settings, the embedded
// julieta, the lock, promotion recovery and gate 1, in that order of
// refusal.
//
//romeu:invariant I7 test
func TestLockAndCheck(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, h *host)
		// id is the error id step 1 stops with, "" for none.
		id   string
		exit int
		// stateWritten says whether step 1 got as far as the state
		// directory, which it makes; before it, nothing is written.
		stateWritten bool
	}{
		{
			name:    "no host settings is usage",
			arrange: func(t *testing.T, h *host) { t.Helper(); require.NoError(t, os.Remove(h.settings)) },
			id:      "RJ-349", exit: 2,
		},
		{
			name:    "a romeu that embeds no julieta stops before any write",
			arrange: func(*testing.T, *host) { embeddedJulieta = func() []byte { return nil } },
			id:      "RJ-312", exit: 2,
		},
		{
			name:    "a HOME that is not absolute is usage",
			arrange: func(_ *testing.T, h *host) { h.env["HOME"] = "relative" },
			id:      "RJ-349", exit: 2,
		},
		{
			name:    "no acknowledged toolchain is gate 1, exit 3",
			arrange: func(*testing.T, *host) {},
			id:      "RJ-302", exit: 3, stateWritten: true,
		},
		{
			name:         "the acknowledged toolchain passes",
			arrange:      func(t *testing.T, h *host) { t.Helper(); h.acknowledge(t) },
			stateWritten: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHost(t)
			tt.arrange(t, h)
			r := h.run(flags{})
			t.Cleanup(r.release)

			err := lockAndCheck(t.Context(), r)

			if tt.id == "" {
				require.NoError(t, err)
				assert.NotNil(t, r.lock, "step 1 holds the lock")
				assert.Equal(t, h.root, r.settings.Root)
				assert.Equal(t, h.home, r.home)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.id, cli.Of(err).ID, err.Error())
				assert.Equal(t, tt.exit, cli.ExitCode(err))
			}
			if !tt.stateWritten {
				assert.NoDirExists(t, h.state, "step 1 wrote before it refused")
			}
		})
	}
}

// TestLockAndCheckStopsOnAnAbandonedPromotion: step 1 runs promotion
// recovery (REQ-0087) on each <name>-env/ under the root, whatever the
// targets: a first promotion killed after its step 1, whose candidate
// is gone, is abandoned, so its render.json is removed, and the command
// stops with interrupted-promotion-abandoned (RJ-304, exit 4), naming
// the dir, its hint the sync of the project. A dir that is not
// <name>-env/ is not read.
func TestLockAndCheckStopsOnAnAbandonedPromotion(t *testing.T) {
	h := newHost(t)
	h.acknowledge(t)
	record := h.interrupt(t, "shop")
	other := filepath.Join(h.root, "notes", ".romeu", "render.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o700))
	require.NoError(t, os.WriteFile(other, []byte("not a record"), 0o600))
	r := h.run(flags{names: []string{"blog"}})
	t.Cleanup(r.release)

	err := lockAndCheck(t.Context(), r)

	require.ErrorIs(t, err, render.ErrPromotionAbandoned)
	e := cli.Of(err)
	assert.Equal(t, InterruptedPromotionAbandoned.ID, e.ID)
	assert.Equal(t, 4, cli.ExitCode(err))
	assert.Contains(t, err.Error(), "shop-env")
	assert.Equal(t, "romeu sync shop", e.Hint())
	assert.NoFileExists(t, record, "recovery left the render.json of the abandoned first promotion")
	assert.FileExists(t, other)
}

// promoting writes into <root>/<name>-env/ a first promotion killed
// after its step 1 whose candidate still verifies, which recovery
// finishes, and returns the candidate's sbxenv.yaml.
func (h *host) promoting(t *testing.T, name string) []byte {
	t.Helper()
	envDir := filepath.Join(h.root, name+"-env")
	id, err := canon.ParseULID("01m5mre3vz48a3n74a11ataacn")
	require.NoError(t, err)
	cand := filepath.Join(envDir, render.CandidatesDir, id.String())
	require.NoError(t, os.MkdirAll(cand, 0o700))
	env := []byte("schema: \"1\"\n")
	require.NoError(t, os.WriteFile(filepath.Join(cand, render.CandidateEnvFile), env, 0o600))
	bin, err := render.NewBin(runtime.GOARCH, embeddedJulieta())
	require.NoError(t, err)
	sum := sha256.Sum256(env)
	rec := render.Record{
		Schema:     render.RecordSchema,
		SpecSource: render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/cfg/main", Commit: strings.Repeat("c", 40)},
		Files:      map[string]string{render.LiveEnvFile: hex.EncodeToString(sum[:])},
	}
	maps.Copy(rec.Files, bin.Files())
	require.NoError(t, render.WriteRecord(render.Candidate{Dir: cand}, rec))
	digest, err := render.CandidateFilesDigest(cand)
	require.NoError(t, err)
	rec.Files = map[string]string{}
	rec.Promoting = &render.Promoting{CandidateID: id, FilesDigest: digest, PreviousFiles: map[string]string{}}
	data, err := render.EncodeRecord(rec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(envDir, render.LiveRecord), data, 0o600))
	return env
}

// TestLockAndCheckFinishesAPromotionWhoseCandidateVerifies: recovery
// finishes an interrupted promotion whose candidate still verifies
// against its filesDigest (REQ-0087): the live sbxenv.yaml is the
// candidate's, .romeu/bin holds the julieta this romeu embeds, the live
// render.json holds no promoting, the candidate dir is gone, and a line
// on stderr says so.
func TestLockAndCheckFinishesAPromotionWhoseCandidateVerifies(t *testing.T) {
	h := newHost(t)
	h.acknowledge(t)
	env := h.promoting(t, "shop")
	envDir := filepath.Join(h.root, "shop-env")
	r := h.run(flags{names: []string{"blog"}})
	t.Cleanup(r.release)

	require.NoError(t, lockAndCheck(t.Context(), r))

	assert.Equal(t, env, mustRead(t, filepath.Join(envDir, render.LiveEnvFile)))
	assert.Equal(t, embeddedJulieta(), mustRead(t, filepath.Join(envDir, ".romeu", "bin", "julieta-linux-"+runtime.GOARCH)))
	rec, err := render.DecodeRecord(mustRead(t, filepath.Join(envDir, render.LiveRecord)))
	require.NoError(t, err)
	assert.Nil(t, rec.Promoting)
	assert.NoDirExists(t, filepath.Join(envDir, render.CandidatesDir, "01m5mre3vz48a3n74a11ataacn"))
	assert.Contains(t, h.stderr.String(), "finished the interrupted promotion of shop")
}

// TestLockAndCheckRecoversBeforeGate1: REQ-0335 orders step 1 as the
// lock with promotion recovery, then gate 1, so with no acknowledged
// toolchain an interrupted promotion is finished first, and then the
// command stops with toolchain-approval-required (RJ-302, exit 3).
//
//romeu:invariant I7 test
func TestLockAndCheckRecoversBeforeGate1(t *testing.T) {
	h := newHost(t)
	h.promoting(t, "shop")
	r := h.run(flags{all: true})
	t.Cleanup(r.release)

	err := lockAndCheck(t.Context(), r)

	require.Error(t, err)
	assert.Equal(t, "RJ-302", cli.Of(err).ID)
	assert.Equal(t, 3, cli.ExitCode(err))
	rec, rerr := render.DecodeRecord(mustRead(t, filepath.Join(h.root, "shop-env", render.LiveRecord)))
	require.NoError(t, rerr)
	assert.Nil(t, rec.Promoting, "recovery did not run before gate 1")
	assert.Contains(t, h.stderr.String(), "finished the interrupted promotion of shop")
}
