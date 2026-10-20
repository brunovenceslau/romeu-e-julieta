// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

// dispatcherDir is the dispatcher's directory below $HOME, the global
// core.hooksPath (02 2.5, REQ-0154).
const dispatcherDir = ".local/share/julieta/hooks"

// DispatcherDir returns the directory below home that holds the
// dispatcher's scripts, which setup sets as the global core.hooksPath
// (04 4.3, REQ-0397).
func DispatcherDir(home string) string { return filepath.Join(home, dispatcherDir) }

// Install writes the dispatcher's scripts below home, one executable
// script for each of Names: git runs it as the hook, and it hands the
// hook and its arguments to "julieta hooks run" through the path of the
// binary, the PATH link of setup, so a newer binary at the link is the
// one that runs. It returns the names it wrote; a script that is
// already as it should be is left alone, so a second call writes none.
//
// The scripts are shell because git runs a hook as a program, and a
// link to the binary would start it under the name of the hook. They
// hold no value but the binary's path, which must be absolute and
// hold only the characters of a plain path, so that it needs no quoting.
func Install(home, binary string) ([]string, error) {
	if !filepath.IsAbs(binary) || filepath.Clean(binary) != binary || strings.Trim(binary, "/abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._+-") != "" {
		return nil, fmt.Errorf("the binary path %q is not an absolute, clean path of plain characters", binary)
	}
	dir := DispatcherDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var wrote []string
	for _, name := range Names {
		path := filepath.Join(dir, name)
		want := script(binary, name)
		if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, want) {
			if info, err := os.Stat(path); err == nil && info.Mode().Perm() == 0o755 {
				continue
			}
		}
		if err := renameio.WriteFile(path, want, 0o755, renameio.WithTempDir(dir)); err != nil {
			return wrote, fmt.Errorf("the dispatcher script %s: %w", name, err)
		}
		wrote = append(wrote, name)
	}
	return wrote, nil
}

// script returns the script of hook: it runs "<binary> hooks run <hook>
// <arguments>". The command stops reading flags at the hook's name, so
// the arguments of the hook are never taken for its own.
func script(binary, hook string) []byte {
	return []byte("#!/bin/sh\n# julieta hook dispatcher: written by julieta setup.\nexec " + binary + " hooks run " + hook + " \"$@\"\n")
}
