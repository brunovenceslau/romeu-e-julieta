// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// linkPath is the PATH link below $HOME (04 4.3, REQ-0397).
const linkPath = ".local/bin/julieta"

// linkBinary points $HOME/.local/bin/julieta at the running binary. A
// link that already does is left as it is, and so is the running binary
// itself when it is the file at the path (a copy installed there, or a
// hard link to it), which a link to itself would lose; anything else at
// the path,
// a file or another link, is replaced by a rename, so the path never
// goes missing. A link that cannot be made leaves the sandbox
// unusable.
func linkBinary(_ context.Context, r *run) {
	path := filepath.Join(r.home, linkPath)
	r.report.Link.Path = path
	target, err := os.Executable()
	if err != nil {
		r.fail("the PATH link: the running binary cannot be found: %v", err)
		return
	}
	r.report.Link.Target = target
	if have, err := os.Readlink(path); err == nil && have == target {
		return
	}
	if target == path || sameFile(target, path) {
		return
	}
	if err := replaceLink(path, target); err != nil {
		r.fail("the PATH link %s: %v", path, err)
		return
	}
	r.report.Link.Changed = true
}

// sameFile reports whether a and b, symbolic links followed, are the
// same file.
func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// replaceLink makes path a symbolic link to target: a new link beside
// it, renamed over it.
func replaceLink(path, target string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".tmp-julieta-%d", os.Getpid()))
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
