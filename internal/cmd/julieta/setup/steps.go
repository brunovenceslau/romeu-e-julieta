// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import "context"

// step is one step of julieta setup. It records what it did in the
// run's report: a warning, or a failure that leaves the sandbox
// unusable (run.unusable), which makes setup exit 1 after the other
// steps ran.
type step func(ctx context.Context, r *run)

// steps are the steps of julieta setup in the order 04 4.3 lists them,
// the seam the tasks of the other steps extend by one line each (plan
// rule 14): the hook dispatcher lands with T054, install with T053,
// memory check and the agent memory dir check with T035, and the
// required agent-profile paths and skills-stale with T079.
var steps = []step{
	linkBinary,
	syncRepos,
	installTools,
	checkMemory,
	recordSnapshotsDisabled,
	installDispatcher,
	checkAgentPaths,
	checkSkillsStale,
}
