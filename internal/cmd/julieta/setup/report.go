// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"encoding/json/v2"
	"fmt"
)

// Schema is the value of the schema field of the --json document of
// julieta setup, the format julieta-setup.v1.
const Schema = "julieta-setup/v1"

// Report is the --json document of julieta setup, the format
// julieta-setup.v1: the PATH link, each repo of the manifest, the steps
// that left the sandbox unusable, and the warnings (04 4.3). The fields
// are in the order of their JSON names.
type Report struct {
	// Errors are the reads that failed and did not stop setup, such as
	// that of a julieta state file it cannot read, which this run's
	// state replaces.
	Errors []string `json:"errors"`
	// Link is the PATH link.
	Link Link `json:"link"`
	// Repos are the repos of the manifest, in its order.
	Repos []Repo `json:"repos"`
	// Schema is "julieta-setup/v1".
	Schema string `json:"schema"`
	// Unusable names each failure that left the sandbox unusable, which
	// makes setup exit 1; empty when it exits 0.
	Unusable []string `json:"unusable"`
	// Version is the version of the julieta that ran.
	Version string `json:"version"`
	// Warnings are the warnings of this run.
	Warnings []Warning `json:"warnings"`
}

// Link is $HOME/.local/bin/julieta.
type Link struct {
	// Changed: setup made or replaced the link in this run.
	Changed bool `json:"changed"`
	// Path is the link.
	Path string `json:"path"`
	// Target is the running binary it points at.
	Target string `json:"target"`
}

// The states of a default branch against origin's (Repo.BranchState).
const (
	ffBehind     = "behind"      // behind origin's tip, with nothing of its own
	ffCurrent    = "current"     // at origin's tip
	ffAhead      = "ahead"       // has commits origin's lacks, and lacks none of origin's
	ffDiverged   = "diverged"    // each has commits the other lacks
	ffNotFetched = "not-fetched" // the fetch failed
	ffNoDefault  = "no-default"  // origin/HEAD is missing or dangling
	ffNoBranch   = "no-branch"   // the repo has no local branch of that name
	ffFailed     = "failed"      // a read failed: Error says why
)

// Repo is what setup did in one repo.
type Repo struct {
	// Cloned: setup cloned the missing secondary repo in this run.
	Cloned bool `json:"cloned"`
	// DefaultBranch is origin's default branch, empty when unknown.
	DefaultBranch string `json:"defaultBranch"`
	// Dir is the repo's dir in the manifest.
	Dir string `json:"dir"`
	// Error is why a step failed in the repo, empty when none did.
	Error string `json:"error"`
	// BranchState is the state of the local default branch against
	// origin's, whatever the work tree holds or HEAD names: behind,
	// current, ahead, diverged, not-fetched, no-default, no-branch or
	// failed. setup moves no branch (REQ-0399). A document an older
	// julieta wrote has none, which a reader takes as unknown.
	BranchState string `json:"branchState,omitzero"`
	// FastForward was the outcome of the fast-forward of the default
	// branch, when setup made one.
	//
	// Deprecated: setup moves no branch since 2026-10-28 (REQ-0399), so
	// FastForward is always empty; BranchState carries the state of the
	// branch. The member stays in julieta-setup/v1 for its readers, and
	// its name is never used again (REQ-0163, REQ-1310).
	FastForward string `json:"fastForward"`
	// Fetched: the fetch of origin succeeded.
	Fetched bool `json:"fetched"`
	// From and To were the commits a move went from and to.
	//
	// Deprecated: setup moves no branch since 2026-10-28 (REQ-0399), so
	// both are always empty. They stay in julieta-setup/v1 for its
	// readers, and their names are never used again (REQ-0163,
	// REQ-1310).
	From string `json:"from"`
	To   string `json:"to"`
	// Path is the repo's path in the sandbox.
	Path string `json:"path"`
}

// Decode reads a --json document of julieta setup: a duplicate
// member and another schema are each an error, and a member this build
// does not know is skipped, as a newer julieta of the same schema may
// add one (REQ-0163).
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
