// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
)

// KitDigest is the digest of the skills and hook lines the matching
// julieta-claude kit installs (06 6.3, REQ-0587): Digest of KitFiles of
// kits/julieta-claude, which a test of this package holds it to.
// julieta setup compares it with the digest of the installed ones and
// reports skills-stale when they differ.
const KitDigest = "sha256:0d4931eda8f618b8e3aee41812f96a5b8016b2ba73b1c131874a52ae5778dd4b"

// Digest returns the digest of a set of files by their slash-separated
// names: the sha256 of one line per file, in name order, of its name
// and the sha256 of its content, each name quoted so that no name can
// stand for two.
func Digest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	h := sha256.New()
	for _, n := range names {
		sum := sha256.Sum256(files[n])
		_, _ = h.Write([]byte(strconv.Quote(n) + " " + hex.EncodeToString(sum[:]) + "\n"))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// KitFiles returns the files of the kit directory kit that the digest
// covers: each file of the skills, as skills/<skill>/<path>, and the
// settings file, which holds the hook lines with the placeholder.
func KitFiles(kit fs.FS) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, s := range Skills {
		if err := readTree(kit, path.Join("skills", s), "", files); err != nil {
			return nil, err
		}
	}
	b, err := fs.ReadFile(kit, SettingsFile)
	if err != nil {
		return nil, err
	}
	files[SettingsFile] = b
	return files, nil
}

// Installed returns the files the kit installed, named as KitFiles
// names them: the skills below home, and the drop-in at dropIn with the
// path of julieta given back as the placeholder, so the hook lines of
// another julieta differ. A skill or a drop-in that is absent is left
// out, which makes the digest differ; any other failed read is an
// error.
func Installed(home, dropIn, julieta string) (map[string][]byte, error) {
	files := map[string][]byte{}
	skills := os.DirFS(filepath.Join(home, SkillsDir))
	for _, s := range Skills {
		err := readTree(skills, s, "skills/", files)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	b, err := os.ReadFile(dropIn)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	case julieta == "":
		files[SettingsFile] = b // no path to give back: the digest differs
	default:
		files[SettingsFile] = bytes.ReplaceAll(b, []byte(julieta), []byte(Placeholder))
	}
	return files, nil
}

// readTree adds every file below dir of fsys to files, by prefix and
// its name in fsys. A dir that is not a directory is an error.
func readTree(fsys fs.FS, dir, prefix string, files map[string][]byte) error {
	info, err := fs.Stat(fsys, dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &fs.PathError{Op: "read", Path: dir, Err: errors.New("not a directory")}
	}
	return fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		files[prefix+p] = b
		return err
	})
}
