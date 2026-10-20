// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

func TestInstallWritesOneExecutableScriptForEachHook(t *testing.T) {
	home := t.TempDir()
	bin := home + "/.local/bin/julieta"
	wrote, err := hooks.Install(home, bin)
	require.NoError(t, err)
	assert.Equal(t, hooks.Names, wrote)
	dir := hooks.DispatcherDir(home)
	assert.Equal(t, filepath.Join(home, ".local/share/julieta/hooks"), dir)
	for _, name := range hooks.Names {
		info, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, name)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), name)
	}
	b, err := os.ReadFile(filepath.Join(dir, "pre-commit"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n# julieta hook dispatcher: written by julieta setup.\nexec '"+bin+"' hooks run pre-commit \"$@\"\n", string(b))
}

func TestInstallIsIdempotentAndRepairsAScript(t *testing.T) {
	home := t.TempDir()
	bin := home + "/.local/bin/julieta"
	_, err := hooks.Install(home, bin)
	require.NoError(t, err)
	wrote, err := hooks.Install(home, bin)
	require.NoError(t, err)
	assert.Empty(t, wrote, "a second call writes nothing")

	path := filepath.Join(hooks.DispatcherDir(home), "post-commit")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	require.NoError(t, os.Chmod(filepath.Join(hooks.DispatcherDir(home), "pre-push"), 0o644))
	wrote, err = hooks.Install(home, bin)
	require.NoError(t, err)
	assert.Equal(t, []string{"post-commit", "pre-push"}, wrote)
}

func TestInstallRefusesABinaryPathThatIsNotAbsoluteAndClean(t *testing.T) {
	home := t.TempDir()
	for _, bin := range []string{"", "julieta", "/a/../b", "/a//b", "/a\x00b"} {
		_, err := hooks.Install(home, bin)
		require.Error(t, err, bin)
	}
	assert.NoDirExists(t, hooks.DispatcherDir(home))
}

// A home with a space, a quote and a dollar sign: the script must run
// the binary at that path and pass the hook and its arguments on.
func TestInstallQuotesTheBinaryPath(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "it's a $HOME")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	bin := filepath.Join(dir, "julieta")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s|' \"$@\"\n"), 0o755))
	_, err := hooks.Install(home, bin)
	require.NoError(t, err)
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{Class: procgroup.ToolProbe,
		Argv: []string{filepath.Join(hooks.DispatcherDir(home), "commit-msg"), "a b", "c"}, Env: []string{}})
	require.NoError(t, err)
	assert.Equal(t, "hooks|run|commit-msg|a b|c|", string(out))
}
