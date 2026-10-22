// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The fake sbx is built once for the tests of this package, and every
// call goes through the real runner: the builders of this package are held
// to the sessions by the fake that replays them, which has its own rules
// for the shapes and for I4 and I25 (e2e/fakesbx does not import this
// package).
var (
	fakeOnce sync.Once
	fakePath string
	fakeErr  error
	fakeDir  string // the directory of the build, set by buildFake
)

// TestMain removes the build directory of the fake.
func TestMain(m *testing.M) {
	code := m.Run()
	if fakeDir != "" {
		_ = os.RemoveAll(fakeDir)
	}
	os.Exit(code)
}

// fakeBinary builds e2e/fakesbx once and returns the path of the binary.
func fakeBinary(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() { fakePath, fakeErr = buildFake() })
	require.NoError(t, fakeErr)
	return fakePath
}

// buildFake builds e2e/fakesbx into a directory of its own, which
// TestMain removes: the build is shared by the tests of the package, so
// no test owns the directory.
func buildFake() (string, error) {
	var err error
	fakeDir, err = os.MkdirTemp("", "sbxdrv-fakesbx-")
	if err != nil {
		return "", err
	}
	goBin, err := procgroup.LookPath("go")
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		return "", err
	}
	path := filepath.Join(fakeDir, "fakesbx")
	var out bytes.Buffer
	if err := procgroup.Run(context.Background(), procgroup.Command{
		Class: procgroup.ToolStep, Argv: []string{goBin, "build", "-o", path, "./e2e/fakesbx"},
		Dir: root, Env: os.Environ(), Stdout: &out, Stderr: &out,
	}); err != nil {
		return "", errors.New(err.Error() + ": " + out.String())
	}
	return path, nil
}

// fakeConfig is the file the fake reads (e2e/fakesbx/config.go).
type fakeConfig struct {
	Session string        `json:"session"`
	State   string        `json:"state"`
	GitURL  string        `json:"gitUrl"`
	Replace []fakeReplace `json:"replace"`
}

// fakeReplace is one replacement of the fake.
type fakeReplace struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// fakeRig is a fake sbx started as the sbx of a runner, with an env dir
// rendered as romeu renders it.
type fakeRig struct {
	runner  *Runner
	fx      Fixture
	root    string
	state   string
	session string
	stdout  *bytes.Buffer
}

// newFakeRig sets up the fake for the session file at path: an env dir
// with a primary clone under a temporary root, the fake linked as sbx
// with its configuration beside it, and a runner that runs it.
func newFakeRig(t *testing.T, sessionPath string) *fakeRig {
	t.Helper()
	bin := fakeBinary(t)
	root := t.TempDir()
	envDir := filepath.Join(root, "shop-env")
	clone := filepath.Join(envDir, "shop-api")
	require.NoError(t, os.MkdirAll(filepath.Join(clone, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(clone, ".git", "config"), []byte("[core]\n\tbare = false\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "sbxenv.yaml"), []byte("schemaVersion: \"1\"\nname: shop\nagent: sbx-kit-claude\nworkspace:\n  path: ./shop-api\n  clone: true\n"), 0o644))

	sbx := filepath.Join(root, "bin", "sbx")
	require.NoError(t, os.MkdirAll(filepath.Dir(sbx), 0o755))
	require.NoError(t, os.Symlink(bin, sbx))
	state := filepath.Join(root, "fake-state.json")
	cfg, err := jsonv2.Marshal(fakeConfig{
		Session: sessionPath, State: state, GitURL: "git://127.0.0.1:9418/shop",
		Replace: []fakeReplace{{From: "/abs/root", To: root}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(sbx+".json", cfg, 0o600))

	out := &bytes.Buffer{}
	r := &Runner{Sbx: sbx, Home: root, Term: "xterm", Terminal: Terminal{Out: out, Err: out}}
	fx := defaultFixture
	fx.Dir = envDir
	return &fakeRig{runner: r, fx: fx, root: root, state: state, session: sessionPath, stdout: out}
}

// sessionFiles lists the sessions of the recorded versions.
func sessionFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(sessionsDir, "*", "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	return files
}

// callFor returns the call of a registered builder that has the shape,
// built from the fixture.
func callFor(t *testing.T, fx Fixture, shape []string) Call {
	t.Helper()
	for _, c := range examples(fx) {
		if shapeKey(c.Shape) == shapeKey(shape) {
			return c
		}
	}
	require.Failf(t, "no builder", "no builder gives the shape %q", shape)
	return Call{}
}

// TestEverySessionReplaysThroughTheRunner is the end of the chain of
// T029: each session under e2e/testdata/sbx is replayed, call by call and
// in its order, by the fake sbx, through the real runner and the real
// builders. A call that exits 0 gives the recorded stdout; one that
// exits otherwise is a CallError with that status; one that hangs is
// ended by the timeout of its class (RJ-323), through the seam of the
// table.
func TestEverySessionReplaysThroughTheRunner(t *testing.T) {
	for _, path := range sessionFiles(t) {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".jsonl"), func(t *testing.T) {
			abs, err := filepath.Abs(path)
			require.NoError(t, err)
			rig := newFakeRig(t, abs)
			s := loadSessionFile(t, path)
			for i, e := range s.Entries {
				c := callFor(t, rig.fx, e.Argv)
				ctx := context.Background()
				if e.Hang {
					ctx = procgroup.WithTestTimeout(ctx, c.Class, 300*time.Millisecond)
				}
				rig.stdout.Reset()
				res, err := rig.runner.Run(ctx, c, Opts{})
				switch {
				case e.Hang:
					te, ok := errors.AsType[*procgroup.TimeoutError](err)
					require.True(t, ok, "entry %d %q: %v", i+1, e.Argv, err)
					assert.Equal(t, "RJ-323", te.ID())
				case *e.Exit != 0:
					ce, ok := errors.AsType[*CallError](err)
					require.True(t, ok, "entry %d %q: %v", i+1, e.Argv, err)
					assert.Contains(t, ce.status, "exit "+strconv.Itoa(*e.Exit), "entry %d", i+1)
				default:
					require.NoError(t, err, "entry %d %q", i+1, e.Argv)
					if c.Mode == Captured {
						assert.Equal(t, strings.ReplaceAll(e.Stdout, "/abs/root", rig.root), string(res.Stdout), "entry %d %q", i+1, e.Argv)
					}
				}
			}
		})
	}
}

// loadSessionFile reads a session by its path.
func loadSessionFile(t *testing.T, path string) session {
	t.Helper()
	for _, s := range loadSessions(t) {
		if filepath.Base(s.Path) == filepath.Base(path) && s.Version == filepath.Base(filepath.Dir(path)) {
			return s
		}
	}
	require.Failf(t, "no session", "%s", path)
	return session{}
}

// TestFakeWritesTheStanzaOfACreate holds REQ-0929 end to end: the create
// of the lifecycle session writes the recorded remote.sandbox-<name>
// stanza into the config of the primary clone.
func TestFakeWritesTheStanzaOfACreate(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join(sessionsDir, "0.48.0", "lifecycle.jsonl"))
	require.NoError(t, err)
	rig := newFakeRig(t, abs)
	_, err = rig.runner.Run(context.Background(), List(), Opts{})
	require.NoError(t, err)
	create, err := EnvCreate(rig.fx.Dir)
	require.NoError(t, err)
	_, err = rig.runner.Run(context.Background(), create, Opts{})
	require.NoError(t, err)
	config, err := os.ReadFile(filepath.Join(rig.fx.Dir, "shop-api", ".git", "config"))
	require.NoError(t, err)
	assert.Contains(t, string(config), "[remote \"sandbox-shop\"]\n\turl = git://127.0.0.1:9418/shop\n\tfetch = +refs/heads/*:refs/remotes/sandbox-shop/*\n")
}

// TestFakeFailsAnOutOfOrderCall holds REQ-0928 end to end: the fake
// fails a call out of the recorded order, and a shape that was not
// recorded, with a status the runner reports.
func TestFakeFailsAnOutOfOrderCall(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join(sessionsDir, "0.48.0", "lifecycle.jsonl"))
	require.NoError(t, err)
	rig := newFakeRig(t, abs)
	stop, err := Stop("shop")
	require.NoError(t, err)
	_, err = rig.runner.Run(context.Background(), stop, Opts{})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "exit 97", ce.status)
	assert.Contains(t, ce.stderr, "out of the recorded order")

	check, err := PolicyCheckNetwork("shop", "github.com")
	require.NoError(t, err)
	_, err = rig.runner.Run(context.Background(), check, Opts{})
	ce, ok = errors.AsType[*CallError](err)
	require.True(t, ok)
	assert.Equal(t, "exit 97", ce.status, "a shape the lifecycle session did not record")
	assert.Contains(t, ce.stderr, "no recorded argv shape")
}

// TestFakeExits99WhereTheGuardsRefuse holds that the guard of this
// package and the fake agree: every argv the guard refuses, the fake
// exits 99 on, whatever the session holds.
func TestFakeExits99WhereTheGuardsRefuse(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join(sessionsDir, "0.48.0", "lifecycle.jsonl"))
	require.NoError(t, err)
	rig := newFakeRig(t, abs)
	for _, argv := range append(append([][]string{}, refusedApprovals...), refusedSecretForms...) {
		t.Run(strings.Join(argv, " "), func(t *testing.T) {
			require.Error(t, Guard(call(argv...)), "the guard refuses it")
			var out bytes.Buffer
			err := procgroup.Run(context.Background(), procgroup.Command{
				Class: procgroup.SbxChange, Argv: append([]string{rig.runner.Sbx}, argv...), Env: []string{"PATH=/usr/bin:/bin"}, Stdout: &out, Stderr: &out,
			})
			ee, ok := errors.AsType[*procgroup.ExitError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, 99, ee.Code, out.String())
		})
	}
}
