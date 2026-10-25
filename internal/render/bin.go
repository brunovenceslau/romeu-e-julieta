// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"

	"golang.org/x/sys/unix"
)

// The modes of <name>-env/.romeu/bin (06 6.3): the sandbox mounts it
// read-only and runs the binary, so the binary is executable and the
// sums file readable. The spec gives no mode for them; these are the
// least a read-only mount needs. The env dir above stays 0700
// (REQ-0141).
const (
	binDirMode  = 0o755
	binFileMode = 0o755
	sumsMode    = 0o644
)

// SumsFile is the name of the sums file in BinDir.
const SumsFile = "SHA256SUMS"

var (
	// ErrJulietaNotEmbedded is NewBin's refusal of an empty binary: a
	// source build without tools/release build embeds none (REQ-0130),
	// and the caller maps this error to julieta-not-embedded.
	ErrJulietaNotEmbedded = errors.New("render: no julieta binary is embedded")
	// ErrBinArch is NewBin's refusal of a name that is not a Go
	// architecture name in form.
	ErrBinArch = errors.New("render: the architecture is not a Go GOARCH name")
)

var archName = regexp.MustCompile(`^[a-z0-9]+$`)

// Bin is the julieta binary of one architecture, as the embed seam
// gives it, with the sums file that sits beside it (06 6.3, step 1).
// The seam is the data the caller passes to NewBin: romeu passes the
// release-built copy and runtime.GOARCH, a test a few fixture bytes.
type Bin struct {
	name string
	data []byte
	sums []byte
}

// NewBin returns the Bin for the binary data of goarch, romeu's own
// GOARCH (REQ-0582, REQ-0584). It fails with ErrJulietaNotEmbedded on
// an empty binary and ErrBinArch on a name that is not a GOARCH name.
func NewBin(goarch string, data []byte) (Bin, error) {
	if !archName.MatchString(goarch) {
		return Bin{}, fmt.Errorf("%w: %q", ErrBinArch, goarch)
	}
	if len(data) == 0 {
		return Bin{}, ErrJulietaNotEmbedded
	}
	name := "julieta-linux-" + goarch
	sum := sha256.Sum256(data)
	// The text form sha256sum -c reads: digest, two spaces, name.
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	return Bin{name: name, data: data, sums: sums}, nil
}

// Path returns the binary's path in <name>-env/, the key of its
// render.json files entry.
func (b Bin) Path() string { return path.Join(BinDir, b.name) }

// Files returns the render.json files entries of the binary and of
// SHA256SUMS: each path in <name>-env/ and the plain sha256 of the
// file's bytes (03 3.7). The binary's entry is the digest the
// compatibility check compares (REQ-0585).
func (b Bin) Files() map[string]string {
	return map[string]string{
		b.Path():                    plainSum(b.data),
		path.Join(BinDir, SumsFile): plainSum(b.sums),
	}
}

// Write writes the binary and SHA256SUMS into BinDir under envDir, the
// absolute path of <name>-env/, through an os.Root on it. Each file is
// written to a temporary name and renamed into place, so a sandbox that
// runs the old binary never sees a partial one, and a repeat is
// idempotent (the promotion's step 2, 01 1.6).
func (b Bin) Write(envDir string) error {
	r, err := os.OpenRoot(envDir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()
	if err := r.MkdirAll(path.Dir(BinDir), 0o700); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := r.MkdirAll(BinDir, binDirMode); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := replaceFile(r, b.Path(), b.data, binFileMode); err != nil {
		return err
	}
	return replaceFile(r, path.Join(BinDir, SumsFile), b.sums, sumsMode)
}

// replaceFile writes data to name in r through a temporary file and a
// rename. The temporary name is fixed: promotion runs under the
// project lock, and a file a crash left is truncated and reused.
func replaceFile(r *os.Root, name string, data []byte, mode os.FileMode) error {
	tmp := name + ".tmp"
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|unix.O_NOFOLLOW, mode)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Chmod(mode), f.Sync(), f.Close()); err != nil {
		return fmt.Errorf("render: %w", errors.Join(err, r.Remove(tmp)))
	}
	if err := r.Rename(tmp, name); err != nil {
		return fmt.Errorf("render: %w", errors.Join(err, r.Remove(tmp)))
	}
	return nil
}

func plainSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
