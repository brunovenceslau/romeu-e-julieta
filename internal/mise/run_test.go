// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// lookup returns a lookup over the map, as os.LookupEnv would.
func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := env[k]; return v, ok }
}

// TestEnviron is the environment of a mise run (04 4.3, REQ-0426, as
// amended on 2026-10-22): what is kept of the caller's, then the data,
// cache and state dirs below the home, the global and the system
// configuration pointed at paths that cannot exist, the variables of the
// list, MISE_PARANOID=1 and the trusted paths joined. The token, the XDG
// dirs and every MISE_ or GO variable of the caller but the kept few are
// dropped.
func TestEnviron(t *testing.T) {
	env, err := mise.Environ(lookup(map[string]string{
		"PATH": "/bin", "HOME": "/h", "HTTPS_PROXY": "http://p:1", "GITHUB_TOKEN": "t", "XDG_CONFIG_HOME": "/x", "XDG_DATA_HOME": "/x",
		"MISE_DATA_DIR": "/evil", "MISE_CACHE_DIR": "/evil", "MISE_GLOBAL_CONFIG_FILE": "/evil", "MISE_LOCKED": "0", "MISE_YES": "0",
		"MISE_TRUSTED_CONFIG_PATHS": "/evil", "MISE_ENV": "x", "GOFLAGS": "-x", "GOSUMDB": "off", "SECRET": "s",
	}), "/h", []string{"/w/api", "/w/web"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"PATH=/bin", "HOME=/h", "HTTPS_PROXY=http://p:1",
		"MISE_DATA_DIR=/h/.local/share/mise", "MISE_CACHE_DIR=/h/.cache/mise", "MISE_STATE_DIR=/h/.local/state/mise",
		"MISE_GLOBAL_CONFIG_FILE=/dev/null/julieta-global.toml", "MISE_SYSTEM_CONFIG_FILE=/dev/null/julieta-system.toml",
		"MISE_LOCKED=1", "MISE_DISABLE_UPDATE_WARNING=1", "MISE_USE_VERSIONS_HOST_TRACK=false",
		"MISE_AUTO_INSTALL=false", "MISE_YES=1", "MISE_PARANOID=1", "MISE_TRUSTED_CONFIG_PATHS=/w/api:/w/web",
	}, env)
}

// TestEnvironNoTrust: with no repo, nothing is trusted.
func TestEnvironNoTrust(t *testing.T) {
	env, err := mise.Environ(lookup(nil), "/h", nil)
	require.NoError(t, err)
	assert.Contains(t, env, "MISE_TRUSTED_CONFIG_PATHS=")
}

// TestEnvironRefusesAPathItCannotTrust: a repo path with a colon, which
// MISE_TRUSTED_CONFIG_PATHS would read as two paths, a relative path and
// one with a control character reach no environment (the audit of
// 2026-10-22).
func TestEnvironRefusesAPathItCannotTrust(t *testing.T) {
	for _, bad := range []string{"/w/a:/etc", "w/api", "/w/a\nb", "/w/a\x00b", ""} {
		_, err := mise.Environ(lookup(nil), "/h", []string{"/w/ok", bad})
		require.Error(t, err, "%q", bad)
		_, err = mise.New(lookup(nil), "/h", []string{bad})
		require.Error(t, err, "%q", bad)
	}
}

// fakeMise puts a program named mise first on PATH that runs script,
// and returns the driver for it. The script gets the working directory
// the run chose and the argv as $@.
func fakeMise(t *testing.T, script string) *mise.Driver {
	t.Helper()
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "mise"), []byte("#!/bin/sh\n"+script+"\n"), 0o700))
	path := bin + ":" + os.Getenv("PATH")
	t.Setenv("PATH", path)
	d, err := mise.New(lookup(map[string]string{"PATH": path, "HOME": "/h"}), "/h", []string{"/w/api"})
	require.NoError(t, err)
	return d
}

// TestInstallRuns proves the call: argv, working directory and
// environment of a run of mise install.
func TestInstallRuns(t *testing.T) {
	repo := t.TempDir()
	rec := filepath.Join(t.TempDir(), "rec")
	d := fakeMise(t, `{ echo "argv: $@"; pwd; env | sort; } > '`+rec+`'`)
	require.NoError(t, d.Install(t.Context(), mise.Target{Dir: repo, Repo: "api"}))
	b, err := os.ReadFile(rec)
	require.NoError(t, err)
	got := string(b)
	assert.Contains(t, got, "argv: install\n")
	realRepo, err := filepath.EvalSymlinks(repo)
	require.NoError(t, err)
	assert.Contains(t, got, realRepo+"\n")
	for _, want := range []string{"MISE_LOCKED=1", "MISE_YES=1", "MISE_AUTO_INSTALL=false", "MISE_DISABLE_UPDATE_WARNING=1", "MISE_USE_VERSIONS_HOST_TRACK=false", "MISE_PARANOID=1", "MISE_GLOBAL_CONFIG_FILE=/dev/null/julieta-global.toml", "MISE_TRUSTED_CONFIG_PATHS=/w/api"} {
		assert.Contains(t, got, want+"\n")
	}
}

// TestLockRuns proves mise lock gets --platform with the list given.
func TestLockRuns(t *testing.T) {
	rec := filepath.Join(t.TempDir(), "rec")
	d := fakeMise(t, `echo "$@" > '`+rec+`'`)
	require.NoError(t, d.Lock(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api"}, []string{"linux-x64", "linux-arm64", "macos-arm64"}))
	b, err := os.ReadFile(rec)
	require.NoError(t, err)
	assert.Equal(t, "lock --platform linux-x64,linux-arm64,macos-arm64\n", string(b))
}

// TestInstallTimeout proves a mise that hangs is killed with its group
// and the run is subprocess-timeout (RJ-323), exit 1, naming the tool
// mise was working on (04 4.1, REQ-0300; 04 4.3). The limit is set
// through the seam of the timeout table.
func TestInstallTimeout(t *testing.T) {
	d := fakeMise(t, `echo "mise ✓ gh@2.102.0  1.0s"; echo "mise   jq@1.7.1"; sleep 30`)
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 300*time.Millisecond)
	start := time.Now()
	err := d.Install(ctx, mise.Target{Dir: t.TempDir(), Repo: "api"})
	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second, "the group was killed")
	e := cli.Of(err)
	assert.Equal(t, "RJ-323", e.ID)
	assert.Equal(t, 1, e.Exit)
	assert.Contains(t, e.Error(), "jq@1.7.1")
	assert.Contains(t, e.Details, cli.Detail{Key: "tool", Value: "jq@1.7.1"})
	assert.Contains(t, e.Details, cli.Detail{Key: "class", Value: "mise, limit 300ms"})
}

// TestInstallTimeoutNamesMise: with no progress line, the tool named is
// the program.
func TestInstallTimeoutNamesMise(t *testing.T) {
	d := fakeMise(t, `sleep 30`)
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 300*time.Millisecond)
	e := cli.Of(d.Install(ctx, mise.Target{Dir: t.TempDir(), Repo: "api"}))
	assert.Equal(t, "RJ-323", e.ID)
	assert.Contains(t, e.Details, cli.Detail{Key: "tool", Value: "mise"})
}

// blockedOutput is what mise 2026.10.3 prints for a download the proxy
// answers with 403, measured against a server that answers 403.
const blockedOutput = `mise by @jdx – installing 1 tool
mise ✗ http:foo@1.0  2ms · failed: HTTP status client error (403 Forbidden) for url (https://Example.ORG:443/foo.tar.gz)
mise ERROR Failed to install http:foo@1.0: HTTP status client error (403 Forbidden) for url (https://Example.ORG:443/foo.tar.gz)
mise ERROR Version: 2026.10.3 linux-arm64 (2026-10-05)
`

// tunnelOutput is what mise prints when a proxy refuses the CONNECT of a
// download: the host is in the retry warnings, not in the error line.
const tunnelOutput = `mise WARN  HTTP GET https://api.github.com/repos/cli/cli/releases/assets/599854680 attempt 1 failed after 714us (transient): error sending request; retrying in 156ms
mise WARN  HTTP GET https://api.github.com/repos/cli/cli/releases/assets/599854680 attempt 3 failed after 606us (transient): error sending request; retrying in 2.3s
mise ERROR Failed to install aqua:cli/cli@2.102.0: error sending request: client error (Connect): tunnel error: unsuccessful
mise ERROR Version: 2026.10.3 linux-arm64 (2026-10-05)
`

// TestBlocked reads the host and the tool out of the output, in the
// two forms mise prints a refusal.
func TestBlocked(t *testing.T) {
	b, ok := mise.Blocked(blockedOutput)
	require.True(t, ok)
	assert.Equal(t, mise.Block{Tool: "http:foo", Host: "example.org"}, b)

	b, ok = mise.Blocked(tunnelOutput)
	require.True(t, ok)
	assert.Equal(t, mise.Block{Tool: "aqua:cli/cli", Host: "api.github.com", Tunnel: true}, b)

	b, ok = mise.Blocked("mise ERROR Failed to install aqua:cli/cli@2.1: error sending request: client error (Connect): tunnel error: unsuccessful\n")
	require.True(t, ok)
	assert.Equal(t, mise.Block{Tool: "aqua:cli/cli", Tunnel: true}, b, "no retry line names the host")

	for _, out := range []string{
		"mise ERROR Failed to install gh@2.1: HTTP status client error (404 Not Found) for url (https://github.com/x)",
		"mise ERROR Failed to install gh@2.1: HTTP status client error (403 Forbidden) for url (not a url",
		"mise ERROR Failed to install gh@2.1: connection reset",
		"",
	} {
		_, ok = mise.Blocked(out)
		assert.False(t, ok, out)
	}
}

// TestInstallBlockedHostRule holds the host of a block to the one rule
// for a domain (03 3.1, spec.ValidDomain) before any command names it:
// a plain domain gets the sbx command, anything else gets none.
func TestInstallBlockedHostRule(t *testing.T) {
	target := mise.Target{Sandbox: "shop"}
	for _, ok := range []string{"github.com", "api.github.com", "a-b.example.org"} {
		e := mise.InstallBlockedError(mise.Block{Tool: "gh", Host: ok}, target)
		assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "on the host, sbx policy allow network --sandbox shop " + ok}, ok)
	}
	for _, bad := range []string{"x", "127.0.0.1", "a;b.com", "a b.com", "a&b", "a'b", "a*b", "a$b", "a!b", "a=b", "-a.com", "a-.com", "a..com", ".a.com", "a.com.", "A.com", "ü.com", "a/b", "a:443", strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 130) + "com"} {
		e := mise.InstallBlockedError(mise.Block{Tool: "gh", Host: bad}, target)
		assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "not offered: the host is not a plain domain name"}, bad)
	}
}

// TestInstallBlockedSandboxRule holds the sandbox name to the form of a
// project name (03 3.1) before the command names it: a name that is not
// one gets no command, as a host that is no domain does.
func TestInstallBlockedSandboxRule(t *testing.T) {
	for _, bad := range []string{"", "Shop", "a;b", "a b", "-shop", "shop-", "a", "$(id)", "a'b", strings.Repeat("a", 41)} {
		e := mise.InstallBlockedError(mise.Block{Tool: "gh", Host: "github.com"}, mise.Target{Sandbox: bad})
		assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "not offered: the sandbox name is not a project name"}, bad)
	}
}

// TestInstallBlocked proves a blocked download is install-blocked
// (RJ-341), exit 1, with the details of 04 4.3: the tool, the host, the
// sbx command that approves it once, that the approval is lost at
// recreate, the durable fix and whether the host is in the egress set;
// the message says what mise saw, since a 403 is not proof of the policy.
func TestInstallBlocked(t *testing.T) {
	d := fakeMise(t, "cat <<'EOF'\n"+blockedOutput+"EOF\nexit 1")
	for _, tt := range []struct {
		name   string
		egress []string
		in     string
	}{
		{"not in the set", []string{"github.com"}, "is not in the egress set"},
		{"in the set", []string{"github.com", "example.org"}, "is in the egress set"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api", Sandbox: "shop", Egress: tt.egress})
			e := cli.Of(err)
			assert.Equal(t, "RJ-341", e.ID)
			assert.Equal(t, 1, e.Exit)
			assert.Contains(t, e.Message, "HTTP 403 from example.org")
			assert.Contains(t, e.Message, "if that is the egress policy")
			assert.Contains(t, e.Details, cli.Detail{Key: "tool", Value: "http:foo"})
			assert.Contains(t, e.Details, cli.Detail{Key: "host", Value: "example.org"})
			assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "on the host, sbx policy allow network --sandbox shop example.org"})
			var all []string
			for _, d := range e.Details {
				all = append(all, d.Key+": "+d.Value)
			}
			text := strings.Join(all, "\n")
			assert.Contains(t, text, tt.in)
			assert.Contains(t, text, "lost when the sandbox is recreated")
			assert.Contains(t, text, "egress.extra")
			assert.Contains(t, text, "gate 2")
		})
	}
}

// TestInstallBlockedTunnel: a proxy that refuses the tunnel is the same
// error, with the message of that form.
func TestInstallBlockedTunnel(t *testing.T) {
	d := fakeMise(t, "cat <<'EOF'\n"+tunnelOutput+"EOF\nexit 1")
	e := cli.Of(d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api", Sandbox: "shop"}))
	assert.Equal(t, "RJ-341", e.ID)
	assert.Contains(t, e.Message, "refused the tunnel to api.github.com")
	assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "on the host, sbx policy allow network --sandbox shop api.github.com"})
}

// TestInstallBlockedOffersNoCommandForAnOddHost: a host that is no plain
// domain, or none at all, is shown and no command is built from it.
func TestInstallBlockedOffersNoCommandForAnOddHost(t *testing.T) {
	d := fakeMise(t, `echo "mise ERROR Failed to install http:foo@1.0: HTTP status client error (403 Forbidden) for url (https://a;touch%20x.example.org/f)" >&2; exit 1`)
	e := cli.Of(d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api", Sandbox: "shop"}))
	assert.Equal(t, "RJ-341", e.ID)
	for _, d := range e.Details {
		assert.NotContains(t, d.Value, "sbx policy allow", "no command for a host that is no domain")
	}
	assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "not offered: the host is not a plain domain name"})

	d = fakeMise(t, `echo "mise ERROR Failed to install aqua:x@1: error sending request: client error (Connect): tunnel error: unsuccessful" >&2; exit 1`)
	e = cli.Of(d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api", Sandbox: "shop"}))
	assert.Contains(t, e.Details, cli.Detail{Key: "approve once", Value: "not offered: mise named no host, so read the sbx approval queue"})
}

// TestInstallBlockedPastTheHead: the line of the refusal is read where
// mise prints it, after more than the 4 KiB the details keep.
func TestInstallBlockedPastTheHead(t *testing.T) {
	d := fakeMise(t, "i=0; while [ $i -lt 400 ]; do echo \"mise  progress line number $i of the output of an install\" >&2; i=$((i+1)); done\ncat >&2 <<'EOF'\n"+blockedOutput+"EOF\nexit 1")
	e := cli.Of(d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api", Sandbox: "shop"}))
	assert.Equal(t, "RJ-341", e.ID)
}

// TestInstallFailure: any other failure is mise-failed (RJ-351), exit 1,
// whose details hold the argv, the status and the start of mise's
// output, and whose message holds none of them.
func TestInstallFailure(t *testing.T) {
	d := fakeMise(t, `echo "mise ERROR boom" >&2; exit 3`)
	err := d.Install(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api"})
	e := cli.Of(err)
	assert.Equal(t, "RJ-351", e.ID)
	assert.Equal(t, 1, e.Exit)
	assert.NotContains(t, e.Message, "boom")
	assert.Contains(t, e.Details, cli.Detail{Key: "argv", Value: "mise install"})
	assert.Contains(t, e.Details, cli.Detail{Key: "status", Value: "exited 3"})
	assert.Contains(t, e.Details, cli.Detail{Key: "stderr", Value: "mise ERROR boom"})

	d = fakeMise(t, `echo "mise ERROR boom" >&2; exit 3`)
	e = cli.Of(d.Lock(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api"}, []string{"linux-x64", "linux-arm64"}))
	assert.Equal(t, "RJ-351", e.ID)
	assert.Contains(t, e.Details, cli.Detail{Key: "argv", Value: "mise lock --platform linux-x64,linux-arm64"})
}

// TestInstallTimeoutDoneMarker: a tool marked done is not the one mise
// hung on, so with no other the tool named is mise.
func TestInstallTimeoutDoneMarker(t *testing.T) {
	d := fakeMise(t, `echo "mise ✓ gh@2.102.0  1.0s"; sleep 30`)
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 300*time.Millisecond)
	e := cli.Of(d.Install(ctx, mise.Target{Dir: t.TempDir(), Repo: "api"}))
	assert.Equal(t, "RJ-323", e.ID)
	assert.Contains(t, e.Details, cli.Detail{Key: "tool", Value: "mise"})
}
