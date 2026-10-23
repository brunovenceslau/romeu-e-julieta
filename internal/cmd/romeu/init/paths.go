// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package initcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// settingsEnv is the one romeu-specific override of a path: the path of
// settings.yaml, used by tests (02 2.4, REQ-0150). No other ROMEU_*
// variable moves a path, and the root comes only from the settings file.
const settingsEnv = "ROMEU_SETTINGS"

// settingsPath returns the path of settings.yaml for the environment
// getenv reads: ROMEU_SETTINGS, or $XDG_CONFIG_HOME/romeu/settings.yaml,
// or $HOME/.config/romeu/settings.yaml (REQ-0148). A XDG variable that
// is not an absolute path counts as unset, as the XDG specification
// says; a ROMEU_SETTINGS that is not absolute is refused.
func settingsPath(getenv func(string) string) (string, error) {
	if p := getenv(settingsEnv); p != "" {
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("%s is not an absolute path", settingsEnv)
		}
		return filepath.Clean(p), nil
	}
	if x := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "romeu", "settings.yaml"), nil
	}
	if h := getenv("HOME"); filepath.IsAbs(h) {
		return filepath.Join(h, ".config", "romeu", "settings.yaml"), nil
	}
	return "", errors.New("neither ROMEU_SETTINGS, XDG_CONFIG_HOME nor HOME names an absolute path")
}

// realPath returns p with its symbolic links resolved once. p need not
// exist: the links of its longest existing prefix are resolved and the
// rest is appended, since init creates the root.
func realPath(p string) (string, error) {
	rest := ""
	for cur := p; ; {
		r, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(r, rest), nil
		}
		parent := filepath.Dir(cur)
		if !errors.Is(err, fs.ErrNotExist) || parent == cur {
			return "", err
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// within reports whether p is dir or below it; both are real paths.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// rootProblem returns why root, a real path, is not a safe root (the
// check root-unsafe of doctor, 04 4.2, REQ-0375; I20), or "" when it
// is: equal to home, inside a git repository, not owned by the user or
// not a directory, or holding the settings file or the state directory.
// settings and state are real paths too.
func rootProblem(root, home, settings, state string) string {
	switch {
	case root == home:
		return "the root is $HOME"
	case within(settings, root):
		return "the root holds the host settings"
	case within(state, root):
		return "the root holds the host state"
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return "the root is inside the git repository at " + dir
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	fi, err := os.Stat(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ""
	case err != nil:
		return "the root cannot be read: " + err.Error()
	case !fi.IsDir():
		return "the root is not a directory"
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Geteuid() {
		return "the root is not owned by the user"
	}
	return ""
}
