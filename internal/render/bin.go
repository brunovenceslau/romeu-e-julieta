// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The error row for julieta-not-embedded (RJ-312).
var (
	// JulietaNotEmbedded is the error julieta-not-embedded (RJ-312): a
	// source build without tools/release build embeds a placeholder; romeu
	// built that way refuses run, sync and promotion (02 2.1, REQ-0130,
	// REQ-0432).
	JulietaNotEmbedded = cli.Row{
		ID:          "RJ-312",
		Slug:        "julieta-not-embedded",
		Exit:        2,
		Fix:         "decide: install a release build of romeu, or build with tools/release build",
		RecoveredIn: "none: the build embeds no julieta, and a release build or one from tools/release build embeds it",
	}
)

// JulietaSource is the seam that supplies the embedded julieta binary
// bytes (06 6.3, REQ-0582).
type JulietaSource func() ([]byte, error)

// placeholderSHA256 is the sha256 of empty content, which source builds embed.
var placeholderSHA256 = sha256.Sum256(nil)

// DefaultJulietaSource is the placeholder seam until T043 lands the embed package.
var DefaultJulietaSource JulietaSource = func() ([]byte, error) {
	return nil, nil
}

// JulietaBinaryName returns the name of the julieta binary for the given
// architecture: julieta-linux-<goarch> (06 6.3 step 1). If goarch is empty,
// runtime.GOARCH is used.
func JulietaBinaryName(goarch string) string {
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return "julieta-linux-" + goarch
}

// BinPayload holds the binary and checksum bytes and digests prepared for .romeu/bin.
type BinPayload struct {
	// BinaryName is julieta-linux-<goarch>.
	BinaryName string
	// BinaryData is the executable bytes.
	BinaryData []byte
	// BinarySHA256 is the 64-character lowercase hex digest of BinaryData.
	BinarySHA256 string
	// SumsData is the content of SHA256SUMS: "<sha256>  <binaryName>\n".
	SumsData []byte
	// SumsSHA256 is the 64-character lowercase hex digest of SumsData.
	SumsSHA256 string
}

// Files returns the files map entries for render.json (03 3.7):
// ".romeu/bin/<binaryName>" -> BinarySHA256,
// ".romeu/bin/SHA256SUMS" -> SumsSHA256.
func (b BinPayload) Files() map[string]string {
	return map[string]string{
		path.Join(BinDir, b.BinaryName): b.BinarySHA256,
		path.Join(BinDir, "SHA256SUMS"): b.SumsSHA256,
	}
}

// PrepareBin reads the julieta binary from src for goarch, checks for the
// placeholder (refusing with RJ-312), and computes its SHA256 and SHA256SUMS.
func PrepareBin(src JulietaSource, goarch string) (BinPayload, error) {
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	if src == nil {
		src = DefaultJulietaSource
	}
	data, err := src()
	if err != nil {
		return BinPayload{}, fmt.Errorf("render: read embedded julieta: %w", err)
	}
	if len(data) == 0 || sha256.Sum256(data) == placeholderSHA256 {
		return BinPayload{}, JulietaNotEmbedded.New("julieta is not embedded in this build")
	}

	binName := JulietaBinaryName(goarch)
	binSum := sha256.Sum256(data)
	binHex := hex.EncodeToString(binSum[:])

	sumsText := fmt.Sprintf("%s  %s\n", binHex, binName)
	sumsData := []byte(sumsText)
	sumsSum := sha256.Sum256(sumsData)
	sumsHex := hex.EncodeToString(sumsSum[:])

	return BinPayload{
		BinaryName:   binName,
		BinaryData:   data,
		BinarySHA256: binHex,
		SumsData:     sumsData,
		SumsSHA256:   sumsHex,
	}, nil
}

// WriteBin writes julieta-linux-<goarch> and SHA256SUMS into envDir/.romeu/bin,
// through an os.Root on envDir, so no write leaves the project's env directory.
// It returns the BinPayload holding the binary's sha256 for render.json.
func WriteBin(envDir string, src JulietaSource, goarch string) (BinPayload, error) {
	payload, err := PrepareBin(src, goarch)
	if err != nil {
		return BinPayload{}, err
	}

	r, err := os.OpenRoot(envDir)
	if err != nil {
		return BinPayload{}, fmt.Errorf("render: open env dir: %w", err)
	}
	defer func() { _ = r.Close() }()

	if err := r.MkdirAll(BinDir, 0o700); err != nil {
		return BinPayload{}, fmt.Errorf("render: mkdir %s: %w", BinDir, err)
	}

	binPath := path.Join(BinDir, payload.BinaryName)
	if err := r.WriteFile(binPath, payload.BinaryData, 0o700); err != nil {
		return BinPayload{}, fmt.Errorf("render: write %s: %w", binPath, err)
	}

	sumsPath := path.Join(BinDir, "SHA256SUMS")
	if err := r.WriteFile(sumsPath, payload.SumsData, FileMode); err != nil {
		return BinPayload{}, fmt.Errorf("render: write %s: %w", sumsPath, err)
	}

	return payload, nil
}
