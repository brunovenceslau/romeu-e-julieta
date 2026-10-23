// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// The files of a repo this package reads, the only ones julieta knows
// (07 7.2: mise.toml and mise.lock of each repo), and the exceptions
// file (ExceptionsFile).
const (
	ConfigFile = "mise.toml"
	LockFile   = "mise.lock"
)

// Raw are the bytes of the files of a repo, as read from the work tree
// or from the index, before any of them is parsed.
type Raw struct {
	Config, Lock, Exceptions          []byte
	HasConfig, HasLock, HasExceptions bool
}

// Files are the files of a repo, parsed.
type Files struct {
	// HasConfig says the repo has a mise.toml; a repo without one
	// contributes nothing (07 7.5, step 1).
	HasConfig bool
	// Config is the mise.toml, when HasConfig.
	Config Config
	// HasLock says the repo has a mise.lock.
	HasLock bool
	// Lock is the mise.lock, when HasLock.
	Lock Lock
	// LockData is the bytes of the mise.lock, when HasLock, which the
	// lock-set digest hashes.
	LockData []byte
	// Exceptions are the tools the exceptions file excuses.
	Exceptions Exceptions
}

// Reader returns the bytes of the file name of a repo, a path relative
// to its top with forward slashes, and whether the file is there. It is
// what ReadRaw is over the work tree and what the pre-commit action
// supplies over the index.
type Reader func(name string) (data []byte, found bool, err error)

// RawFrom reads the three files through read.
func RawFrom(read Reader) (Raw, error) {
	var r Raw
	var err error
	if r.Config, r.HasConfig, err = read(ConfigFile); err != nil {
		return Raw{}, err
	}
	if r.Lock, r.HasLock, err = read(LockFile); err != nil {
		return Raw{}, err
	}
	if r.Exceptions, r.HasExceptions, err = read(ExceptionsFile); err != nil {
		return Raw{}, err
	}
	return r, nil
}

// ReadRaw reads the files of the repo at dir from its work tree. A file
// that is not there is not an error; one that is not a regular file or
// is over MaxFileSize is. The reads go through os.Root, so no name
// leaves the repo.
func ReadRaw(dir string) (Raw, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Raw{}, err
	}
	defer func() { _ = root.Close() }()
	return RawFrom(func(name string) ([]byte, bool, error) { return readFile(root, name) })
}

// ReadConfig reads and parses the mise.toml of the repo at dir alone,
// which julieta lock needs when the lock is stale or broken.
func ReadConfig(dir string) (Files, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Files{}, err
	}
	defer func() { _ = root.Close() }()
	data, ok, err := readFile(root, ConfigFile)
	if err != nil || !ok {
		return Files{}, err
	}
	cfg, err := ParseConfig(data)
	if err != nil {
		return Files{}, errors.Join(ErrUnparsed, err)
	}
	return Files{HasConfig: true, Config: cfg}, nil
}

// CheckLockTarget refuses a mise.lock that exists and is not a regular
// file, before mise writes it: mise would follow a link and write where
// it points (the audit of 2026-10-22). A lock that is not there is fine.
func CheckLockTarget(dir string) error {
	info, err := os.Lstat(filepath.Join(dir, LockFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return err
	case !info.Mode().IsRegular():
		return fmt.Errorf("%s is not a regular file", LockFile)
	}
	return nil
}

// readFile reads the regular file name of root, at most MaxFileSize
// bytes, and says whether it is there.
func readFile(root *os.Root, name string) ([]byte, bool, error) {
	info, err := root.Lstat(path.Clean(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s is not a regular file", name)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > MaxFileSize {
		return nil, false, fmt.Errorf("%s: more than the cap of %d bytes", name, MaxFileSize)
	}
	return data, true, nil
}

// CheckDir is Raw.Check of the files of the repo at dir, which julieta
// lock --check and tools/ci mise each run: it reads and writes nothing
// else. A repo with no mise.toml has nothing to check and no findings; a
// mise.toml with no mise.lock has a finding for each of its tools; a
// file that does not parse is a finding.
func CheckDir(dir string) (Report, Raw, error) {
	raw, err := ReadRaw(dir)
	if err != nil {
		return Report{}, Raw{}, err
	}
	return raw.Check(), raw, nil
}
