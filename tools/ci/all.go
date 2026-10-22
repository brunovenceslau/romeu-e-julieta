// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// coverFile is the name of the cover profile the race step writes, in
// a temporary directory of its own, so that no run leaves a file in the
// working tree.
const coverFile = "cover.out"

// inJob returns whether a check runs in all as part of job, one of
// check.Jobs: the checks that the subcommand of that name runs.
func inJob(job string) func(check.Check) bool {
	return func(c check.Check) bool { return c.All && c.JobName() == job }
}

// allSteps returns the command steps of all (stepsOf): the steps of
// fast and the rows of 10 10.2 that have code to check today and are not
// in fast, vulnerabilities, license and the other steps, with the race
// run that writes the cover profile to profile. The coverage check reads
// that profile in this process, after them (runChecks).
//
// The order keeps every step that judges the tree before the first
// one that runs the change's tests, race: vulnerabilities and license
// read the tree and run none of its code, so they run after lint and
// before race, and a test of the change cannot rewrite a file they
// read (a test that prepends an SPDX header to a file without one
// passed license when it ran after the tests: measured at cab976e). That is
// all the order guarantees: it closes the file channel. A test runs in
// the same process tree and account as tools/ci, so it can reach the
// tools/ci process itself (ptrace, root on a hosted runner) and forge
// a verdict or the exit status; the order does not reach that, and the
// review of the diff covers it (the threat model of misefiles.go, Not
// covered). race and coverage come last, and they measure code of the
// change, which can game its own result: a test can write the cover
// profile itself, or exercise less than it claims. The review covers
// that too; runChecks fails a run whose steps left a file changed.
func allSteps(ctx context.Context, root string, tools check.Tools, profile string) []check.Step {
	return stepsOf(ctx, inAll, check.Plan{Root: root, Tools: tools, Profile: profile})
}

// runAll runs every check of 10 10.2 that has code to check today, the
// union of the three jobs of the hosted workflow in the order of
// today (runChecks). It is what a developer runs before a pull request;
// the hosted workflow runs one job per runner, the static, race and e2e
// subcommands (runJob). The pr step joins with tools/ci pr.
func runAll(ctx context.Context, e check.Env, args []string) (bool, error) {
	return runChecks(ctx, e, args, "all", inAll)
}

// runJob returns the subcommand that runs the checks of job alone
// (inJob), with the reports and the exit status of all.
func runJob(job string) func(ctx context.Context, e check.Env, args []string) (bool, error) {
	return func(ctx context.Context, e check.Env, args []string) (bool, error) {
		return runChecks(ctx, e, args, job, inJob(job))
	}
}

// runChecks runs the checks that in holds, for the command named name,
// all or one job: the command steps (stepsOf) on the working tree as
// it is, then the checks of the commit HEAD names, read and judged
// before any step starts (commitChecks, as in runFast), then the checks
// that judge what the steps left, as coverage reads the profile of the
// race step.
//
// It reads HEAD, the index and the working tree before mise and the
// steps run (treeState), and the "working tree" check fails when they
// differ after the steps (changedSince): a step that ran code of the
// change and rewrote a file, staged one or moved HEAD may have left a
// check that judged the tree a copy that is not the commit's.
//
// A run whose only failed checks are infrastructure errors (a
// precondition of the container e2e that could not be met, REQ-0798)
// returns an error that wraps check.ErrInfrastructure, so it exits
// check.ExitInfra; beside any other failed check it exits as a failure.
//
// It first prints the platform it runs on, as the go command and "uname
// -m" name it, so that a run's log shows which machine each runner
// label gave (10 10.2, Runners).
func runChecks(ctx context.Context, e check.Env, args []string, name string, in func(check.Check) bool) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("%s takes no argument\n%s", name, check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	now := e.Clock()
	repo := e.Repo()
	repo.Dir = root
	if err := nestedModules(ctx, repo); err != nil {
		return false, err
	}
	head, err := check.CommitOf(ctx, repo, "HEAD")
	if err != nil {
		return false, err
	}
	// The commit is read and judged before mise or any step runs code
	// of the change (commitChecks).
	verdicts, err := commitChecks(ctx, repo, head, nil, in, now)
	if err != nil {
		return false, err
	}
	machine := e.Machine
	if machine == nil {
		machine = unameMachine
	}
	// uname comes from the host machine, not from the repository.
	uname := machine(ctx)
	check.Say(e.Stdout, "platform  %s/%s, uname -m %s\n", runtime.GOOS, runtime.GOARCH, uname)

	// The steps must leave the tree as they found it (changedSince,
	// after them), mise included.
	before, err := readTreeState(ctx, repo)
	if err != nil {
		return false, err
	}
	profile := e.Profile
	steps := e.Steps
	if steps == nil {
		dir, err := os.MkdirTemp("", "ci-cover-")
		if err != nil {
			return false, err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		profile = filepath.Join(dir, coverFile)
		if err := mise.CheckVersion(ctx, root); err != nil {
			return false, err
		}
		tools, err := mise.ResolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = stepsOf(ctx, in, check.Plan{Root: root, Tools: tools, Profile: profile})
	}
	c := check.NewPrinterAt(e.Stdout, now)
	translated := e.Translated
	if translated == nil {
		translated = procTranslated
	}
	archStart := now()
	archDetail := ""
	// uname comes from the host machine, not from the repository.
	switch {
	case !archMatches(uname, runtime.GOARCH):
		archDetail = fmt.Sprintf("uname -m reports %s and this binary runs as %s: the tools of this run are not the ones of the machine\n", uname, runtime.GOARCH)
	case translated(ctx):
		archDetail = fmt.Sprintf("sysctl.proc_translated is 1: this %s binary runs under Rosetta, and the tools of this run are not the ones of the machine\n", runtime.GOARCH)
	}
	c.Report("architecture", archDetail == "", archDetail, now().Sub(archStart))
	c.Run(ctx, root, steps)
	treeStart := now()
	tree := ""
	if err := changedSince(ctx, repo, before); err != nil {
		tree = "the steps changed HEAD, the index or the working tree, so a check may not have judged the commit; what changed:\n" + git.SafeLines(err.Error()) + "\n"
	}
	c.Report("working tree", tree == "", tree, now().Sub(treeStart))

	// The checks that judge what the steps left, the cover profile of
	// the race step, come last (check.Check.After).
	for _, ch := range registered(in) {
		if ch.After != nil {
			start := now()
			v := ch.After(root, profile)
			verdicts = append(verdicts, timed{v, now().Sub(start)})
		}
	}
	printTimed(c, verdicts)
	c.Last(name)
	if c.InfraOnly() {
		return false, fmt.Errorf("%s: %s: %w", name, strings.Join(c.Infra, ", "), check.ErrInfrastructure)
	}
	return c.OK, nil
}

// unameMachine returns what "uname -m" prints, or why it printed
// nothing.
func unameMachine(ctx context.Context) string {
	out, err := probe(ctx, "uname", "-m")
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	return out
}

// probe runs a short probe of the machine in the environment of this
// process, under the class procgroup.ToolProbe, and returns its
// standard output without the surrounding space; what it writes to
// standard error is dropped.
func probe(ctx context.Context, argv ...string) (string, error) {
	var stdout strings.Builder
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolProbe,
		Argv:   argv,
		Env:    os.Environ(),
		Stdout: &stdout,
	})
	return strings.TrimSpace(stdout.String()), err
}

// procTranslated reports whether this process runs under Rosetta: on
// macOS "sysctl -n sysctl.proc_translated" prints 1 then, while uname
// -m prints the architecture the process emulates, so archMatches
// cannot see it. Elsewhere, and where the key does not exist (an Intel
// Mac), it is false.
func procTranslated(ctx context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	out, err := probe(ctx, "sysctl", "-n", "sysctl.proc_translated")
	return err == nil && out == "1"
}

// archMatches reports whether what "uname -m" prints (x86_64, aarch64 or
// arm64) is the architecture goarch names: a Go built for the other
// architecture is not a run on the machine the runner label names.
// Anything else, "unknown" included, does not match. A run under
// Rosetta passes this check, and procTranslated catches it.
func archMatches(uname, goarch string) bool {
	switch uname {
	case "x86_64":
		return goarch == "amd64"
	case "aarch64", "arm64":
		return goarch == "arm64"
	}
	return false
}
