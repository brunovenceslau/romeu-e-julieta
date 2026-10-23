// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// write creates the file at path, and the directories above it.
func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// hostChecksOnly runs step 5 on a host whose sbx listings are clean.
func hostChecksOnly(t *testing.T, host Input, mod func(*Deps)) error {
	t.Helper()
	d := (&trace{}).deps(t)
	if mod != nil {
		mod(&d)
	}
	_, err := RunSteps(t.Context(), d, host, 5, 5)
	return err
}

// TestPreflightStep5HomeSymlinks shows I23 (REQ-0378) with HOME in a
// temporary directory: a link directly under HOME, or one or two levels
// below HOME/.config, that resolves into the root fails the step.
func TestPreflightStep5HomeSymlinks(t *testing.T) {
	tests := []struct {
		name    string
		link    string // relative to home; empty for none
		target  func(root string) string
		wantErr bool
	}{
		{"a clean home passes", "", nil, false},
		{"a link under HOME to the root", ".zshrc", func(root string) string { return root }, true},
		{"a link under HOME into the root", ".gitconfig", func(root string) string { return filepath.Join(root, "dot", "gitconfig") }, true},
		{"a link under .config", ".config/nvim", func(root string) string { return filepath.Join(root, "nvim") }, true},
		{"a link below a directory of .config", ".config/app/settings", func(root string) string { return filepath.Join(root, "s") }, true},
		{"a link three levels down is out of reach", ".config/a/b/c", func(root string) string { return root }, false},
		{"a link two levels below HOME outside .config is out of reach", "work/notes", func(root string) string { return root }, false},
		{"a link to somewhere else passes", ".zshrc", func(string) string { return os.TempDir() }, false},
		{"a link to the parent of the root is not into it", ".zshrc", func(root string) string { return filepath.Dir(root) }, false},
		{"a link to a root that is not there yet", ".zshrc", func(root string) string { return filepath.Join(root, "later") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := quietHost(t)
			if tt.link != "" {
				link := filepath.Join(host.Home, tt.link)
				require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
				require.NoError(t, os.Symlink(tt.target(host.Root), link))
			}
			err := hostChecksOnly(t, host, nil)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-331", err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "link", Value: filepath.Join(host.Home, tt.link)})
		})
	}
	t.Run("a relative link into the root", func(t *testing.T) {
		host := quietHost(t)
		rel, err := filepath.Rel(host.Home, host.Root)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(rel, filepath.Join(host.Home, "r")))
		testsupport.RequireID(t, "RJ-331", hostChecksOnly(t, host, nil))
	})
	t.Run("a link inside a linked directory is never followed", func(t *testing.T) {
		host := quietHost(t)
		elsewhere := filepath.Join(t.TempDir(), "elsewhere")
		require.NoError(t, os.MkdirAll(elsewhere, 0o755))
		require.NoError(t, os.Symlink(host.Root, filepath.Join(elsewhere, "x")))
		require.NoError(t, os.MkdirAll(filepath.Join(host.Home, ".config"), 0o755))
		require.NoError(t, os.Symlink(elsewhere, filepath.Join(host.Home, ".config", "linked")))
		require.NoError(t, hostChecksOnly(t, host, nil))
	})
	t.Run("XDG_CONFIG_HOME does not move the places that are scanned", func(t *testing.T) {
		host := quietHost(t)
		host.Getenv = func(k string) string {
			if k == "XDG_CONFIG_HOME" {
				return filepath.Join(host.Home, "cfg")
			}
			return ""
		}
		require.NoError(t, os.MkdirAll(filepath.Join(host.Home, ".config"), 0o755))
		require.NoError(t, os.Symlink(host.Root, filepath.Join(host.Home, ".config", "x")))
		testsupport.RequireID(t, "RJ-331", hostChecksOnly(t, host, nil))
	})
}

// TestPreflightStep5AutoTrust shows I23 (REQ-0379): mise trusted paths
// and direnv allow lists that cover the root fail the step.
func TestPreflightStep5AutoTrust(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, host *Input)
		wantErr bool
	}{
		{"nothing configured passes", func(*testing.T, *Input) {}, false},
		{"mise trusts the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[settings]\ntrusted_config_paths = [\""+h.Root+"\"]\n")
		}, true},
		{"mise trusts a parent of the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[settings]\ntrusted_config_paths = ['"+filepath.Dir(h.Root)+"']\n")
		}, true},
		{"mise trusts a path under the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[settings]\ntrusted_config_paths = ['"+filepath.Join(h.Root, "shop-env")+"']\n")
		}, true},
		{"mise trusts a path under HOME with a tilde", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[settings]\ntrusted_config_paths = ['~/root']\n")
			h.Root = filepath.Join(h.Home, "root")
			require.NoError(t, os.MkdirAll(h.Root, 0o755))
		}, true},
		{"mise trusts an unrelated path", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[settings]\ntrusted_config_paths = ['/opt/elsewhere']\n[tools]\nnode = '22'\n")
		}, false},
		{"mise config with no settings", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "mise", "config.toml"), "[tools]\nnode = '22'\n")
		}, false},
		{"the environment variable of mise", func(t *testing.T, h *Input) {
			t.Helper()
			h.Getenv = func(k string) string {
				if k == "MISE_TRUSTED_CONFIG_PATHS" {
					return "/opt/elsewhere" + string(os.PathListSeparator) + h.Root
				}
				return ""
			}
		}, true},
		{"a direnv prefix over the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "direnv", "direnv.toml"), "[whitelist]\nprefix = ['"+filepath.Dir(h.Root)+"']\n")
		}, true},
		{"a direnv exact entry under the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "direnv", "direnv.toml"), "[whitelist]\nexact = ['"+filepath.Join(h.Root, "a", ".envrc")+"']\n")
		}, true},
		{"a direnv prefix elsewhere", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".config", "direnv", "direnv.toml"), "[whitelist]\nprefix = ['/opt/elsewhere']\n")
		}, false},
		{"an .envrc allowed by direnv allow under the root", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".local", "share", "direnv", "allow", "0123abcd"), filepath.Join(h.Root, "shop-env", ".envrc")+"\n")
		}, true},
		{"an .envrc allowed elsewhere", func(t *testing.T, h *Input) {
			t.Helper()
			write(t, filepath.Join(h.Home, ".local", "share", "direnv", "allow", "0123abcd"), "/opt/elsewhere/.envrc\n")
		}, false},
		{"XDG_CONFIG_HOME moves the config of mise", func(t *testing.T, h *Input) {
			t.Helper()
			cfg := filepath.Join(h.Home, "cfg")
			write(t, filepath.Join(cfg, "mise", "config.toml"), "[settings]\ntrusted_config_paths = ['"+h.Root+"']\n")
			h.Getenv = func(k string) string {
				if k == "XDG_CONFIG_HOME" {
					return cfg
				}
				return ""
			}
		}, true},
		{"XDG_DATA_HOME moves the allow files of direnv", func(t *testing.T, h *Input) {
			t.Helper()
			data := filepath.Join(h.Home, "data")
			write(t, filepath.Join(data, "direnv", "allow", "f"), h.Root+"/.envrc")
			h.Getenv = func(k string) string {
				if k == "XDG_DATA_HOME" {
					return data
				}
				return ""
			}
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := quietHost(t)
			tt.setup(t, &host)
			err := hostChecksOnly(t, host, nil)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-332", err)
			assert.Equal(t, 2, cli.ExitCode(err))
		})
	}
	t.Run("a config that does not parse cannot be checked", func(t *testing.T) {
		host := quietHost(t)
		write(t, filepath.Join(host.Home, ".config", "mise", "config.toml"), "[settings\n")
		testsupport.RequireID(t, "RJ-332", hostChecksOnly(t, host, nil))
	})
	t.Run("a direnv.toml that does not parse cannot be checked", func(t *testing.T) {
		host := quietHost(t)
		write(t, filepath.Join(host.Home, ".config", "direnv", "direnv.toml"), "= =")
		testsupport.RequireID(t, "RJ-332", hostChecksOnly(t, host, nil))
	})
	t.Run("a config over the cap cannot be checked", func(t *testing.T) {
		host := quietHost(t)
		write(t, filepath.Join(host.Home, ".config", "mise", "config.toml"), "#"+string(make([]byte, maxSettingsFile)))
		testsupport.RequireID(t, "RJ-332", hostChecksOnly(t, host, nil))
	})
	t.Run("an allow file of exactly its cap is a path", func(t *testing.T) {
		host := quietHost(t)
		write(t, filepath.Join(host.Home, ".local", "share", "direnv", "allow", "edge"), host.Root+strings.Repeat(" ", maxAllowFile-len(host.Root)))
		testsupport.RequireID(t, "RJ-332", hostChecksOnly(t, host, nil))
	})
	t.Run("an allow file one byte over its cap is not a path", func(t *testing.T) {
		host := quietHost(t)
		write(t, filepath.Join(host.Home, ".local", "share", "direnv", "allow", "big"), host.Root+strings.Repeat(" ", maxAllowFile-len(host.Root)+1))
		require.NoError(t, hostChecksOnly(t, host, nil))
	})
}
