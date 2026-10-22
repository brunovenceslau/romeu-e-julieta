// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is what this package reads of a mise.toml: the tools it names,
// in the order it names them, and the platforms of lockfile_platforms.
type Config struct {
	// Tools are the keys of the [tools] table.
	Tools []string
	// LockfilePlatforms is the lockfile_platforms setting, empty when
	// the file sets none.
	LockfilePlatforms []string
}

// rawConfig is the part of mise.toml this package decodes.
type rawConfig struct {
	Settings struct {
		LockfilePlatforms []string `toml:"lockfile_platforms"`
	} `toml:"settings"`
	Tools map[string]any `toml:"tools"`
}

// ParseConfig reads a mise.toml, which is TOML and only read (12 12.1,
// REQ-1045): a duplicate key, nesting past MaxDepth and a file over
// MaxFileSize are errors.
// Every other table of the file is left to mise.
func ParseConfig(data []byte) (Config, error) {
	if len(data) > MaxFileSize {
		return Config{}, fmt.Errorf("mise.toml: %d bytes pass the cap of %d", len(data), MaxFileSize)
	}
	if err := CheckShape(data); err != nil {
		return Config{}, fmt.Errorf("mise.toml: %w", err)
	}
	var raw rawConfig
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&raw)
	if err != nil {
		return Config{}, fmt.Errorf("mise.toml: %w", err)
	}
	cfg := Config{LockfilePlatforms: raw.Settings.LockfilePlatforms}
	for _, key := range md.Keys() {
		if len(key) == 2 && key[0] == "tools" {
			cfg.Tools = append(cfg.Tools, key[1])
		}
	}
	return cfg, nil
}

// Platforms returns the platforms julieta lock passes to mise (07 7.3,
// REQ-0614): the two linux platforms a sandbox runs, then those of
// lockfile_platforms, each once. An explicit --platform makes mise
// ignore that setting (measured at mise 2026.10.3), so the setting's
// platforms are named here too.
func Platforms(cfg Config) []string {
	out := []string{LinuxX64, LinuxArm64}
	for _, p := range cfg.LockfilePlatforms {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// versionOnlyPrefixes are the backends the pinned mise exempts from the
// lock's URL check, by the part of backend:tool before the colon (07
// 7.3, REQ-0616; mise 2026.10.3, read from its lock documentation at
// that tag). A pin bump reads the list again.
var versionOnlyPrefixes = []string{"asdf", "cargo", "gem", "go", "npm", "pypi", "pipx", "ubi", "vfox"}

// versionOnlyCore are the core backends the pinned mise exempts as
// well, and versionOnlyShort the short names mise.toml gives them.
var (
	versionOnlyCore  = []string{"core:dotnet", "core:rust", "core:swift"}
	versionOnlyShort = []string{"dotnet", "rust", "swift"}
)

// VersionOnly reports whether the tool a mise.toml names by key is
// version-only: the lock holds its version and no platform (07 7.3). The
// key decides, and never the backend a lock entry claims: a lock a pull
// request edits could name another backend to be excused from the
// platforms and the checksum (the audit of 2026-10-22).
func VersionOnly(key string) bool {
	if slices.Contains(versionOnlyCore, key) || slices.Contains(versionOnlyShort, key) {
		return true
	}
	prefix, _, ok := strings.Cut(key, ":")
	return ok && slices.Contains(versionOnlyPrefixes, prefix)
}
