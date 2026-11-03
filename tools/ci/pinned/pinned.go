// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package pinned resolves a tool that mise.toml pins for the trees
// outside tools/ci that start one, tools/release first (12 12.1,
// REQ-1053): the path "mise which" gives, held to the version mise.lock
// locks, by the mise package of tools/ci, which the internal rule of Go
// keeps from them. It adds no rule of its own.
package pinned

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// Which returns the path of the tool name, one of the pinned tools of
// the tree at root (mise.PinnedTools), as "mise which" resolves it and
// mise.WhichPinned checks it: a tool whose lock key and install
// directory are its own name, which is every tool mise installs from
// a release archive (gh, git-cliff) and the go command. A name that is
// not pinned, or a tool that is not installed at the locked version,
// is an error that says what to run.
func Which(ctx context.Context, root, name string) (string, error) {
	return mise.WhichPinned(ctx, root, name, name, name)
}
