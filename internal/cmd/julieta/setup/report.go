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

// The outcomes of the fast-forward of a default branch (Repo.FastForward).
const (
	ffMoved         = "moved"           // fast-forwarded from From to To
	ffCurrent       = "current"         // already at origin's tip
	ffNotFetched    = "not-fetched"     // the fetch failed
	ffNoDefault     = "no-default"      // origin/HEAD is missing or dangling
	ffNotCheckedOut = "not-checked-out" // HEAD is another branch, detached or unborn
	ffNotClean      = "not-clean"       // the working tree holds a change
	ffAhead         = "ahead"           // the branch has commits origin's lacks, and lacks none of origin's
	ffDiverged      = "diverged"        // each has commits the other lacks
	ffFailed        = "failed"          // a read or the fast-forward failed: Error says why
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
	// FastForward is the outcome of the fast-forward of the default
	// branch: moved, current, not-fetched, no-default,
	// not-checked-out, not-clean, ahead, diverged or failed.
	FastForward string `json:"fastForward"`
	// Fetched: the fetch of origin succeeded.
	Fetched bool `json:"fetched"`
	// From and To are the commits a move went from and to; empty unless
	// FastForward is moved.
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
