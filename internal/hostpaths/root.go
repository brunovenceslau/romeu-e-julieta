// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hostpaths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Paths are the host paths a root must stay clear of (I20), each a real
// path: $HOME, the settings file and the state directory.
type Paths struct {
	Home, Settings, State string
}

// RealPaths returns the real paths of home, the settings file and the
// state directory, which need not exist. Its error names the path it
// could not resolve.
func RealPaths(home, settings, state string) (Paths, error) {
	var p Paths
	var err error
	if p.Settings, err = RealPath(settings); err != nil {
		return Paths{}, fmt.Errorf("the settings path: %w", err)
	}
	if p.State, err = RealPath(state); err != nil {
		return Paths{}, fmt.Errorf("the state path: %w", err)
	}
	if p.Home, err = RealPath(home); err != nil {
		return Paths{}, fmt.Errorf("HOME: %w", err)
	}
	return p, nil
}

// RealPath returns p with its symbolic links resolved once. p need not
// exist: the links of its longest existing prefix are resolved and the
// rest is appended, since init creates the root. A relative p is
// returned as it is, never resolved against the working directory, so
// that RootProblem refuses it rather than a path it was turned into.
func RealPath(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return p, nil
	}
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

// RootProblem returns the rule of the check root-unsafe (04 4.2,
// REQ-0375, REQ-1277; I20, REQ-1281, REQ-1282) that root breaks, or ""
// when it breaks none. root is the root as the settings hold it, a real
// path, and need not exist yet; env is the name <cfg>-env of the config
// project's env dir under it, or "" to leave that directory out. The
// rules, in the order they are checked:
//
//   - the root is not an absolute, clean path: 04 4.2 says absolute,
//     and 03 3.4 stores the root as its real path, which is clean, so a
//     root that is not clean is not one the settings can hold (init
//     cleans the flag's value before it resolves and checks it);
//   - the root is $HOME;
//   - the root holds the host settings or the host state directory;
//   - the root is inside the host settings directory or the host state
//     directory;
//   - the root, when it exists, is a symbolic link, not a directory,
//     writable by its group or others, or not owned by the user
//     (DirProblem);
//   - the root is inside a git repository;
//   - <root>/<env>, when it exists, breaks a rule of DirProblem.
//
// The comparisons with the paths of p go by identity too (os.SameFile),
// which a link or a case variant of a path on a case-insensitive volume
// satisfies and a comparison of strings does not.
func RootProblem(root, env string, p Paths) string {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "the root is not an absolute, clean path"
	}
	switch {
	case root == p.Home || same(root, p.Home):
		return "the root is $HOME"
	case within(p.Settings, root) || holds(root, p.Settings):
		return "the root holds the host settings"
	case within(p.State, root) || holds(root, p.State):
		return "the root holds the host state"
	case within(root, filepath.Dir(p.Settings)) || holds(filepath.Dir(p.Settings), root):
		return "the root is inside the host settings directory"
	case within(root, p.State) || holds(p.State, root):
		return "the root is inside the host state directory"
	}
	if why := existingDirProblem(root); why != "" {
		return "the root " + why
	}
	if why := gitProblem(root); why != "" {
		return why
	}
	if env == "" {
		return ""
	}
	if why := existingDirProblem(filepath.Join(root, env)); why != "" {
		return env + " " + why
	}
	return ""
}

// gitProblem returns why root sits inside a git repository, found by a
// .git in root or one of its parents, or "".
func gitProblem(root string) string {
	for dir := root; ; dir = filepath.Dir(dir) {
		_, err := os.Lstat(filepath.Join(dir, ".git"))
		switch {
		case err == nil:
			return "the root is inside the git repository at " + dir
		case !errors.Is(err, fs.ErrNotExist):
			return "the root cannot be checked for a git repository: " + err.Error()
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// existingDirProblem returns DirProblem of the directory at p, or ""
// when nothing is at p yet.
func existingDirProblem(p string) string {
	if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	return DirProblem(os.Lstat, p)
}

// DirProblem returns why the directory at p, read with lstat, is not one
// the host tree can sit in, or "": a symbolic link, a file that is not a
// directory, a directory its group or others can write, or one the user
// does not own. init runs it again on the root and <root>/<cfg>-env once
// it has made them, through the handle of the root for the second, so a
// directory or a link created in between is caught.
func DirProblem(lstat func(string) (fs.FileInfo, error), p string) string {
	fi, err := lstat(p)
	if err != nil {
		return "cannot be read: " + err.Error()
	}
	return InfoProblem(fi)
}

// InfoProblem holds fi, the lstat of a directory of the host tree or
// the stat of a handle open on one, to the rules of DirProblem.
func InfoProblem(fi fs.FileInfo) string {
	switch {
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

// within reports whether p is dir or below it; both are real paths.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
