// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// globalRunner returns a sandbox runner whose HOME holds the global
// config text, and the path of that config.
func globalRunner(t *testing.T, mode Mode, text string) (*Runner, string) {
	t.Helper()
	home := t.TempDir()
	config := filepath.Join(home, ".gitconfig")
	if text != "" {
		require.NoError(t, os.WriteFile(config, []byte(text), 0o600))
	}
	r, err := New(Config{Mode: mode, Git: gitPath(t), Home: home})
	require.NoError(t, err)
	return r, config
}

// configKeys lists the keys and values of the config file, as plain
// git reads it.
func configKeys(t *testing.T, file string) []string {
	t.Helper()
	return strings.Split(plainGit(t, t.TempDir(), "config", "--file", file, "--list"), "\n")
}

// TestSetGlobalHooksPathRealGit sets core.hooksPath in the sandbox's
// global config and leaves every other key as it was; a second run is a
// no-op that returns the path it found; a global config that holds
// another value is left byte for byte as it was, and that value is
// returned.
func TestSetGlobalHooksPathRealGit(t *testing.T) {
	const path = "/home/agent/.local/share/julieta/hooks"
	r, config := globalRunner(t, Sandbox, "[user]\n\tname = agent\n[core]\n\teditor = vi\n[alias]\n\tst = status\n")
	before := configKeys(t, config)

	found, err := r.SetGlobalHooksPath(t.Context(), path)
	require.NoError(t, err)
	assert.Empty(t, found, "the key was not set")
	assert.Equal(t, path, plainGit(t, t.TempDir(), "config", "--file", config, "core.hooksPath"))
	after := configKeys(t, config)
	assert.Equal(t, before, slices.DeleteFunc(slices.Clone(after), func(kv string) bool { return strings.HasPrefix(kv, "core.hookspath=") }),
		"every other key is as it was")
	assert.Len(t, after, len(before)+1)

	written, err := os.ReadFile(config)
	require.NoError(t, err)
	found, err = r.SetGlobalHooksPath(t.Context(), path)
	require.NoError(t, err)
	assert.Equal(t, path, found)
	again, err := os.ReadFile(config)
	require.NoError(t, err)
	assert.Equal(t, written, again, "a second run writes nothing")

	other, otherConfig := globalRunner(t, Sandbox, "[core]\n\thooksPath = /opt/kit/hooks\n")
	planted, err := os.ReadFile(otherConfig)
	require.NoError(t, err)
	found, err = other.SetGlobalHooksPath(t.Context(), path)
	require.NoError(t, err)
	assert.Equal(t, "/opt/kit/hooks", found)
	kept, err := os.ReadFile(otherConfig)
	require.NoError(t, err)
	assert.Equal(t, planted, kept, "another value is left as it was")

	none, noneConfig := globalRunner(t, Sandbox, "")
	found, err = none.SetGlobalHooksPath(t.Context(), path)
	require.NoError(t, err)
	assert.Empty(t, found)
	assert.Equal(t, path, plainGit(t, t.TempDir(), "config", "--file", noneConfig, "core.hooksPath"), "no global config yet: git makes it")
}

// TestSetGlobalHooksPathRefuses: host mode, a path that is not absolute
// and one that holds a character unsafeRune refuses are refused before
// git starts.
func TestSetGlobalHooksPathRefuses(t *testing.T) {
	var rec recorder
	host, _ := globalRunner(t, Host, "")
	rec.install(host)
	_, err := host.SetGlobalHooksPath(t.Context(), "/a/hooks")
	require.ErrorIs(t, err, ErrRefused)

	sandbox, _ := globalRunner(t, Sandbox, "")
	rec.install(sandbox)
	for _, p := range []string{"", "hooks", "~/hooks", "/a\nb", "/a\u202eb", "/a\x00b", "/a\xffb"} {
		_, err := sandbox.SetGlobalHooksPath(t.Context(), p)
		require.ErrorIs(t, err, ErrRefused, "%q", p)
	}
	assert.Empty(t, rec.commands, "no git started")
}

// withSystemConfig makes r's git read file as its system config, as
// the system config of the sandbox kit would be read: the test adds
// GIT_CONFIG_SYSTEM to each command, which gitsafe itself never passes.
func withSystemConfig(r *Runner, file string) {
	r.start = func(ctx context.Context, c procgroup.Command) error {
		c.Env = append(slices.Clone(c.Env), "GIT_CONFIG_SYSTEM="+file)
		return procgroup.Run(ctx, c)
	}
}

// TestSystemHooksPathRealGit reads a planted system core.hooksPath, and
// none from a system config that sets none; a value in the global
// config is not the system one.
func TestSystemHooksPathRealGit(t *testing.T) {
	r, _ := globalRunner(t, Sandbox, "[core]\n\thooksPath = /global/hooks\n")
	system := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(system, []byte("[core]\n\thooksPath = /opt/kit/hooks\n"), 0o600))
	withSystemConfig(r, system)
	path, set, err := r.SystemHooksPath(t.Context())
	require.NoError(t, err)
	assert.True(t, set)
	assert.Equal(t, "/opt/kit/hooks", path)

	require.NoError(t, os.WriteFile(system, []byte("[core]\n\thooksPath = ~/kit/hooks\n"), 0o600))
	path, set, err = r.SystemHooksPath(t.Context())
	require.NoError(t, err)
	assert.True(t, set)
	assert.Equal(t, filepath.Join(home(t, r), "kit/hooks"), path, "~ expands to the HOME of the mode, as git expands it")

	require.NoError(t, os.WriteFile(system, []byte("[core]\n\thooksPath = kit/hooks\n"), 0o600))
	path, set, err = r.SystemHooksPath(t.Context())
	require.NoError(t, err)
	assert.True(t, set)
	assert.Equal(t, "kit/hooks", path, "a relative value is returned as it is, for the caller to report")

	require.NoError(t, os.WriteFile(system, []byte("[user]\n\tname = kit\n"), 0o600))
	path, set, err = r.SystemHooksPath(t.Context())
	require.NoError(t, err)
	assert.False(t, set, "got %q", path)

	require.NoError(t, os.WriteFile(system, []byte("[core]\n\thooksPath = /a\u202eb\n"), 0o600))
	_, _, err = r.SystemHooksPath(t.Context())
	require.ErrorIs(t, err, ErrRefused, "the checks of HooksPath")
}

// TestSystemHooksPathRefusesHostMode: host mode reads no system config
// (05 5.1), so it has no system value to read.
func TestSystemHooksPathRefusesHostMode(t *testing.T) {
	var rec recorder
	r, _ := globalRunner(t, Host, "")
	rec.install(r)
	_, _, err := r.SystemHooksPath(t.Context())
	require.ErrorIs(t, err, ErrRefused)
	assert.Empty(t, rec.commands)
}

// TestGlobalArgv pins the commands of the two verbs: the read and the
// write of the global key, and the read of the system one, each in no
// repository (the root directory), with -c core.hooksPath=/dev/null and
// the other flags of sandbox mode, and no GIT_* variable but those of
// the mode and of the reads.
func TestGlobalArgv(t *testing.T) {
	var commands []procgroup.Command
	r := newRunner(t, Sandbox)
	r.start = func(_ context.Context, c procgroup.Command) error {
		commands = append(commands, c)
		if slices.Contains(c.Argv, "--get") {
			return &procgroup.ExitError{Argv0: c.Argv[0], Code: 1} // every key unset
		}
		return nil
	}
	found, err := r.SetGlobalHooksPath(t.Context(), "/h/hooks")
	require.NoError(t, err)
	assert.Empty(t, found)
	_, set, err := r.SystemHooksPath(t.Context())
	require.NoError(t, err)
	assert.False(t, set)

	readFlags := 1 + 2*(len(r.settings)+len(historySettings))
	writeFlags := 1 + 2*len(r.settings)
	want := []struct {
		flags int
		args  []string
	}{
		{readFlags, []string{"config", "--global", "--includes", "--type=path", "-z", "--get", "core.hooksPath"}},
		{writeFlags, []string{"config", "--global", "core.hooksPath", "/h/hooks"}},
		{readFlags, []string{"config", "--system", "--includes", "--type=path", "-z", "--get", "core.hooksPath"}},
	}
	require.Len(t, commands, len(want))
	for i, c := range commands {
		assert.Equal(t, want[i].args, c.Argv[want[i].flags:], "command %d", i)
		assert.Equal(t, []string{"-c", "core.hooksPath=/dev/null"}, c.Argv[1:3], "command %d", i)
		for _, s := range sandboxSettings {
			assert.Contains(t, c.Argv, s.key+"="+s.value)
		}
		assert.Equal(t, "/", c.Dir, "no repository")
		for _, kv := range c.Env {
			if strings.HasPrefix(kv, "GIT_") {
				assert.Contains(t, slices.Concat(r.env, statusEnv), kv, "command %d", i)
			}
		}
		assert.Subset(t, c.Env, r.env)
	}
}

// home returns the HOME the runner's git runs with.
func home(t *testing.T, r *Runner) string {
	t.Helper()
	for _, kv := range r.env {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			return v
		}
	}
	require.FailNow(t, "no HOME")
	return ""
}

// TestSetGlobalHooksPathExpandsATilde: a global value written with ~ is
// read expanded, so it compares equal to the path it names.
func TestSetGlobalHooksPathExpandsATilde(t *testing.T) {
	r, _ := globalRunner(t, Sandbox, "[core]\n\thooksPath = ~/.local/share/julieta/hooks\n")
	found, err := r.SetGlobalHooksPath(t.Context(), filepath.Join(home(t, r), ".local/share/julieta/hooks"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home(t, r), ".local/share/julieta/hooks"), found)
}
