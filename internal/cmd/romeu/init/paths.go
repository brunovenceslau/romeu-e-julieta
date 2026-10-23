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
// not a directory, holding the settings file or the state directory, or
// inside the settings directory or the state directory.
// settings and state are real paths too.
func rootProblem(root, home, settings, state string) string {
	switch {
	case root == home || same(root, home):
		return "the root is $HOME"
	case within(settings, root) || holds(root, settings):
		return "the root holds the host settings"
	case within(state, root) || holds(root, state):
		return "the root holds the host state"
	case within(root, filepath.Dir(settings)) || holds(filepath.Dir(settings), root):
		return "the root is inside the host settings directory"
	case within(root, state) || holds(state, root):
		return "the root is inside the host state directory"
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		_, err := os.Lstat(filepath.Join(dir, ".git"))
		switch {
		case err == nil:
			return "the root is inside the git repository at " + dir
		case !errors.Is(err, fs.ErrNotExist):
			return "the root cannot be checked for a git repository: " + err.Error()
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

// dirProblem returns why the directory at p, read with lstat, is not one
// init can put the host tree in, or "": a symbolic link, a file that is
// not a directory, a directory the user does not own, or one its group
// or others can write.
func dirProblem(lstat func(string) (fs.FileInfo, error), p string) string {
	fi, err := lstat(p)
	switch {
	case err != nil:
		return "cannot be read: " + err.Error()
	case fi.Mode()&fs.ModeSymlink != 0:
		return "is a symbolic link"
	case !fi.IsDir():
		return "is not a directory"
	case fi.Mode().Perm()&0o022 != 0:
		return "is writable by its group or others"
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Geteuid() {
		return "is not owned by the user"
	}
	return ""
}

// same reports whether a and b name the same file, by identity (os.SameFile
// on their stat results), which a case variant of a path on a case-insensitive
// volume or a link to it satisfies and a comparison of strings does not.
func same(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	return err == nil && os.SameFile(fa, fb)
}

// holds reports whether dir is p or one of its parents, by identity.
func holds(dir, p string) bool {
	for cur := p; ; cur = filepath.Dir(cur) {
		if same(dir, cur) {
			return true
		}
		if filepath.Dir(cur) == cur {
			return false
		}
	}
}
