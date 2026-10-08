// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// coverFile is the name of the cover profile the race step writes, in
// a temporary directory of its own, so that no run leaves a file in the
// working tree.
const coverFile = "cover.out"

// govulncheckLock and govulncheckDir name govulncheck in mise.lock and
// the directory below the mise installs directory where mise puts it:
// the name of its backend with ":", "/" and "." written as "-" (measured
// with mise 2026.10.3). It runs by the path "mise which" resolves, and
// not through "mise exec": with the tool missing, "mise exec" warned and
// ran a govulncheck of the search path, exit status 0 (measured with mise
// 2026.10.3), which is not the version mise.lock locks.
const (
	govulncheckLock = "go:golang.org/x/vuln/cmd/govulncheck"
	govulncheckDir  = "go-golang-org-x-vuln-cmd-govulncheck"
)

// raceTimeout bounds the race run, which builds every package with the
// race detector and runs every test: a budget of its own, and not the
// stepTimeout of a fast step. The job of ci.yml has 60 minutes.
const raceTimeout = 40 * time.Minute

// reuseImage is the container image of the license step: reuse 6.2.0,
// with its Python and every dependency, pinned by the digest of the
// image (12 12.1). mise cannot pin reuse for every platform: reuse
// publishes one wheel, for linux x86_64, and the mise pypi backend
// does not build the others from source.
const reuseImage = "fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e"

// allSteps returns the command steps of all: the steps of fast and the
// rows of 10 10.2 that have code to check today and are not in fast,
// vulnerabilities, license and the race run that writes the cover
// profile. The coverage step reads that profile in this process, after
// them (runAll).
//
// The order keeps every step that judges the tree before the first
// one that runs the change's tests, unit: vulnerabilities and license
// read the tree and run none of its code, so they run after lint and
// before unit, and a test of the change cannot rewrite a file they
// read (a test that prepends an SPDX header to a file without one
// passed license when it ran after unit: measured at cab976e). That is
// all the order guarantees: it closes the file channel. A test runs in
// the same process tree and account as tools/ci, so it can reach the
// tools/ci process itself (ptrace, root on a hosted runner) and forge
// a verdict or the exit status; the order does not reach that, and the
// review of the diff covers it (the threat model of misefiles.go, Not
// covered). race and coverage come last, and they measure code of the
// change, which can game its own result: a test can write the cover
// profile itself, or exercise less than it claims. The review covers
// that too; runAll fails a run whose steps left a file changed.
//
// The race run passes "-count=1" for the reason the unit step does:
// a test that scans the repository would otherwise be served from the
// test cache. govulncheck runs in stepEnv with networkPassThrough, since
// it reads the Go vulnerability database, from the path whichPinned
// finds below the directory where mise installs it and at the version
// mise.lock locks. A tool that is not found fails its step and no other.
func allSteps(ctx context.Context, root string, tools lintTools, profile string) []step {
	env := stepEnv(tools.goDir)
	gitCommon, gitErr := gitCommonDir(ctx, root)
	vuln := step{name: "vulnerabilities", environ: withProcessEnv(env, networkPassThrough)}
	if path, err := whichPinned(ctx, root, "govulncheck", govulncheckLock, govulncheckDir); err != nil {
		vuln.unavailable = err
	} else {
		vuln.argv = []string{path, "./..."}
	}
	goCmd := filepath.Join(tools.goDir, "go")
	license := licenseStep(root, gitCommon, runtime.GOOS, withProcessEnv(env, dockerPassThrough))
	if gitErr != nil && license.skip == "" {
		license.unavailable = gitErr
	}
	before, unit := fastPhases(root, tools)
	return slices.Concat(before, []step{vuln, license, unit,
		{name: "race", argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=" + profile, "./..."}, environ: env, timeout: raceTimeout}})
}

// gitCommonDir returns the absolute path of the git directory that
// holds the repository's objects and configuration: root/.git in a
// clone, and the main clone's .git in a linked worktree.
func gitCommonDir(ctx context.Context, root string) (string, error) {
	out, err := git.Repo{Dir: root}.Run(ctx, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// licenseStep runs "reuse lint" (REUSE 3.3, S11) from reuseImage on
// linux, the platform its image is built for, with the working tree
// and the git directory mounted read-only, each at its own path, and no
// network for the container. The container runs as the user nobody
// (65534), with no capability, no way to gain privileges, and a
// read-only root file system. Its git is told that both paths are safe:
// the tree belongs to another user, and without it git refuses the
// repository and reuse reads ignored files too (measured with reuse
// 6.2.0: an ignored file without a header then fails). The git
// directory is mounted because a linked worktree's ".git" file points
// to it, outside the tree: without it git fails the same way, and a
// worktree is where agents work (measured with reuse 6.2.0: an ignored
// file without a header failed there). In a clone it lies inside the
// tree, and the second mount repeats a part of the first. On another
// platform the step is not run, and all says so in its output with a
// "--" line instead of "ok": the check reads file content only, which
// is the same on every platform, and the Linux runners of the same
// commit run it.
//
// Threat model of the step. The image is named by its digest, so the
// code that runs is the code that was reviewed, whatever the tag points
// at later. The container reads the tree and the git directory through
// read-only mounts, so it cannot change what the other checks read, and
// it has no network, so it can neither fetch code nor send what it
// reads anywhere. It runs before unit and race (allSteps), so no test
// of the change can rewrite a file before reuse reads it, and runAll
// fails a run whose steps left the tree changed. The code inside the
// image is out of scope, as the code of a pinned action is: the review
// of the pin covers it.
func licenseStep(root, gitCommon, goos string, env []string) step {
	s := step{name: "license"}
	if goos != "linux" {
		s.skip = "not run on " + goos + ": reuse runs from a Linux container image; the hosted Linux runners run it"
		return s
	}
	s.argv = []string{"docker", "run", "--rm", "--network", "none",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534",
		"--env", "GIT_CONFIG_COUNT=2",
		"--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=" + root,
		"--env", "GIT_CONFIG_KEY_1=safe.directory", "--env", "GIT_CONFIG_VALUE_1=" + gitCommon,
		"--mount", "type=bind,source=" + root + ",target=" + root + ",readonly",
		"--mount", "type=bind,source=" + gitCommon + ",target=" + gitCommon + ",readonly",
		"--workdir", root, reuseImage, "lint"}
	s.environ = env
	for _, path := range []string{root, gitCommon} {
		if strings.ContainsAny(path, ",=") {
			s.unavailable = fmt.Errorf("a path holds \",\" or \"=\", which the bind mount of the license step cannot name: %s", path)
		}
	}
	return s
}

// runAll runs every check of 10 10.2 that has code to check today: the
// command steps (allSteps) on the working tree as it is, then hygiene
// and workflows on the commit HEAD names, read and judged before any
// step starts (commitChecks, as in runFast), then coverage on the
// profile of the race step. It is what the hosted workflow runs, and
// what a developer runs before a pull request. The pr step joins with
// tools/ci pr.
//
// It reads HEAD, the index and the working tree before mise and the
// steps run (treeState), and the "working tree" check fails when they
// differ after the steps (changedSince): a step that ran code of the
// change and rewrote a file, staged one or moved HEAD may have left a
// check that judged the tree a copy that is not the commit's.
//
// It first prints the platform it runs on, as the go command and "uname
// -m" name it, so that a run's log shows which machine each runner
// label gave (10 10.2, Runners).
func runAll(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("all takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	repo := e.repo()
	repo.Dir = root
	if err := nestedModules(ctx, repo); err != nil {
		return false, err
	}
	head, err := commitOf(ctx, repo, "HEAD")
	if err != nil {
		return false, err
	}
	// The commit is read and judged before mise or any step runs code
	// of the change (commitChecks).
	verdicts, err := commitChecks(ctx, repo, head)
	if err != nil {
		return false, err
	}
	machine := e.machine
	if machine == nil {
		machine = unameMachine
	}
	uname := machine(ctx)
	say(e.stdout, "platform  %s/%s, uname -m %s\n", runtime.GOOS, runtime.GOARCH, uname)

	// The steps must leave the tree as they found it (changedSince,
	// after them), mise included.
	before, err := readTreeState(ctx, repo)
	if err != nil {
		return false, err
	}
	profile := e.profile
	steps := e.steps
	if steps == nil {
		dir, err := os.MkdirTemp("", "ci-cover-")
		if err != nil {
			return false, err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		profile = filepath.Join(dir, coverFile)
		if err := checkMiseVersion(ctx, root); err != nil {
			return false, err
		}
		tools, err := resolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = allSteps(ctx, root, tools, profile)
	}
	c := newChecks(e.stdout)
	translated := e.translated
	if translated == nil {
		translated = procTranslated
	}
	archDetail := ""
	switch {
	case !archMatches(uname, runtime.GOARCH):
		archDetail = fmt.Sprintf("uname -m reports %s and this binary runs as %s: the tools of this run are not the ones of the machine\n", uname, runtime.GOARCH)
	case translated(ctx):
		archDetail = fmt.Sprintf("sysctl.proc_translated is 1: this %s binary runs under Rosetta, and the tools of this run are not the ones of the machine\n", runtime.GOARCH)
	}
	c.report("architecture", archDetail == "", archDetail)
	c.run(ctx, root, steps)
	tree := ""
	if err := changedSince(ctx, repo, before); err != nil {
		tree = "the steps changed HEAD, the index or the working tree, so a check may not have judged the commit; what changed:\n" + err.Error() + "\n"
	}
	c.report("working tree", tree == "", tree)

	findings, err := coverageAt(root, profile)
	for _, v := range append(verdicts, verdict{name: "coverage", findings: findings, err: err}) {
		c.verdict(v)
	}
	return c.ok, nil
}

// unameMachine returns what "uname -m" prints, or why it printed
// nothing.
func unameMachine(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "uname", "-m").Output()
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	return strings.TrimSpace(string(out))
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
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "sysctl.proc_translated").Output()
	return err == nil && strings.TrimSpace(string(out)) == "1"
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

// coverageAt applies coverage to the profile at path, for the module of
// the go.mod in root.
func coverageAt(root, path string) ([]finding, error) {
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	module, err := modulePath(goMod)
	if err != nil {
		return nil, err
	}
	profile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the cover profile of the race step: %w", err)
	}
	return coverage(profile, module)
}

// runCoverage applies the thresholds of S9 to a cover profile a
// developer wrote, as "go test -coverprofile=cover.out ./..." does.
func runCoverage(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 1 {
		return false, fmt.Errorf("coverage takes the path of a cover profile\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	path := args[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.dir, path)
	}
	findings, err := coverageAt(root, path)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("coverage", len(findings)))
	return len(findings) == 0, nil
}

// runWorkflows applies the grammar of 10 10.2 to the workflows of the
// tree at HEAD.
func runWorkflows(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("workflows takes no argument\n%s", usage)
	}
	findings, err := workflows(ctx, e.repo(), "HEAD")
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("workflows", len(findings)))
	return len(findings) == 0, nil
}

// runSetup prepares a machine to run the checks, in three steps: it
// trusts the repository's mise.toml, installs the tools mise.lock
// locks, and fills the module cache with the modules of go.sum, which
// the steps read with GOPROXY=off. The first two are what a new
// machine runs by hand otherwise ("mise trust", "mise install"). The
// hosted workflow runs it before all: there the mise action trusts the
// configuration only through MISE_TRUSTED_CONFIG_PATHS, which no step
// of tools/ci keeps (passThrough), and installs the tools already, so
// the second step finds nothing to do.
//
// Both mise steps run with miseEnv, at the top of the tree. In the
// hosted workflow, setup first checks that its own environment holds
// miseEnv, as the workflow's env table gives every step
// (hostedMiseEnv), and before any mise step it refuses a mise of
// another version than the one ci.yml pins (checkMiseVersion).
//
// The download runs the pinned go command in stepEnv with the proxy
// the go command uses by default, the one step of tools/ci that
// fetches modules; go.sum checks what it fetches. It and "mise
// install" keep networkPassThrough.
func runSetup(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("setup takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	if err := hostedMiseEnv(); err != nil {
		return false, err
	}
	if err := checkMiseVersion(ctx, root); err != nil {
		return false, err
	}
	c := newChecks(e.stdout)
	mise := []step{
		{name: "mise trust", argv: []string{"mise", "trust", filepath.Join(root, "mise.toml")}, environ: miseEnviron(passThroughEnv())},
		{name: "mise install", argv: []string{"mise", "install"}, environ: miseEnviron(withProcessEnv(passThroughEnv(), networkPassThrough))},
	}
	c.run(ctx, root, mise)
	if !c.ok {
		return false, nil
	}
	tools, err := resolveLintTools(ctx, root)
	if err != nil {
		return false, err
	}
	download := step{
		name:    "go mod download",
		argv:    []string{filepath.Join(tools.goDir, "go"), "mod", "download"},
		environ: withProcessEnv(slices.DeleteFunc(stepEnv(tools.goDir), func(kv string) bool { return kv == "GOPROXY=off" }), networkPassThrough),
	}
	c.run(ctx, root, []step{download})
	return c.ok, nil
}
