// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

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
// A directory that another user owns, or that its group or others may
// write, is binary-unwritable too: another user could swap the new
// binary between its checks and its rename.
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
	info, err := os.Stat(dir)
	if err != nil {
		return "", cli.Internal.Wrap(err, "upgrade: the directory of the running binary")
	}
	where := cli.Detail{Key: "directory", Value: dir}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return "", BinaryUnwritable.New(fmt.Sprintf("the directory of the running romeu is not owned by this user (mode %s)", info.Mode()), where)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return "", BinaryUnwritable.New(fmt.Sprintf("the directory of the running romeu is writable by its group or others (mode %s)", info.Mode()), where)
	}
	if err := writable(dir); err != nil {
		return "", BinaryUnwritable.Wrap(err, "the directory of the running romeu is not writable by this user", where)
	}
	return exe, nil
}

// extract writes the romeu binary of the tar.gz archive to a new file
// under a temporary name in dir, mode 0755, and returns its path and the
// sha256 of the bytes it wrote. The
// archive holds regular files only; a romeu member that is missing,
// that is no regular file, or that passes assetCap is verify-failed,
// since a verified archive of the release build holds one. A file it
// made is removed when it fails.
func extract(archive []byte, dir string) (string, [sha256.Size]byte, error) {
	var none [sha256.Size]byte
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", none, VerifyFailed.Wrap(err, "the archive is not a gzip stream")
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return "", none, VerifyFailed.New("the archive holds no " + binaryMember)
		}
		if err != nil {
			return "", none, VerifyFailed.Wrap(err, "the archive does not read")
		}
		if h.Name != binaryMember {
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > assetCap {
			return "", none, VerifyFailed.New(fmt.Sprintf("the %s of the archive is not a regular file of at most %d bytes", binaryMember, assetCap))
		}
		return writeTemp(dir, tr, h.Size)
	}
}

// writeTemp copies size bytes of r into a new file in dir, made with
// O_EXCL under tempPattern, synced and made 0755, and returns its path
// and the sha256 of what it wrote; it removes the file when a step
// fails.
func writeTemp(dir string, r io.Reader, size int64) (path string, sum [sha256.Size]byte, err error) {
	f, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return "", sum, cli.Internal.Wrap(err, "upgrade: a temporary file beside the running binary")
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cli.Internal.Wrap(cerr, "upgrade: close the new binary")
		}
		if err != nil {
			removeTemp(f.Name())
		}
	}()
	h := sha256.New()
	if _, err := io.CopyN(io.MultiWriter(f, h), r, size); err != nil {
		return "", sum, VerifyFailed.Wrap(err, "the archive ends inside the "+binaryMember)
	}
	if err := f.Chmod(0o755); err != nil {
		return "", sum, cli.Internal.Wrap(err, "upgrade: make the new binary executable")
	}
	if err := f.Sync(); err != nil {
		return "", sum, cli.Internal.Wrap(err, "upgrade: sync the new binary")
	}
	copy(sum[:], h.Sum(nil))
	return f.Name(), sum, nil
}

// removeTemp removes the new binary extracted under a temporary name,
// when the operator declines or a step fails before the swap (REQ-1329,
// I16).
func removeTemp(path string) { _ = os.Remove(path) }

// swapStep is what the swap step is given.
type swapStep struct {
	// temp is the new binary under its temporary name, and exe the
	// running binary it replaces.
	temp, exe string
	// sum is the sha256 of the new binary's bytes as extract wrote them
	// from the verified archive.
	sum [sha256.Size]byte
	// stateDir is the romeu state directory, where the finish token goes.
	stateDir string
	// running is the version of the running romeu, the argument of
	// --finish.
	running string
	// expires is when the finish token stops holding.
	expires time.Time
}

// argv is the command line of the finish, which every later release
// keeps (REQ-1330).
func (s swapStep) argv() []string { return []string{s.exe, "upgrade", "--finish", s.running} }

// swap is the swap step (REQ-1330, I16): it checks that the file at
// temp is still the new binary extract wrote (a regular file of this
// user, mode 0755, with the same sha256), renames it over the running
// one, writes the finish token (token.go) and replaces the process with
// the new binary, as argv, with the process's own environment. It
// returns only when a step fails: a new binary that changed is
// verify-failed and removed, a failed rename leaves the running binary
// as it was, and a failed token or exec leaves the new binary in place,
// with no token.
func swap(s swapStep) error {
	if err := checkNew(s.temp, s.sum); err != nil {
		removeTemp(s.temp)
		return err
	}
	if err := os.Rename(s.temp, s.exe); err != nil {
		return cli.Internal.Wrap(err, "upgrade: rename the new binary over the running one")
	}
	installed := "upgrade: the new romeu is installed at " + s.exe + " and did not start; run romeu approve --toolchain, then romeu sync --all"
	if err := writeToken(s.stateDir, finishToken{version: s.running, sum: s.sum, expires: s.expires}); err != nil {
		return cli.Internal.Wrap(err, installed)
	}
	err := syscall.Exec(s.exe, s.argv(), os.Environ())
	_ = removeToken(filepath.Join(s.stateDir, tokenName))
	return cli.Internal.Wrap(err, installed)
}

// checkNew returns verify-failed unless the file at path, not followed
// through a link, is a regular file of this user, mode 0755, whose
// sha256 is sum.
func checkNew(path string, sum [sha256.Size]byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return VerifyFailed.Wrap(err, "the new binary is gone from its temporary name")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.Mode().IsRegular() || info.Mode() != 0o755 || !ok || int(st.Uid) != os.Getuid() {
		return VerifyFailed.New(fmt.Sprintf("the new binary at its temporary name is no longer a regular file of this user, mode 0755 (mode %s)", info.Mode()))
	}
	got, err := fileSum(path)
	if err != nil {
		return VerifyFailed.Wrap(err, "the new binary at its temporary name does not read")
	}
	if subtle.ConstantTimeCompare(got[:], sum[:]) != 1 {
		return VerifyFailed.New("the new binary at its temporary name changed after its extraction",
			cli.Detail{Key: "want", Value: hex.EncodeToString(sum[:])}, cli.Detail{Key: "got", Value: hex.EncodeToString(got[:])})
	}
	return nil
}
