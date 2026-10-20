// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
)

// TestKindsAreRegisteredByFile checks the kinds this package registers,
// each from its own file, and the usage text built from them in name
// order.
func TestKindsAreRegisteredByFile(t *testing.T) {
	var names []string
	for _, k := range kinds.sorted() {
		names = append(names, k.name)
	}
	assert.Equal(t, []string{"adr", "command", "error", "kit", "task"}, names)
	assert.Equal(t, "usage:\n  go run ./tools/new adr <title>\n  go run ./tools/new command <binary> <command> [<subcommand>]\n  go run ./tools/new error <package> <slug>\n  go run ./tools/new kit <name>\n  go run ./tools/new task <title>\n", usage())
	for _, name := range names {
		assert.FileExists(t, name+".go", "the file that registers %s", name)
	}
}

// TestAddRefusesADuplicateKind checks that a second kind with a name the
// registry holds panics, so two files cannot claim one kind; an empty
// name panics too.
func TestAddRefusesADuplicateKind(t *testing.T) {
	r := registry{}
	r.add(kind{name: "x", args: "<a>", run: func(env, []string) error { return nil }})
	assert.PanicsWithValue(t, `new: the kind "x" is registered twice`, func() {
		r.add(kind{name: "x", args: "<b>", run: func(env, []string) error { return nil }})
	})
	assert.Panics(t, func() { r.add(kind{args: "<a>", run: func(env, []string) error { return nil }}) })
	assert.Panics(t, func() { r.add(kind{name: "y", args: "<a>"}) }, "a kind without a run")
}

// TestUnknownKind checks that a kind no file registers is refused with
// the usage text, and so is a run with no kind.
func TestUnknownKind(t *testing.T) {
	f := newFixture(t)
	assert.Equal(t, exitError, run(f.e, []string{"nonesuch", "x"}))
	assert.Contains(t, f.stderr.String(), `new: unknown kind "nonesuch"`)
	assert.Contains(t, f.stderr.String(), "go run ./tools/new adr <title>")
	f = newFixture(t)
	assert.Equal(t, exitError, run(f.e, nil))
	assert.Equal(t, usage(), f.stderr.String())
}

// fixtureKind is the one file that adds the kind of
// TestAFixtureKindIsAddedByOneFile. It prints its arguments.
const fixtureKind = `package main

import "strings"

func init() {
	register(kind{name: "fixture", args: "<word>", run: func(e env, args []string) error {
		say(e.stdout, "fixture %s\n", strings.Join(args, " "))
		return nil
	}})
}
`

// TestAFixtureKindIsAddedByOneFile builds tools/new with one file added
// through an overlay, and no file edited: the kind it registers runs,
// and the usage text names it in name order.
func TestAFixtureKindIsAddedByOneFile(t *testing.T) {
	root := moduleRoot(t)
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	dir := t.TempDir()
	path := filepath.Join(root, "tools", "new", "fixture.go")
	require.NoFileExists(t, path, "the overlay adds the file")
	copied := filepath.Join(dir, "fixture.go")
	require.NoError(t, os.WriteFile(copied, []byte(fixtureKind), 0o600))
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{path: copied}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "overlay.json"), overlay, 0o600))
	bin := filepath.Join(dir, "new")
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{goBin, "build", "-overlay=" + filepath.Join(dir, "overlay.json"), "-o", bin, "./tools/new"},
		Dir:   root,
		Env:   buildEnv(),
	})
	require.NoError(t, err, "%s", out)

	out, err = procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{bin, "fixture", "a", "b"},
		Dir:   dir,
		Env:   []string{"PATH=" + os.Getenv("PATH")},
	})
	require.NoError(t, err, "%s", out)
	assert.Equal(t, "fixture a b\n", string(out))

	out, err = procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{bin},
		Dir:   dir,
		Env:   []string{"PATH=" + os.Getenv("PATH")},
	})
	require.Error(t, err)
	assert.Equal(t, "usage:\n  go run ./tools/new adr <title>\n  go run ./tools/new command <binary> <command> [<subcommand>]\n  go run ./tools/new error <package> <slug>\n  go run ./tools/new fixture <word>\n  go run ./tools/new kit <name>\n  go run ./tools/new task <title>\n", string(out))
}

// moduleRoot returns the directory of go.mod above the test's directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no go.mod above the test's directory")
		dir = parent
	}
}

// buildEnv is the environment of the test's go build: the variables of
// goenv.PassThrough this process has, then goenv.Pin and goenv.Build, so
// no go env file, go.work or GOFLAGS of the caller changes the build.
func buildEnv() []string {
	var env []string
	for _, key := range goenv.PassThrough {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return slices.Concat(env, goenv.Pin, goenv.Build)
}
