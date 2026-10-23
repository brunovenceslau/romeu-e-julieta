// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// identityStep is step 6, the sandbox identity of I26 (REQ-0076,
// REQ-0511). An absent sandbox passes. A present one must have a
// generation romeu recorded that is open (open, salvaging, or removing
// for rm, recreate and retire) and the workspace
// $ROMEU_ROOT/<name>-env/<primary>; otherwise it is not this project's
// sandbox, and the error is unknown-sandbox naming romeu adopt, for both
// cases (REQ-0347). A generation in the state removing with its sandbox
// present passes for rm, recreate and retire and is removal-pending for
// every other command, salvage --from-host among them.
//
// The id or creation time of the sandbox is not compared: probe A4 has
// not shown that sbx reports one, and until then a sandbox recreated by
// hand under the same name and path passes (REQ-0511).
func identityStep(ctx context.Context, d *Deps, in *Input) error {
	list, err := d.Sbx.List(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(s sbxdrv.Sandbox) bool { return s.Name == in.Project })
	if i < 0 {
		return nil
	}
	got := list[i]
	g, ok := liveGeneration(in.Generations)
	if !ok {
		return unknownSandbox(in, "romeu has no open generation for the sandbox "+in.Project)
	}
	want := filepath.Join(in.Root, in.Project+"-env", in.Primary)
	if have := got.PrimaryWorkspace(); filepath.Clean(have) != want {
		return unknownSandbox(in, fmt.Sprintf("the workspace of the sandbox %s is not %s", in.Project, want),
			cli.Detail{Key: "workspace", Value: have}, cli.Detail{Key: "expected", Value: want})
	}
	if g.State == state.GenerationRemoving && !in.Command.removes() {
		e := RemovalPending.New(fmt.Sprintf("the sandbox %s is being removed, and %s does not finish a removal", in.Project, in.Command))
		e.Fill = map[string]string{"<name>": in.Project}
		return e
	}
	return nil
}

// liveGeneration returns the newest generation that is not closed.
func liveGeneration(gens []state.Generation) (state.Generation, bool) {
	for i := len(gens) - 1; i >= 0; i-- {
		switch gens[i].State {
		case state.GenerationOpen, state.GenerationSalvaging, state.GenerationRemoving:
			return gens[i], true
		}
	}
	return state.Generation{}, false
}

// unknownSandbox returns the error unknown-sandbox with its hint filled.
func unknownSandbox(in *Input, message string, details ...cli.Detail) *cli.Error {
	e := UnknownSandbox.New(message, details...)
	e.Fill = map[string]string{"<name>": in.Project, "<primary>": in.Primary}
	return e
}
