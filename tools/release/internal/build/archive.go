// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
)

// ChecksumsName is the checksum file of a release (REQ-0904).
const ChecksumsName = "checksums.txt"

// File is one member of an archive.
type File struct {
	Name string
	Mode int64
	Data []byte
}

// Archive writes a tar.gz with fixed modification times, owners and
// order (REQ-0905): every member carries mtime, owner 0 with no owner
// name, and the members come in the order given. The gzip header holds
// no name and no time.
func Archive(files []File, mtime time.Time) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	tw := tar.NewWriter(gz)
	for _, f := range files {
		err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     f.Name,
			Mode:     f.Mode,
			Size:     int64(len(f.Data)),
			ModTime:  mtime.UTC(),
			Format:   tar.FormatPAX,
		})
		if err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Checksums returns checksums.txt for the archives, name to content, in
// the format of sha256sum: the hex digest, two spaces and the name, one
// line each, in the order of the names.
func Checksums(archives map[string][]byte) []byte {
	names := make([]string, 0, len(archives))
	for name := range archives {
		names = append(names, name)
	}
	slices.Sort(names)
	var b strings.Builder
	for _, name := range names {
		sum := sha256.Sum256(archives[name])
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return []byte(b.String())
}
