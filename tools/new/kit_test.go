// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kitFrontend = "docker/sandbox-kit:3.0.0-m.11@sha256:adbc0d1ea63451871d5d78111120bb29f2fd2b265e63d39c664ae2c3d722c805"

// kitFixture is newFixture with a pin file that pins the frontend.
func kitFixture(t *testing.T, pinFile string) fixture {
	t.Helper()
	f := newFixture(t)
	f.repo.Write("kits/pins.yaml", pinFile)
	return f
}

// TestNewKitWritesTheDescriptor is the acceptance case: kits/<name>/
// holds <name>.yaml, whose first line is the frontend of the pin file
// (source: sitting M0 for the file name and the line).
func TestNewKitWritesTheDescriptor(t *testing.T) {
	f := kitFixture(t, "frontend:\n  ref: "+kitFrontend+"\n")
	require.Equal(t, exitOK, run(f.e, []string{"kit", "julieta"}), f.stderr.String())
	assert.Equal(t, "kits/julieta/julieta.yaml\n", f.stdout.String())
	want := "# syntax=" + kitFrontend + "\n" +
		"# SPDX-FileCopyrightText: 2026 Bruno Venceslau\n" +
		"# " + spdxLicense + "\n" +
		"schemaVersion: \"3\"\n" +
		"kind: mixin\n" +
		"displayName: julieta\n" +
		"description: TODO\n"
	assert.Equal(t, want, f.read(t, "kits/julieta/julieta.yaml"))
}

// TestNewKitRefuses checks the arguments and the state it refuses: a
// bad name, a kit that exists, no pin file, a pin file with no frontend.
func TestNewKitRefuses(t *testing.T) {
	pinned := "frontend:\n  ref: " + kitFrontend + "\n"
	tests := []struct {
		name string
		args []string
		pin  string // "" for no pin file
		pre  func(f fixture)
		want string
	}{
		{"no name", []string{"kit"}, pinned, nil, "kit takes the name as one argument"},
		{"two names", []string{"kit", "a", "b"}, pinned, nil, "kit takes the name as one argument"},
		{"an upper-case name", []string{"kit", "Julieta"}, pinned, nil, "is not lower-case letters"},
		{"a name with a slash", []string{"kit", "a/b"}, pinned, nil, "is not lower-case letters"},
		{"a name that reads as a flag", []string{"kit", "-x"}, pinned, nil, "is not lower-case letters"},
		{"a parent directory", []string{"kit", ".."}, pinned, nil, "is not lower-case letters"},
		{"a kit that exists", []string{"kit", "julieta"}, pinned, func(f fixture) { f.repo.Write("kits/julieta/kit.yaml", "x\n") }, "kits/julieta exists already"},
		{"no pin file", []string{"kit", "julieta"}, "", nil, "the frontend pin is read from kits/pins.yaml"},
		{"no frontend in the pin file", []string{"kit", "julieta"}, "herdr:\n  version: 0.9.5\n  sha256:\n    linux-amd64: " + hex64 + "\n    linux-arm64: " + hex64 + "\n", nil, "pins no frontend"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.pin != "" {
				f.repo.Write("kits/pins.yaml", tt.pin)
			}
			if tt.pre != nil {
				tt.pre(f)
			}
			assert.Equal(t, exitError, run(f.e, tt.args))
			assert.Contains(t, f.stderr.String(), tt.want)
			assert.Empty(t, f.stdout.String())
		})
	}
}

// spdxLicense is split so reuse does not read it as a tag of this file.
const spdxLicense = "SPDX-" + "License-Identifier: GPL-3.0-only"

const hex64 = "0000000000000000000000000000000000000000000000000000000000000000"
