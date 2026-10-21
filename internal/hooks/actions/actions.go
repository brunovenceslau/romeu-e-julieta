// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package actions is the seam of julieta's own hook actions (04 4.3,
// REQ-0408; rule 14 of docs/plan.md): the action that runs first in
// the dispatcher's chain for a hook, before the repo's tracked hook
// and the local one. Each action is one file of this package that
// registers itself by file, as a package-level
//
//	var _ = register("pre-commit", Action{...})
//
// during initialization, so the registry is written then and read-only
// after. The snapshot (08 8.4) is the action of post-commit,
// post-rewrite and post-merge, one file each (snapshot_post_commit.go,
// snapshot_post_rewrite.go, snapshot_post_merge.go); each runs the
// handler the package of julieta snapshot sets with HandleSnapshot
// (snapshot_action.go), which calls the writer Snapshot (snapshot.go).
// The file that lands with a later task is lock_check.go (pre-commit,
// T053); until it lands, pre-commit has no action and its chain starts
// at the tracked hook.
package actions

import (
	"context"
	"io"
	"slices"
	"strings"
)

// Input is what the dispatcher gives an action: the hook it runs for
// and the arguments git gave that hook, the directory the hooks run in
// (the root of the work tree, or the git dir of a bare repository),
// the environment git gave the hook, and the writers for the output.
// An action has no stdin: that goes to the first of the repo's own
// hooks that runs.
type Input struct {
	Hook   string
	Args   []string
	Dir    string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

// Action is one action of julieta.
type Action struct {
	// Name names the stage in the line the dispatcher prints on a
	// failure: "julieta hooks: <hook> stage julieta (<Name>) exited
	// <n>".
	Name string
	// Run does the work. A non-nil error is a failure of the stage: if
	// it carries an error of the table (cli.Error) the dispatcher
	// prints its id and fix and exits with its code, otherwise with 1.
	Run func(ctx context.Context, in Input) error
	// Continues says that a failure is recorded in julieta state and
	// does not stop the chain, which goes on to the repo's own hooks:
	// the snapshot (04 4.3, REQ-0409; 08 8.4, REQ-0691).
	Continues bool
}

// registry maps a hook name to its actions. It is written only during
// initialization, by register.
var registry = map[string][]Action{}

// register adds a to the actions of hook and returns it, so that a
// file can write var _ = register(...). It is called during
// initialization only.
func register(hook string, a Action) Action {
	registry[hook] = append(registry[hook], a)
	return a
}

// For returns the actions of hook, by name, so that the order does not
// depend on the order the files initialize in.
func For(hook string) []Action {
	out := slices.Clone(registry[hook])
	slices.SortStableFunc(out, func(a, b Action) int { return strings.Compare(a.Name, b.Name) })
	return out
}
