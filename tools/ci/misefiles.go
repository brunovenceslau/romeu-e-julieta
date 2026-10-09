// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// Threat model of the mise configuration channel. mise reads its
// configuration from files of the working tree, and a configuration
// can run a program: an [env] value, or a version in .tool-versions,
// is a template whose exec() runs a command when mise loads the file.
// "mise trust <top>/mise.toml" trusts the directory that holds it, the
// configuration root, and not that one file: it reports "mise trusted
// <top>" and records the directory (measured with mise 2026.10.3). So
// every configuration file mise reads from that root is trusted with
// it (measured with mise 2026.10.3: with mise.toml trusted, a committed
// mise.local.toml, .config/mise/conf.d/x.toml, .tool-versions, and a
// mise.ci.toml that a .miserc.toml selects, each ran its command on
// "mise install" and on "mise which"; so did a conf.d file reached
// through a committed .config symbolic link), while a mise.toml in a
// subdirectory, another root, fails closed: "mise env" started there
// exits 1 with "not trusted" and runs none of its commands (measured
// with mise 2026.10.3). A configuration can also
// change the version of a pinned tool, or add a tool. Every program
// tools/ci starts through mise, and mise-action and the mise shim of go
// in the hosted workflow, would read these files at the top of the
// working tree, before any check has judged them: such a command could
// stage a good copy of a generated file, so that "generated" passes a
// commit that holds another, and remove itself (measured on the
// default branch at e1aaa6a: fast exited 0 on a commit whose
// .github/CODEOWNERS was edited by hand). What is at stake is the
// pinned toolchain the checks run, the verdicts of the checks, and
// what the machine that runs them holds: in CI a read-only checkout
// without persisted credentials, on a developer's machine whatever the
// user can reach.
//
// What this code does:
//
//  1. One environment, miseEnv, for every mise run: tools/ci adds it to
//     each mise command it starts (miseEnviron), the workflow sets it
//     in its env table, which the workflows check holds to this list,
//     and the pre-push hook starts go, and so the mise shim, with it
//     (TestHook holds the hook's line to this list). With it, mise
//     2026.10.3 reads mise.toml as the one configuration file of the
//     tree, and mise.lock as its lock (TestMiseEnvStopsConfigs measures
//     both halves: without miseEnv each file above runs its command;
//     with it none does, and each variable left out alone lets one
//     through). In the hosted workflow, setup fails a run whose
//     environment does not hold miseEnv (hostedMiseEnv).
//  2. One list, miseFiles: mise.toml and mise.lock, the two mise files
//     the repository tracks, and every other path below the top of the
//     tree that mise 2026.10.3 reads as configuration when it runs
//     there without miseEnv (LOCAL_CONFIG_FILENAMES and
//     env_config_patterns in src/config/mod.rs, find_miserc_files in
//     src/config/miserc.rs). The dependencies surface of the ask-first
//     list holds exactly these globs besides go.mod, go.sum and the
//     workflows (TestMiseFilesAskFirst), so a change to any of them
//     asks first.
//  3. hygiene refuses a file of the list other than mise.toml and
//     mise.lock, by their exact names, and a pre-push run refuses an
//     untracked one at any depth (isGateInput), so a mise run without
//     miseEnv, as a developer's own shell starts, finds none on the
//     default branch. Three rules close the names that alias a path
//     of the list on another file system: both match the path in any
//     case (isMiseFile), since a case-insensitive file system gives
//     mise Mise.local.toml for mise.local.toml; hygiene refuses every
//     path with a byte outside printable ASCII, so no fold beyond
//     ASCII (U+017F, the long s, for the s of mise.local.toml) and no
//     composed or decomposed accent reaches mise; and hygiene refuses
//     every symbolic link and submodule, which could lead a path of
//     the list to a file under another name (a .config link to a
//     directory that holds mise/conf.d). The list is of the top of the
//     tree only, which is the root mise trusts; a file below it is in
//     another root, which mise refuses as not trusted (above).
//  4. tools/ci starts mise with the top of the tree as its working
//     directory: mise finds .miserc.toml from its working directory and
//     the configuration from its -C directory, so it reads the files of
//     this list, from the root it trusts, and none of a subdirectory.
//  5. fast, all and setup refuse a mise of another version than the
//     one ci.yml pins for mise-action (checkMiseVersion): the files
//     mise reads and the variables of miseEnv are measured with that
//     version only.
//  6. fast and all read the commit that hygiene, workflows and the
//     pushed range judge before they start mise or any step
//     (commitChecks); the generated step, and in all the steps that
//     judge the tree, run before any step that runs the change's tests
//     (TestGeneratedRunsBeforeTheTests, TestAllSteps); and all, like a
//     pre-push run, fails when the steps leave the tree changed
//     (changedSince). This order closes one channel, a test that
//     rewrites a file a later judging step reads, and no other.
//
// Not covered: mise still reads a .miserc.toml of the tree. Of its
// seven keys (the "rc" settings of settings.toml, mise 2026.10.3),
// miseEnv overrides the four that select configuration files (env,
// auto_env and the two override lists; src/env.rs), env_conf_d acts
// only on the files of an environment, of which none is selected, and
// ceiling_paths and ignored_config_paths only narrow what mise reads;
// its templates have no exec() (render_miserc_template in
// src/config/miserc.rs). A mise run that does not come from tools/ci,
// the workflow or the pre-push hook, such as a developer's shell with
// go on the mise shims, has no miseEnv, so a change that adds a file
// of the list runs its command there before hygiene fails it; hygiene
// and the code-owner review keep it off the default branch. That the
// hosted runner delivers the empty MISE_ENV of the env table as a set
// variable is read from the runner's source (actions/runner v2.338.0
// keeps an empty value from the job message to the process), but the
// service that writes the job message is not public, which is why
// setup checks it in every hosted run. Windows also takes a name with
// a trailing space or dot for the name without it (mise.local.toml.);
// no runner of the CI matrix is Windows and no rule refuses such a
// name, a decision to reopen if Windows joins the matrix
// (TestPrintableASCII shows the gap). A caller whose mise configuration
// below HOME sets idiomatic_version_file_enable_tools makes mise read
// .go-version and go.mod as well (measured with mise 2026.10.3 and
// miseEnv: "mise config ls" lists both); mise.toml still decides the
// version (the same "mise ls --current go"), neither file holds a
// template, and whichPinned holds the install to mise.lock.
//
// Not covered either: an attack of the change's tests on the tools/ci
// process itself. A test runs in the same process tree and account as
// tools/ci (a test of unit attached to the "ci all" process with
// PtraceAttach and got no error: observed by the ship gate), and a
// hosted runner gives that account sudo, so a test could forge a
// verdict or the exit status that tools/ci computes. The order of the
// steps does not reach that; the review of the diff covers it, and
// running the tests in a job of their own is a deferred decision
// (docs/spec.md, Deferred decisions). Refusing ptrace to the process
// (PR_SET_DUMPABLE, PT_DENY_ATTACH) is not taken: it does not stop
// root, which a hosted runner has. And the gate holds against a
// hostile change only where the change's tests do not run on the same
// machine and account as a later run of the gate: a test run on a
// developer's machine can plant an ignored Go file in tools/ci, hidden
// by .git/info/exclude, or poison GOCACHE, and so change later
// pre-push runs there.
//
// Trusted, as the toolchain is (ADR 0007): mise itself, its
// configuration below HOME, the system configuration of mise
// (/etc/mise, or the file MISE_SYSTEM_CONFIG_FILE names, which mise
// reads under miseEnv too, and whose exec() templates run: observed by
// the ship gate), and the files of the directories above the tree. No
// mise run of tools/ci comes after a step that runs the change's
// tests, so no test can plant a system configuration that a later
// mise run of the same gate reads; a decision to reopen if such a run
// is ever added.

// miseEnv is the environment of every mise run, as "KEY=value": the
// four mise variables below, then goPinEnv. The configuration file
// mise reads is mise.toml alone, it reads no
// .tool-versions ("none" is the empty list, src/env.rs of mise
// 2026.10.3), and it selects no environment's files, neither one named
// by MISE_ENV or a .miserc.toml (an empty MISE_ENV is set, so mise
// does not fall back to the .miserc.toml) nor one of the platform
// (auto_env). The workflow sets the same table (the workflows check),
// and the pre-push hook the same variables (TestHook).
//
// goPinEnv ends the list: "mise install" builds govulncheck with the go
// command ("go install", the go: backend), so that build reads no "go
// env -w" file and starts no other go than the pinned one, as every go
// step of tools/ci does (stepEnv).
var miseEnv = slices.Concat([]string{
	"MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml",
	"MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none",
	"MISE_ENV=",
	"MISE_AUTO_ENV=false",
}, goPinEnv)

// miseEnviron returns env with miseEnv after it, for a mise command.
// No list of variables a step keeps from this process holds a key of
// miseEnv (TestMiseEnvList), so env has none, and a value of the
// caller's never reaches mise.
func miseEnviron(env []string) []string {
	return append(slices.Clone(env), miseEnv...)
}

// hostedMiseEnv fails a run in the hosted workflow (GITHUB_ACTIONS is
// "true", as the runner sets it for every step) whose environment does
// not hold each variable of miseEnv with its value: the env table of
// the workflow sets them for mise-action and for the mise shim that
// starts go, and an empty MISE_ENV must arrive set, not dropped. Off
// the hosted workflow it checks nothing: there tools/ci sets miseEnv
// for each mise run it starts, and the shell that starts tools/ci is
// the developer's own.
func hostedMiseEnv() error {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return nil
	}
	for _, kv := range miseEnv {
		key, want, _ := strings.Cut(kv, "=")
		got, set := os.LookupEnv(key)
		switch {
		case !set:
			return fmt.Errorf("setup: the workflow sets %s for every mise run, and this run has %s unset", kv, key)
		case got != want:
			return fmt.Errorf("setup: the workflow sets %s for every mise run, and this run has %s=%s", kv, key, got)
		}
	}
	return nil
}

// pinnedMiseFiles are the mise files the repository tracks.
var pinnedMiseFiles = []string{"mise.toml", "mise.lock"}

// miseFiles are the paths that mise 2026.10.3, started at the top of
// the tree, reads as its configuration or as the lock of the pinned
// tools, as globs of the ask-first grammar (12 12.4): pinnedMiseFiles,
// then every other one.
var miseFiles = append(slices.Clone(pinnedMiseFiles),
	".mise.toml", "mise.*.toml", ".mise.*.toml",
	"mise/**", ".mise/**",
	".config/mise.toml", ".config/mise.*.toml", ".config/mise/**",
	".tool-versions",
	".miserc.toml", ".miserc.local.toml", ".config/miserc.toml",
)

// isMiseFile reports whether mise, started at the top of the tree,
// reads the file at path, relative to that top with forward slashes,
// as one of miseFiles. The path is matched in lower case, as every
// glob of miseFiles is written (TestMiseFilesLowercase): on a
// case-insensitive file system, the default of macOS, mise finds
// .TOOL-VERSIONS when it looks for .tool-versions. The lowering folds
// ASCII letters only, and relies on hygiene's path rule
// (printableASCII) for every other byte.
func isMiseFile(path string) bool {
	lower := strings.ToLower(path)
	return slices.ContainsFunc(miseFiles, func(g string) bool { return globMatch(g, lower) })
}

// isMiseFileAtAnyDepth reports whether mise reads the file at path as
// one of miseFiles when it starts in the directory that holds it or in
// one above it: isMiseFile of the path, or of what follows any "/" in
// it. A test, or a developer's shell, can start mise below the top,
// where such a file is in a root mise does not trust and fails closed
// (the threat model above); a pre-push run refuses it all the same,
// since a developer may have trusted that root.
func isMiseFileAtAnyDepth(path string) bool {
	for {
		if isMiseFile(path) {
			return true
		}
		_, rest, ok := strings.Cut(path, "/")
		if !ok {
			return false
		}
		path = rest
	}
}

// miseAction is the action that installs mise in the hosted workflow,
// and pinnedMiseWorkflow the workflow whose "version" input for it
// pins the version of mise itself: the one version under which miseEnv
// is measured (TestMiseEnvStopsConfigs).
const (
	miseAction         = "jdx/mise-action"
	pinnedMiseWorkflow = workflowsDir + "ci" + workflowExt
)

// pinnedMiseVersion reads the version of mise that pinnedMiseWorkflow
// pins, in the tree at root: the "version" input of each step that uses
// miseAction (the action matched without case, as the workflows check
// matches it), which must be one and the same.
func pinnedMiseVersion(root string) (string, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pinnedMiseWorkflow)))
	if err != nil {
		return "", fmt.Errorf("the version of mise is pinned in %s: %w", pinnedMiseWorkflow, err)
	}
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string         `yaml:"uses"`
				With map[string]any `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(src, &wf); err != nil {
		return "", fmt.Errorf("%s: %w", pinnedMiseWorkflow, err)
	}
	var versions []string
	for _, job := range wf.Jobs {
		for _, s := range job.Steps {
			action, _, _ := strings.Cut(s.Uses, "@")
			if parts := strings.SplitN(strings.ToLower(action), "/", 3); len(parts) < 2 || parts[0]+"/"+parts[1] != miseAction {
				continue
			}
			version, _ := s.With["version"].(string)
			if !slices.Contains(versions, version) {
				versions = append(versions, version)
			}
		}
	}
	if len(versions) != 1 || versions[0] == "" {
		return "", fmt.Errorf("%s pins the version of mise with one \"version\" input of %s, and it holds %q", pinnedMiseWorkflow, miseAction, versions)
	}
	return versions[0], nil
}

// checkMiseVersion fails unless the mise on the search path is the
// version pinnedMiseVersion reads: miseEnv is measured under that
// version, and the variables it sets, or the files mise reads without
// them, may differ in another (the threat model above). "mise
// --version" prints the version as its first word (measured with mise
// 2026.10.3: "2026.10.3 linux-arm64 (2026-10-05)"). It runs as every
// mise run of tools/ci does: at the top of the tree, with miseEnv.
func checkMiseVersion(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, "mise", "--version")
	cmd.Dir = root
	cmd.Env = miseEnviron(passThroughEnv())
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("mise is not on the search path: install the mise that %s pins, then run \"go run ./tools/ci setup\" in %s", pinnedMiseWorkflow, root)
	}
	if err != nil {
		return fmt.Errorf("mise --version: %w", err)
	}
	pinned, err := pinnedMiseVersion(root)
	if err != nil {
		return err
	}
	if got, _, _ := strings.Cut(strings.TrimSpace(string(out)), " "); got != pinned {
		return fmt.Errorf("mise is at version %s, and %s pins %s, the version miseEnv is measured with (tools/ci/misefiles.go): install mise %s", git.Printable(got), pinnedMiseWorkflow, pinned, pinned)
	}
	return nil
}
