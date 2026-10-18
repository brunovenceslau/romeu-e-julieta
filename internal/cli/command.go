// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import "github.com/spf13/cobra"

// The five side-effect fields of a command definition (12 12.3; 04 4.1,
// Idempotency and Locking). Each is a type whose zero value is "unset",
// so a definition that leaves one out fails Register, and the generator
// and the meta-tests of 10 10.1 read the same fields.
type (
	// Preflight says whether the command runs the preflight of 01 1.5
	// before its sbx calls: the P marker of 04 4.2.
	Preflight int
	// Lock says whether the command takes the romeu lock (04 4.1,
	// Locking).
	Lock int
	// Snapshot says whether the command writes a julieta snapshot, as
	// julieta snapshot does (04 4.1, Idempotency).
	Snapshot int
	// Idempotency says whether a second run with no input change leaves
	// the files and refs as they were, adds one thing per run, or writes
	// nothing at all (04 4.1, Idempotency).
	Idempotency int
)

// The values of Preflight. Each type's zero value is its unset value.
const (
	PreflightUnset Preflight = iota
	PreflightRuns
	PreflightNone
)

// The values of Lock.
const (
	LockUnset Lock = iota
	LockTaken
	LockNone
)

// The values of Snapshot.
const (
	SnapshotUnset Snapshot = iota
	SnapshotWrites
	SnapshotNone
)

// The values of Idempotency.
const (
	IdempotencyUnset Idempotency = iota
	Converges
	AppendsEachRun
	ReadsOnly
)

// Appending names what an appending command adds on each run (04 4.1,
// Idempotency): the appending list, which the idempotency meta-test of
// 10 10.1 reads. Its zero value is unset; Appends and AppendsNothing
// make the two set values.
type Appending struct {
	set  bool
	What string
}

// Appends returns the Appending of a command that adds what on each run.
func Appends(what string) Appending { return Appending{set: true, What: what} }

// AppendsNothing returns the Appending of a command that adds nothing on
// a run.
func AppendsNothing() Appending { return Appending{set: true} }

// Effects holds the five side-effect fields of a definition.
type Effects struct {
	Preflight   Preflight
	Lock        Lock
	Appending   Appending
	Snapshot    Snapshot
	Idempotency Idempotency
}

// Command is the definition of one command of a binary (ADR 0015,
// decision 2). A command package builds it with Define, fills the other
// fields, and registers it from a package-level variable:
// var _ = cli.Register(def).
type Command struct {
	// Binary is the binary the command belongs to: romeu or julieta.
	Binary string
	// Name is the command's name on the command line.
	Name string
	// Sub is the subcommand's name, such as "add" in "julieta memory
	// add", or empty for a command with no subcommands (04 4.1,
	// Parsing). A command name either has subcommands, each a definition
	// of its own, or is one definition with no Sub.
	Sub string
	// JSON says that the command takes --json (04 4.1, Output), which
	// the tree adds to its flags and which sets Env.JSON.
	JSON bool
	// New returns a new *cobra.Command for one run: its Use starts with
	// the last word of the command's path, it sets Args, and its RunE
	// does the work with env. Short is the line the help of the level
	// above prints for it, and Example holds the command lines its help
	// shows, one per line, each one the definitions accept (04 4.1,
	// Error ids, item 6). It sets no other run function and no
	// Deprecated, Version, help or json flag; Register refuses those.
	New func(env *Env) *cobra.Command

	effects Effects
}

// Define returns a definition with its five side-effect fields, which
// are its only arguments, so none has a default (12 12.3).
func Define(preflight Preflight, lock Lock, appending Appending, snapshot Snapshot, idempotency Idempotency) Command {
	return Command{effects: Effects{
		Preflight:   preflight,
		Lock:        lock,
		Appending:   appending,
		Snapshot:    snapshot,
		Idempotency: idempotency,
	}}
}

// Line returns the command line that names the definition after the
// binary: its name, then its subcommand when it has one.
func (c Command) Line() string {
	if c.Sub == "" {
		return c.Name
	}
	return c.Name + " " + c.Sub
}

// Effects returns the side-effect fields of the definition.
func (c Command) Effects() Effects { return c.effects }
