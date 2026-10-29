// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The writers of a salvage dir. This package is linked by romeu too, so
// it does not import memstore/write (10 10.7): the payload files are
// new files of a new directory, and the manifest, the one file whose
// presence says the salvage finished, is renamed into place.

// payloadSync syncs a file a salvage wrote; a test replaces it.
var payloadSync = (*os.File).Sync

// writePayload writes the new file p of root, 0600, from fill, syncs it
// and returns its size and sha256. A failure removes the file.
func writePayload(root *os.Root, p string, fill func(io.Writer) error) (File, error) {
	f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, memstore.FileMode)
	if err != nil {
		return File{}, err
	}
	h := sha256.New()
	cw := &countWriter{w: io.MultiWriter(f, h)}
	err = errors.Join(fill(cw), f.Chmod(memstore.FileMode), payloadSync(f), f.Close())
	if err != nil {
		return File{}, errors.Join(err, root.Remove(p))
	}
	return File{Path: p, SHA256: hex.EncodeToString(h.Sum(nil)), Size: cw.n}, nil
}

// countWriter counts the bytes written through it.
type countWriter struct {
	w io.Writer
	n int64
}

// Write writes p and counts it.
func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// writeManifest writes the manifest of run last (REQ-0708): a temporary
// file .tmp-<ULID> of the salvage dir, 0600, synced, renamed to
// manifest.json, and the directory synced.
func (s *salvager) writeManifest(run *repoRun) (Written, error) {
	data, err := EncodeManifest(run.m)
	if err != nil {
		return Written{}, err
	}
	m := run.m
	m.Schema, m.Complete = ManifestField, complete(m.Skipped)
	u, err := canon.NewULID(s.now, s.c.ULIDs)
	if err != nil {
		return Written{}, err
	}
	tmp := path.Join(run.dir, memstore.TempName(u))
	if _, err := writePayload(run.root, tmp, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}); err != nil {
		return Written{}, err
	}
	final := path.Join(run.dir, ManifestFile)
	if err := run.root.Rename(tmp, final); err != nil {
		return Written{}, errors.Join(err, run.root.Remove(tmp))
	}
	d, err := run.root.Open(run.dir)
	if err != nil {
		return Written{}, err
	}
	if err := errors.Join(payloadSync(d), d.Close()); err != nil {
		return Written{}, err
	}
	return Written{Repo: run.repo.Dir, Path: filepath.Join(run.repo.Memory, filepath.FromSlash(final)), Manifest: m}, nil
}
