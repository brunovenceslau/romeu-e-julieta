// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// procgroup, which starts the go command here, builds on unix only.
//go:build unix

package canon

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// buildLimit bounds one "go build" of a fixture. The run goes through
// procgroup, the one package that starts a subprocess; no class of its
// table is the go command's, so the test takes the class without a
// timeout and gives it this limit through the seam of the table.
const buildLimit = 5 * time.Minute

// fixturePrelude opens every fixture package: one type of the kind
// memory-content and one of the kind fingerprint.
const fixturePrelude = `
import "github.com/brunovenceslau/romeu-e-julieta/internal/canon"

type entry struct {
	Title string ` + "`json:\"title\"`" + `
}

func (entry) CanonKind() canon.MemoryContentKind { return canon.MemoryContent }

type fingerprint struct {
	Title string ` + "`json:\"title\"`" + `
}

func (fingerprint) CanonKind() canon.FingerprintKind { return canon.Fingerprint }
`

// fixtures are the bodies of the fixture packages, each after the
// prelude. The control mixes no kinds; each other one mixes two once.
// They live in strings, not under testdata, since every Go file of the
// repository is linted and a fixture that does not compile cannot be.
var fixtures = map[string]string{
	// Each value digested under its own kind, each digest compared with
	// one of its kind.
	"onekind": `
func Same() (bool, error) {
	m, err := canon.Digest(canon.MemoryContent, entry{})
	if err != nil {
		return false, err
	}
	f, err := canon.Digest(canon.Fingerprint, fingerprint{})
	if err != nil {
		return false, err
	}
	var g canon.Sum[canon.FingerprintKind] = f
	return m == m && f == g, nil
}
`,
	// A memory-content value digested as a fingerprint.
	"valuekind": `
func Mixed() error {
	_, err := canon.Digest(canon.Fingerprint, entry{})
	return err
}
`,
	// A memory-content digest compared with a fingerprint digest.
	"comparedigests": `
func Mixed() bool {
	m, _ := canon.Digest(canon.MemoryContent, entry{})
	f, _ := canon.Digest(canon.Fingerprint, fingerprint{})
	return m == f
}
`,
	// A memory-content digest stored where a fingerprint digest goes.
	"assigndigest": `
func Mixed() canon.Sum[canon.FingerprintKind] {
	m, _ := canon.Digest(canon.MemoryContent, entry{})
	return m
}
`,
}

// goBuild runs "go build" on the fixture package name and returns what
// the go command wrote, and its error. The package is a directory of
// this package's that does not exist on disk: an overlay (go help
// build, -overlay) puts the fixture's file there, written to a
// temporary directory, so the fixture may import this internal package
// and the source tree stays untouched. The go command is the one "go
// test" puts first in PATH, and GOPROXY=off keeps it off the network.
func goBuild(t *testing.T, name string) (string, error) {
	t.Helper()
	tmp := t.TempDir()
	src := filepath.Join(tmp, name+".go")
	body := "package " + name + "\n" + fixturePrelude + fixtures[name]
	require.NoError(t, os.WriteFile(src, []byte(body), 0o600))
	wd, err := os.Getwd()
	require.NoError(t, err)
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(wd, "kindmix", name, name+".go"): src},
	})
	require.NoError(t, err)
	overlayFile := filepath.Join(tmp, "overlay.json")
	require.NoError(t, os.WriteFile(overlayFile, overlay, 0o600))

	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Unbounded, buildLimit)
	var out bytes.Buffer
	err = procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Unbounded,
		Argv:   []string{"go", "build", "-overlay", overlayFile, "./kindmix/" + name},
		Env:    append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=readonly"),
		Stdout: &out,
		Stderr: &out,
	})
	return out.String(), err
}

// TestKindMixDoesNotCompile is the compile-fail harness of the kinds:
// a fixture package that mixes two kinds fails "go build" with the
// type error of that mix, and the control fixture, which mixes none,
// builds, so a harness that fails every build for another reason is
// caught.
func TestKindMixDoesNotCompile(t *testing.T) {
	t.Parallel()
	t.Run("the control builds", func(t *testing.T) {
		t.Parallel()
		out, err := goBuild(t, "onekind")
		require.NoError(t, err, out)
	})
	tests := []struct {
		fixture string
		msgs    []string
	}{
		{"valuekind", []string{"valuekind.go:", "T (type entry) does not satisfy canon.Input[K]", "have CanonKind() canon.MemoryContentKind"}},
		{"comparedigests", []string{"comparedigests.go:", "mismatched types canon.Sum[canon.MemoryContentKind] and canon.Sum[canon.FingerprintKind]"}},
		{"assigndigest", []string{"assigndigest.go:", "cannot use m", "canon.Sum[canon.FingerprintKind] value in return statement"}},
	}
	assert.Len(t, fixtures, len(tests)+1, "each fixture has a case")
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			out, err := goBuild(t, tt.fixture)
			var exit *procgroup.ExitError
			require.ErrorAs(t, err, &exit, out)
			for _, msg := range tt.msgs {
				assert.Contains(t, out, msg)
			}
		})
	}
}
