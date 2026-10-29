// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// promoted returns the fixture of steps 7 and 8 after a first sync of
// shop, promoted with an approval, and a second sync rendered.
func promoted(t *testing.T) *gateFixture {
	t.Helper()
	f := gateRun(t)
	f.approve(t)
	f.rerender(t)
	return f
}

// TestCheckDriftPassesUntouchedLiveFiles: before the first promotion
// there is nothing live, and after it the live files are those
// render.json records, so step 7 passes and keeps no candidate from
// step 8.
func TestCheckDriftPassesUntouchedLiveFiles(t *testing.T) {
	t.Run("first sync", func(t *testing.T) {
		f := gateRun(t)

		require.NoError(t, checkDrift(t.Context(), f.r))

		assert.Len(t, f.candidates(t), 1)
		assert.Empty(t, f.r.overwritten)
	})
	t.Run("after a promotion", func(t *testing.T) {
		f := promoted(t)

		require.NoError(t, checkDrift(t.Context(), f.r))

		assert.Len(t, f.candidates(t), 1)
		assert.Empty(t, f.r.overwritten)
	})
}

// TestCheckDriftStopsAnEditedLiveFile: an edited, or missing, live
// file makes sync exit 4 with drift-detected (RJ-303) naming it, the
// hint filled with the project; the candidate is declined, so no
// candidate dir is left and nothing is promoted (REQ-0342).
func TestCheckDriftStopsAnEditedLiveFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		edit func(t *testing.T, p string)
	}{
		{"an edited sbxenv.yaml", render.LiveEnvFile, func(t *testing.T, p string) {
			t.Helper()
			require.NoError(t, os.WriteFile(p, []byte("edited: true\n"), 0o600))
		}},
		{"a missing julieta binary", render.BinDir + "/julieta-linux-", func(t *testing.T, p string) {
			t.Helper()
			matches, err := filepath.Glob(p + "*")
			require.NoError(t, err)
			require.Len(t, matches, 1)
			require.NoError(t, os.Remove(matches[0]))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := promoted(t)
			tt.edit(t, f.in(tt.path))
			before := f.outsideCandidates(t)

			err := checkDrift(t.Context(), f.r)

			e := cli.Of(err)
			require.NotNil(t, e, "%v", err)
			assert.Equal(t, cli.DriftDetected.ID, e.ID)
			assert.Equal(t, 4, cli.ExitCode(err))
			assert.Contains(t, e.Message, tt.path, "the error names the file")
			assert.Equal(t, "shop", e.Fill["<name>"])
			assert.Empty(t, f.candidates(t), "the candidate is declined")
			assert.Equal(t, state.CandidateDeclined, f.r.candidates[0].dir.State())
			assert.Equal(t, before, f.outsideCandidates(t), "the edited file is left as it is")
		})
	}
}

// TestOverwriteDriftReplacesTheEditedFile: with --overwrite-drift, step
// 7 lets the edited live sbxenv.yaml through, and step 8's promotion
// replaces it with the approved render and says its edit is lost.
func TestOverwriteDriftReplacesTheEditedFile(t *testing.T) {
	f := promoted(t)
	want := f.liveEnv(t)
	require.NoError(t, os.WriteFile(f.in(render.LiveEnvFile), []byte("edited: true\n"), 0o600))
	f.r.flags.overwriteDrift = true
	noTerminal(t)

	require.NoError(t, checkDrift(t.Context(), f.r))
	require.NoError(t, gateAndPromote(t.Context(), f.r))

	assert.Equal(t, []lostEdit{{project: "shop", path: render.LiveEnvFile}}, f.r.overwritten)
	assert.Equal(t, want, f.liveEnv(t), "the edit is replaced")
	assert.Equal(t, "replaced the edited sbxenv.yaml of shop; its edit is lost\n", warned(f.r), "a warning, which step 10 prints last")
	assert.NotContains(t, f.stderr.String(), "its edit is lost", "step 8 prints no warning itself")
}

// TestOverwriteDriftKeepsTheEditOnADecline: --overwrite-drift lets the
// edited live file through, and a changed widening declined with n
// promotes nothing, so the edit stays and nothing is reported replaced.
func TestOverwriteDriftKeepsTheEditOnADecline(t *testing.T) {
	f := promoted(t)
	edited := []byte("edited: true\n")
	require.NoError(t, os.WriteFile(f.in(render.LiveEnvFile), edited, 0o600))
	f.r.targets[0].Kits = []spec.Kit{{Product: "plain"}, {Product: "withcaps"}}
	f.rerender(t)
	f.r.flags.overwriteDrift = true
	require.NoError(t, checkDrift(t.Context(), f.r))

	_, err := f.gateOnPTY(t, "n\n")

	requireDeclined(t, err, "shop")
	assert.Equal(t, edited, f.liveEnv(t), "the edit stays")
	assert.Empty(t, warned(f.r), "nothing is reported replaced")
}

// TestCheckDriftEscapesTheFileName: a live render.json that names a
// path holding an ESC and a bidi control stops step 7 before any drift
// is reported (render refuses a path that is not clean), and the error
// holds neither byte (I29), so no drifted name of a record can reach a
// terminal raw.
//
//romeu:invariant I29 test
func TestCheckDriftEscapesTheFileName(t *testing.T) {
	f := promoted(t)
	data, err := os.ReadFile(f.in(render.LiveRecord))
	require.NoError(t, err)
	rec, err := render.DecodeRecord(data)
	require.NoError(t, err)
	rec.Files[".romeu/x\x1b[31m\u202e"] = plainSum([]byte("absent"))
	out, err := jsonv2.Marshal(rec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f.in(render.LiveRecord), out, 0o600))

	err = checkDrift(t.Context(), f.r)

	require.ErrorIs(t, err, render.ErrRecordInvalid, "render refuses the name before it can drift")
	assert.Contains(t, err.Error(), "shop", "the error names the project")
	assert.NotContains(t, err.Error(), "\x1b", "no ESC reaches the message")
	assert.NotContains(t, err.Error(), "\u202e", "no bidi control reaches the message")
	assert.Empty(t, f.candidates(t), "the candidate is declined")
}

// TestOverwriteDriftCannotReplaceAnEditedKitDir: a kit dir is verified,
// never rewritten (01 1.6), so --overwrite-drift lets an edited one
// through step 7 and the promotion refuses it (render.ErrKitDirDiffers,
// exit 1); the candidate is declined. Replacing the kit dir needs a
// change of internal/render.
func TestOverwriteDriftCannotReplaceAnEditedKitDir(t *testing.T) {
	f := promoted(t)
	kit := f.in(f.r.candidates[0].kits[0].LivePath() + "/files/run.sh")
	require.NoError(t, os.Chmod(kit, 0o755))
	require.NoError(t, os.WriteFile(kit, []byte("#!/bin/sh\necho edited\n"), 0o755))
	f.r.flags.overwriteDrift = true
	noTerminal(t)
	require.NoError(t, checkDrift(t.Context(), f.r))

	err := gateAndPromote(t.Context(), f.r)

	require.ErrorIs(t, err, render.ErrKitDirDiffers)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, f.candidates(t))
}
