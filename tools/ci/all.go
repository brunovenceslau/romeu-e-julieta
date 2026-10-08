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
)

// coverFile is the name of the cover profile the race step writes, in
// a temporary directory of its own, so that no run leaves a file in the
// working tree.
const coverFile = "cover.out"

// execTools are the tools of all that tools/ci starts through "mise
// exec" (10 10.2), each with the directory below the mise installs
// directory where mise puts it: the name of its backend with ":", "/"
// and "." written as "-" (measured with mise 2026.10.3).
var execTools = map[string]string{
	"govulncheck": "go-golang-org-x-vuln-cmd-govulncheck",
}

// reuseImage is the container image of the license step: reuse 6.2.0,
// with its Python and every dependency, pinned by the digest of the
// image (12 12.1). mise cannot pin reuse for every platform: reuse
// publishes one wheel, for linux x86_64, and the mise pypi backend
// does not build the others from source.
const reuseImage = "fsfe/reuse:6.2.0@sha256:85462a75c0f8efda09ddd190b92816b70e7662577c8427429e11e1b9f25a992e"

// allSteps returns the command steps of all: the steps of fast, then
// the rows of 10 10.2 that have code to check today and are not in
// fast: vulnerabilities, the race run that writes the cover profile,
// and license. The coverage step reads that profile in this process,
// after them (runAll).
//
// The race run passes "-count=1" for the reason the unit step does:
// a test that scans the repository would otherwise be served from the
// test cache. govulncheck is started through "mise exec", in stepEnv
// with networkPassThrough, since it reads the Go vulnerability
// database,
// once whichPinned has found it below the directory where mise
// installs it: "mise exec" runs a program of the search path when the
// configuration does not pin it, and that would be a tool no lock
// holds. A tool that is not found fails its step and no other.
func allSteps(ctx context.Context, root string, tools lintTools, profile string) []step {
	env := stepEnv(tools.goDir)
	viaMise := func(name, tool string, args ...string) step {
		s := step{name: name, argv: append([]string{"mise", "exec", "--", tool}, args...), environ: withProcessEnv(env, networkPassThrough)}
		if _, err := whichPinned(ctx, root, tool, execTools[tool]); err != nil {
			s.unavailable = err
		}
		return s
	}
	goCmd := filepath.Join(tools.goDir, "go")
	return append(fastSteps(root, tools),
		viaMise("vulnerabilities", "govulncheck", "./..."),
		step{name: "race", argv: []string{goCmd, "test", "-race", "-count=1", "-coverprofile=" + profile, "./..."}, environ: env},
		licenseStep(root, runtime.GOOS, withProcessEnv(env, dockerPassThrough)),
	)
}

// licenseStep runs "reuse lint" (REUSE 3.3, S11) from reuseImage on
// linux, the platform its image is built for, with the working tree
// mounted read-only and no network for the container. The container
// runs as the user nobody (65534), with no capability, no way to gain
// privileges, and a read-only root file system. Its git is told that
// /data is safe: the tree belongs to another user, and without it git
// refuses the repository and reuse reads ignored files too (measured
// with reuse 6.2.0: an ignored file without a header then fails). On another
// platform it is not run, and all says so in its output with a "--"
// line instead of "ok": the check reads file content only, which is
// the same on every platform, and the Linux runners of the same commit
// run it.
//
// Threat model of the step. The image is named by its digest, so the
// code that runs is the code that was reviewed, whatever the tag points
// at later. The container reads the tree through a read-only mount, so
// it cannot change what the other checks read, and it has no network,
// so it can neither fetch code nor send the tree anywhere. The code
// inside the image is out of scope, as the code of a pinned action is:
// the review of the pin covers it. In a linked worktree, whose git
// directory is outside the mount, reuse reads ignored files as well.
func licenseStep(root, goos string, env []string) step {
	s := step{name: "license"}
	if goos != "linux" {
		s.skip = "not run on " + goos + ": reuse runs from a Linux container image; the hosted Linux runners run it"
		return s
	}
	s.argv = []string{"docker", "run", "--rm", "--network", "none",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534",
		"--env", "GIT_CONFIG_COUNT=1", "--env", "GIT_CONFIG_KEY_0=safe.directory", "--env", "GIT_CONFIG_VALUE_0=/data",
		"--mount", "type=bind,source=" + root + ",target=/data,readonly", reuseImage, "lint"}
	s.environ = env
	if strings.ContainsAny(root, ",=") {
		s.unavailable = fmt.Errorf("the working tree's path holds \",\" or \"=\", which the bind mount of the license step cannot name: %s", root)
	}
	return s
}

// runAll runs every check of 10 10.2 that has code to check today, on
// the working tree as it is: the command steps (allSteps), then
// hygiene and workflows on HEAD, then coverage on the profile of the
// race step. It is what the hosted workflow runs, and what a developer
// runs before a pull request. The pr step joins with tools/ci pr.
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
	say(e.stdout, "platform  %s/%s, uname -m %s\n", runtime.GOOS, runtime.GOARCH, unameMachine(ctx))

	profile := e.profile
	steps := e.steps
	if steps == nil {
		dir, err := os.MkdirTemp("", "ci-cover-")
		if err != nil {
			return false, err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		profile = filepath.Join(dir, coverFile)
		tools, err := resolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = allSteps(ctx, root, tools, profile)
	}
	c := newChecks(e.stdout)
	c.run(ctx, root, steps)

	for _, check := range []struct {
		name string
		find func() ([]finding, error)
	}{
		{"hygiene", func() ([]finding, error) { return hygiene(ctx, repo, "HEAD") }},
		{"workflows", func() ([]finding, error) { return workflows(ctx, repo, "HEAD") }},
		{"coverage", func() ([]finding, error) { return coverageAt(root, profile) }},
	} {
		findings, err := check.find()
		if err != nil {
			c.report(check.name, false, err.Error()+"\n")
			continue
		}
		c.report(check.name, len(findings) == 0, lines(findings))
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
	c := newChecks(e.stdout)
	mise := []step{
		{name: "mise trust", argv: []string{"mise", "trust", filepath.Join(root, "mise.toml")}, environ: passThroughEnv()},
		{name: "mise install", argv: []string{"mise", "install"}, environ: withProcessEnv(passThroughEnv(), networkPassThrough)},
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
