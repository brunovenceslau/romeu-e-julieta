// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/minisign"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// The paths of mise's trust root and of its signed checksum file, from
// the top of the repository (07 7.4).
const (
	miseKeyPath  = "kits/mise-minisign.pub"
	miseSumsPath = "kits/mise/SHASUMS256.txt"
	miseSigPath  = "kits/mise/SHASUMS256.txt.minisig"
)

// pinMise verifies the signature of the release's SHASUMS256.txt with
// the committed key, and only then writes the two files and the pin.
func pinMise(ctx context.Context, e Env, w *writer, p *pins.Pins, version string) (string, error) {
	if !pins.Version.MatchString(version) {
		return "", fmt.Errorf("the version %q is not three numbers, as 2026.10.3", version)
	}
	if p.Mise != nil && older(version, p.Mise.Version) {
		return "", fmt.Errorf("the version %s is lower than the pinned %s; a pin only moves forward", version, p.Mise.Version)
	}
	keyFile, err := w.root.ReadFile(miseKeyPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s is missing: it is the trust root of the check, mise's public key as the maintainer confirmed it", miseKeyPath)
	}
	if err != nil {
		return "", err
	}
	key, err := minisign.ParsePublicKey(keyFile)
	if err != nil {
		return "", fmt.Errorf("%s: %w", miseKeyPath, err)
	}
	dir := "/v" + version + "/"
	sums, _, err := fetch(ctx, e, e.Mise, dir+"SHASUMS256.txt", nil)
	if err != nil {
		return "", err
	}
	sig, _, err := fetch(ctx, e, e.Mise, dir+"SHASUMS256.txt.minisig", nil)
	if err != nil {
		return "", err
	}
	trusted, err := minisign.Verify(key, sums, sig)
	if err != nil {
		return "", fmt.Errorf("SHASUMS256.txt of mise %s: %w", version, err)
	}
	if !slices.Contains(strings.Split(trusted, "\t"), "file:SHASUMS256.txt") {
		return "", fmt.Errorf("the signed trusted comment %q does not name file:SHASUMS256.txt", trusted)
	}
	amd64, arm64, err := miseHashes(sums, version)
	if err != nil {
		return "", err
	}
	w.stage(miseSumsPath, sums)
	w.stage(miseSigPath, sig)
	p.Mise = &pins.Release{Version: version, SHA256: pins.Arches{Amd64: amd64, Arm64: arm64}}
	return fmt.Sprintf("mise %s: SHASUMS256.txt signed by key %s; linux-amd64 %s, linux-arm64 %s", version, key.KeyID(), amd64, arm64), nil
}

// miseHashes returns the sha256 of the two linux binaries of mise
// version from the lines of a verified SHASUMS256.txt, each
// "<64 hex>  ./<name>". A name listed twice, or not at all, is an
// error.
func miseHashes(sums []byte, version string) (amd64, arm64 string, err error) {
	want := map[string]*string{
		"mise-v" + version + "-linux-x64":   &amd64,
		"mise-v" + version + "-linux-arm64": &arm64,
	}
	seen := map[string]bool{}
	for line := range strings.Lines(string(sums)) {
		sum, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "  ")
		if !ok {
			continue
		}
		name = strings.TrimPrefix(name, "./")
		out, wanted := want[name]
		if !wanted {
			continue
		}
		if seen[name] {
			return "", "", fmt.Errorf("SHASUMS256.txt lists %s twice", name)
		}
		seen[name] = true
		if !pins.SHA256(sum) {
			return "", "", fmt.Errorf("SHASUMS256.txt: the hash of %s, %q, is not 64 lower-case hex digits", name, sum)
		}
		*out = sum
	}
	for _, name := range []string{"mise-v" + version + "-linux-x64", "mise-v" + version + "-linux-arm64"} {
		if !seen[name] {
			return "", "", fmt.Errorf("SHASUMS256.txt lists no %s", name)
		}
	}
	return amd64, arm64, nil
}

// older reports whether version a is lower than version b, both three
// numbers (pins.Version).
func older(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range 3 {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return false
}
