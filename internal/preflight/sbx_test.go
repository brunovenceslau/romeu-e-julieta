// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package preflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// hostSbx returns the real adapter over a script that stands for sbx: it
// answers sbx version, and prints the two listings from the bodies given.
// The listing bodies are the synthetic fixtures (rule 10 of docs/plan.md)
// until probe A9 records the real ones. A body of "!" makes the call fail.
func hostSbx(t *testing.T, secrets, policies []byte) *HostSbx {
	t.Helper()
	dir := t.TempDir()
	put := func(name string, body []byte) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, body, 0o600))
		return p
	}
	body := "case \"$*\" in\n" +
		"  version) echo 'sbx version v" + sbxdrv.Floor + " 0123456789abcdef0123456789abcdef01234567' ;;\n" +
		"  'secret ls') cat '" + put("secrets", secrets) + "' ;;\n" +
		"  'policy ls --json') cat '" + put("policies", policies) + "' ;;\n" +
		"  *) echo \"unexpected call: $*\" >&2; exit 64 ;;\n" +
		"esac\n"
	sbx := filepath.Join(dir, "sbx")
	require.NoError(t, os.WriteFile(sbx, []byte("#!/bin/sh\n"+body), 0o755))
	r := &sbxdrv.Runner{Sbx: sbx, Home: dir, Term: "xterm"}
	_, err := r.RequireFloor(t.Context(), sbxdrv.Opts{})
	require.NoError(t, err)
	return &HostSbx{Runner: r}
}

// TestHostSbxFailsTheSecretCheck holds the acceptance of T168 end to end:
// through the real adapter, the runner and a script for sbx, a listing
// with a global secret for a service the project uses fails step 5 with
// sbx-global-secret (RJ-329), and an empty listing passes it.
func TestHostSbxFailsTheSecretCheck(t *testing.T) {
	tests := []struct {
		name    string
		secrets string
		wantID  string
	}{
		{"a global secret for a used service", "secrets-global-used.txt", "RJ-329"},
		{"a listing with no global row", "secrets-no-global.txt", ""},
		{"a listing with a header and no row", "=" + secretHeader + "\n", ""},
		{"an output with nothing at all", "secrets-empty.txt", "RJ-310"},
		{"a shape that is not recorded", "secrets-changed-container.txt", "RJ-310"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := (&trace{}).deps(t)
			body := []byte(strings.TrimPrefix(tt.secrets, "="))
			if !strings.HasPrefix(tt.secrets, "=") {
				body = synthetic(t, tt.secrets)
			}
			d.Sbx = hostSbx(t, body, synthetic(t, "policies-recorded.json"))
			_, err := RunSteps(t.Context(), d, quietHost(t), 5, 5)
			if tt.wantID == "" {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, tt.wantID, err)
		})
	}
}

// TestHostSbxReadsThePolicies shows the policy listing goes through the
// same adapter: a broad global allow rule fails step 5 with
// sbx-broad-allow (RJ-330).
func TestHostSbxReadsThePolicies(t *testing.T) {
	d := (&trace{}).deps(t)
	d.Sbx = hostSbx(t, synthetic(t, "secrets-no-global.txt"), synthetic(t, "policies-broad-doublestar.json"))
	_, err := RunSteps(t.Context(), d, quietHost(t), 5, 5)
	testsupport.RequireID(t, "RJ-330", err)
}

// TestHostSbxFailedListingIsSbxUnknown shows that a listing sbx fails to
// give is sbx-unknown (RJ-308), never an empty list.
func TestHostSbxFailedListingIsSbxUnknown(t *testing.T) {
	h := hostSbx(t, nil, nil)
	dir := filepath.Dir(h.Runner.Sbx)
	// The runner has passed the floor check, so it does not read sbx
	// version again: the script is replaced by one that fails the listings.
	failing := filepath.Join(dir, "sbx")
	require.NoError(t, os.WriteFile(failing, []byte("#!/bin/sh\n[ \"$*\" = version ] && { echo 'sbx version v"+sbxdrv.Floor+" 0123456789abcdef0123456789abcdef01234567'; exit 0; }\necho down >&2\nexit 1\n"), 0o755))
	for _, call := range []func() ([]byte, error){
		func() ([]byte, error) { return h.SecretList(t.Context()) },
		func() ([]byte, error) { return h.PolicyList(t.Context()) },
	} {
		out, err := call()
		assert.Empty(t, out)
		testsupport.RequireID(t, sbxdrv.SbxUnknown.ID, err)
		assert.NotContains(t, err.Error(), "down", "no output of sbx in the message")
	}
}
