// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// fixture returns the content of testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// TestDecodeProjectReadsTheExample decodes the example of 03 3.2.
func TestDecodeProjectReadsTheExample(t *testing.T) {
	p, err := spec.DecodeProject(fixture(t, "valid-project.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "shop", p.Name)
	require.Len(t, p.Repos, 2)
	assert.Equal(t, "shop-web", p.Repos[1].Dir, "a dir left out is the URL's base name")
	assert.Equal(t, []string{"github"}, p.Secrets)
	require.NotNil(t, p.Run)
	assert.InDelta(t, 0.6, p.Run.Tabs[0].Root.Ratio, 1e-9)
	assert.Equal(t, "web", p.Run.Tabs[0].Root.Second.Second.Pane)
	assert.Equal(t, [][]string{{"npm", "ci"}, {"npm", "run", "dev"}}, p.Run.Panes[2].Steps)
}

// TestDecodeHostSettingsReadsTheExample decodes the example of 03 3.4.
func TestDecodeHostSettingsReadsTheExample(t *testing.T) {
	h, err := spec.DecodeHostSettings(fixture(t, "valid-host-settings.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/example/my-config", h.Config.URL)
	assert.Equal(t, []string{"/opt/homebrew/bin/op", "read", "op://dev/npm/token"}, h.Secrets["npm@shop"].Argv)
}

// TestDecodeIsStrict refuses a fixture with a duplicate key and one with
// an unknown key, and a spec with command, argv or env under secrets
// (03, opening; I8).
//
//romeu:invariant I8 test
//romeu:invariant I12 test
func TestDecodeIsStrict(t *testing.T) {
	for _, name := range []string{
		"invalid-duplicate-key.yaml", "invalid-unknown-key.yaml",
		"invalid-secret-command.yaml", "invalid-secret-argv.yaml", "invalid-secret-env.yaml",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := spec.DecodeProject(fixture(t, name))
			assert.ErrorIs(t, err, spec.ErrInvalid)
		})
	}
}

// valid returns the example spec with old replaced by repl, once.
func valid(t *testing.T, old, repl string) []byte {
	t.Helper()
	s := string(fixture(t, "valid-project.yaml"))
	require.Equal(t, 1, strings.Count(s, old), "the example holds %q once", old)
	return []byte(strings.Replace(s, old, repl, 1))
}

// TestDecodeRefusesTheGrammarOutsideTheSubset refuses each construct of
// YAML the subset grammar leaves out (REQ-0168), a value of the wrong
// kind, a missing required key, a second document and a file over the
// cap.
func TestDecodeRefusesTheGrammarOutsideTheSubset(t *testing.T) {
	cases := map[string][]byte{
		"an anchor and an alias": valid(t, "agent: sbx-kit-claude\n", "agent: &a sbx-kit-claude\nports: [*a]\n"),
		"a merge key":            valid(t, "  cpus: 4\n", "  <<: {cpus: 4}\n"),
		"a tag":                  valid(t, "agent: sbx-kit-claude", "agent: !custom sbx-kit-claude"),
		"a null":                 valid(t, "agent: sbx-kit-claude", "agent: ~"),
		"a number for a string":  valid(t, "agent: sbx-kit-claude", "agent: 12"),
		"a string for a number":  valid(t, "  cpus: 4", "  cpus: \"4\""),
		"a missing key":          valid(t, "agent: sbx-kit-claude\n", ""),
		"a second document":      append(fixture(t, "valid-project.yaml"), []byte("---\nschema: project/v1\n")...),
		"over the cap":           append(fixture(t, "valid-project.yaml"), []byte("#"+strings.Repeat("x", spec.MaxFileSize))...),
		"not UTF-8":              valid(t, "agent: sbx-kit-claude", "agent: sbx-kit-\xff"),
		"not a mapping":          []byte("- schema: project/v1\n"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := spec.DecodeProject(data)
			assert.ErrorIs(t, err, spec.ErrInvalid)
		})
	}
}

// TestDecodeHostSettingsRefusesAnArgv refuses a relative or missing
// argv[0] and a shell string, and each token shape with secret-in-argv
// (RJ-314), one fixture per shape, at load (03 3.4, I25). The message
// never holds the element.
//
//romeu:invariant I25 test
func TestDecodeHostSettingsRefusesAnArgv(t *testing.T) {
	for _, name := range []string{"invalid-argv-relative.yaml", "invalid-argv-missing.yaml", "invalid-argv-shell.yaml"} {
		t.Run(name, func(t *testing.T) {
			_, err := spec.DecodeHostSettings(fixture(t, name))
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.NotEqual(t, "RJ-314", idOf(err))
		})
	}
	for _, shape := range []string{"ghp", "gho", "ghs", "ghu", "ghr", "github_pat", "hex40"} {
		name := "invalid-argv-token-" + shape + ".yaml"
		t.Run(name, func(t *testing.T) {
			data := fixture(t, name)
			_, err := spec.DecodeHostSettings(data)
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.Equal(t, "RJ-314", idOf(err))
			assert.NotContains(t, err.Error(), "EXAMPLE")
			assert.NotContains(t, err.Error(), "0123456789abcdef")
		})
	}
}

// idOf returns the id of the error of 04 4.4 that err is, or "".
func idOf(err error) string {
	if e, ok := errors.AsType[*cli.Error](err); ok {
		return e.ID
	}
	return ""
}

// TestDecodeRefusesMoreShapes refuses a file with no schema key, a
// duplicate key below the top, a key that is not a string, and a value
// of each wrong kind, each naming where it is without the value.
func TestDecodeRefusesMoreShapes(t *testing.T) {
	cases := map[string][]byte{
		"no schema key":          valid(t, "schema: project/v1\n", ""),
		"a nested duplicate key": valid(t, "    dir: shop-api\n", "    dir: shop-api\n    dir: shop-api\n"),
		"a key that is a number": valid(t, "  cpus: 4\n", "  4: cpus\n"),
		"a string for a boolean": valid(t, "    primary: true\n", "    primary: \"yes\"\n"),
		"a number for a number":  valid(t, "        ratio: 0.6\n", "        ratio: \"0.6\"\n"),
		"a mapping for a list":   valid(t, "secrets: [github]", "secrets: {github: x}"),
		"a list for a mapping":   valid(t, "  cpus: 4\n  memory: 8g\n", "  - 4\n"),
		"a list for a map":       valid(t, "    args: {tz: Etc/UTC}\n", "    args: [tz]\n"),
		"a long key twice":       valid(t, "agent: sbx-kit-claude\n", "agent: sbx-kit-claude\n"+strings.Repeat("é", 40)+": 1\n"+strings.Repeat("é", 40)+": 1\n"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := spec.DecodeProject(data)
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.LessOrEqual(t, len(err.Error()), 300, "a message stays short")
		})
	}
}

// TestLoadSetNamesTheFileOfAFinding sets the file of each finding of a
// spec that decodes and breaks a rule.
func TestLoadSetNamesTheFileOfAFinding(t *testing.T) {
	_, err := spec.LoadSet([]spec.File{{Path: "projects/shop.yaml", Data: valid(t, "secrets: [github]", "secrets: [GitHub]")}}, nil)
	var inv *spec.InvalidError
	require.ErrorAs(t, err, &inv)
	require.Len(t, inv.Findings, 1)
	assert.Equal(t, "projects/shop.yaml", inv.Findings[0].File)
	assert.Equal(t, "projects/shop.yaml: secrets[0]: is not a secret name (secret-name)", inv.Findings[0].String())
}
