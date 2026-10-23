// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// hostStepWith runs step 5 alone, with the secret and policy fixtures
// given and the host input, and returns its error.
func hostStepWith(t *testing.T, secrets, policies string, host Input) error {
	t.Helper()
	sbx := passingSbx(t)
	sbx.secrets, sbx.policies = synthetic(t, secrets), synthetic(t, policies)
	d := (&trace{}).deps(t)
	d.Sbx = sbx
	_, err := RunSteps(t.Context(), d, host, 5, 5)
	return err
}

// TestPreflightStep5SbxGlobals shows I22 (REQ-0506, REQ-0371, REQ-0372)
// over the synthetic fixtures: exit 2 on each shape, and a pass on a
// recording with no global row and on a global row that names no used
// service and no broad pattern.
func TestPreflightStep5SbxGlobals(t *testing.T) {
	tests := []struct {
		name     string
		secrets  string
		policies string
		wantID   string
	}{
		{"a recording with no global row passes", "secrets-no-global.txt", "policies-recorded.json", ""},
		{"a global secret for a service no project uses passes only because it names none", "secrets-global-unused.txt", "policies-global-placeholder.json", ""},
		{"a global secret for a used service", "secrets-global-used.txt", "policies-recorded.json", "RJ-329"},
		{"a secret scoped to two sandboxes counts as global", "secrets-scope-two-sandboxes.txt", "policies-recorded.json", "RJ-329"},
		{"a secret of a scope not recorded counts as global", "secrets-scope-unknown.txt", "policies-recorded.json", "RJ-329"},
		{"a global allow rule of **", "secrets-no-global.txt", "policies-broad-doublestar.json", "RJ-330"},
		{"a global allow rule over a whole top-level domain", "secrets-no-global.txt", "policies-broad-tld.json", "RJ-330"},
		{"a permissive default network policy", "secrets-no-global.txt", "policies-broad-default.json", "RJ-330"},
		{"a broad rule of a scope not recorded counts as global", "secrets-no-global.txt", "policies-broad-scope-unknown.json", "RJ-330"},
		{"a secret listing with a changed container shape", "secrets-changed-container.txt", "policies-recorded.json", "RJ-310"},
		{"an unparseable secret listing", "secrets-unparseable.txt", "policies-recorded.json", "RJ-310"},
		{"a policy listing with a changed container shape", "secrets-no-global.txt", "policies-changed-container.json", "RJ-310"},
		{"an unparseable policy listing", "secrets-no-global.txt", "policies-unparseable.json", "RJ-310"},
		{"a policy of a kind not recorded", "secrets-no-global.txt", "policies-unknown-kind.json", "RJ-310"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hostStepWith(t, tt.secrets, tt.policies, quietHost(t))
			if tt.wantID == "" {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, tt.wantID, err)
			assert.Equal(t, 2, cli.ExitCode(err))
		})
	}
	t.Run("the same global secret passes for a project that does not use the service", func(t *testing.T) {
		host := quietHost(t)
		host.UsedServices = []string{"svc-beta"}
		require.NoError(t, hostStepWith(t, "secrets-global-used.txt", "policies-recorded.json", host))
	})
	t.Run("the hint names the per-sandbox form and the service, with no placeholder left", func(t *testing.T) {
		err := hostStepWith(t, "secrets-global-used.txt", "policies-recorded.json", quietHost(t))
		hint := cli.Of(err).Hint()
		assert.Contains(t, hint, "sbx secret set svc-alpha --sandbox shop and the --command that uses it")
		assert.NotRegexp(t, `<[a-z]+>`, hint)
	})
	t.Run("the upstream-shape error names sbx and the recorded versions", func(t *testing.T) {
		err := hostStepWith(t, "secrets-unparseable.txt", "policies-recorded.json", quietHost(t))
		details := cli.Of(err).Details
		assert.Contains(t, details, cli.Detail{Key: "upstream", Value: "sbx"})
		assert.Contains(t, details, cli.Detail{Key: "recorded sbx versions", Value: "0.49.2"})
	})
	t.Run("a listing sbx cannot give stops the step as it is", func(t *testing.T) {
		sbx := &fakeSbx{}
		d := (&trace{}).deps(t)
		d.Sbx = sbx
		_, err := RunSteps(t.Context(), d, quietHost(t), 5, 5)
		assert.ErrorIs(t, err, errNoSbxStubs)
	})
}

// TestPreflightBroadPattern shows the patterns of sbx-broad-allow, the
// spellings that must not slip past it, and the resources it refuses.
func TestPreflightBroadPattern(t *testing.T) {
	tests := []struct {
		pattern string
		broad   bool
		invalid bool
	}{
		{pattern: "**", broad: true}, {pattern: "*", broad: true}, {pattern: "*.*", broad: true}, {pattern: "*.com", broad: true},
		{pattern: "**.io", broad: true}, {pattern: "*.COM", broad: true}, {pattern: " *.com ", broad: true},
		{pattern: "*.com:443", broad: true}, {pattern: "example.*", broad: true}, {pattern: "", broad: true},
		{pattern: "*.com.", broad: true}, {pattern: "**.", broad: true}, {pattern: "*.com.:443", broad: true},
		{pattern: "*.example.com"}, {pattern: "example.com"}, {pattern: "example.com."}, {pattern: "example.test:443"},
		{pattern: "api.example.test"}, {pattern: "com"}, {pattern: "localhost"},
		{pattern: "https://*.com", invalid: true}, {pattern: "https://**", invalid: true}, {pattern: "*.com/path", invalid: true},
		{pattern: "*.com:", invalid: true}, {pattern: "*.com:https", invalid: true}, {pattern: "a b.com", invalid: true},
		{pattern: "[::1]", invalid: true}, {pattern: "user@*.com", invalid: true},
	}
	for _, tt := range tests {
		t.Run(strconv.Quote(tt.pattern), func(t *testing.T) {
			broad, err := broadPattern(tt.pattern)
			if tt.invalid {
				testsupport.RequireID(t, "RJ-310", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.broad, broad)
		})
	}
}

// TestPreflightStep5RefusesAResourceItCannotJudge shows that a global
// allow rule that is not host[:port] fails the step with upstream-shape
// (RJ-310), not with a pass.
func TestPreflightStep5RefusesAResourceItCannotJudge(t *testing.T) {
	sbx := passingSbx(t)
	sbx.policies = []byte(`[{"scope":"global","kind":"rule","effect":"allow","resource":"https://*.com"}]`)
	d := (&trace{}).deps(t)
	d.Sbx = sbx
	_, err := RunSteps(t.Context(), d, quietHost(t), 5, 5)
	testsupport.RequireID(t, "RJ-310", err)
	t.Run("a rule of one sandbox is not judged", func(t *testing.T) {
		sbx.policies = []byte(`[{"scope":"sandbox:shop","kind":"rule","effect":"allow","resource":"https://*.com"}]`)
		_, err := RunSteps(t.Context(), d, quietHost(t), 5, 5)
		require.NoError(t, err)
	})
}

// TestPreflightHostFindings shows that doctor's reading returns every
// finding in the order of step 5, where the step stops at the first.
func TestPreflightHostFindings(t *testing.T) {
	host := quietHost(t)
	require.NoError(t, os.Symlink(host.Root, filepath.Join(host.Home, "dotfile")))
	sbx := passingSbx(t)
	sbx.secrets = synthetic(t, "secrets-global-used.txt")
	d := (&trace{}).deps(t)
	d.Sbx = sbx
	var ids []string
	findings, unchecked := HostFindings(t.Context(), d, host)
	for _, err := range findings {
		ids = append(ids, cli.Of(err).ID)
	}
	assert.Equal(t, []string{"RJ-329", "RJ-331"}, ids)
	assert.Equal(t, []string{UncheckedVSCodeFolders}, unchecked, "the folders of VS Code were not given, and the report says so")
	assert.Equal(t, []string{"sbx-global-secret", "sbx-broad-allow", "home-symlink", "auto-trust", "vscode-trust"},
		func() (slugs []string) {
			for _, c := range hostChecks {
				slugs = append(slugs, c.slug)
			}
			return slugs
		}())
}

// TestPreflightTouchesNoNetwork shows REQ-0077 and 10 10.1: no file of
// the package imports a package that opens a connection, starts a
// process or reads a registry.
func TestPreflightTouchesNoNetwork(t *testing.T) {
	banned := []string{"net", "net/http", "os/exec", "github.com/brunovenceslau/romeu-e-julieta/internal/oci"}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range parsed.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			require.NoError(t, err)
			assert.False(t, slices.Contains(banned, path) || strings.HasPrefix(path, "github.com/google/go-containerregistry"),
				"%s imports %s", f, path)
		}
	}
}

// TestPreflightStep5SaysWhatItDidNotCheck shows that a pass with no seam
// for the trusted folders of VS Code is reported as unchecked, and that
// a pass with the seam is not.
func TestPreflightStep5SaysWhatItDidNotCheck(t *testing.T) {
	d := (&trace{}).deps(t)
	res, err := Run(t.Context(), d, quietHost(t))
	require.NoError(t, err)
	assert.Equal(t, []string{"vscode-trusted-folders"}, res.Unchecked)
	d.VSCodeTrustedFolders = func() ([]string, error) { return nil, nil }
	res, err = Run(t.Context(), d, quietHost(t))
	require.NoError(t, err)
	assert.Empty(t, res.Unchecked)
	_, unchecked := HostFindings(t.Context(), d, quietHost(t))
	assert.Empty(t, unchecked)
}
