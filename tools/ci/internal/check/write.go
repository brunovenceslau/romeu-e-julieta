// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// generatedFileMode is the mode of a file WriteIfChanged writes.
const generatedFileMode = 0o644

// WriteIfChanged writes content to path below root, with its
// directories, unless a regular file holds exactly those bytes already
// and no execute bit, and reports whether it wrote. A file that is
// current keeps its modification time. The bytes go to a new file in
// the same directory that then replaces path, so a symbolic link or a
// hard link at path is replaced by a regular file and never written
// through: a local hard link to a file outside the repository keeps its
// own bytes.
func WriteIfChanged(root *os.Root, path string, content []byte) (bool, error) {
	info, err := root.Lstat(path)
	switch {
	case err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 == 0:
		old, err := root.ReadFile(path)
		if err != nil {
			return false, err
		}
		if bytes.Equal(old, content) {
			return false, nil
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	dir := filepath.Dir(path)
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	tmp := filepath.Join(dir, "."+filepath.Base(path)+"."+rand.Text()+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, generatedFileMode)
	if err != nil {
		return false, err
	}
	_, err = f.Write(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	// The mode is set after the umask took its share, and an execute bit
	// is the one thing git tracks of a mode.
	if err == nil {
		err = root.Chmod(tmp, generatedFileMode)
	}
	if err == nil {
		err = root.Rename(tmp, path)
	}
	if err != nil {
		return false, errors.Join(err, removeIfExists(root, tmp))
	}
	return true, nil
}

// removeIfExists removes the file at path below root, and is not an
// error when there is none.
func removeIfExists(root *os.Root, path string) error {
	if err := root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
