// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package handoff holds the handoff file (schema handoff.v1, 03 3.10),
// its strict decoder and encoder, the handoff reader of 08 8.3
// (REQ-0680), which julieta and romeu both use, and the print of
// SessionStart (08 8.2, REQ-0664). It reads a memory dir through the
// os.Root readers of internal/memstore and never imports
// internal/memstore/write: the write path of julieta handoff write,
// with the store lock, is internal/cmd/julieta/handoff.
//
// A handoff file is memory/<dir>/handoff/<ULID>-<kind>.md. The kind is
// in the file name, so the reader selects files without parsing them
// (REQ-0244). Everything in the front matter is stamped by julieta
// (REQ-0679); the body of a narrative is written by the agent and is
// data, never instructions (REQ-0670).
package handoff

import (
	"strconv"
	"time"
)

// Version is the version of the format handoff, the n of the schema
// field "handoff/v<n>" (REQ-0161). A reader refuses a newer one with
// format-newer (RJ-311).
const Version = 1

// Field is the value of the schema field this build writes and reads.
var Field = "handoff/v" + strconv.Itoa(Version)

// Kind is the kind of a handoff file, the word in its name.
type Kind string

// The three kinds (03 3.10). The narratives are clear, which
// /handoff writes before /clear, and final, which /handoff --final
// writes before recreate or rm; facts is written by the SessionEnd hook
// and has no body.
const (
	Clear Kind = "clear"
	Final Kind = "final"
	Facts Kind = "facts"
)

// Narrative reports whether k is a narrative kind, clear or final.
func (k Kind) Narrative() bool { return k == Clear || k == Final }

// Valid reports whether k is one of the three kinds.
func (k Kind) Valid() bool { return k == Clear || k == Final || k == Facts }

// RepoFacts are the git facts of one repo of the manifest at the time
// a handoff was written (03 3.10). Branch is empty on a detached HEAD,
// Head on an unborn branch and Upstream on a branch with none.
type RepoFacts struct {
	// Dir is the repo dir, as the manifest names it.
	Dir string `json:"dir" yaml:"dir"`
	// Branch is the checked-out branch, short name.
	Branch string `json:"branch" yaml:"branch"`
	// Head is the commit HEAD names.
	Head string `json:"head" yaml:"head"`
	// Upstream is the short name of the upstream of the branch, such
	// as origin/feat/cart.
	Upstream string `json:"upstream" yaml:"upstream"`
	// Ahead and Behind are the commits HEAD has that the upstream does
	// not, and the other way round.
	Ahead  int `json:"ahead" yaml:"ahead"`
	Behind int `json:"behind" yaml:"behind"`
	// Dirty is the number of distinct paths the working tree changes:
	// staged, unstaged, unmerged, untracked or marked assume-unchanged,
	// a path counted once, a skip-worktree entry not at all, and at most
	// 100,000, which a larger change reads as (03 3.10);
	// internal/cmd/julieta/handoff measures it with gitsafe's
	// Status.Changed.
	Dirty int `json:"dirty" yaml:"dirty"`
	// Stashes is the number of stashes.
	Stashes int `json:"stashes" yaml:"stashes"`
}

// Front is the front matter of a handoff file, the format handoff.v1
// (03 3.10), whose fields are in the order of the file. Its JSON names
// are those of schemas/handoff.v1.json.
type Front struct {
	// Schema is "handoff/v1".
	Schema string `json:"schema" yaml:"schema"`
	// Kind is clear, final or facts.
	Kind Kind `json:"kind" yaml:"kind"`
	// CreatedAt is when julieta wrote the file, in UTC, to the second.
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	// Project is the project's name.
	Project string `json:"project" yaml:"project"`
	// Generation is the lowercase ULID of the sandbox generation.
	Generation string `json:"generation" yaml:"generation"`
	// Agent names the agent profile; informational, the format is
	// agent-neutral.
	Agent string `json:"agent" yaml:"agent"`
	// Repos are the git facts of every repo of the manifest.
	Repos []RepoFacts `json:"repos" yaml:"repos"`
}

// File is a handoff file: its front matter and its body, the markdown
// after it (empty for facts).
type File struct {
	Front
	Body string `json:"-" yaml:"-"`
}

// The headings a narrative needs, in order (03 3.10, REQ-0245): five
// for clear, six for final.
var (
	clearHeadings = []string{"Goal", "State", "Decisions", "Next steps", "Open questions"}
	finalHeadings = append(append([]string{}, clearHeadings...), "What would be lost")
)

// Headings returns the headings a narrative of kind k needs, in order;
// none for facts.
func Headings(k Kind) []string {
	switch k {
	case Clear:
		return append([]string{}, clearHeadings...)
	case Final:
		return append([]string{}, finalHeadings...)
	}
	return nil
}

// KeepFacts is the number of facts files julieta keeps in handoff/
// after a facts write, the newest ones (08 8.6, REQ-0649).
const KeepFacts = 50
