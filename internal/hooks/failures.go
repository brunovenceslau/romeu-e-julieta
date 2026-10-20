// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/renameio/v2"
)

// Failure is one failing run of a hook, recorded in julieta state until
// a later run of the same hook in the same repo succeeds (04 4.3,
// REQ-0409; 08 8.2, REQ-0675): the hook, the repo, the stage that
// failed and its path, the exit status and the time.
type Failure struct {
	At       time.Time `json:"at"`
	ExitCode int       `json:"exitCode"`
	Hook     string    `json:"hook"`
	Path     string    `json:"path"`
	Repo     string    `json:"repo"`
	Stage    string    `json:"stage"`
}

// failureFile returns the file name of the failure of hook in repo: one
// file for each pair, so that recording and clearing are a rename and a
// remove and two hooks that finish together lose no record of each
// other. The repo enters the name as a digest, since a path is not a
// name.
func failureFile(repo, hook string) string {
	sum := sha256.Sum256([]byte(repo))
	return hook + "-" + hex.EncodeToString(sum[:8]) + ".json"
}

// Record writes f below dir, replacing the failure of the same hook in
// the same repo, by a rename, so a reader sees the old file or the new
// one (03 3.8). The directory is the user's alone.
func Record(dir string, f Failure) error {
	data, err := json.Marshal(f, json.Deterministic(true))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return renameio.WriteFile(filepath.Join(dir, failureFile(f.Repo, f.Hook)), append(data, '\n'), 0o600, renameio.WithTempDir(dir))
}

// Clear removes the failure of hook in repo, if there is one.
func Clear(dir, repo, hook string) error {
	err := os.Remove(filepath.Join(dir, failureFile(repo, hook)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Failures returns the failures recorded below dir, the oldest first.
// A file that cannot be read, or that is not a failure of the name it
// has, is skipped and reported in the error, which joins one error for
// each; the failures read are returned with it.
func Failures(dir string) ([]Failure, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Failure
	var errs []error
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var f Failure
		if err := json.Unmarshal(data, &f, json.RejectUnknownMembers(true)); err != nil {
			errs = append(errs, fmt.Errorf("hook failure %s: %w", e.Name(), err))
			continue
		}
		if failureFile(f.Repo, f.Hook) != e.Name() {
			errs = append(errs, fmt.Errorf("hook failure %s: its repo and hook name another file", e.Name()))
			continue
		}
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b Failure) int {
		if c := a.At.Compare(b.At); c != 0 {
			return c
		}
		return strings.Compare(failureFile(a.Repo, a.Hook), failureFile(b.Repo, b.Hook))
	})
	return out, errors.Join(errs...)
}
