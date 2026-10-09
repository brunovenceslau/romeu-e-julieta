// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// stepTimeout bounds one command step. The unit tests are the longest,
// and a cold build cache is the slow case this leaves room for.
const stepTimeout = 15 * time.Minute

// step is one check of fast that is a command.
type step struct {
	name string
	argv []string
	// quiet marks a command whose output is its finding, and it means
	// two things: the step passes when it prints nothing, as "gofmt -l"
	// does, and what it prints goes through git.SafeLines, because that
	// output is paths of the repository (checks.run).
	quiet bool
	// environ is the whole environment of the command (stepEnv). The
	// environment of this process never reaches it, so an empty one is
	// an empty environment.
	environ []string
	// unavailable, when set, says why the step's tool could not be
	// resolved: the step fails with it and starts nothing.
	unavailable error
	// timeout, when set, replaces stepTimeout for this step.
	timeout time.Duration
	// skip, when set, says why the step does not run on this platform:
	// it is reported with "--", neither passed nor failed.
	skip string
}

// fastSteps returns the command steps of fast, from the In fast column
// of 10 10.2: format, vet, generated, lint (once per lintTargets) and
// unit. The other rows of that column join with the code they check.
//
// Every step runs a pinned tool by its path: gofmt and go from the
// directory of the go command that mise resolves, and the golangci-lint
// it resolves, directly and not through "mise exec", which would add
// the [env] table of a mise configuration to the environment. Each
// one runs in stepEnv, so the go command that vets and tests is the one
// the linter runs, whatever go is first on the search path. "--config"
// names .golangci.yml so that no other .golangci.* file takes its place.
//
// Lint runs once for each of lintTargets, because the linter sees only
// the files that build for the GOOS and GOARCH it runs under: a file for
// another one (as pty_darwin_test.go, or a file named for amd64 on an
// arm64 machine) would never be linted otherwise.
//
// The unit step names the two directories the column names, and leaves
// out one that does not exist yet, which "go test" would take as an
// error. It passes "-count=1": tests that scan the repository would
// otherwise be served from the test cache, which cannot see a file
// added since the cached run.
func fastSteps(root string, tools lintTools) []step {
	before, unit := fastPhases(root, tools)
	return append(before, unit)
}

// fastPhases returns the steps of fastSteps in two parts: the ones
// that run none of the change's tests (generated runs the tools/ci of
// the change, which the ask-first review of tools/ci covers), and
// unit, the first that runs them. all puts its own steps that judge
// the tree between the two (allSteps).
func fastPhases(root string, tools lintTools) ([]step, step) {
	goCmd := filepath.Join(tools.goDir, "go")
	unit := []string{goCmd, "test", "-count=1"}
	for _, dir := range []string{"internal", "tools"} {
		if info, err := os.Stat(filepath.Join(root, dir)); err == nil && info.IsDir() {
			unit = append(unit, "./"+dir+"/...")
		}
	}
	steps := []step{
		{name: "format", argv: []string{filepath.Join(tools.goDir, "gofmt"), "-l", "."}, quiet: true, environ: stepEnv(tools.goDir)},
		{name: "vet", argv: []string{goCmd, "vet", "./..."}, environ: stepEnv(tools.goDir)},
		{name: "generated", argv: []string{goCmd, "run", "./tools/ci", "generated"}, environ: stepEnv(tools.goDir)},
	}
	for _, target := range lintTargets {
		steps = append(steps, step{
			name:    "lint " + target.String(),
			argv:    []string{tools.linter, "run", "--config", ".golangci.yml"},
			environ: lintEnv(target, tools.goDir),
		})
	}
	return steps, step{name: "unit", argv: unit, environ: stepEnv(tools.goDir)}
}

// lintTools are the pinned tools of fast, as mise resolves them: the
// path of golangci-lint, and the directory of the go command.
type lintTools struct {
	linter, goDir string
}

// resolveLintTools asks "mise which" for the paths of the versions that
// mise.toml pins and mise.lock locks, in the environment of
// passThroughEnv with miseEnv. "mise which" runs no tool and installs
// none: a tool that is not installed is an error that names the two
// commands a new machine runs first, so fast fails closed and never
// falls back to another golangci-lint or go on the search path.
//
// Each path must lie, once its symbolic links are resolved, in the
// directory where mise installs that tool (miseInstalls), and the error
// names that directory. A mise configuration may name a tool by a path
// ("path:" in [tools], in the repository or in the global
// configuration); such a tool is no pinned one, and it is an error here
// too.
func resolveLintTools(ctx context.Context, root string) (lintTools, error) {
	linter, err := whichPinned(ctx, root, "golangci-lint", "golangci-lint", "golangci-lint")
	if err != nil {
		return lintTools{}, err
	}
	goBin, err := whichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return lintTools{}, err
	}
	return lintTools{linter: linter, goDir: filepath.Dir(goBin)}, nil
}

// whichPinned returns the path "mise which" resolves for the program
// name, once its symbolic links are resolved, and fails unless it lies
// in dir, the directory below the installs directory (miseInstalls)
// where mise installs that tool. The next path segment, the version
// directory, must equal the version mise.lock locks under key (the
// tool's name in the lock), so that a stale install left beside the
// locked one is refused; a tool locked at two different versions is an
// error, as the lock then holds no one version to hold an install to.
// It runs in passThroughEnv with miseEnv, at the top of the tree.
func whichPinned(ctx context.Context, root, name, key, dir string) (string, error) {
	installsDir, err := miseInstalls()
	if err != nil {
		return "", err
	}
	installs, err := filepath.EvalSymlinks(installsDir)
	if err != nil {
		return "", fmt.Errorf("the mise installs directory %s: %w; run \"mise trust\" and \"mise install\" in %s", installsDir, err, root)
	}
	cmd := exec.CommandContext(ctx, "mise", "-C", root, "which", name)
	// mise finds .miserc.toml from its working directory (misefiles.go).
	cmd.Dir = root
	cmd.Env = miseEnviron(passThroughEnv())
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("mise is not on the search path: install mise, then run \"mise trust\" and \"mise install\" in %s", root)
	}
	if err != nil {
		detail := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			detail = "\n" + string(bytes.TrimSpace(exit.Stderr))
		}
		return "", fmt.Errorf("mise which %s: %w; the tool is not installed or the configuration is not trusted: run \"mise trust\" and \"mise install\" in %s%s", name, err, root, detail)
	}
	path, err := filepath.EvalSymlinks(string(bytes.TrimSpace(out)))
	if err != nil {
		return "", fmt.Errorf("mise which %s: %w", name, err)
	}
	want := filepath.Join(installs, dir)
	if rel, err := filepath.Rel(want, path); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("mise which %s: %s is outside %s, where mise installs it: the mise configuration names a tool mise did not install", name, path, want)
	} else if err := lockedInstall(root, name, key, filepath.ToSlash(rel)); err != nil {
		return "", fmt.Errorf("%w: %s", err, path)
	}
	return path, nil
}

// lockedInstall fails unless the first segment of rel, the path of a
// tool below its install directory, is the version mise.lock locks for
// key.
func lockedInstall(root, name, key, rel string) error {
	lock, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	if err != nil {
		return fmt.Errorf("mise which %s: mise.lock: %w; the version a tool runs at is the one it locks", name, err)
	}
	versions, err := lockedVersions(lock)
	if err != nil {
		return fmt.Errorf("mise which %s: %w", name, err)
	}
	version, ok := versions[key]
	if !ok {
		return fmt.Errorf("mise which %s: mise.lock locks no version of %s", name, key)
	}
	if first, _, _ := strings.Cut(rel, "/"); first != version {
		return fmt.Errorf("mise which %s: the install is not the version %s that mise.lock locks: run \"mise install\" in %s", name, version, root)
	}
	return nil
}

// lockedVersions returns the version mise.lock locks for each tool: the
// "version" key that follows each "[[tools.<name>]]" header, until any
// other table header. A tool locked at two different versions is an
// error; the same version written twice is not.
func lockedVersions(data []byte) (map[string]string, error) {
	locked := map[string]string{}
	tool := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(line, "[[tools."); ok {
			tool = strings.Trim(strings.TrimSuffix(name, "]]"), `"`)
			continue
		}
		if strings.HasPrefix(line, "[") {
			tool = "" // a version below another table is no tool's
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && tool != "" && strings.TrimSpace(key) == "version" {
			version := strings.Trim(strings.TrimSpace(value), `"`)
			if prev, dup := locked[tool]; dup && prev != version {
				return nil, fmt.Errorf("mise.lock locks %s at two different versions", tool)
			}
			locked[tool] = version
			tool = ""
		}
	}
	return locked, nil
}

// miseInstalls returns the directory where mise installs tools:
// .local/share/mise/installs below HOME, the default of the mise data
// directory when neither MISE_DATA_DIR nor XDG_DATA_HOME is set
// (measured on linux with mise 2026.10.3, whose "mise doctor" reports
// it; TestMiseInstalls repeats that measurement on every machine that
// runs the tests).
// Either variable would let the caller point mise at an installs
// directory of their own, so a run with one of them set fails closed,
// and passThroughEnv, the environment of "mise which", has neither.
func miseInstalls() (string, error) {
	for _, key := range []string{"MISE_DATA_DIR", "XDG_DATA_HOME"} {
		if os.Getenv(key) != "" {
			return "", fmt.Errorf("%s is set: fast runs only the tools mise installs below HOME; unset it", key)
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME is not set: fast finds the tools mise installs below it")
	}
	return filepath.Join(home, ".local", "share", "mise", "installs"), nil
}

// lintTarget is one platform the lint step lints for, named as the go
// command names it.
type lintTarget struct {
	goos, goarch string
}

func (t lintTarget) String() string {
	return t.goos + "/" + t.goarch
}

// lintTargets are the supported targets, linux and darwin each on amd64
// and arm64, and so the platforms the lint step runs for. Every one runs
// with cgo off (lintEnv), so a file that builds only with cgo is a file
// no target builds. The tests that check that the linter reaches each
// Go file read this same list.
var lintTargets = []lintTarget{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// passThrough are the variables of this process that the steps of fast
// and "mise which" keep. Each one says where a program or a file is
// (the search path, the home and temporary directories, the
// directories of mise and the caches of Go), never what to check or
// how. The data directory of mise is not among them: miseInstalls
// derives it from HOME.
var passThrough = []string{
	"PATH", "HOME", "TMPDIR",
	"XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
	"MISE_CACHE_DIR", "MISE_CONFIG_DIR", "MISE_STATE_DIR",
	"GOPATH", "GOCACHE", "GOMODCACHE",
}

// The variables of passThrough name where things are on this machine.
// Two more sets say how to reach something, and only the steps that
// reach it keep them (withProcessEnv): networkPassThrough, a proxy and
// a certificate bundle, for the steps that use the network, and
// dockerPassThrough, the Docker daemon, for the license step. Neither
// holds MISE_DATA_DIR or XDG_DATA_HOME, which miseInstalls refuses.
var (
	networkPassThrough = []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}
	dockerPassThrough  = []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG"}
)

// withProcessEnv returns env with the variables of keys that are set in
// this process, in the order of keys.
func withProcessEnv(env []string, keys []string) []string {
	out := slices.Clone(env)
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			out = append(out, key+"="+value)
		}
	}
	return out
}

// passThroughEnv returns the variables of passThrough that are set in
// this process, in the order of the list. An empty variable is left
// out, as the go command reads an empty variable as an unset one.
func passThroughEnv() []string {
	var env []string
	for _, key := range passThrough {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// goPinEnv holds the two variables that keep the go command on the
// pinned go and on the process's own environment: GOENV=off (no "go
// env -w" file is read) and GOTOOLCHAIN=local (no other go is
// downloaded or run). It is the one list that stepEnv and miseEnv
// (every mise run, "mise install" and its build of govulncheck
// among them) both hold.
var goPinEnv = []string{
	"GOENV=off",
	"GOTOOLCHAIN=local",
}

// stepEnv is the environment of every step of fast, built from nothing
// rather than from this process's: PATH with goDir first, so that a go
// command the linter or a test starts is the pinned one, the rest of
// passThroughEnv, then the variables that decide what the go command
// builds and runs, each with a fixed value. Nothing else reaches a
// step, so a variable that changes what a step checks (GOFLAGS with
// "-run" or build tags, a GOLANGCI_ or GL_ variable, GOEXPERIMENT or
// GOAMD64, which change the build tags) has no effect on it.
//
//   - goPinEnv: GOENV=off, so the go command reads no "go env -w" file,
//     which could set GOFLAGS or any of the variables below, and
//     GOTOOLCHAIN=local, so the go of goDir runs and no other is
//     downloaded.
//   - GOWORK=off: a go.work file in a parent directory does not change
//     the modules.
//   - GOPROXY=off: no step fetches a module, so the module cache must
//     hold the modules of go.sum (10 10.1). Two steps of all reach the
//     network for other reasons: vulnerabilities reads the Go
//     vulnerability database, so its result depends on the date, and
//     license may pull its image, by digest.
//   - GOFLAGS=-mod=readonly: the go command builds from go.mod and the
//     module cache, never from a vendor directory
//     (https://go.dev/ref/mod#build-commands); in a pre-push run,
//     judgeCommit also refuses one that is not tracked.
func stepEnv(goDir string) []string {
	path := goDir
	if value := os.Getenv("PATH"); value != "" {
		path += string(os.PathListSeparator) + value
	}
	rest := slices.DeleteFunc(passThroughEnv(), func(kv string) bool { return strings.HasPrefix(kv, "PATH=") })
	return slices.Concat([]string{"PATH=" + path}, rest, goPinEnv, []string{
		"GOWORK=off",
		"GOPROXY=off",
		"GOFLAGS=-mod=readonly",
	})
}

// lintEnv is the environment of the linter for the target: stepEnv,
// then the target, the same for every machine, with cgo off.
func lintEnv(target lintTarget, goDir string) []string {
	return append(stepEnv(goDir),
		"GOOS="+target.goos,
		"GOARCH="+target.goarch,
		"CGO_ENABLED=0",
	)
}

// run runs the step in dir and returns its output. The command gets
// exactly s.environ: os/exec gives a nil Cmd.Env the environment of
// this process, so a nil environ becomes an empty, non-nil one.
func (s step) run(ctx context.Context, dir string) ([]byte, error) {
	if s.unavailable != nil {
		return nil, s.unavailable
	}
	limit := cmp.Or(s.timeout, stepTimeout)
	parent := ctx
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, s.environ...)
	killWithGroup(cmd)
	out, err := cmd.CombinedOutput()
	// os/exec calls Cancel only while the leader runs, so a grandchild
	// that outlives a leader that ended, and holds the output pipe until
	// WaitDelay, is still ours to kill. A group that is gone is not an
	// error (ESRCH). While the group has members its pgid is not reused,
	// so -pgid reaches only this group. The window after the group has
	// fully emptied (between the last reap and the kill, which a reuse
	// would need a full pid wrap to hit) is accepted, as it is on the
	// Cancel path. Errors other than ESRCH are dropped on purpose: EPERM
	// cannot occur for our own children.
	if cmd.Process != nil {
		_ = killGroup(cmd.Process.Pid)
	}
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		if errors.Is(parent.Err(), context.DeadlineExceeded) {
			// The deadline of the caller fired, before the limit of the step.
			err = fmt.Errorf("%w at the deadline of the run: %w", context.DeadlineExceeded, err)
		} else {
			err = fmt.Errorf("%w after %s: %w", context.DeadlineExceeded, limit, err)
		}
	}
	if err == nil && s.quiet && len(bytes.TrimSpace(out)) > 0 {
		err = errors.New("the command printed what it found")
	}
	return out, err
}

// runFast runs the fast checks: the command steps, then hygiene and
// workflows, and in a pre-push run the pushed range. Every check runs,
// so one run shows everything there is to fix.
//
// Hygiene, workflows and the pushed range judge one commit, which
// runFast resolves to its id, reads and judges before mise or any step
// starts (commitChecks), and reports after the steps: nothing a step
// writes to the working tree, the index, HEAD or the object store
// while it runs changes what they judged. A step that attacks the
// tools/ci process itself is out of reach of that order (the threat
// model of misefiles.go, Not covered).
//
// Run by hand, with no argument, fast judges the commit HEAD names,
// and its steps run on the working tree as it is, staged, modified and
// untracked files included: development stays free, and nothing leaves
// the machine.
//
// Run by the pre-push hook, with the two arguments git gives it (the
// remote's name and its URL) and one line per pushed ref on standard
// input (https://git-scm.com/docs/githooks#_pre_push), fast judges the
// commit the push sends. Before any check it refuses, through
// judgeCommit, a push whose tip is not HEAD and a working tree that
// differs from HEAD in a file the checks read. The steps take minutes,
// and the tree may change meanwhile, so after the last step it judges
// the tree again and refuses the push when a refusal appeared or when
// HEAD, the index or the working tree differ from what it read before
// the steps (changedSince).
//
// In both modes it refuses a go.mod below the module root
// (nestedModules).
func runFast(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 && len(args) != 2 {
		return false, fmt.Errorf("fast takes no argument, or the two of a pre-push hook\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	hook := len(args) == 2
	var push pushed.Push
	if hook {
		if push, err = pushed.Parse(e.stdin); err != nil {
			return false, err
		}
	}
	// repo is what every git command of the judging and of the checks
	// reads with. In a pre-push run it has no repoLocalEnv variable,
	// so a GIT_DIR or an object directory that git exported to the
	// hook, or that the caller set, changes none of what is read
	// (nestedModules too: with the variables cleared it finds the same
	// repository from root, which is what it wants).
	repo := git.Repo{Dir: root, Env: e.gitEnv}
	if hook {
		repo.Env = hookGitEnv(e.gitEnv)
	}
	// judge is what holds before the steps and, in a pre-push run,
	// after them too. It returns "HEAD" by hand, and the id of the
	// commit judged in a pre-push run.
	judge := func() (string, error) {
		commit := "HEAD"
		if hook {
			var err error
			if commit, err = judgeCommit(ctx, repo, push); err != nil {
				return "", err
			}
		}
		return commit, nestedModules(ctx, repo)
	}
	head, err := judge()
	if err != nil {
		return false, err
	}
	// The commit, and in a pre-push run the range, are read and judged
	// before mise or any step runs code of the change (commitChecks).
	verdicts, err := commitChecks(ctx, repo, head)
	if err != nil {
		return false, err
	}
	if hook {
		ranged, err := pushedRange(ctx, repo, args[0], push, head)
		if err != nil {
			return false, err
		}
		verdicts = append(verdicts, ranged...)
	}
	// In a pre-push run the steps must leave the tree as judgeCommit
	// judged it (changedSince, after them).
	var before treeState
	if hook {
		if before, err = readTreeState(ctx, repo); err != nil {
			return false, err
		}
	}
	steps := e.steps
	if steps == nil {
		if err := checkMiseVersion(ctx, root); err != nil {
			return false, err
		}
		tools, err := resolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = fastSteps(root, tools)
	}
	c := newChecks(e.stdout)
	c.run(ctx, root, steps)
	if hook {
		_, err := judge()
		if err == nil {
			err = changedSince(ctx, repo, before)
		}
		if err != nil {
			return false, fmt.Errorf("the working tree changed while the steps ran, so they may not have tested commit %s; push again once nothing changes it:\n%w", head, err)
		}
	}

	for _, v := range verdicts {
		c.verdict(v)
	}
	return c.ok, nil
}

// checks prints the verdict of each check as it ends, and remembers
// whether every one passed. Every check runs, so one run shows
// everything there is to fix.
type checks struct {
	out io.Writer
	ok  bool
}

func newChecks(out io.Writer) *checks {
	return &checks{out: out, ok: true}
}

// report prints one verdict, with what the check found below it.
func (c *checks) report(name string, passed bool, detail string) {
	verdict := "ok  "
	if !passed {
		verdict, c.ok = "FAIL", false
	}
	say(c.out, "%s  %s\n%s", verdict, name, detail)
}

// verdict is what one check of the commit judged found: its findings,
// or why it could not judge (err), or, with a note, why it did not
// check and so neither passed nor failed.
type verdict struct {
	name     string
	findings []finding
	err      error
	note     string
}

// commitChecks judges commit with hygiene and workflows, from one read
// of its tree with every blob in it. fast and all call it before they
// start mise or any step, and report its verdicts after the steps
// (checks.verdict). The steps run code of the change under test, and
// that code could rewrite the working tree, the index, HEAD or an
// object of the store (git cat-file does not hash what it reads, so a
// rewritten loose object is read as the commit's: measured with git
// 2.53.0), so the checks judge what was read before any of it ran. A
// tree that cannot be read is an error; a check that cannot judge
// what was read is a verdict with its error. git and the object store
// are trusted up to that read; what mise may run before it is the
// threat model of misefiles.go.
func commitChecks(ctx context.Context, repo git.Repo, commit string) ([]verdict, error) {
	files, err := headTree(ctx, repo, commit)
	if err != nil {
		return nil, err
	}
	findings, err := hygieneOf(files)
	return []verdict{
		{name: "hygiene", findings: findings, err: err},
		{name: "workflows", findings: workflowsOf(files)},
	}, nil
}

// verdict reports one verdict of commitChecks: one that could not
// judge fails with its reason, and one with a note prints it with
// "--".
func (c *checks) verdict(v verdict) {
	switch {
	case v.note != "":
		say(c.out, "--    %s: %s\n", v.name, v.note)
	case v.err != nil:
		c.report(v.name, false, git.SafeLines(v.err.Error())+"\n")
	default:
		c.report(v.name, len(v.findings) == 0, lines(v.findings))
	}
}

// moduleLookupOff is what the go command prints when a step needs a
// module that the module cache does not hold: the steps run with
// GOPROXY=off (stepEnv).
const moduleLookupOff = "module lookup disabled by GOPROXY=off"

// run runs each step in root and reports it. A step that failed for a
// module missing from the module cache names the command that fills
// the cache.
func (c *checks) run(ctx context.Context, root string, steps []step) {
	for _, s := range steps {
		if s.skip != "" {
			say(c.out, "--    %s: %s\n", s.name, s.skip)
			continue
		}
		out, err := s.run(ctx, root)
		detail := ""
		if err != nil {
			// out is the step's own output, which is printed as it is, for
			// readability; the unit step in particular lets the change print
			// anything. A quiet step is the exception: gofmt -l prints only
			// paths of the repository, so escaping its output costs no
			// readability and is defense in depth. err, the error of the
			// run, may hold a path and is made safe too.
			text := string(out)
			if s.quiet {
				text = git.SafeLines(text)
			}
			detail = fmt.Sprintf("%s%s\n", text, git.SafeLines(err.Error()))
			if bytes.Contains(out, []byte(moduleLookupOff)) {
				detail += "the module cache lacks a module of go.sum, and no step fetches one (GOPROXY=off): run \"go run ./tools/ci setup\", which runs \"go mod download\", then run this again\n"
			}
		}
		c.report(s.name, err == nil, detail)
	}
}

// repoLocalEnv are the variables that tell git which repository, index,
// object store, work tree or configuration to use: the names "git
// rev-parse --local-env-vars" lists (git 2.53.0; TestRepoLocalEnv
// compares the list with the git that runs the tests), and every
// variable whose name starts with GIT_CONFIG, the "git -c" and
// GIT_CONFIG_COUNT settings among them. Git exports such variables to a
// hook, and the githooks page clears this same list before a hook runs
// git on another tree (https://git-scm.com/docs/githooks).
var repoLocalEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
	"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

// hookGitEnv is the environment of the git commands of judgeCommit:
// base (this process's when nil) without the variables of repoLocalEnv,
// so that git finds the repository from the top of the working tree,
// with its own index and objects, and with GIT_OPTIONAL_LOCKS=0, so
// that "git status" does not write the index
// (https://git-scm.com/docs/git#Documentation/git.txt-GITOPTIONALLOCKS).
// The steps of fast get none of these variables either: stepEnv keeps
// only those of passThrough.
func hookGitEnv(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	env := slices.DeleteFunc(slices.Clone(base), func(kv string) bool {
		key, _, _ := strings.Cut(kv, "=")
		return slices.Contains(repoLocalEnv, key) || strings.HasPrefix(key, "GIT_CONFIG_") || key == "GIT_OPTIONAL_LOCKS"
	})
	return append(env, "GIT_OPTIONAL_LOCKS=0")
}

// judgeCommit refuses a pre-push run whose checks would not test the
// commit the push sends. The steps run on the working tree, and the
// review sees only commits, so:
//
//   - every pushed tip that is not a deletion, followed to its commit
//     (an annotated tag to the commit it tags), must be HEAD;
//   - no tracked file may differ from HEAD, staged or not, an
//     intent-to-add entry ("git add -N") included, since a test may
//     read any tracked file;
//   - no untracked or ignored file may be one the go command or the
//     checks read (untrackedInputs).
//
// It returns the id of the commit HEAD names. The error names the
// paths, printable, and never what the files hold.
func judgeCommit(ctx context.Context, repo git.Repo, push pushed.Push) (string, error) {
	head, err := commitOf(ctx, repo, "HEAD")
	if err != nil {
		return "", err
	}
	var others []string
	for _, tip := range push.Tips() {
		commit, err := commitOf(ctx, repo, tip)
		if err != nil {
			return "", fmt.Errorf("pushed tip %s: %w", tip, err)
		}
		if commit != head {
			others = append(others, tip)
		}
	}
	if len(others) > 0 {
		return "", fmt.Errorf("fast tests the commit checked out, HEAD %s, and this push sends another; check out the ref you push and push it again:\n%s",
			head, strings.Join(others, "\n"))
	}
	// --no-renames prints one path per entry; the untracked files are
	// read by untrackedInputs, with the ignored ones.
	out, err := repo.Run(ctx, nil, "status", "--porcelain=v1", "-z", "--no-renames", "--untracked-files=no", "--ignore-submodules=none")
	if err != nil {
		return "", err
	}
	var found []string
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if len(entry) > 3 {
			found = append(found, git.Printable(entry[3:]))
		}
	}
	untracked, err := untrackedInputs(ctx, repo)
	if err != nil {
		return "", err
	}
	found = append(found, untracked...)
	if len(found) == 0 {
		return head, nil
	}
	return "", fmt.Errorf("fast judges the commit a push sends, and the working tree differs from HEAD in these files the checks read; commit them or remove them, then push again:\n%s",
		strings.Join(found, "\n"))
}

// treeState is what the steps of a run must leave as they found it:
// the commit HEAD names, the index (each entry's mode, blob and path),
// and the snapshot of the working tree that generated takes too (each
// file, tracked or untracked, by its type, mode and content). A file
// that differed from HEAD before the steps is compared by its content,
// so a change on top of work in progress is seen. A file that git
// ignores is not compared: no step that judges the tree runs after the
// first step that runs code of the change (allSteps).
type treeState struct {
	head, index string
	files       map[string]string
}

// readTreeState reads the treeState of the repository at repo.Dir.
func readTreeState(ctx context.Context, repo git.Repo) (treeState, error) {
	head, err := commitOf(ctx, repo, "HEAD")
	if err != nil {
		return treeState{}, err
	}
	index, err := repo.Run(ctx, nil, "ls-files", "--stage", "-z")
	if err != nil {
		return treeState{}, err
	}
	root, err := os.OpenRoot(repo.Dir)
	if err != nil {
		return treeState{}, err
	}
	defer func() { _ = root.Close() }()
	files, err := snapshot(ctx, repo, root)
	if err != nil {
		return treeState{}, err
	}
	return treeState{head: head, index: string(index), files: files}, nil
}

// changedSince returns nil when the repository is in the state before
// was read in, and otherwise an error that says what changed: HEAD,
// the index, and each path whose file changed, appeared or went away,
// printable and sorted.
func changedSince(ctx context.Context, repo git.Repo, before treeState) error {
	after, err := readTreeState(ctx, repo)
	if err != nil {
		return fmt.Errorf("read the tree after the steps: %w", err)
	}
	var what []string
	if after.head != before.head {
		what = append(what, "HEAD is now "+after.head)
	}
	if after.index != before.index {
		what = append(what, "the index changed")
	}
	var paths []string
	for path, digest := range before.files {
		if after.files[path] != digest {
			paths = append(paths, path)
		}
	}
	for path := range after.files {
		if _, ok := before.files[path]; !ok {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	for _, path := range paths {
		what = append(what, git.Printable(path))
	}
	if len(what) == 0 {
		return nil
	}
	return errors.New(strings.Join(what, "\n"))
}

// commitOf returns the commit that rev names.
func commitOf(ctx context.Context, repo git.Repo, rev string) (string, error) {
	out, err := repo.Run(ctx, nil, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	return strings.TrimSpace(string(out)), err
}

// untrackedInputs returns the untracked files, ignored or not, that the
// go command or the checks of fast read (isGateInput), printable. "git
// ls-files --others" without an exclude option lists every untracked
// file, the ignored ones included, and a nested repository as its
// directory alone, with a trailing slash
// (https://git-scm.com/docs/git-ls-files).
func untrackedInputs(ctx context.Context, repo git.Repo) ([]string, error) {
	out, err := repo.Run(ctx, nil, "ls-files", "-z", "--others")
	if err != nil {
		return nil, err
	}
	var found []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path != "" && isGateInput(path) {
			found = append(found, git.Printable(path))
		}
	}
	return found, nil
}

// isGateInput reports whether the path, relative to the top of the
// working tree and with forward slashes as git prints it, names an
// untracked file that the go command or the checks read: a Go file (an
// untracked TestMain could end a test run early), a go.work or
// go.work.sum file, a vendor directory at the root, a file mise reads
// when it starts in the directory that holds it or above
// (isMiseFileAtAnyDepth), or a nested repository, which git lists as a
// directory ending in "/" while "go list" still reads the packages
// inside it.
func isGateInput(path string) bool {
	base := filepath.Base(filepath.FromSlash(path))
	return strings.HasSuffix(path, "/") ||
		strings.HasSuffix(base, ".go") ||
		base == "go.work" || base == "go.work.sum" ||
		strings.HasPrefix(path, "vendor/") ||
		isMiseFileAtAnyDepth(path)
}

// nestedModules refuses a go.mod below the root of the working tree,
// tracked or not, ignored or not: it makes its directory a module of
// its own, which "./..." of the root module leaves out, so the steps
// would neither vet, lint nor test the code below it
// (https://go.dev/ref/mod#modules-overview).
func nestedModules(ctx context.Context, repo git.Repo) error {
	out, err := repo.Run(ctx, nil, "ls-files", "-z", "--cached", "--others")
	if err != nil {
		return err
	}
	var found []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if strings.HasSuffix(path, "/go.mod") && !slices.Contains(found, git.Printable(path)) {
			found = append(found, git.Printable(path))
		}
	}
	if len(found) == 0 {
		return nil
	}
	return fmt.Errorf("the checks read one module, and a go.mod below its root takes a directory out of it; remove these files:\n%s",
		strings.Join(found, "\n"))
}

func lines(findings []finding) string {
	var b bytes.Buffer
	for _, f := range findings {
		fmt.Fprintln(&b, f)
	}
	return b.String()
}

// pushedRange applies the name matcher to what a push would publish:
// the remote ref names, the text of each pushed annotated tag and the
// four readings of each pushed commit. It applies the prologue rule to
// the paths those commits add, which needs no denylist entry.
//
// The push is judged by the denylist at head, the commit judgeCommit
// judged, together with the one at the remote's default branch
// (defaultBranchDenylist): an entry that the pushed commit removes
// stays in force until its removal reaches the default branch through
// review. A finding says which of the two lists holds the name.
// It returns the verdicts of the "pushed range" check: its findings,
// and a note when neither list has an entry, as there is nothing to
// match names with then (the prologue rule ran all the same). A
// denylist at head that cannot be read is a verdict with its error,
// as commitChecks reports a check that cannot judge what it read; a
// missing or empty one is hygiene's finding. Any other failure, such
// as a remote that is not configured, is an error.
func pushedRange(ctx context.Context, repo git.Repo, remote string, push pushed.Push, head string) ([]verdict, error) {
	const name = "pushed range"
	atHead, _, err := loadDenylist(ctx, repo, head)
	if err != nil {
		return []verdict{{name: name, err: err}}, nil
	}
	atDefault, err := defaultBranchDenylist(ctx, repo, remote)
	if err != nil {
		return nil, err
	}
	var findings []finding
	list := &names.List{}
	list.Merge(atHead)
	list.Merge(atDefault)
	err = pushed.Walk(ctx, repo, remote, push, func(r pushed.Reading) {
		if r.Field == pushed.FieldPath && isPrologue(r.Path) {
			// A path that holds a listed name is not printed.
			where := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				where = "(withheld)"
			}
			findings = append(findings, finding{readingAt(r) + "added path " + where, "prologue", prologueMsg})
		}
		if !list.Match(r.Text) {
			return
		}
		listed := "a name listed at HEAD"
		if !atHead.Match(r.Text) {
			listed = "a name listed at the remote's default branch"
		}
		of := readingAt(r)
		switch r.Field {
		case pushed.FieldRef:
			findings = append(findings, finding{"pushed ref " + strconv.Itoa(r.Line), "name", "the remote ref name holds " + listed})
		case pushed.FieldMessage:
			findings = append(findings, finding{of + "message line " + strconv.Itoa(r.Line), "name", "the line holds " + listed})
		case pushed.FieldAuthor, pushed.FieldCommitter, pushed.FieldTagger:
			findings = append(findings, finding{of + r.Field, "name", "the name and email hold " + listed})
		case pushed.FieldTagName:
			findings = append(findings, finding{of + r.Field, "name", "the tag's name holds " + listed})
		case pushed.FieldPath:
			findings = append(findings, finding{of + "added path (withheld)", "name", "the path holds " + listed})
		case pushed.FieldLine:
			path := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				path = "(path withheld)"
			}
			findings = append(findings, finding{of + path + ":" + strconv.Itoa(r.Line), "name", "the added line holds " + listed})
		}
	})
	if err != nil {
		return nil, err
	}
	checked := list.Len() > 0
	var verdicts []verdict
	if checked || len(findings) > 0 {
		verdicts = append(verdicts, verdict{name: name, findings: findings})
	}
	if !checked {
		verdicts = append(verdicts, verdict{name: name, note: "not checked against names, no denylist entry at HEAD or at the remote's default branch"})
	}
	return verdicts, nil
}

// readingAt says where a reading is: its commit or its tag, and the
// mergetag header it comes from, with the colon and space that follow.
func readingAt(r pushed.Reading) string {
	of := "commit " + r.Commit + ": "
	if r.Tag != "" {
		of = "tag " + r.Tag + ": "
	}
	if r.Mergetag > 0 {
		of += "mergetag " + strconv.Itoa(r.Mergetag) + " "
	}
	return of
}

// defaultBranchDenylist reads the denylist at the default branch of
// remote as this clone knows it: the commit of
// refs/remotes/<remote>/HEAD, which "git fetch" creates when the remote
// has a HEAD and the clone has no such ref yet
// (https://git-scm.com/docs/git-config#Documentation/git-config.txt-remotenamefollowRemoteHEAD)
// and "git remote set-head <remote> --auto" sets
// (https://git-scm.com/docs/git-remote#Documentation/git-remote.txt-set-head).
//
// It fails closed: a remote that is not configured, such as a push to a
// URL, which is never printed, and a ref that is missing, stop the
// push, and so does a denylist there that cannot be read. A default
// branch that holds no denylist adds nothing: the denylist enters the
// repository through a pull request, and until that one is merged the
// default branch has none. The local refs and the git binary are
// trusted, as the toolchain is (ADR 0007, Threat model).
func defaultBranchDenylist(ctx context.Context, repo git.Repo, remote string) (*names.List, error) {
	out, err := repo.Run(ctx, nil, "remote")
	if err != nil {
		return nil, err
	}
	if !slices.Contains(strings.Fields(string(out)), remote) {
		return nil, errors.New("the push names no configured remote, so the denylist of its default branch is not known here; add the remote, run \"git fetch\" for it, and push to it by name")
	}
	commit, err := commitOf(ctx, repo, "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return nil, fmt.Errorf("the default branch of %s is not known here, and its denylist judges the push; run \"git fetch %s\" (or \"git remote set-head %s --auto\") and push again; if the remote has no commit yet, no fetch can succeed, and the operator makes the first push of a new repository with --no-verify: %w", remote, remote, remote, err)
	}
	list, _, err := denylistAt(ctx, repo, commit)
	if err != nil {
		return nil, fmt.Errorf("%s at the default branch of %s: %w", names.Path, remote, err)
	}
	return list, nil
}
