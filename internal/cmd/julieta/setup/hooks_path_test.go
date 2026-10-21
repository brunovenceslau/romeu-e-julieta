// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// systemAnswers makes setup read path as the system core.hooksPath (set
// false for none, or err), since a test cannot plant a system config
// for gitsafe's git.
func systemAnswers(t *testing.T, path string, set bool, err error) {
	t.Helper()
	t.Cleanup(setup.SetSystemHooksPath(func(context.Context, *gitsafe.Runner) (string, bool, error) {
		return path, set, err
	}))
}

// globalHooksPath returns core.hooksPath of the global config below
// home, as plain git reads that file.
func globalHooksPath(t *testing.T, home string) string {
	t.Helper()
	return git(t, t.TempDir(), "config", "--file", filepath.Join(home, ".gitconfig"), "core.hooksPath")
}

// TestSetupPointsTheGlobalHooksPathAtTheDispatcher: setup records the
// system core.hooksPath in julieta state and points the global key at
// the dispatcher it wrote (REQ-0397); a second run writes neither again
// (REQ-0396).
func TestSetupPointsTheGlobalHooksPathAtTheDispatcher(t *testing.T) {
	s := newSandbox(t)
	systemAnswers(t, "/opt/kit/hooks", true, nil)
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Unusable)
	assert.Equal(t, hooks.DispatcherDir(s.home), globalHooksPath(t, s.home))
	recorded, err := hooks.ReadSystem(setup.StateDir(s.home))
	require.NoError(t, err)
	assert.Equal(t, "/opt/kit/hooks", recorded)

	config, err := os.ReadFile(filepath.Join(s.home, ".gitconfig"))
	require.NoError(t, err)
	code, _, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	again, err := os.ReadFile(filepath.Join(s.home, ".gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, config, again, "a second run leaves the global config as it was")

	systemAnswers(t, "", false, nil)
	code, _, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	recorded, err = hooks.ReadSystem(setup.StateDir(s.home))
	require.NoError(t, err)
	assert.Empty(t, recorded, "a system config that sets none clears the record")
}

// TestSetupReportsAForeignGlobalHooksPath: a global core.hooksPath that
// is neither the dispatcher's nor empty is reported, the sandbox
// unusable since git does not run the dispatcher, and left byte for
// byte as it was.
func TestSetupReportsAForeignGlobalHooksPath(t *testing.T) {
	s := newSandbox(t)
	systemAnswers(t, "", false, nil)
	planted := []byte("[core]\n\thooksPath = /opt/other/hooks\n")
	require.NoError(t, os.WriteFile(filepath.Join(s.home, ".gitconfig"), planted, 0o600))
	code, rep, _ := setupJSON(t)
	assert.Equal(t, 1, code)
	require.Len(t, rep.Unusable, 1)
	assert.Contains(t, rep.Unusable[0], `"/opt/other/hooks"`)
	assert.Contains(t, rep.Unusable[0], hooks.DispatcherDir(s.home))
	assert.Contains(t, rep.Unusable[0], "git config --global --unset core.hooksPath", "the fix")
	assert.Contains(t, rep.Unusable[0], "julieta setup")
	kept, err := os.ReadFile(filepath.Join(s.home, ".gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, planted, kept)
}

// TestSetupLeavesTheGlobalKeyWhenTheSystemReadFails: a system value
// setup cannot read leaves the sandbox unusable, and the global key is
// not set, which would hide the system hooks from git with no record
// for the dispatcher to run them from.
func TestSetupLeavesTheGlobalKeyWhenTheSystemReadFails(t *testing.T) {
	s := newSandbox(t)
	systemAnswers(t, "", false, errors.New("planted failure"))
	code, rep, _ := setupJSON(t)
	assert.Equal(t, 1, code)
	require.Len(t, rep.Unusable, 1)
	assert.Contains(t, rep.Unusable[0], "planted failure")
	_, err := os.Stat(filepath.Join(s.home, ".gitconfig"))
	assert.ErrorIs(t, err, os.ErrNotExist, "no global config written")
}

// TestSetupTakesTheDispatcherWithATrailingSlash: a global value that
// names the dispatcher dir with a trailing slash is the dispatcher, not
// a foreign value, and is left as it is.
func TestSetupTakesTheDispatcherWithATrailingSlash(t *testing.T) {
	s := newSandbox(t)
	systemAnswers(t, "", false, nil)
	planted := []byte("[core]\n\thooksPath = " + hooks.DispatcherDir(s.home) + "/\n")
	require.NoError(t, os.WriteFile(filepath.Join(s.home, ".gitconfig"), planted, 0o600))
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Unusable)
	kept, err := os.ReadFile(filepath.Join(s.home, ".gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, planted, kept)
}

// TestSetupReportsARelativeSystemHooksPath: a system core.hooksPath that
// is set and not absolute, which the dispatcher would ignore, is
// reported, the sandbox unusable, and neither recorded nor hidden by
// the global key.
func TestSetupReportsARelativeSystemHooksPath(t *testing.T) {
	s := newSandbox(t)
	systemAnswers(t, "kit/hooks", true, nil)
	code, rep, _ := setupJSON(t)
	assert.Equal(t, 1, code)
	require.Len(t, rep.Unusable, 1)
	assert.Contains(t, rep.Unusable[0], `"kit/hooks"`)
	assert.Contains(t, rep.Unusable[0], "not an absolute path")
	recorded, err := hooks.ReadSystem(setup.StateDir(s.home))
	require.NoError(t, err)
	assert.Empty(t, recorded)
	_, err = os.Stat(filepath.Join(s.home, ".gitconfig"))
	assert.ErrorIs(t, err, os.ErrNotExist, "no global config written")
}
