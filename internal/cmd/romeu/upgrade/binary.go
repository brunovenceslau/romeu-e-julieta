// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// binaryMember is the name of the romeu binary in an archive
// (tools/release/internal/build).
const binaryMember = "romeu"

// tempPattern is the pattern of the temporary name the new binary is
// extracted under, beside the running one.
const tempPattern = ".romeu-upgrade-*"

// The seams of the running binary; a test replaces them.
var (
	// executable returns the path of the running binary.
	executable = os.Executable
	// writable reports whether the user can write the directory dir.
	writable = func(dir string) error { return unix.Access(dir, unix.W_OK) }
)

// runningBinary returns the path of the running binary with its links
// resolved, and binary-unwritable when the user cannot write its
// directory (REQ-1326, REQ-1335), which is checked before any download.
func runningBinary() (string, error) {
	exe, err := executable()
	if err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: the path of the running binary")
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: the path of the running binary, its links resolved")
	}
	dir := filepath.Dir(exe)
	if err := writable(dir); err != nil {
		return "", BinaryUnwritable.Wrap(err, "the directory of the running romeu is not writable by this user",
			cli.Detail{Key: "directory", Value: dir})
	}
	return exe, nil
}

// extract writes the romeu binary of the tar.gz archive to a new file
// under a temporary name in dir, mode 0755, and returns its path. The
// archive holds regular files only; a romeu member that is missing,
// that is no regular file, or that passes assetCap is verify-failed,
// since a verified archive of the release build holds one. A file it
// made is removed when it fails.
func extract(archive []byte, dir string) (string, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", VerifyFailed.Wrap(err, "the archive is not a gzip stream")
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", VerifyFailed.New("the archive holds no " + binaryMember)
		}
		if err != nil {
			return "", VerifyFailed.Wrap(err, "the archive does not read")
		}
		if h.Name != binaryMember {
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > assetCap {
			return "", VerifyFailed.New(fmt.Sprintf("the %s of the archive is not a regular file of at most %d bytes", binaryMember, assetCap))
		}
		return writeTemp(dir, tr, h.Size)
	}
}

// writeTemp copies size bytes of r into a new file in dir, made with
// O_EXCL under tempPattern, synced and made 0755, and returns its path;
// it removes the file when a step fails.
func writeTemp(dir string, r io.Reader, size int64) (path string, err error) {
	f, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: a temporary file beside the running binary")
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cli.Internal.Wrap(cerr, "upgrade: close the new binary")
		}
		if err != nil {
			removeTemp(f.Name())
		}
	}()
	if _, err := io.CopyN(f, r, size); err != nil {
		return "", VerifyFailed.Wrap(err, "the archive ends inside the "+binaryMember)
	}
	if err := f.Chmod(0o755); err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: make the new binary executable")
	}
	if err := f.Sync(); err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: sync the new binary")
	}
	return f.Name(), nil
}

// removeTemp removes the new binary extracted under a temporary name,
// when the operator declines or a step fails before the swap (REQ-1329,
// I16).
func removeTemp(path string) { _ = os.Remove(path) }

// swap is the swap step (REQ-1330, I16): it renames the new binary at
// temp over the running one at exe, then replaces the process with it,
// as argv, with the process's own environment. It returns only when one
// of the two fails; a failed rename leaves the running binary as it
// was, and a failed exec leaves the new binary in place.
func swap(temp, exe string, argv []string) error {
	if err := os.Rename(temp, exe); err != nil {
		return cli.Internal.Wrap(err, "upgrade: rename the new binary over the running one")
	}
	err := syscall.Exec(exe, argv, os.Environ())
	return cli.Internal.Wrap(err, "upgrade: the new romeu is installed at "+exe+" and did not start; run romeu approve --toolchain, then romeu sync --all")
}
