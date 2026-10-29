// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The reads of a salvage dir on the host. The memory dir is mounted
// into the sandbox, which may still write it while romeu reads, so
// every read goes through an os.Root (I24), judges each path component
// with Lstat, opens the file without following a link or waiting on a
// FIFO, and checks the open file is the one judged.

// openSalvageDir returns an os.Root on the salvage dir
// salvage/<generation>/<salvage-id> of the memory dir, and fs.ErrNotExist
// when one of its components is absent. A component that is a symbolic
// link or no directory is salvage-path (RJ-307). generation and id are
// ULIDs the caller parsed.
func openSalvageDir(memory, generation, id string) (*os.Root, error) {
	r, err := memstore.Open(memory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	dir := path.Join(memstore.SalvageDir, generation, id)
	if err := checkComponents(r.Root(), dir, true); err != nil {
		return nil, err
	}
	return r.Root().OpenRoot(dir)
}

// checkComponents judges each component of the path p of root with
// Lstat, from the first: one absent is fs.ErrNotExist; a symbolic link
// is salvage-path (RJ-307), and so is one above the last that is no
// directory. With dir, the last must be a directory too; without, a
// regular file.
func checkComponents(root *os.Root, p string, dir bool) error {
	parts := strings.Split(p, "/")
	for i := range parts {
		sub := strings.Join(parts[:i+1], "/")
		info, err := root.Lstat(sub)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fs.ErrNotExist
			}
			return err
		}
		last := i == len(parts)-1
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return SalvagePath.New(fmt.Sprintf("%q holds the symbolic link %q", clip(p), clip(sub)))
		case (!last || dir) && !info.IsDir():
			return SalvagePath.New(fmt.Sprintf("%q is not a directory", clip(sub)))
		case last && !dir && !info.Mode().IsRegular():
			return SalvagePath.New(fmt.Sprintf("%q is not a regular file", clip(sub)))
		}
	}
	return nil
}

// errMismatch is the cause of a payload-mismatch: the payload is
// absent, or its size or sha256 is not the manifest's.
var errMismatch = errors.New("the payload differs from the manifest")

// readPayload reads the payload p of the salvage dir root, which the
// manifest says holds size bytes whose sha256 is sum, and copies what it
// reads to dst when dst is not nil. It reads at most size+1 bytes, so a
// longer file costs no more than the manifest declared. An absent file,
// or one whose size or sha256 differs, is errMismatch; a path that
// breaks the rule of REQ-0252 on the filesystem is salvage-path
// (RJ-307); any other error is a read that failed.
func readPayload(root *os.Root, p string, size int64, sum string, dst io.Writer) error {
	f, err := openRegular(root, p)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s is absent", errMismatch, p)
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	w := io.Writer(h)
	if dst != nil {
		w = io.MultiWriter(h, dst)
	}
	n, err := io.Copy(w, io.LimitReader(f, size+1))
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("%w: %s holds more or fewer than %d bytes", errMismatch, p, size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%w: the sha256 of %s is %s, not %s", errMismatch, p, got, sum)
	}
	return nil
}

// openRegular opens the regular file p of the salvage dir root for
// reading: each component judged with Lstat (checkComponents), opened
// without following a link or waiting on a FIFO, and the open file the
// one judged, or salvage-path (RJ-307). An absent one is
// fs.ErrNotExist.
func openRegular(root *os.Root, p string) (*os.File, error) {
	if err := checkComponents(root, p, false); err != nil {
		return nil, err
	}
	judged, err := root.Lstat(p)
	if err != nil {
		return nil, err
	}
	f, err := root.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, SalvagePath.New(fmt.Sprintf("%q became a symbolic link", clip(p)))
	}
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || !os.SameFile(judged, info)) {
		err = SalvagePath.New(fmt.Sprintf("%q changed while romeu read it", clip(p)))
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}
