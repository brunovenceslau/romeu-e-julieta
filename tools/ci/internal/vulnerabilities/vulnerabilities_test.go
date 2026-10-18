// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package vulnerabilities

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	assert.Equal(t, []check.Step{{Name: "vulnerabilities", Argv: []string{real, "./..."}, Environ: env, NoticeOn: fetchFailed, Notice: notice}}, Steps(t.Context(), plan))
}

// TestProgressFetchFailed holds REQ-0831: when govulncheck cannot fetch
// the database, the failed step says that this all is not the
// equivalent run, and it still fails. A fake govulncheck prints what the
// real one prints behind an unreachable proxy.
func TestProgressFetchFailed(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "govulncheck")
	script := "#!/bin/sh\necho 'govulncheck: fetching vulnerabilities: Get \"https://vuln.go.dev/index/modules.json.gz\": dial tcp: lookup vuln.go.dev: no such host' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o700))
	step := Steps(t.Context(), check.Plan{Root: t.TempDir()})[0]
	step.Unavailable, step.Argv, step.Environ = nil, []string{fake, "./..."}, os.Environ()

	var out bytes.Buffer
	c := check.NewPrinterAt(&out, func() time.Time { return time.Unix(0, 0) })
	c.Run(t.Context(), t.TempDir(), []check.Step{step})
	assert.False(t, c.OK, "the step still fails")
	assert.Contains(t, out.String(), "FAIL  vulnerabilities (0s)\n")
	assert.Contains(t, out.String(), "vulnerabilities: could not fetch the vulnerability database, so this all is not the equivalent run (10 10.2)\n")

	// A finding is not a failed fetch, and the prefix counts only at the
	// start of a line.
	out.Reset()
	step.Argv = []string{"sh", "-c", "echo 'see govulncheck: fetching vulnerabilities: in a log'; exit 1"}
	c = check.NewPrinterAt(&out, func() time.Time { return time.Unix(0, 0) })
	c.Run(t.Context(), t.TempDir(), []check.Step{step})
	assert.NotContains(t, out.String(), "so this all is not the equivalent run")
}
