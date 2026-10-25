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
	"slices"

	"github.com/google/renameio/v2"
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
	// source build without tools/release build embeds a placeholder
	// (REQ-0130). The embed seam must hand nil or empty bytes for that
	// placeholder, or the caller maps it to this error itself; the caller
	// maps this error to the error id julieta-not-embedded.
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
// GOARCH (REQ-0582, REQ-0584). It copies data. It fails with ErrJulietaNotEmbedded on
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
	return Bin{name: name, data: slices.Clone(data), sums: sums}, nil
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
// absolute path of <name>-env/. It opens an os.Root on envDir and calls
// WriteTo. It is the simpler entry; WriteTo is for a caller that holds
// the root of the env dir already.
func (b Bin) Write(envDir string) error {
	r, err := os.OpenRoot(envDir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()
	return b.WriteTo(r)
}

// WriteTo writes the binary and SHA256SUMS into BinDir below r, the
// root of <name>-env/ (the promotion's step 2, 01 1.6). The dirs are
// created and then set to their modes through r, so the umask and a
// dir that already exists with another mode are corrected. Each file is
// replaced by the atomic write rule of 03 3.8 (REQ-0239), through
// github.com/google/renameio/v2 as internal/state does (ADR 0038): a
// uniquely named temporary file that is created new, never an existing
// inode, so a link or a FIFO planted by name is not written through;
// fsync, rename, then fsync of the dir. A repeat is idempotent.
//
// The binary is renamed before SHA256SUMS, so a reader between the two
// renames sees a sums file that does not match the binary. That fails
// closed (the digest check of REQ-0585 reads render.json, not this
// file), and recovery rolls the promotion forward (REQ-0086).
func (b Bin) WriteTo(r *os.Root) error {
	for _, d := range []struct {
		name string
		mode os.FileMode
	}{{path.Dir(BinDir), 0o700}, {BinDir, binDirMode}} {
		if err := r.MkdirAll(d.name, d.mode); err != nil {
			return fmt.Errorf("render: %w", err)
		}
		if err := r.Chmod(d.name, d.mode); err != nil {
			return fmt.Errorf("render: %w", err)
		}
	}
	bin, err := r.OpenRoot(BinDir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = bin.Close() }()
	if err := replaceFile(bin, b.name, b.data, binFileMode); err != nil {
		return err
	}
	if err := replaceFile(bin, SumsFile, b.sums, sumsMode); err != nil {
		return err
	}
	return syncDir(bin)
}

// replaceFile replaces name below root, the bin dir itself, with data.
func replaceFile(root *os.Root, name string, data []byte, mode os.FileMode) error {
	t, err := renameio.NewPendingFile(name, renameio.WithRoot(root), renameio.WithStaticPermissions(mode))
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := t.CloseAtomicallyReplace(); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// syncDir makes the renames durable: fsync of the directory. It is the
// few lines of syncDir in internal/state/write.go, which is not
// exported.
func syncDir(root *os.Root) error {
	d, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("render: sync the directory: %w", err)
	}
	return nil
}

func plainSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
