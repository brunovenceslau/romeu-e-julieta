// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// Schema is the value of the schema field of the --json document of
// julieta status, the format julieta-status.v1.
const Schema = "julieta-status/v1"

// Report is the --json document of julieta status, the format
// julieta-status.v1 (04 4.3, REQ-0424). Each section of sections.go
// fills its own fields. The fields are in the order of their JSON
// names.
type Report struct {
	// Egress is the egress set romeu applied, from the manifest (07
	// 7.5, REQ-0224).
	Egress []string `json:"egress"`
	// Errors are the reads outside a repo that failed, such as that of
	// julieta state.
	Errors []string `json:"errors"`
	// HookFailures are the hook failures recorded in julieta state, the
	// oldest first, each until a later run of the same hook in the same
	// repo succeeds (08 8.2, REQ-0675).
	HookFailures []HookFailure `json:"hookFailures"`
	// Project is the project of the manifest.
	Project string `json:"project"`
	// Repos are the repos of the manifest, in its order.
	Repos []Repo `json:"repos"`
	// Schema is "julieta-status/v1".
	Schema string `json:"schema"`
	// SpecCommit is the live spec commit (03 3.6, REQ-0220).
	SpecCommit string `json:"specCommit"`
	// Warnings are julieta's warnings, those setup recorded among them.
	Warnings []setup.Warning `json:"warnings"`
}

// HookFailure is one recorded failing run of a hook: a git hook's, of
// the stage system, julieta, tracked or local, or a session hook's,
// SessionStart or SessionEnd (08 8.2).
type HookFailure struct {
	// At is the time of the run.
	At time.Time `json:"at"`
	// ExitCode is the exit status of the stage that failed.
	ExitCode int `json:"exitCode"`
	// Hook is the git hook's name, or SessionStart or SessionEnd.
	Hook string `json:"hook"`
	// Message is what a session hook's failure said; empty for a git
	// hook.
	Message string `json:"message"`
	// Path is the script of the stage that failed; empty for a session
	// hook.
	Path string `json:"path"`
	// Repo is the path of the repo a git hook ran in, or the dir of the
	// repo a session hook ran for; empty when a session hook failed
	// before it knew one.
	Repo string `json:"repo"`
	// Stage is the stage of a git hook that failed, or the command of a
	// session hook.
	Stage string `json:"stage"`
}

// Repo is the state of one repo (04 4.3, REQ-0424).
type Repo struct {
	// Branch is the branch checked out, without refs/heads/; empty when
	// HEAD is detached.
	Branch string `json:"branch"`
	// Branches are the local branches, sorted by name.
	Branches []Branch `json:"branches"`
	// Dir is the repo's dir in the manifest.
	Dir string `json:"dir"`
	// Dirty names what the working tree holds, of staged, unstaged,
	// untracked, unmerged and hidden (an entry git status does not
	// compare, as a sparse checkout makes); empty when it is clean.
	Dirty []string `json:"dirty"`
	// Errors are the reads of the repo that failed, each with git's
	// error; the fields of a read that failed are empty.
	Errors []string `json:"errors"`
	// Head is the commit HEAD names; empty on an unborn branch.
	Head string `json:"head"`
	// HooksPath is the core.hooksPath the repo's own config sets.
	HooksPath string `json:"hooksPath"`
	// Path is the repo's path in the sandbox.
	Path string `json:"path"`
	// SnapshotsDisabled: the repo's own config sets core.hooksPath, so
	// the dispatcher, and the snapshot after each commit, do not run
	// in it (08 8.4).
	SnapshotsDisabled bool `json:"snapshotsDisabled"`
	// Stashes are the stashes, newest first.
	Stashes []Stash `json:"stashes"`
	// Worktrees are the worktrees, the main one first. A path is the
	// repo's to choose: status prints it and acts on none.
	Worktrees []Worktree `json:"worktrees"`
}

// Branch is one local branch.
type Branch struct {
	// Ahead and Behind count the commits the branch has that its
	// upstream lacks, and the reverse; zero when it tracks none.
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
	// Current: HEAD points at it.
	Current bool `json:"current"`
	// Head is the commit it points at.
	Head string `json:"head"`
	// Name is its name, without refs/heads/.
	Name string `json:"name"`
	// Upstream is the full ref name of its upstream, as the config sets
	// it; empty when it has none.
	Upstream string `json:"upstream"`
	// UpstreamGone: the upstream ref does not exist.
	UpstreamGone bool `json:"upstreamGone"`
}

// Stash is one stash.
type Stash struct {
	// Head is the stash's commit.
	Head string `json:"head"`
	// Subject is its message, escaped.
	Subject string `json:"subject"`
}

// Worktree is one worktree.
type Worktree struct {
	// Bare, Detached, Locked and Prunable are git's flags.
	Bare bool `json:"bare"`
	// Branch is the branch its HEAD points at, full; empty when
	// detached or bare.
	Branch   string `json:"branch"`
	Detached bool   `json:"detached"`
	// Head is the commit its HEAD names.
	Head   string `json:"head"`
	Locked bool   `json:"locked"`
	// Path is its root, as git records it.
	Path     string `json:"path"`
	Prunable bool   `json:"prunable"`
}

// Decode reads a --json document of julieta status: a duplicate member and
// another schema are each an error, and a member this build does not
// know is skipped, as a newer julieta of the same schema may add one
// (REQ-0163).
func Decode(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, err
	}
	if r.Schema != Schema {
		return Report{}, fmt.Errorf("the schema %q is not %s", r.Schema, Schema)
	}
	return r, nil
}
