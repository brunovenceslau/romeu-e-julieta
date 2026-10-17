// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Pin is the mise that the workflows pin for one runner of their
// matrices: the "version" input of mise-action, and the mise_sha256 of
// the matrix entry whose os is Label, the hash of the bare binary of
// that platform in mise's release (Asset).
type Pin struct {
	Version, Sha256, Label string
}

// runnerAssets maps each runner label of the matrix (the Runners list
// of 10 10.2) to the platform part of the name of mise's binary for it;
// runnerLabels maps the platform of a Go binary to its runner label.
var (
	runnerAssets = map[string]string{
		"ubuntu-26.04":     "linux-x64",
		"ubuntu-26.04-arm": "linux-arm64",
		"macos-26":         "macos-arm64",
		"macos-26-intel":   "macos-x64",
	}
	runnerLabels = map[string]string{
		"linux/amd64":  "ubuntu-26.04",
		"linux/arm64":  "ubuntu-26.04-arm",
		"darwin/arm64": "macos-26",
		"darwin/amd64": "macos-26-intel",
	}
)

// pinVersion and pinSha256 are the shapes a version and a sha256 of
// the pin must have before either reaches a path or a URL.
var (
	pinVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	pinSha256  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Asset is the name of the bare binary of mise for the pin's runner in
// mise's release, "mise-v<version>-<os>-<arch>"; no archive is read.
func (p Pin) Asset() string {
	return "mise-v" + p.Version + "-" + runnerAssets[p.Label]
}

// ReadPin reads the pin of this platform from the workflows that pin
// mise in the tree at root (pinWorkflows): the version from
// PinnedWorkflow, which ScheduledWorkflow repeats, and the sha256 from
// the matrix that names this platform's runner.
func ReadPin(root string) (Pin, error) {
	return readPin(root, runtime.GOOS, runtime.GOARCH)
}

// readPin is ReadPin for the platform goos/goarch. The version must be
// three numbers joined by dots and the sha256 64 lowercase hex digits,
// and the matrices must give the platform's runner one sha256.
func readPin(root, goos, goarch string) (Pin, error) {
	label, ok := runnerLabels[goos+"/"+goarch]
	if !ok {
		return Pin{}, fmt.Errorf("no runner of %s or %s is %s/%s, so they pin no mise for it", PinnedWorkflow, ScheduledWorkflow, goos, goarch)
	}
	srcs, err := pinWorkflows(root)
	if err != nil {
		return Pin{}, err
	}
	version, err := versionOf(srcs)
	if err != nil {
		return Pin{}, err
	}
	if !pinVersion.MatchString(version) {
		return Pin{}, fmt.Errorf("%s pins the version of mise %q, which is not three numbers joined by dots", PinnedWorkflow, version)
	}
	var shas []matrixSha
	paths := make([]string, len(srcs))
	for i, w := range srcs {
		paths[i] = w.path
		if err := matrixShas(w.path, w.src, label, &shas); err != nil {
			return Pin{}, err
		}
	}
	switch {
	case len(shas) == 0:
		return Pin{}, fmt.Errorf("no matrix of %s gives a mise_sha256 for %s, the runner of this platform", strings.Join(paths, " or "), label)
	case len(shas) > 1:
		found := make([]string, len(shas))
		for i, s := range shas {
			found[i] = s.sha + " in " + s.path
		}
		return Pin{}, fmt.Errorf("the matrices give two different mise_sha256 for %s: %s", label, strings.Join(found, ", "))
	case !pinSha256.MatchString(shas[0].sha):
		return Pin{}, fmt.Errorf("the mise_sha256 %q of %s in %s is not 64 lowercase hex digits", shas[0].sha, label, shas[0].path)
	}
	return Pin{Version: version, Sha256: shas[0].sha, Label: label}, nil
}

// matrixSha is one mise_sha256 of a runner, with the workflow that
// holds it.
type matrixSha struct{ sha, path string }

// matrixShas adds to shas, once for each value, the mise_sha256 of
// every matrix entry of the workflow src whose os is label.
func matrixShas(path string, src []byte, label string, shas *[]matrixSha) error {
	var wf struct {
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct {
					// Strings, so that a hash of decimal digits alone
					// keeps its text.
					Include []struct {
						OS     string `yaml:"os"`
						Sha256 string `yaml:"mise_sha256"`
					} `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(src, &wf); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, job := range wf.Jobs {
		for _, entry := range job.Strategy.Matrix.Include {
			if entry.OS == label && !slices.ContainsFunc(*shas, func(s matrixSha) bool { return s.sha == entry.Sha256 }) {
				*shas = append(*shas, matrixSha{entry.Sha256, path})
			}
		}
	}
	return nil
}

// cacheApp is the directory of this repository below the user cache
// directory.
const cacheApp = "romeu-e-julieta"

// CacheDirs returns the directories that hold the copy of mise setup
// installs for version, outermost first: romeu-e-julieta, mise and the
// version's, below the user cache directory of the XDG Base Directory
// rule (cacheBase). version must have the shape of a pin's (ReadPin).
func CacheDirs(version string) ([]string, error) {
	base, err := cacheBase()
	if err != nil {
		return nil, fmt.Errorf("the user cache directory, where setup installs mise: %w", err)
	}
	app := filepath.Join(base, cacheApp)
	return []string{app, filepath.Join(app, "mise"), filepath.Join(app, "mise", version)}, nil
}

// cacheBase is the user cache directory of the XDG Base Directory rule,
// on every platform, macOS included: $XDG_CACHE_HOME when it is an
// absolute path (a relative one is ignored, as the rule says), and
// otherwise $HOME/.cache.
func cacheBase() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME is not set, and XDG_CACHE_HOME names no absolute path")
	}
	return filepath.Join(home, ".cache"), nil
}

// CachePath returns the path of the copy of mise setup installs for
// version, in the last of CacheDirs.
func CachePath(version string) (string, error) {
	dirs, err := CacheDirs(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs[len(dirs)-1], "mise"), nil
}

// CheckDir fails unless path is a directory, not a symbolic link, that
// this user owns and no one else can write.
func CheckDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		return fmt.Errorf("%s is a symbolic link, and the directories of the mise cache are not", path)
	case !info.IsDir():
		return fmt.Errorf("%s is not a directory", path)
	case info.Mode().Perm()&0o022 != 0:
		return fmt.Errorf("others can write %s (mode %v), a directory of the mise cache", path, info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s, a directory of the mise cache, is not this user's", path)
	}
	return nil
}

// Sha256File returns the sha256 of the file at path, in lowercase hex.
func Sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cachedCopy returns the path of the copy of mise that setup installed
// for the pin of the tree at root, when it passes every rule of
// REQ-1236: a regular file, not a symbolic link, that its owner can run
// and no one else can write, in directories this user owns and no one
// else can write, whose sha256, read now, is the pin's. Any failure, a tree without a pin included, is no copy.
func cachedCopy(root string) (string, bool) {
	pin, err := ReadPin(root)
	if err != nil {
		return "", false
	}
	dirs, err := CacheDirs(pin.Version)
	if err != nil {
		return "", false
	}
	for _, dir := range dirs {
		if CheckDir(dir) != nil {
			return "", false
		}
	}
	path := filepath.Join(dirs[len(dirs)-1], "mise")
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() ||
		info.Mode().Perm()&0o100 == 0 || info.Mode().Perm()&0o022 != 0 {
		return "", false
	}
	if sum, err := Sha256File(path); err != nil || sum != pin.Sha256 {
		return "", false
	}
	return path, true
}

// Path returns the one absolute path by which tools/ci starts every
// mise command of the tree at root (REQ-1236): the copy that setup
// installed for the pin, checked again on every call (cachedCopy), and
// otherwise the mise on the search path. No mise at all is
// procgroup.ErrNotFound. A writer with this user's account can swap
// the copy between the hash and the start; that account is trusted, as
// everywhere in tools/ci.
func Path(root string) (string, error) {
	if path, ok := cachedCopy(root); ok {
		return path, nil
	}
	path, err := procgroup.LookPath("mise")
	if err != nil {
		if errors.Is(err, procgroup.ErrNotFound) {
			return "", procgroup.ErrNotFound
		}
		return "", err
	}
	return filepath.Abs(path)
}
