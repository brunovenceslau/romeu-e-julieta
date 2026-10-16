// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package vulnerabilities

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// TestSteps checks that the vulnerabilities step runs the govulncheck
// mise resolves, with the proxy variables of the process, and that a
// govulncheck mise cannot resolve makes the step unavailable.
func TestSteps(t *testing.T) {
	for _, key := range check.NetworkPassThrough {
		t.Setenv(key, "from-the-process")
	}
	f := citest.NewFakeMise(t)
	vuln := f.Tool(t, mise.GovulncheckDir, "1.8.0", "bin", "govulncheck")
	root := citest.LockedRoot(t, citest.LockFixture)
	plan := check.Plan{Root: root, Tools: check.Tools{GoDir: "/pinned/bin"}}
	env := check.WithProcessEnv(check.StepEnv("/pinned/bin"), check.NetworkPassThrough)

	f.Script(t, "", "", "")
	steps := Steps(t.Context(), plan)
	require.Len(t, steps, 1)
	assert.Equal(t, "vulnerabilities", steps[0].Name)
	assert.Equal(t, env, steps[0].Environ)
	require.Error(t, steps[0].Unavailable, "mise resolves no govulncheck")
	assert.Empty(t, steps[0].Argv)

	f.Script(t, "case \"$4\" in\ngovulncheck) echo '"+vuln+"'; exit 0 ;;\nesac\n", "", "")
	real, err := filepath.EvalSymlinks(vuln)
	require.NoError(t, err)
	assert.Equal(t, []check.Step{{Name: "vulnerabilities", Argv: []string{real, "./..."}, Environ: env}}, Steps(t.Context(), plan))
}
