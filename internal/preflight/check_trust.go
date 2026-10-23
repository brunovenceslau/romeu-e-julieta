// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// maxSettingsFile is the most a settings file of a host tool may hold
// for this package to read it.
const maxSettingsFile = 1 << 20

// root returns $ROMEU_ROOT with its links resolved, so that a path
// compared with it is compared in one spelling (a temporary directory
// under a link on macOS among them).
func (h *hostEnv) root() string { return resolve(h.in.Root) }

// configHome is $XDG_CONFIG_HOME, or $HOME/.config.
func (h *hostEnv) configHome() string {
	if v := h.getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(h.in.Home, ".config")
}

// dataHome is $XDG_DATA_HOME, or $HOME/.local/share.
func (h *hostEnv) dataHome() string {
	if v := h.getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	return filepath.Join(h.in.Home, ".local", "share")
}

// getenv reads the environment of the input.
func (h *hostEnv) getenv(key string) string {
	if h.in.Getenv == nil {
		return ""
	}
	return h.in.Getenv(key)
}

// resolve returns p with its links resolved, or p cleaned when it cannot
// be (a path that does not exist).
func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// within reports whether child is parent or below it.
func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// covers reports whether a trust entry and $ROMEU_ROOT overlap: the
// entry is the root, a parent of it (the trust reaches the root), or a
// path under it (the trust reaches content an agent writes).
func (h *hostEnv) covers(entry string) bool {
	if entry == "" {
		return false
	}
	e := resolve(h.expand(entry))
	r := h.root()
	return within(e, r) || within(r, e)
}

// expand replaces a leading ~ with the home directory.
func (h *hostEnv) expand(p string) string {
	switch {
	case p == "~":
		return h.in.Home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(h.in.Home, p[2:])
	}
	return p
}

// readSettings reads a settings file below the cap; an absent file is
// reported as absent.
func readSettings(path string) (data []byte, present bool, err error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	data, err = io.ReadAll(io.LimitReader(f, maxSettingsFile+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxSettingsFile {
		return nil, false, fmt.Errorf("the file is longer than %d bytes", maxSettingsFile)
	}
	return data, true, nil
}

// checkHomeSymlinks is the row home-symlink of I23 (REQ-0378): a
// symbolic link directly under $HOME, or one or two levels below
// $HOME/.config, that resolves into $ROMEU_ROOT. A dotfiles manager that
// links a directory of the root into the home makes the root's content
// load as the operator's own configuration.
func checkHomeSymlinks(_ context.Context, h *hostEnv) error {
	cfg := filepath.Join(h.in.Home, ".config")
	if err := h.scanLinks(h.in.Home, 1); err != nil {
		return err
	}
	return h.scanLinks(cfg, 2)
}

// scanLinks looks at the entries of dir and, while depth is above 1, at
// those of its real subdirectories (a link is never followed).
func (h *hostEnv) scanLinks(dir string, depth int) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return HomeSymlink.Wrap(err, "the links under "+dir+" cannot be looked at", cli.Detail{Key: "directory", Value: dir})
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		switch {
		case e.Type()&fs.ModeSymlink != 0:
			if target := linkTarget(p); within(h.root(), target) {
				return HomeSymlink.New(fmt.Sprintf("the link %s resolves into $ROMEU_ROOT", p),
					cli.Detail{Key: "link", Value: p}, cli.Detail{Key: "target", Value: target})
			}
		case e.IsDir() && depth > 1:
			if err := h.scanLinks(p, depth-1); err != nil {
				return err
			}
		}
	}
	return nil
}

// linkTarget returns where the link p leads, with its links resolved
// when the target exists, and as the link says it when it does not (a
// link to a root that is not there yet).
func linkTarget(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	t, err := os.Readlink(p)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(t) {
		t = filepath.Join(filepath.Dir(p), t)
	}
	return filepath.Clean(t)
}

// miseConfig is the part of mise's global configuration the check reads.
type miseConfig struct {
	Settings struct {
		TrustedConfigPaths []string `toml:"trusted_config_paths"`
	} `toml:"settings"`
}

// direnvConfig is the part of direnv.toml the check reads: the
// allow-list of direnv, which loads an .envrc below a listed path
// without asking.
type direnvConfig struct {
	Whitelist struct {
		Prefix []string `toml:"prefix"`
		Exact  []string `toml:"exact"`
	} `toml:"whitelist"`
}

// checkAutoTrust is the row auto-trust of I23 (REQ-0379): mise's
// trusted_config_paths (the global configuration and the environment
// variable MISE_TRUSTED_CONFIG_PATHS), direnv's allow list
// (direnv.toml, and the files direnv writes under its data directory for
// each .envrc allowed with direnv allow) that cover $ROMEU_ROOT. A file
// that cannot be read cannot be checked, and fails the row.
func checkAutoTrust(_ context.Context, h *hostEnv) error {
	var entries []trustEntry
	for _, p := range filepath.SplitList(h.getenv("MISE_TRUSTED_CONFIG_PATHS")) {
		entries = append(entries, trustEntry{p, "MISE_TRUSTED_CONFIG_PATHS"})
	}

	miseFile := filepath.Join(h.configHome(), "mise", "config.toml")
	var mise miseConfig
	if err := decodeToml(miseFile, &mise); err != nil {
		return autoTrustUnreadable(miseFile, err)
	}
	for _, p := range mise.Settings.TrustedConfigPaths {
		entries = append(entries, trustEntry{p, miseFile + " trusted_config_paths"})
	}

	direnvFile := filepath.Join(h.configHome(), "direnv", "direnv.toml")
	var direnv direnvConfig
	if err := decodeToml(direnvFile, &direnv); err != nil {
		return autoTrustUnreadable(direnvFile, err)
	}
	for _, p := range append(direnv.Whitelist.Prefix, direnv.Whitelist.Exact...) {
		entries = append(entries, trustEntry{p, direnvFile + " whitelist"})
	}

	allowed, err := h.direnvAllowed()
	if err != nil {
		return autoTrustUnreadable(filepath.Join(h.dataHome(), "direnv", "allow"), err)
	}
	entries = append(entries, allowed...)

	for _, e := range entries {
		if h.covers(e.path) {
			return AutoTrust.New(fmt.Sprintf("a trust entry covers $ROMEU_ROOT: %s", e.path),
				cli.Detail{Key: "entry", Value: e.path}, cli.Detail{Key: "from", Value: e.from})
		}
	}
	return nil
}

// trustEntry is a path a tool trusts, and where it was listed.
type trustEntry struct {
	path string
	from string
}

// autoTrustUnreadable is the failure of the row for a file it cannot read.
func autoTrustUnreadable(path string, err error) error {
	return AutoTrust.Wrap(err, "the trust settings in "+path+" cannot be read, so they cannot be checked",
		cli.Detail{Key: "file", Value: path})
}

// decodeToml reads the TOML file at path into v; an absent file leaves v
// as it was. Members the structs do not name are skipped.
func decodeToml(path string, v any) error {
	data, present, err := readSettings(path)
	if err != nil || !present {
		return err
	}
	return toml.Unmarshal(data, v)
}

// maxAllowFile is the most a file of direnv's allow directory may hold:
// it holds the path of one .envrc.
const maxAllowFile = 4096

// direnvAllowed returns the .envrc paths that direnv allow recorded: the
// files of $XDG_DATA_HOME/direnv/allow each hold the path of one.
func (h *hostEnv) direnvAllowed() ([]trustEntry, error) {
	dir := filepath.Join(h.dataHome(), "direnv", "allow")
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []trustEntry
	for _, f := range files {
		if !f.Type().IsRegular() {
			continue
		}
		data, _, err := readSettings(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		if len(data) > maxAllowFile {
			continue
		}
		if p := strings.TrimSpace(string(data)); p != "" {
			out = append(out, trustEntry{p, filepath.Join(dir, f.Name())})
		}
	}
	return out, nil
}
