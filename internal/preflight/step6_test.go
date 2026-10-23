// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// sandboxAt is the sandbox of project "shop" at the workspace given.
func sandboxAt(workspace string) sbxdrv.Sandbox {
	return sbxdrv.Sandbox{
		Name: "shop", Status: "running", Workspaces: []string{workspace},
		ID: "00000000-0000-4000-8000-000000000001", CreatedAt: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
	}
}

// generations returns one generation per state given, oldest first.
func generations(states ...state.State) []state.Generation {
	out := make([]state.Generation, len(states))
	for i, s := range states {
		out[i] = state.Generation{State: s}
	}
	return out
}

// atPrimary is the sandbox of "shop" at its right workspace under root.
func atPrimary(root string) []sbxdrv.Sandbox {
	return []sbxdrv.Sandbox{sandboxAt(filepath.Join(root, "shop-env", "shop-api"))}
}

// identity runs step 6 alone for the command and the generations given,
// with the sandboxes list builds from the root of the input.
func identity(t *testing.T, cmd Command, list func(root string) []sbxdrv.Sandbox, gens []state.Generation) error {
	t.Helper()
	host := quietHost(t)
	host.Command, host.Generations = cmd, gens
	sbx := passingSbx(t)
	sbx.list = list(host.Root)
	d := (&trace{}).deps(t)
	d.Sbx = sbx
	_, err := RunSteps(t.Context(), d, host, 6, 6)
	return err
}

// TestPreflightStep6Identity shows I26 (REQ-0076, REQ-0511): an absent
// sandbox passes, and a present one needs an open generation and the
// workspace $ROMEU_ROOT/<name>-env/<primary>.
func TestPreflightStep6Identity(t *testing.T) {
	none := func(string) []sbxdrv.Sandbox { return nil }
	tests := []struct {
		name   string
		list   func(root string) []sbxdrv.Sandbox
		gens   []state.Generation
		wantID string
	}{
		{"an absent sandbox passes", none, nil, ""},
		{"an absent sandbox with an open generation passes", none, generations(state.GenerationOpen), ""},
		{"a sandbox of another name is not this project's", func(root string) []sbxdrv.Sandbox {
			s := atPrimary(root)[0]
			s.Name = "other"
			return []sbxdrv.Sandbox{s}
		}, nil, ""},
		{"an open generation and the right path pass", atPrimary, generations(state.GenerationOpen), ""},
		{"a salvaging generation counts as open", atPrimary, generations(state.GenerationSalvaging), ""},
		{"a foreign sandbox of the name, with no record", atPrimary, nil, "RJ-203"},
		{"only closed generations", atPrimary, generations(state.GenerationClosedRemoved, state.GenerationClosedLost), "RJ-203"},
		{"a path mismatch", func(root string) []sbxdrv.Sandbox {
			return []sbxdrv.Sandbox{sandboxAt(filepath.Join(root, "shop-env", "other-repo"))}
		}, generations(state.GenerationOpen), "RJ-203"},
		{"a workspace outside the root", func(string) []sbxdrv.Sandbox {
			return []sbxdrv.Sandbox{sandboxAt("/elsewhere/shop-env/shop-api")}
		}, generations(state.GenerationOpen), "RJ-203"},
		{"no workspace at all", func(string) []sbxdrv.Sandbox {
			s := sandboxAt("")
			s.Workspaces = nil
			return []sbxdrv.Sandbox{s}
		}, generations(state.GenerationOpen), "RJ-203"},
		{"the newest open generation decides, not an older closed one", atPrimary,
			generations(state.GenerationClosedLost, state.GenerationOpen), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := identity(t, CmdRun, tt.list, tt.gens)
			if tt.wantID == "" {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, tt.wantID, err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Contains(t, cli.Of(err).Hint(), "romeu adopt shop")
		})
	}
}

// TestPreflightStep6RemovalPending shows the table of REQ-0076: a
// removing generation with its sandbox present passes for rm, recreate
// and retire, and every other P command, salvage --from-host included,
// exits 2 with removal-pending (RJ-336); a removing generation with its
// sandbox absent passes for all.
func TestPreflightStep6RemovalPending(t *testing.T) {
	removing := generations(state.GenerationRemoving)
	commands := []struct {
		cmd    Command
		passes bool
	}{
		{CmdRm, true}, {CmdRecreate, true}, {CmdRetire, true},
		{CmdSync, false}, {CmdRun, false}, {CmdStop, false}, {CmdSalvage, false}, {CmdSalvageFromHost, false}, {CmdPull, false},
	}
	for _, c := range commands {
		t.Run(string(c.cmd)+" with the sandbox present", func(t *testing.T) {
			err := identity(t, c.cmd, atPrimary, removing)
			if c.passes {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-336", err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Contains(t, cli.Of(err).Hint(), "romeu rm shop")
		})
		t.Run(string(c.cmd)+" with the sandbox absent", func(t *testing.T) {
			require.NoError(t, identity(t, c.cmd, func(string) []sbxdrv.Sandbox { return nil }, removing))
		})
	}
	t.Run("a path mismatch is an identity failure before the removal", func(t *testing.T) {
		err := identity(t, CmdSync, func(string) []sbxdrv.Sandbox { return []sbxdrv.Sandbox{sandboxAt("/elsewhere")} }, removing)
		testsupport.RequireID(t, "RJ-203", err)
	})
}

// TestPreflightStep6ReadsSbxFailures shows that a listing sbx cannot give
// stops the step as it is, never as an absent sandbox (REQ-0083).
func TestPreflightStep6ReadsSbxFailures(t *testing.T) {
	host := quietHost(t)
	sbx := passingSbx(t)
	sbx.listErr = sbxdrv.SbxUnknown.New("sbx ls --json failed")
	d := (&trace{}).deps(t)
	d.Sbx = sbx
	_, err := RunSteps(t.Context(), d, host, 6, 6)
	testsupport.RequireID(t, "RJ-308", err)
}
