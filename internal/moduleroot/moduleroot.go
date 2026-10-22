// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package moduleroot finds the root of this module from a directory
// inside it. It is the one copy that tools/ci's tests (citest) and the
// container e2e suite (e2e/internal/container) share: both are tests
// that start in the directory of their package and need the repository
// root, and a package under tools/ci/internal or e2e/internal is visible
// to one of them only, so the package sits under the root internal/.
// Table 10 10.7 gives it a row (REQ-1256): no binary links it.
//
// It is hand-written for want of a function to import: the go command
// answers "go env GOMOD" with a subprocess, which a test of the e2e
// suite and the tests of tools/ci do not otherwise need, and the
// standard library has none.
package moduleroot

import (
	"errors"
	"os"
	"path/filepath"
)

// Find returns the nearest directory at or above dir that holds go.mod.
func Find(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above " + dir)
		}
		dir = parent
	}
}
