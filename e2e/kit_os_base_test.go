// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/container"
)

// The os-base kit (06 6.4, REQ-0593), read from kits/os-base/: its
// descriptor, and its install step run in the workload's base (10 10.1),
// on its own and, in TestKitJulietaInstall, after the julieta kit's.

// The paths and the probe the cases use. debianSources is the workload's
// apt source file the step reads; mirrorProbe is an index of it that
// tells a mirror that cannot be reached (an infrastructure error, Q27)
// from a step that fails.
const (
	debianSources = "/etc/apt/sources.list.d/debian.sources"
	mirrorProbe   = "http://deb.debian.org/debian/dists/trixie/InRelease"
)

// a12Packages is the set A12 measured on 2026-10-23: the packages the
// pinned workload lacks.
var a12Packages = []string{"tzdata", "zsh"}

// installedPackages returns the packages of the apt-get install line of
// script, the words after its last option.
func installedPackages(t *testing.T, script string) []string {
	t.Helper()
	var found [][]string
	for line := range strings.SplitSeq(script, "\n") {
		f := strings.Fields(line)
		i := slices.Index(f, "install")
		if i < 0 || !slices.ContainsFunc(f[:i], func(w string) bool { return strings.HasSuffix(w, "apt-get") }) {
			continue
		}
		var pkgs []string
		for _, w := range f[i+1:] {
			if !strings.HasPrefix(w, "-") {
				pkgs = append(pkgs, w)
			}
		}
		found = append(found, pkgs)
	}
	require.Len(t, found, 1, "one apt-get install line")
	return found[0]
}

// TestKitOSBaseDescriptor holds what the descriptor declares: a mixin of
// schema 3 on the capabilities of sitting M0; the packages of A12 and
// no other; the arg tz with the default Etc/UTC carried as TZ (REQ-0593,
// 02 2.1); one root step in bash with -e and -u that reads the
// workload's Debian sources only; and an install-phase network policy
// that names each host of those sources, on the port of its scheme, and
// no other host.
func TestKitOSBaseDescriptor(t *testing.T) {
	_, d, src := readKit(t, "os-base")
	assert.Equal(t, "3", d.SchemaVersion)
	assert.Equal(t, "mixin", d.Kind)
	assert.Equal(t, "os-base", d.DisplayName)
	assert.Contains(t, d.Description, "unpinned", "the apt delta is not pinned (05 5.4)")
	first, _, _ := strings.Cut(string(src), "\n")
	assert.True(t, strings.HasPrefix(first, "# syntax=docker/sandbox-kit:3.0.0-m."), "line 1 is the frontend line: %q", first)

	var types []string
	for _, c := range d.Capabilities {
		types = append(types, c.Type)
	}
	assert.Equal(t, []string{"com.docker.sandbox/network-policy@1", "com.docker.sandbox/lifecycle@1"}, types)

	assert.Equal(t, "Etc/UTC", d.Args["tz"].Default)
	assert.Equal(t, "TZ", d.Args["tz"].Env)
	assert.NotEmpty(t, d.Args["tz"].Pattern)

	hooks := d.hooks(t)
	require.Len(t, hooks, 1)
	h := hooks[0]
	assert.Equal(t, "0", h.User, "apt runs as root")
	assert.Equal(t, []string{"bash", "-euc"}, h.Command[:2])
	script := h.Command[len(h.Command)-1]
	assert.NotContains(t, script, "${{", "the step pastes no kit arg into its script")
	assert.Contains(t, script, "Dir::Etc::SourceList="+debianSources)
	assert.Contains(t, script, "Dir::Etc::SourceParts=/dev/null", "no other source file is read")
	assert.Contains(t, script, "--error-on=any update", "an index that fails to download fails the step")
	assert.Equal(t, a12Packages, installedPackages(t, script), "the set A12 measured")

	// The hosts of the workload's Debian sources, read from the image.
	k := startKit(t)
	out, code := k.sh("0", "cat "+debianSources)
	require.Zero(t, code, out)
	var want []string
	for line := range strings.SplitSeq(out, "\n") {
		rest, ok := strings.CutPrefix(line, "URIs:")
		if !ok {
			continue
		}
		for raw := range strings.FieldsSeq(rest) {
			u, err := url.Parse(raw)
			require.NoError(t, err, raw)
			port := map[string]string{"http": "80", "https": "443"}[u.Scheme]
			require.NotEmpty(t, port, "the scheme of %s", raw)
			want = append(want, u.Hostname()+":"+port)
		}
	}
	slices.Sort(want)
	want = slices.Compact(want)
	require.NotEmpty(t, want, "the workload's Debian sources name a mirror")
	np := d.network(t)
	require.Len(t, np, 1, "one phase")
	require.Contains(t, np, "install", "the install phase only")
	assert.Equal(t, want, np["install"].Allow)
}

// runBase runs the os-base step, after a probe of the mirror that ends
// the case as an infrastructure error when the mirror cannot be reached.
func (k kitSandbox) runBase() {
	k.t.Helper()
	out, code := k.exec("0", nil, "curl", "-sS", "-o", "/dev/null", "--max-time", "60", mirrorProbe)
	if slices.Contains(transport, code) {
		container.Fail(k.t, &container.InfraError{Precondition: "the Debian mirror of the os-base kit", Err: fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out))})
	}
	k.runAll(k.base.hooks(k.t)...)
}

// assertZones holds that TZ names a real zone: the default of the arg tz
// prints UTC, and a zone with summer time prints one of its two
// abbreviations.
func assertZones(t *testing.T, k kitSandbox) {
	t.Helper()
	out, code := k.exec("1000", nil, "env", "TZ="+k.base.Args["tz"].Default, "date", "+%Z%z")
	require.Zero(t, code, out)
	assert.Equal(t, "UTC+0000\n", out, "the default of the arg tz")
	out, code = k.exec("1000", nil, "env", "TZ=Europe/Lisbon", "date", "+%Z")
	require.Zero(t, code, out)
	assert.Contains(t, []string{"WET\n", "WEST\n"}, out, "Europe/Lisbon")
}

// TestKitOSBaseInstall runs the step in a container of the workload and
// asserts what it leaves: the zone database, so TZ names a real zone,
// zsh, and no apt lists, as the image ships none.
func TestKitOSBaseInstall(t *testing.T) {
	k := startKit(t)
	_, code := k.sh("0", "test ! -e /usr/share/zoneinfo")
	require.Zero(t, code, "the workload carries tzdata now: A12 and os-base are read again")
	_, code = k.sh("0", "! command -v zsh")
	require.Zero(t, code, "the workload carries zsh now: A12 and os-base are read again")

	k.runBase()

	assertZones(t, k)
	out, code := k.exec("1000", nil, "zsh", "-c", "print -r -- $ZSH_VERSION")
	require.Zero(t, code, out)
	assert.NotEmpty(t, strings.TrimSpace(out))
	out, code = k.sh("0", "ls -A /var/lib/apt/lists")
	require.Zero(t, code, out)
	assert.Empty(t, strings.TrimSpace(out), "the apt lists are removed")
}

// TestKitOSBaseMirrorDown is the fault test of --error-on=any: with the
// mirror's name pointed at a closed port, apt-get update fails the step
// instead of going on with no index, and nothing is installed.
func TestKitOSBaseMirrorDown(t *testing.T) {
	k := startKit(t, "--add-host=deb.debian.org:127.0.0.1")
	hooks := k.base.hooks(t)
	require.Len(t, hooks, 1)
	out, code := k.run(hooks[0])
	assert.Equal(t, 100, code, "apt-get's status for an index that failed: %s", out)
	assert.Contains(t, out, "deb.debian.org")
	_, code = k.sh("0", "test ! -e /usr/share/zoneinfo")
	assert.Zero(t, code, "tzdata was installed with no index")
	_, code = k.sh("0", "! command -v zsh")
	assert.Zero(t, code, "zsh was installed with no index")
}
