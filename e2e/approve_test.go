// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/moduleroot"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// question is the question of gate 1, the last text before the answer.
const question = "acknowledge this toolchain? [y/N] "

// approveArgs is the command line of gate 1.
var approveArgs = []string{"approve", "--toolchain"}

// approveSetup is an Env with host settings and the fake sbx replaying
// the handshake of the recorded sbx 0.49.2, which answers sbx version.
func approveSetup(t *testing.T) *romeu.Env {
	t.Helper()
	e := romeu.New(t, nil)
	e.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, e.SetSbxSession(session))
	return e
}

// sbxSession points the fake sbx at a session whose sbx version is
// version, written for the test: no recording set holds a version
// outside the tested window, which is what it stands for.
func sbxSession(t *testing.T, e *romeu.Env, version string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "version.jsonl")
	data := `{"session":"version","sbxVersion":"` + version + `","capturedAt":"2026-10-23","synthetic":true,` +
		`"source":"written by e2e/approve_test.go","description":"sbx version only"}` + "\n" +
		`{"argv":["version"],"exit":0,"stdout":"sbx version v` + version + ` 0123456789abcdef\n","stderr":""}` + "\n"
	require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
	require.NoError(t, e.SetSbxSession(p))
}

// toolchainRecord is the part of toolchain.json the tests read.
type toolchainRecord struct {
	Schema     string `json:"schema"`
	SbxVersion string `json:"sbxVersion"`
	Romeu      struct {
		Version string `json:"version"`
		SHA256  string `json:"sha256"`
	} `json:"romeu"`
	Julieta struct {
		Version  string `json:"version"`
		Protocol int    `json:"protocol"`
		SHA256   string `json:"sha256"`
	} `json:"julieta"`
	CatalogDigest string         `json:"catalogDigest"`
	Catalog       map[string]any `json:"catalog"`
}

// readToolchain reads toolchain.json of the Env.
func readToolchain(t *testing.T, e *romeu.Env) toolchainRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.StateDir(), "toolchain.json"))
	require.NoError(t, err)
	var r toolchainRecord
	require.NoError(t, json.Unmarshal(data, &r))
	return r
}

// approve acknowledges the toolchain of the Env and returns the run.
func approve(t *testing.T, e *romeu.Env) romeu.Result {
	t.Helper()
	res := e.RunTerminal(t, question, "y\n", approveArgs)
	require.Equal(t, 0, res.Code, res.Stderr)
	return res
}

// TestApproveNeedsATerminal: gate 1 is TTY-only and stdin decides the
// TTY (01 1.4, REQ-0059; 04 4.1, REQ-0272): with a pipe on stdin, a "y"
// in it, the command exits 2 before it calls sbx, and writes nothing.
func TestApproveNeedsATerminal(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)

	res := e.RunStdin(t, "y\n", approveArgs)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "no-tty")
	assert.NotContains(t, res.Stderr, question)
	assert.Empty(t, e.SbxCalls(t), "no sbx call without a terminal")
	assert.NoFileExists(t, filepath.Join(e.StateDir(), "toolchain.json"))
}

// TestApproveWritesToolchainJSON: on a terminal, gate 1 prints the
// toolchain with the tested window to stderr, never stdout, before the
// prompt, and on y writes toolchain.json (03 3.8, REQ-0228). It reads
// sbx version and makes no other sbx call (04 4.2, REQ-0310); the
// source build records a dev version, v0.0.0-dev+<commit> (REQ-0065).
func TestApproveWritesToolchainJSON(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)

	res := approve(t, e)

	assert.Contains(t, res.Stderr, "gate 1: the toolchain of this machine\n")
	assert.Contains(t, res.Stderr, "  sbx 0.49.2 (none recorded) (newest recorded 0.49.2; tested window 0.49.2)\n")
	assert.Contains(t, res.Stderr, "\n  julieta v0.0.0-placeholder (none recorded), protocol 1 (none recorded), sha256 e3b0c44298fc (none recorded)\n")
	assert.Contains(t, res.Stderr, "\n    tool core:go (new): ")
	assert.True(t, strings.HasSuffix(res.Stderr, question), "the prompt comes last, on stderr: %q", res.Stderr)
	assert.NotContains(t, res.Stdout, question)
	assert.Contains(t, res.Stdout, "acknowledged  "+filepath.Join(e.StateDir(), "toolchain.json"))

	r := readToolchain(t, e)
	assert.Equal(t, "state-toolchain/v1", r.Schema)
	assert.Equal(t, "0.49.2", r.SbxVersion)
	assert.True(t, strings.HasPrefix(r.Romeu.Version, "v0.0.0-dev+"), r.Romeu.Version)
	assert.Len(t, r.Romeu.SHA256, 64)
	assert.Equal(t, 1, r.Julieta.Protocol)
	assert.Len(t, r.CatalogDigest, 64)
	assert.Contains(t, r.Catalog, "tools")
	for _, call := range e.SbxCalls(t) {
		assert.Equal(t, []string{"version"}, call)
	}
}

// TestApproveDeclined: any answer but y declines, and toolchain.json is
// not written.
func TestApproveDeclined(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)

	res := e.RunTerminal(t, question, "yes\n", approveArgs)

	assert.Equal(t, 0, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "declined: toolchain.json is unchanged")
	assert.NoFileExists(t, filepath.Join(e.StateDir(), "toolchain.json"))
}

// TestApproveOncePerBuild: a dev build is acknowledged once per build
// per machine (REQ-0065): the same build again finds nothing to ask,
// and another build of the same tree asks again, in one line, since
// julieta and the catalog are unchanged.
func TestApproveOncePerBuild(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)
	approve(t, e)
	first := readToolchain(t, e)

	again := e.RunTerminal(t, "", "", approveArgs)
	assert.Equal(t, 0, again.Code, again.Stderr)
	assert.Contains(t, again.Stdout, "acknowledged already")
	assert.Equal(t, first, readToolchain(t, e))

	e.Romeu = buildRomeu(t, "-X main.commit=feedface")
	res := approve(t, e)
	prompt := res.Stderr[:strings.Index(res.Stderr, question)+len(question)]
	assert.NotContains(t, prompt, "\n", "with julieta and the catalog unchanged the prompt is one line")
	assert.Contains(t, prompt, "tested window 0.49.2")
	assert.Contains(t, prompt, " -> v0.0.0-dev+feedface, sha256 ")
	assert.Equal(t, "v0.0.0-dev+feedface", readToolchain(t, e).Romeu.Version)
}

// TestApproveSbxVersion: another sbx version flips gate 1; the prompt
// prints the live version beside the recorded one and the newest
// recorded, says it is outside the tested window, and marks a lower
// one downgrade (01 1.4, I7).
func TestApproveSbxVersion(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)
	approve(t, e)

	sbxSession(t, e, "0.50.0")
	res := approve(t, e)
	assert.Contains(t, res.Stderr, "gate 1: sbx 0.49.2 -> 0.50.0 (newest recorded 0.49.2; tested window 0.49.2; 0.50.0 is outside it, untested), romeu ")
	assert.Equal(t, "0.50.0", readToolchain(t, e).SbxVersion)

	sbxSession(t, e, "0.49.2")
	res = approve(t, e)
	assert.Contains(t, res.Stderr, "gate 1: sbx 0.50.0 -> 0.49.2 downgrade (newest recorded 0.49.2; tested window 0.49.2), romeu ")
	assert.Equal(t, "0.49.2", readToolchain(t, e).SbxVersion)
}

// TestApproveCatalogDiff: a catalog other than the acknowledged one
// prints the catalog diff, per backend:tool, before the prompt (07
// 7.6, REQ-0638). The acknowledged catalog is edited in the record, as
// the record of an older release would hold it.
func TestApproveCatalogDiff(t *testing.T) {
	t.Parallel()
	e := approveSetup(t)
	approve(t, e)
	path := filepath.Join(e.StateDir(), "toolchain.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	tools := doc["catalog"].(map[string]any)["tools"].(map[string]any)
	goDomains := tools["core:go"].([]any)
	require.NotEmpty(t, goDomains)
	removed := goDomains[0].(string)
	tools["core:go"] = goDomains[1:]
	doc["catalogDigest"] = strings.Repeat("0", 63) + "1"
	data, err = json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	res := approve(t, e)

	assert.Contains(t, res.Stderr, "\n  catalog 000000000000 -> ")
	assert.Contains(t, res.Stderr, "\n    tool core:go: + "+removed)
	assert.Contains(t, res.Stderr, "; projects: none\n")
	assert.NotEqual(t, strings.Repeat("0", 63)+"1", readToolchain(t, e).CatalogDigest)
}

// buildRomeu builds romeu from this tree with the linker flags ldflags,
// another build of the same source.
func buildRomeu(t *testing.T, ldflags string) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	root, err := moduleroot.Find(wd)
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "romeu")
	b, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{"go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/romeu"},
		Dir:   root,
		Env:   os.Environ(),
	})
	require.NoError(t, err, "%s", b)
	return out
}
