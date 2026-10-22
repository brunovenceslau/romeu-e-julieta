// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// fixture reads testdata/ls/<name>.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "ls", name))
	require.NoError(t, err)
	return data
}

// TestParseListOK reads the documents the parser accepts, and skips the
// members it does not know, at any depth (ADR 0041).
func TestParseListOK(t *testing.T) {
	list, err := ParseList(fixture(t, "ok-empty.json"))
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list, "the array [] is an empty list")

	list, err = ParseList(fixture(t, "ok-one.json"))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, Sandbox{
		Name: "shop", Status: "running", Workspaces: []string{"/abs/root/shop-env/shop-api"},
		ID: "sbx_0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d", CreatedAt: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), Detached: true,
	}, list[0])
	assert.True(t, list[0].Running())
	assert.False(t, list[0].Stopped())
	assert.Equal(t, "/abs/root/shop-env/shop-api", list[0].PrimaryWorkspace())

	list, err = ParseList(fixture(t, "ok-unknown-members.json"))
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.True(t, list[0].Stopped())
	assert.Equal(t, []string{"/abs/root/shop-env/shop-api", "/abs/root/shop-env/.romeu/bin"}, list[0].Workspaces)
	assert.Empty(t, list[1].PrimaryWorkspace(), "a sandbox with no workspace has no primary one")
	assert.False(t, list[1].Detached)
}

// TestParseListShape holds the acceptance of T029: a list with a read
// field missing or mistyped, a truncated document, an unknown top-level
// shape (the renamed list among them), a duplicate member name and an
// output that is no JSON are upstream-shape (RJ-310), naming sbx and the
// recorded versions, and never an empty result.
func TestParseListShape(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "ls", "bad-*.json"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 25)
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".json"), func(t *testing.T) {
			list, err := ParseList(fixture(t, filepath.Base(f)))
			testsupport.RequireID(t, "RJ-310", err)
			assert.Nil(t, list, "never an empty list for a shape that was not recognized")
			e := cli.Of(err)
			assert.Equal(t, 2, e.Exit)
			assert.Contains(t, e.Details, cli.Detail{Key: "upstream", Value: "sbx"})
			assert.Contains(t, e.Details, cli.Detail{Key: "recorded sbx versions", Value: strings.Join(RecordedVersions, ", ")})
		})
	}
}

// TestParseListRefusesWhatFilesCannotHold covers the inputs a fixture
// file cannot: bytes that are not UTF-8 and output over the cap.
func TestParseListRefusesWhatFilesCannotHold(t *testing.T) {
	_, err := ParseList([]byte("[{\"name\":\"sh\xffop\",\"status\":\"running\",\"workspaces\":[]}]"))
	testsupport.RequireID(t, "RJ-310", err)

	big := "[" + strings.Repeat(`{"name":"a","status":"running","workspaces":[]},`, maxListOutput/40) + `{"name":"a","status":"running","workspaces":[]}]`
	require.Greater(t, len(big), maxListOutput)
	_, err = ParseList([]byte(big))
	testsupport.RequireID(t, "RJ-310", err)
}

// TestParseListIsNeverEmptyForShapes lists every document that is a
// shape and not a list: none of them returns a list, empty or not.
func TestParseListIsNeverEmptyForShapes(t *testing.T) {
	for _, doc := range []string{`{"sandboxes":[]}`, `{}`, `null`, `""`, `0`, `true`, ``, `   `, `[`, `]`} {
		t.Run(doc, func(t *testing.T) {
			list, err := ParseList([]byte(doc))
			testsupport.RequireID(t, "RJ-310", err)
			assert.Nil(t, list)
		})
	}
}

// listRunner returns a runner whose sbx prints the fixture and exits
// with the status.
func listRunner(t *testing.T, file string, status int) *Runner {
	t.Helper()
	body := `cat '` + filepath.Join(mustAbs(t, "testdata", "ls"), file) + `'`
	if status != 0 {
		body += "\necho 'Error: cannot connect to the sandbox daemon' >&2\nexit " + string(rune('0'+status))
	}
	r := runner(script(t, body))
	r.Versions = Versions{Sbx: "0.99.0"}
	return r
}

// mustAbs returns the absolute path of the elements.
func mustAbs(t *testing.T, elem ...string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(elem...))
	require.NoError(t, err)
	return p
}

// TestRunnerList reads the list through the runner.
func TestRunnerList(t *testing.T) {
	list, err := listRunner(t, "ok-one.json", 0).List(context.Background(), Opts{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "shop", list[0].Name)
}

// TestRunnerListFailsAsSbxUnknown is the failure session of an sbx ls
// error: a non-zero exit is sbx-unknown (RJ-308, exit 1), with the
// details of the failed call, and not an empty list (01 1.6).
func TestRunnerListFailsAsSbxUnknown(t *testing.T) {
	list, err := listRunner(t, "ok-empty.json", 1).List(context.Background(), Opts{Step: "preflight step 6"})
	testsupport.RequireID(t, "RJ-308", err)
	assert.Nil(t, list)
	e := cli.Of(err)
	assert.Equal(t, 1, e.Exit)
	assert.Contains(t, e.Details, cli.Detail{Key: "status", Value: "exit 1"})
	assert.Contains(t, e.Details, cli.Detail{Key: "step", Value: "preflight step 6"})
	assert.Contains(t, e.Details, cli.Detail{Key: "argv", Value: "sbx ls --json"})
	_, isCall := errors.AsType[*CallError](err)
	assert.True(t, isCall, "the failed call is in the chain")
}

// TestRunnerListTimeout stays the timeout of its class (RJ-323): sbx
// ls past its limit is not read as an sbx that answered with an error.
func TestRunnerListTimeout(t *testing.T) {
	ctx := procgroup.WithTestTimeout(context.Background(), procgroup.SbxRead, 200*time.Millisecond)
	_, err := runner(script(t, `sleep 30`)).List(ctx, Opts{})
	te, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "RJ-323", te.ID())
	_, isE := errors.AsType[*cli.Error](err)
	assert.False(t, isE, "it is not sbx-unknown")
}

// TestRunnerListShape adds the live version to the details of the
// error upstream-shape.
func TestRunnerListShape(t *testing.T) {
	_, err := listRunner(t, "bad-before-0.48.json", 0).List(context.Background(), Opts{})
	testsupport.RequireID(t, "RJ-310", err)
	assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "sbx version", Value: "0.99.0"})

	r := listRunner(t, "bad-before-0.48.json", 0)
	r.Versions = Versions{}
	_, err = r.List(context.Background(), Opts{})
	testsupport.RequireID(t, "RJ-310", err)
	for _, d := range cli.Of(err).Details {
		assert.NotEqual(t, "sbx version", d.Key, "no live version is known")
	}
}

// TestThePackageHoldsNoState makes the acceptance of T029 that a parser
// which refuses an output changes no state record a fact of the build:
// internal/sbxdrv, with everything it links, does not import
// internal/state, so no call of it can write a record. A test that
// compared a store before and after could not fail, since nothing here
// knows the store. The positive control is a package the list does hold.
func TestThePackageHoldsNoState(t *testing.T) {
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	var out, errOut bytes.Buffer
	require.NoError(t, procgroup.Run(context.Background(), procgroup.Command{
		Class: procgroup.ToolStep, Argv: []string{goBin, "list", "-deps", "-f", "{{.ImportPath}}", "."},
		Env: os.Environ(), Stdout: &out, Stderr: &errOut,
	}), errOut.String())
	deps := strings.Fields(out.String())
	assert.Contains(t, deps, "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup", "the list is the dependencies of this package")
	assert.NotContains(t, deps, "github.com/brunovenceslau/romeu-e-julieta/internal/state")
}
