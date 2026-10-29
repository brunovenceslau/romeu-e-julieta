// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

const (
	testDir     = "/abs/root/shop-env"
	testJulieta = "/abs/root/shop-env/.romeu/bin/julieta-linux-arm64"
	testHelper  = "/usr/bin/security find-generic-password -w -s github-shop"
)

// builtCalls are the calls of every builder, as the table of the
// argv shapes of 11.1 lists them: the argv a builder gives for fixed
// values, its shape, its timeout class and its mode.
func builtCalls(t *testing.T) []struct {
	name        string
	argv, shape []string
	class       procgroup.Class
	mode        Mode
} {
	t.Helper()
	must := func(c Call, err error) Call {
		require.NoError(t, err)
		return c
	}
	calls := []Call{
		Version(),
		List(),
		must(EnvPlan(testDir)),
		must(EnvCreate(testDir)),
		must(EnvStart(testDir)),
		must(EnvExec(testDir, procgroup.JulietaExec, nil, []string{testJulieta, "version", "--json"})),
		must(EnvExec(testDir, procgroup.JulietaSetup, []string{"JULIETA_MANIFEST=e30"}, []string{"/home/agent/.local/bin/julieta", "setup", "--json"})),
		must(EnvExec(testDir, procgroup.Unbounded, []string{"JULIETA_MANIFEST=e30"}, []string{"/home/agent/.local/bin/julieta", "salvage", "--json"})),
		must(EnvExecTTY(testDir, []string{"JULIETA_MANIFEST=e30"}, []string{"/home/agent/.local/bin/julieta", "layout", "up"})),
		must(EnvExecTTY(testDir, nil, []string{"/bin/bash", "-l"})),
		must(Stop("shop")),
		must(EnvRm(testDir)),
		must(PolicyAllowNetwork("shop", "github.com")),
		must(PolicyCheckNetwork("shop", "github.com")),
		must(PolicyRmNetwork("shop", "github.com")),
		PolicyList(),
		SecretList(),
		must(SecretSet("github", "shop", testHelper, "")),
		must(SecretSet("github", "shop", testHelper, "55m")),
		must(SettingsSet("env.rememberHostCommands", "true")),
	}
	var out []struct {
		name        string
		argv, shape []string
		class       procgroup.Class
		mode        Mode
	}
	for _, c := range calls {
		out = append(out, struct {
			name        string
			argv, shape []string
			class       procgroup.Class
			mode        Mode
		}{c.Name, c.Argv, c.Shape, c.Class, c.Mode})
	}
	return out
}

// TestBuilderArgv holds the argv, the shape, the timeout class and the
// mode of each builder against the table of 04 4.1 (Starting values):
// the interactive calls are sbx env run -d [--clone] and the env exec -it
// of the run layout and the shell, and every other call is captured.
func TestBuilderArgv(t *testing.T) {
	want := []struct {
		name        string
		argv, shape []string
		class       procgroup.Class
		mode        Mode
	}{
		{"version", []string{"version"}, []string{"version"}, procgroup.SbxRead, Captured},
		{"ls", []string{"ls", "--json"}, []string{"ls", "--json"}, procgroup.SbxRead, Captured},
		{"env-plan", []string{"env", "plan", testDir}, []string{"env", "plan", "<dir>"}, procgroup.SbxRead, Captured},
		{"env-run-create", []string{"env", "run", "-d", "--clone", testDir}, []string{"env", "run", "-d", "--clone", "<dir>"}, procgroup.SbxCreate, Interactive},
		{"env-run-start", []string{"env", "run", "-d", testDir}, []string{"env", "run", "-d", "<dir>"}, procgroup.SbxStart, Interactive},
		{"env-exec", []string{"env", "exec", testDir, "--", testJulieta, "version", "--json"}, []string{"env", "exec", "<dir>", "--", "<command...>"}, procgroup.JulietaExec, Captured},
		{"env-exec", []string{"env", "exec", testDir, "--env", "JULIETA_MANIFEST=e30", "--", "/home/agent/.local/bin/julieta", "setup", "--json"}, []string{"env", "exec", "<dir>", "--env", "<env>", "--", "<command...>"}, procgroup.JulietaSetup, Captured},
		{"env-exec", []string{"env", "exec", testDir, "--env", "JULIETA_MANIFEST=e30", "--", "/home/agent/.local/bin/julieta", "salvage", "--json"}, []string{"env", "exec", "<dir>", "--env", "<env>", "--", "<command...>"}, procgroup.Unbounded, Captured},
		{"env-exec-tty", []string{"env", "exec", "-it", testDir, "--env", "JULIETA_MANIFEST=e30", "--", "/home/agent/.local/bin/julieta", "layout", "up"}, []string{"env", "exec", "-it", "<dir>", "--env", "<env>", "--", "<command...>"}, procgroup.Unbounded, Interactive},
		{"env-exec-tty", []string{"env", "exec", "-it", testDir, "--", "/bin/bash", "-l"}, []string{"env", "exec", "-it", "<dir>", "--", "<command...>"}, procgroup.Unbounded, Interactive},
		{"stop", []string{"stop", "shop"}, []string{"stop", "<name>"}, procgroup.SbxChange, Captured},
		{"env-rm", []string{"env", "rm", "--force", testDir}, []string{"env", "rm", "--force", "<dir>"}, procgroup.SbxChange, Captured},
		{"policy-allow-network", []string{"policy", "allow", "network", "--sandbox", "shop", "github.com"}, []string{"policy", "allow", "network", "--sandbox", "<name>", "<domain>"}, procgroup.SbxChange, Captured},
		{"policy-check-network", []string{"policy", "check", "network", "--sandbox", "shop", "github.com"}, []string{"policy", "check", "network", "--sandbox", "<name>", "<domain>"}, procgroup.SbxChange, Captured},
		{"policy-rm-network", []string{"policy", "rm", "network", "--sandbox", "shop", "--resource", "github.com"}, []string{"policy", "rm", "network", "--sandbox", "<name>", "--resource", "<domain>"}, procgroup.SbxChange, Captured},
		{"policy-ls", []string{"policy", "ls", "--json"}, []string{"policy", "ls", "--json"}, procgroup.SbxRead, Captured},
		{"secret-ls", []string{"secret", "ls"}, []string{"secret", "ls"}, procgroup.SbxRead, Captured},
		{"secret-set", []string{"secret", "set", "github", "--sandbox", "shop", "--command", testHelper}, []string{"secret", "set", "<service>", "--sandbox", "<name>", "--command", "<command>"}, procgroup.SbxChange, Captured},
		{"secret-set", []string{"secret", "set", "github", "--sandbox", "shop", "--command", testHelper, "--refresh", "55m"}, []string{"secret", "set", "<service>", "--sandbox", "<name>", "--command", "<command>", "--refresh", "<duration>"}, procgroup.SbxChange, Captured},
		{"settings-set", []string{"settings", "set", "env.rememberHostCommands", "true"}, []string{"settings", "set", "<key>", "<value>"}, procgroup.SbxChange, Captured},
	}
	got := builtCalls(t)
	require.Len(t, got, len(want))
	for i := range want {
		t.Run(want[i].name+" "+strings.Join(want[i].shape, " "), func(t *testing.T) {
			assert.Equal(t, want[i].name, got[i].name)
			assert.Equal(t, want[i].argv, got[i].argv)
			assert.Equal(t, want[i].shape, got[i].shape)
			assert.Equal(t, want[i].class, got[i].class)
			assert.Equal(t, want[i].mode, got[i].mode)
		})
	}
}

// TestRegistryHasEveryBuilder holds that the registry of the builders,
// which the session test and the scans read, gives every shape the
// builders above give: a builder that is not registered would escape the
// scans (REQ-0996).
func TestRegistryHasEveryBuilder(t *testing.T) {
	registered := map[string]bool{}
	for _, c := range examples(defaultFixture) {
		registered[shapeKey(c.Shape)] = true
	}
	for _, c := range builtCalls(t) {
		assert.True(t, registered[shapeKey(c.shape)], "the shape of %s is in no registered example: %q", c.name, c.shape)
	}
}

// TestBuildersRefuseWhatTheirPlaceholderRefuses holds that no string
// becomes an argv element without its validator (REQ-0271).
func TestBuildersRefuseWhatTheirPlaceholderRefuses(t *testing.T) {
	cmd := []string{testJulieta, "version"}
	tests := []struct {
		name string
		call func() (Call, error)
	}{
		{"a relative dir", func() (Call, error) { return EnvPlan("shop-env") }},
		{"an unclean dir", func() (Call, error) { return EnvPlan("/abs/../shop-env") }},
		{"a dir with a newline", func() (Call, error) { return EnvCreate("/abs/a\nb") }},
		{"an empty dir", func() (Call, error) { return EnvStart("") }},
		{"a flag as a dir", func() (Call, error) { return EnvRm("--force") }},
		{"a name in upper case", func() (Call, error) { return Stop("Shop") }},
		{"a name with a space", func() (Call, error) { return Stop("my shop") }},
		{"an empty name", func() (Call, error) { return Stop("") }},
		{"a flag as a name", func() (Call, error) { return Stop("--all") }},
		{"a wildcard domain", func() (Call, error) { return PolicyAllowNetwork("shop", "*.example.com") }},
		{"a domain with a port", func() (Call, error) { return PolicyCheckNetwork("shop", "example.com:443") }},
		{"an address as a domain", func() (Call, error) { return PolicyRmNetwork("shop", "10.0.0.1") }},
		{"all hosts", func() (Call, error) { return PolicyAllowNetwork("shop", "**") }},
		{"a domain that is a flag", func() (Call, error) { return PolicyAllowNetwork("shop", "--sandbox") }},
		{"a sandbox that is not a name", func() (Call, error) { return PolicyAllowNetwork("a b", "github.com") }},
		{"a service in upper case", func() (Call, error) { return SecretSet("GitHub", "shop", testHelper, "") }},
		{"a service that is a flag", func() (Call, error) { return SecretSet("--token", "shop", testHelper, "") }},
		{"a relative command", func() (Call, error) { return SecretSet("github", "shop", "gh auth token", "") }},
		{"an empty command", func() (Call, error) { return SecretSet("github", "shop", "", "") }},
		{"a command with a newline", func() (Call, error) { return SecretSet("github", "shop", "/bin/a\n/bin/b", "") }},
		{"a refresh without a unit", func() (Call, error) { return SecretSet("github", "shop", testHelper, "55") }},
		{"a refresh of zero", func() (Call, error) { return SecretSet("github", "shop", testHelper, "0m") }},
		{"a setting key in upper case", func() (Call, error) { return SettingsSet("Env", "true") }},
		{"a setting key that is a flag", func() (Call, error) { return SettingsSet("--x", "true") }},
		{"a setting value with a space", func() (Call, error) { return SettingsSet("a.b", "x y") }},
		{"an empty setting value", func() (Call, error) { return SettingsSet("a.b", "") }},
		{"a class that is not an exec's", func() (Call, error) { return EnvExec(testDir, procgroup.SbxRead, nil, cmd) }},
		{"a class that is none", func() (Call, error) { return EnvExec(testDir, 0, nil, cmd) }},
		{"a lower case variable", func() (Call, error) { return EnvExec(testDir, procgroup.JulietaExec, []string{"k=v"}, cmd) }},
		{"a variable with no value", func() (Call, error) { return EnvExec(testDir, procgroup.JulietaExec, []string{"K"}, cmd) }},
		{"a variable with a newline", func() (Call, error) { return EnvExecTTY(testDir, []string{"K=a\nb"}, cmd) }},
		{"no command", func() (Call, error) { return EnvExec(testDir, procgroup.JulietaExec, nil, nil) }},
		{"an empty word", func() (Call, error) { return EnvExec(testDir, procgroup.JulietaExec, nil, []string{testJulieta, ""}) }},
		{"a NUL in the command", func() (Call, error) { return EnvExecTTY(testDir, nil, []string{"a\x00b"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.call()
			require.Error(t, err)
			assert.Equal(t, Call{}, c, "a refused call is the zero Call")
		})
	}
}

// TestSecretSetIsOnlyEverTheCommandForm holds I25 for the builder: for
// every value it accepts, the call carries --command and none of the
// flags that bring a value (REQ-0509).
func TestSecretSetIsOnlyEverTheCommandForm(t *testing.T) {
	for _, service := range []string{"github", "openai", "a"} {
		for _, refresh := range []string{"", "on-demand", "55m", "2h", "30s"} {
			c, err := SecretSet(service, "shop", testHelper, refresh)
			require.NoError(t, err)
			require.NoError(t, Guard(c))
			assert.Contains(t, c.Argv, "--command")
			for _, banned := range []string{"-t", "--token", "--password-stdin", "--oauth", "--ref", "--force"} {
				assert.NotContains(t, c.Argv, banned)
			}
			assert.NotEqual(t, "-", c.Argv[len(c.Argv)-1])
		}
	}
}

// TestNoCallCarriesAnApproval holds I4 over the builders: no call has
// --auto-approve, -y, or --force on env run and env create; env rm has
// --force alone, which approves no plan.
func TestNoCallCarriesAnApproval(t *testing.T) {
	for _, c := range examples(defaultFixture) {
		assert.NotContains(t, c.Argv, "--auto-approve", c.Name)
		assert.NotContains(t, c.Argv, "-y", c.Name)
		if approvesPlan(c.Argv) {
			assert.NotContains(t, c.Argv, "--force", c.Name)
			assert.NotContains(t, c.Argv, "-f", c.Name)
		}
		if slices.Contains(c.Argv, "--force") {
			assert.Equal(t, "env-rm", c.Name, "--force is on env rm alone")
		}
	}
}

// TestPlaceholderTypesAgreeWithTheFake runs the one table of values
// (e2e/testdata/sbx/placeholder-cases.jsonl) through the placeholder
// types of this package; the fake sbx runs the same table through its
// own matchers and imports nothing from here, so the table is what holds
// the two lists of types equal.
func TestPlaceholderTypesAgreeWithTheFake(t *testing.T) {
	data, err := os.ReadFile("../../e2e/testdata/sbx/placeholder-cases.jsonl")
	require.NoError(t, err)
	byToken := map[string]Placeholder{}
	for _, p := range placeholders {
		byToken[p.Token] = p
	}
	n := 0
	for line := range bytes.Lines(data) {
		var c struct {
			Token string `json:"token"`
			Value string `json:"value"`
			Valid bool   `json:"valid"`
		}
		require.NoError(t, jsonv2.Unmarshal(line, &c))
		p, ok := byToken[c.Token]
		require.True(t, ok, c.Token)
		n++
		assert.Equal(t, c.Valid, p.Check(c.Value) == nil, "%s %q", c.Token, c.Value)
	}
	assert.Greater(t, n, 50)
}
