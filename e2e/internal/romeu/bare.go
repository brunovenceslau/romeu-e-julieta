// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import "path/filepath"

// Bare returns the path of the bare repository owner/repo that the
// origins serve, for a test that sets up a fault in the origin itself
// (a HEAD that names no branch) with Origins.Git.
func (o *Origins) Bare(ownerRepo string) string {
	return filepath.Join(o.root, filepath.FromSlash(ownerRepo))
}
