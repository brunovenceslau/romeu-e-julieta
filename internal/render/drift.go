// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
)

// CheckDrift compares each live file and kit dir that render.json names
// with its digest (01 1.2, REQ-0047; I15): the plain sha256 of a file
// (REQ-0256), the kit-tree digest of a kit dir. It returns the paths
// that differ or are missing, sorted; the caller reports the first as
// drift-detected, exit 4. Before the first promotion there is no
// render.json and nothing drifts. A render.json with promoting is
// ErrPromotionInterrupted, never drift: recovery runs first.
func CheckDrift(envDir string) ([]string, error) {
	root, err := os.OpenRoot(envDir)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	rec, found, err := readLiveRecord(root)
	if err != nil || !found {
		return nil, err
	}
	if rec.Promoting != nil {
		return nil, ErrPromotionInterrupted
	}
	var drifted []string
	for _, p := range slices.Sorted(maps.Keys(rec.Files)) {
		ok, err := liveMatches(root, p, rec.Files[p])
		if err != nil {
			return nil, err
		}
		if !ok {
			drifted = append(drifted, p)
		}
	}
	return drifted, nil
}

// liveMatches reports whether the live path p holds the digest want; a
// missing path does not.
func liveMatches(root *os.Root, p, want string) (bool, error) {
	got, err := liveDigest(root, p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return got == want, nil
}

// liveDigest returns the digest render.json holds for the live path p.
func liveDigest(root *os.Root, p string) (string, error) {
	if isKitPath(p) {
		d, err := kitDirDigest(root, p)
		return d, err
	}
	data, err := root.ReadFile(p)
	if err != nil {
		return "", err
	}
	return plainSum(data), nil
}

// kitDirDigest returns the kit-tree digest of the live kit dir p.
func kitDirDigest(root *os.Root, p string) (string, error) {
	sub, err := root.OpenRoot(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = sub.Close() }()
	files, err := kitFiles(sub.FS())
	if err != nil {
		return "", fmt.Errorf("render: read the kit dir %s: %w", p, err)
	}
	d, err := gate.KitTreeDigest(files)
	if err != nil {
		return "", fmt.Errorf("render: the kit dir %s: %w", p, err)
	}
	return d.String(), nil
}
