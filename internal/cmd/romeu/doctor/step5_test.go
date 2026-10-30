// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// synthetic returns the body of a synthesized listing of T046 under
// internal/preflight/testdata/synthetic, its first line, the synthetic
// marker, left out.
func synthetic(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "preflight", "testdata", "synthetic", name))
	require.NoError(t, err)
	first, rest, _ := strings.Cut(string(b), "\n")
	require.True(t, strings.HasPrefix(first, "# synthetic: true"), name)
	return rest
}

// usingAlpha gives the fixture a secret entry for the service svc-alpha,
// which the synthesized global secret names, with an argv that prints.
func usingAlpha(t *testing.T, f *fixture) {
	t.Helper()
	argv := filepath.Join(f.tmp, "bin", "token")
	require.NoError(t, os.WriteFile(argv, []byte("#!/bin/sh\necho value\n"), 0o700))
	f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{argv}}}
}

// TestDoctorI22Rows: the two I22 checks of step 5 fail, and doctor exits
// 6 with check-findings, on each synthesized fixture of T046, and pass on
// the recorded shapes; on the two shape fixtures both rows report
// upstream-shape and doctor exits 1 (REQ-0371, REQ-0372; RJ-310).
func TestDoctorI22Rows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, secrets, policies string
		want                    map[string]string
		exit                    int
	}{
		{"a global secret of a used service", "secrets-global-used.txt", "policies-recorded.json",
			map[string]string{slugGlobalSecret: resultFail, slugBroadAllow: resultPass}, 6},
		{"a global secret of a service no project uses", "secrets-global-unused.txt", "policies-recorded.json",
			map[string]string{slugGlobalSecret: resultPass, slugBroadAllow: resultPass}, 0},
		{"a global row of placeholder values", "secrets-no-global.txt", "policies-global-placeholder.json",
			map[string]string{slugGlobalSecret: resultPass, slugBroadAllow: resultPass}, 0},
		{"a global allow of **", "secrets-no-global.txt", "policies-broad-doublestar.json",
			map[string]string{slugGlobalSecret: resultPass, slugBroadAllow: resultFail}, 6},
		{"a wildcard over a top-level domain", "secrets-no-global.txt", "policies-broad-tld.json",
			map[string]string{slugGlobalSecret: resultPass, slugBroadAllow: resultFail}, 6},
		{"a permissive default policy", "secrets-no-global.txt", "policies-broad-default.json",
			map[string]string{slugGlobalSecret: resultPass, slugBroadAllow: resultFail}, 6},
		{"a changed container shape", "secrets-changed-container.txt", "policies-changed-container.json",
			map[string]string{slugGlobalSecret: resultError, slugBroadAllow: resultError}, 1},
		{"an unparseable body", "secrets-unparseable.txt", "policies-unparseable.json",
			map[string]string{slugGlobalSecret: resultError, slugBroadAllow: resultError}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			usingAlpha(t, f)
			f.sbx.secrets, f.sbx.policies = synthetic(t, tc.secrets), synthetic(t, tc.policies)

			rep, out, err := f.output(t)

			assert.Equal(t, tc.exit, exitOf(err), out)
			for slug, result := range tc.want {
				c := check(t, rep, slug)
				assert.Equal(t, result, c.Result, "%s: %s", slug, c.Message)
				if result == resultError {
					assert.Equal(t, "upstream-shape", c.Error, slug)
				}
			}
		})
	}
}

// TestDoctorI22RowsPlaceAShapeError: an upstream-shape of one listing is
// reported on the row of that listing, by the slug the preflight gives
// its finding, and the other row keeps its own result.
func TestDoctorI22RowsPlaceAShapeError(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	usingAlpha(t, f)
	f.sbx.secrets, f.sbx.policies = synthetic(t, "secrets-unparseable.txt"), synthetic(t, "policies-broad-doublestar.json")

	rep, out, err := f.output(t)

	assert.Equal(t, 1, exitOf(err), out)
	assert.Equal(t, "upstream-shape", check(t, rep, slugGlobalSecret).Error)
	assert.Equal(t, resultFail, check(t, rep, slugBroadAllow).Result)
}

// TestDoctorI23Rows: the three I23 checks of step 5 fail on a link from
// HOME into the root, a mise trust entry covering it and VS Code's
// workspace trust turned off, with HOME in a temporary directory
// (REQ-0378, REQ-0379, REQ-0380).
func TestDoctorI23Rows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, slug string
		setup      func(t *testing.T, f *fixture)
	}{
		{"a link from HOME into the root", slugHomeSymlink, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.Symlink(f.root, filepath.Join(f.home, "code")))
		}},
		{"a mise trust entry over the root", slugAutoTrust, func(t *testing.T, f *fixture) {
			t.Helper()
			f.env["MISE_TRUSTED_CONFIG_PATHS"] = f.root
		}},
		{"VS Code workspace trust turned off", slugVSCodeTrust, func(t *testing.T, f *fixture) {
			t.Helper()
			dir := filepath.Join(f.configHome, "Code", "User")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"security.workspace.trust.enabled": false}`), 0o600))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			tc.setup(t, f)

			rep, out, err := f.output(t)

			assert.Equal(t, 6, exitOf(err), out)
			for _, slug := range []string{slugHomeSymlink, slugAutoTrust, slugVSCodeTrust} {
				want := resultPass
				switch slug {
				case tc.slug:
					want = resultFail
				case slugVSCodeTrust:
					want = resultWarn
				}
				assert.Equal(t, want, check(t, rep, slug).Result, slug)
			}
		})
	}
}

// TestDoctorVSCodeTrustSaysWhatIsNotChecked: with workspace trust on,
// vscode-trust is a warning that says the trusted folders, which live in
// VS Code's state database, were not checked, and changes no exit
// (decided on 2026-10-30).
func TestDoctorVSCodeTrustSaysWhatIsNotChecked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rep, out, err := f.output(t)

	require.NoError(t, err, out)
	c := check(t, rep, slugVSCodeTrust)
	assert.Equal(t, resultWarn, c.Result)
	assert.Contains(t, c.Message, "not checked")
}

// TestDoctorKeepsAFindingWithNoRow: a finding of step 5 whose check has
// no row in doctor, such as a check the preflight gained first, is
// reported as an error under its slug, never dropped, and doctor exits 1.
func TestDoctorKeepsAFindingWithNoRow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.hostFindings = func(ctx context.Context, d preflight.Deps, in preflight.Input) ([]preflight.Finding, []string) {
		findings, unchecked := preflight.HostFindings(ctx, d, in)
		return append(findings, preflight.Finding{Slug: "signing-socket", Err: errors.New("the socket answers no key")}), unchecked
	}

	rep, out, err := f.output(t)

	assert.Equal(t, 1, exitOf(err), out)
	c := check(t, rep, "signing-socket")
	assert.Equal(t, resultError, c.Result)
	assert.Contains(t, c.Message, "the socket answers no key")
}
