// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func herdrRoutes(t *testing.T, file string) map[string]route {
	t.Helper()
	return map[string]route{"/repos/herdrdev/herdr/releases/tags/v0.9.5": {body: fixture(t, "herdr/"+file)}}
}

// TestHerdrPinsTheDigests is the acceptance case: the hashes come from
// the API's digest fields of an immutable release.
func TestHerdrPinsTheDigests(t *testing.T) {
	h := newHarness(t, herdrRoutes(t, "v0.9.5.json"))
	require.Equal(t, exitOK, h.run(t, "herdr", "0.9.5"), h.stderr.String())
	assert.Equal(t, header()+`herdr:
  version: 0.9.5
  sha256:
    linux-amd64: 14d9e8bc8b4a9e0118d77c32266f76d461f09b40342eb23752960bd89f59929d
    linux-arm64: 17d7536f2da50859523680ef279dffcac8e4a235f4e46fe8da6c2ec44141897c
`, h.read(t, "kits/pins.yaml"))
}

// TestHerdrRefuses: a release not marked immutable, and a digest field
// that is missing or not a sha256, named.
func TestHerdrRefuses(t *testing.T) {
	cases := []struct{ name, file, want string }{
		{"mutable", "mutable.json", "is not marked immutable (immutable is false)"},
		{"no_immutable_field", "no-immutable.json", "has no field immutable"},
		{"no_digest_field", "no-digest.json", "assets[herdr-linux-x86_64].digest is missing"},
		{"sha512_digest", "sha512-digest.json", `assets[herdr-linux-aarch64].digest "sha512:`},
		{"no_arm64_asset", "no-arm64.json", "has no asset herdr-linux-aarch64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, herdrRoutes(t, c.file))
			h.repo.Write("kits/pins.yaml", startPins)
			assert.Equal(t, exitError, h.run(t, "herdr", "0.9.5"))
			assert.Contains(t, h.stderr.String(), c.want)
			assert.Equal(t, startPins, h.read(t, "kits/pins.yaml"))
		})
	}
}

func TestHerdrRefusesAnotherTag(t *testing.T) {
	h := newHarness(t, map[string]route{"/repos/herdrdev/herdr/releases/tags/v0.9.6": {body: fixture(t, "herdr/v0.9.5.json")}})
	assert.Equal(t, exitError, h.run(t, "herdr", "0.9.6"))
	assert.Contains(t, h.stderr.String(), `the release "v0.9.5", not v0.9.6`)
}

// TestHerdrRefusesAnAmbiguousImmutable: the decoder matches member names
// exactly and refuses a duplicate one, so neither a second immutable
// member nor one of another case can mark a release immutable.
func TestHerdrRefusesAnAmbiguousImmutable(t *testing.T) {
	good := fixture(t, "herdr/v0.9.5.json")
	cases := []struct{ name, old, new, want string }{
		{"duplicate", `"immutable": true`, `"immutable": false, "immutable": true`, `duplicate object member name "immutable"`},
		{"another_case", `"immutable": true`, `"Immutable": true`, "has no field immutable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := bytes.Replace(good, []byte(c.old), []byte(c.new), 1)
			require.NotEqual(t, string(good), string(body), "the case changes the fixture")
			h := newHarness(t, map[string]route{"/repos/herdrdev/herdr/releases/tags/v0.9.5": {body: body}})
			h.repo.Write("kits/pins.yaml", startPins)
			assert.Equal(t, exitError, h.run(t, "herdr", "0.9.5"))
			assert.Contains(t, h.stderr.String(), c.want)
			assert.Equal(t, startPins, h.read(t, "kits/pins.yaml"))
		})
	}
}
