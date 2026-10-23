// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package preflight is the preflight of 01 1.5: the six ordered checks
// every command marked P of 04 4.2 runs before a mutating sbx call. It
// reads sbx and the host and changes nothing, and it never touches the
// network (REQ-0077, 10 10.1).
//
// Steps 1 to 4 belong to other packages (the toolchain acknowledgement,
// the promotion recovery, the drift check, the widening digest); the
// preflight receives each as a function in Deps and fixes only the order
// and the stop rule. Steps 5 and 6 are here: step5.go lists the host
// checks, and step6.go is the sandbox identity of I26.
package preflight

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// Command is a romeu command marked P (04 4.2).
type Command string

// The commands that run the preflight.
const (
	CmdSync            Command = "sync"
	CmdRun             Command = "run"
	CmdStop            Command = "stop"
	CmdSalvage         Command = "salvage"
	CmdSalvageFromHost Command = "salvage --from-host"
	CmdRm              Command = "rm"
	CmdRecreate        Command = "recreate"
	CmdRetire          Command = "retire"
	CmdPull            Command = "pull"
)

// known reports whether c is a command of the list above.
func (c Command) known() bool {
	switch c {
	case CmdSync, CmdRun, CmdStop, CmdSalvage, CmdSalvageFromHost, CmdRm, CmdRecreate, CmdRetire, CmdPull:
		return true
	}
	return false
}

// protective reports whether c is one of the protective commands of
// REQ-0078: salvage, salvage --from-host, stop and pull widen nothing
// and forward no socket.
func (c Command) protective() bool {
	switch c {
	case CmdSalvage, CmdSalvageFromHost, CmdStop, CmdPull:
		return true
	}
	return false
}

// removes reports whether c is one of the commands for which a
// generation in the state removing, with its sandbox present, passes
// step 6 (REQ-0076).
func (c Command) removes() bool {
	return c == CmdRm || c == CmdRecreate || c == CmdRetire
}

// Sbx is what the preflight reads from sbx. Every method is a read; the
// bytes of the two listings are decoded here by the upstream rule of 03
// (REQ-0170), and List is decoded by sbxdrv.ParseList.
type Sbx interface {
	// List is sbx ls --json, decoded: the sandboxes of the host.
	List(ctx context.Context) ([]sbxdrv.Sandbox, error)
	// PolicyList is the output of sbx policy ls --json.
	PolicyList(ctx context.Context) ([]byte, error)
	// SecretList is the output of the sbx secret listing, global rows
	// included.
	SecretList(ctx context.Context) ([]byte, error)
}

// Deps are the seams of the preflight. Steps 1 to 4 are functions that
// return nil, or the error of their own row; a nil one is a defect.
type Deps struct {
	// Toolchain is step 1, gate 1: the toolchain acknowledgement
	// (REQ-0075, exit 3). The check function of romeu approve
	// --toolchain (T042) is this function. It returns nil, or an error
	// of the table that the preflight passes on unchanged, whatever its
	// row: toolchain-approval-required (RJ-302), sbx-floor (RJ-343), the
	// failure of an sbx call, upstream-shape (RJ-310), record-unreadable
	// (RJ-334) or internal (RJ-348).
	Toolchain func(ctx context.Context) error
	// Promotion is step 2: an interrupted promotion is finished or
	// reported (REQ-1203, exit 4).
	Promotion func(ctx context.Context) error
	// Drift is step 3: drift of every live derived file against
	// render.json (REQ-1204, exit 4).
	Drift func(ctx context.Context) error
	// Widening is step 4, gate 2: the widening digest recomputed from the
	// live files equals the approval (REQ-1205, exit 3).
	Widening func(ctx context.Context) error
	// Sbx is what steps 5 and 6 read from sbx.
	Sbx Sbx
	// VSCodeTrustedFolders returns the folders VS Code trusts, as paths
	// or file:// URIs. VS Code keeps them in its state database, which
	// this package does not read (it would need an SQLite reader); a nil
	// function leaves the trusted folders unchecked, and the setting
	// security.workspace.trust.enabled is checked all the same.
	VSCodeTrustedFolders func() ([]string, error)
}

// Input is what the preflight needs to know about the command.
type Input struct {
	// Command is the command that runs the preflight.
	Command Command
	// SalvageHalf says that rm, recreate or retire is running its
	// salvage half, which runs the steps as a protective command does
	// (REQ-0078); the removal half runs them again with it false.
	SalvageHalf bool
	// Project is the project's name, which is its sandbox's name.
	Project string
	// Primary is the directory name of the primary repo.
	Primary string
	// Root is $ROMEU_ROOT.
	Root string
	// Home is the operator's home directory.
	Home string
	// OS is runtime.GOOS; it decides where VS Code keeps its settings.
	OS string
	// Getenv reads the environment (XDG_CONFIG_HOME, XDG_DATA_HOME,
	// MISE_TRUSTED_CONFIG_PATHS); nil reads nothing.
	Getenv func(string) string
	// UsedServices are the service names of the secrets entries of the
	// projects romeu reads: a global sbx secret for one of them widens
	// that sandbox (I22).
	UsedServices []string
	// Generations are the generations of the project's record, newest
	// last; none when the project has no record.
	Generations []state.Generation
}

// Printed is a failure that a protective command prints and goes on
// from (REQ-0078).
type Printed struct {
	// Step is the step, 1 to 6.
	Step int
	// Err is the error of the step.
	Err error
}

// Result is what a preflight that passed has to report.
type Result struct {
	// Printed are the failures of steps 1, 4 and 5 that a protective
	// command prints and goes on from, in the order of the steps.
	Printed []Printed
	// Unchecked names the host checks that could not run in full because
	// a seam was not given, so that a pass does not read as a complete
	// check: "vscode-trusted-folders" when Deps.VSCodeTrustedFolders is
	// nil. A caller that reports (doctor, status) says so.
	Unchecked []string
}

// step is one step of 01 1.5.
type step struct {
	n int
	// printed says that a protective command prints the failure of the
	// step and goes on (steps 1, 4 and 5, REQ-0078).
	printed bool
	run     func(ctx context.Context, d *Deps, in *Input, res *Result) error
}

// steps are the six steps of 01 1.5 in their order (REQ-0073).
var steps = []step{
	{1, true, func(ctx context.Context, d *Deps, _ *Input, _ *Result) error { return d.Toolchain(ctx) }},
	{2, false, func(ctx context.Context, d *Deps, _ *Input, _ *Result) error { return d.Promotion(ctx) }},
	{3, false, func(ctx context.Context, d *Deps, _ *Input, _ *Result) error { return d.Drift(ctx) }},
	{4, true, func(ctx context.Context, d *Deps, _ *Input, _ *Result) error { return d.Widening(ctx) }},
	{5, true, hostStep},
	{6, false, identityStep},
}

// Run runs the six steps in order and stops at the first failure, which
// it returns with the exit of its row. A protective command, or the
// salvage half of rm, recreate and retire, prints a failure of steps 1,
// 4 and 5 in Result and goes on (REQ-0078).
func Run(ctx context.Context, d Deps, in Input) (Result, error) {
	return RunSteps(ctx, d, in, 1, len(steps))
}

// RunSteps runs steps from to to, both included. sync runs step 1 before
// its first sbx read and steps 2 to 6 before its first mutating call
// (REQ-0074), so it calls RunSteps twice.
func RunSteps(ctx context.Context, d Deps, in Input, from, to int) (Result, error) {
	if err := validate(&d, &in, from, to); err != nil {
		return Result{}, err
	}
	protective := in.Command.protective() || in.SalvageHalf
	var res Result
	for _, s := range steps[from-1 : to] {
		err := s.run(ctx, &d, &in, &res)
		if err == nil {
			continue
		}
		if protective && s.printed {
			res.Printed = append(res.Printed, Printed{Step: s.n, Err: err})
			continue
		}
		return res, err
	}
	return res, nil
}

// validate refuses a call a caller got wrong: a defect of the code.
func validate(d *Deps, in *Input, from, to int) error {
	switch {
	case from < 1 || to > len(steps) || from > to:
		return cli.Internal.New(fmt.Sprintf("the preflight has no steps %d to %d", from, to))
	case !in.Command.known():
		return cli.Internal.New(fmt.Sprintf("the preflight does not know the command %q", in.Command))
	case in.SalvageHalf && !in.Command.removes():
		return cli.Internal.New(fmt.Sprintf("only rm, recreate and retire have a salvage half, not %q", in.Command))
	case !filepath.IsAbs(in.Root) || !filepath.IsAbs(in.Home):
		return cli.Internal.New("the preflight was given a root or a home that is not an absolute path")
	case in.Project == "" || in.Primary == "":
		return cli.Internal.New("the preflight was given no project or no primary repo")
	case in.UsedServices == nil:
		return cli.Internal.New("the preflight was given no list of used services (an empty list is []string{})")
	case d.Toolchain == nil || d.Promotion == nil || d.Drift == nil || d.Widening == nil || d.Sbx == nil:
		return cli.Internal.New("the preflight was given a nil step")
	}
	return nil
}
