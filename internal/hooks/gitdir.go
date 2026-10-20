// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

// hooksDir returns the hooks directory of the repository git ran a hook
// in, or "" when dir is no repository. It is $GIT_DIR/hooks, where
// GIT_DIR is the variable git gave the hook (git sets it in a linked
// work tree), else the .git of dir, a directory or a "gitdir: <path>"
// file, else dir itself, which is the git dir of a bare repository. A
// linked work tree's git dir holds a commondir file naming the git dir
// it shares its hooks with, as git itself reads it.
//
// It reads the layout git documents (gitrepository-layout(5)) and does
// not start git: the dispatcher runs inside a hook, where a git call
// would only repeat the discovery git has just done.
func hooksDir(dir string, env []string) string {
	gitDir := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GIT_DIR="); ok && v != "" {
			gitDir = v
		}
	}
	if gitDir == "" {
		dotGit := filepath.Join(dir, ".git")
		info, err := os.Stat(dotGit)
		switch {
		case err == nil && info.IsDir():
			gitDir = dotGit
		case err == nil:
			b, err := os.ReadFile(dotGit)
			if err != nil {
				return ""
			}
			target, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir: ")
			if !ok {
				return ""
			}
			gitDir = target
		default:
			gitDir = dir
		}
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}
	if b, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		common := strings.TrimSpace(string(b))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
		gitDir = common
	}
	return filepath.Join(gitDir, "hooks")
}
