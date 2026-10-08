// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	// quiet marks a command whose output is its finding: it passes
	// when it prints nothing, as "gofmt -l" does.
	quiet bool
	// environ is the whole environment of the command (stepEnv). The
	// environment of this process never reaches it, so an empty one is
	// an empty environment.
	environ []string
}

// fastSteps returns the command steps of fast, from the In fast column
// of 10 10.2: format, vet, lint (once per lintTargets) and unit. The
// other rows of that column join with the code they check.
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
	}
	for _, target := range lintTargets {
		steps = append(steps, step{
			name:    "lint " + target.String(),
			argv:    []string{tools.linter, "run", "--config", ".golangci.yml"},
			environ: lintEnv(target, tools.goDir),
		})
	}
	return append(steps, step{name: "unit", argv: unit, environ: stepEnv(tools.goDir)})
}

// lintTools are the pinned tools of fast, as mise resolves them: the
// path of golangci-lint, and the directory of the go command.
type lintTools struct {
	linter, goDir string
}

// resolveLintTools asks "mise which" for the paths of the versions that
// mise.toml pins and mise.lock locks, in the environment of
// passThroughEnv. "mise which" runs no tool and installs none: a tool
// that is not installed is an error that names the two commands a new
// machine runs first, so fast fails closed and never falls back to
// another golangci-lint or go on the search path.
//
// Each path must lie, once its symbolic links are resolved, in the
// directory where mise installs that tool (miseInstalls), and the error
// names that directory. A mise configuration may name a tool by a path
// ("path:" in [tools], in the repository or in the global
// configuration); such a tool is no pinned one, and it is an error here
// too.
func resolveLintTools(ctx context.Context, root string) (lintTools, error) {
	installsDir, err := miseInstalls()
	if err != nil {
		return lintTools{}, err
	}
	installs, err := filepath.EvalSymlinks(installsDir)
	if err != nil {
		return lintTools{}, fmt.Errorf("the mise installs directory %s: %w; run \"mise trust\" and \"mise install\" in %s", installsDir, err, root)
	}
	which := func(name string) (string, error) {
		cmd := exec.CommandContext(ctx, "mise", "-C", root, "which", name)
		cmd.Env = passThroughEnv()
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
		dir := filepath.Join(installs, name)
		if rel, err := filepath.Rel(dir, path); err != nil || !filepath.IsLocal(rel) {
			return "", fmt.Errorf("mise which %s: %s is outside %s, where mise installs it: the mise configuration names a tool mise did not install", name, path, dir)
		}
		return path, nil
	}
	linter, err := which("golangci-lint")
	if err != nil {
		return lintTools{}, err
	}
	goBin, err := which("go")
	if err != nil {
		return lintTools{}, err
	}
	return lintTools{linter: linter, goDir: filepath.Dir(goBin)}, nil
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

// stepEnv is the environment of every step of fast, built from nothing
// rather than from this process's: PATH with goDir first, so that a go
// command the linter or a test starts is the pinned one, the rest of
// passThroughEnv, then the variables that decide what the go command
// builds and runs, each with a fixed value. Nothing else reaches a
// step, so a variable that changes what a step checks (GOFLAGS with
// "-run" or build tags, a GOLANGCI_ or GL_ variable, GOEXPERIMENT or
// GOAMD64, which change the build tags) has no effect on it.
//
//   - GOENV=off: the go command reads no "go env -w" file, which could
//     set GOFLAGS or any of the variables below.
//   - GOTOOLCHAIN=local: the go of goDir runs, and no other is
//     downloaded.
//   - GOWORK=off: a go.work file in a parent directory does not change
//     the modules.
//   - GOPROXY=off: no step reaches the network, so the module cache
//     must hold the modules of go.sum (10 10.1).
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
	return slices.Concat([]string{"PATH=" + path}, rest, []string{
		"GOENV=off",
		"GOTOOLCHAIN=local",
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
	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append([]string{}, s.environ...)
	out, err := cmd.CombinedOutput()
	if err == nil && s.quiet && len(bytes.TrimSpace(out)) > 0 {
		err = errors.New("the command printed what it found")
	}
	return out, err
}

// runFast runs the fast checks: the command steps, then hygiene. Every
// check runs, so one run shows everything there is to fix.
//
// Run by hand, with no argument, fast checks the working tree as it is,
// staged, modified and untracked files included: development stays
// free, and nothing leaves the machine.
//
// Run by the pre-push hook, with the two arguments git gives it (the
// remote's name and its URL) and one line per pushed ref on standard
// input (https://git-scm.com/docs/githooks#_pre_push), fast judges the
// commit the push sends. Before any check it refuses, through
// judgeCommit, a push whose tip is not HEAD and a working tree that
// differs from HEAD in a file the checks read. The steps take minutes,
// and the tree may change meanwhile, so after the last step it judges
// the tree again and refuses the push when HEAD moved or a refusal
// appeared. Hygiene and the pushed range read the commit judged, by
// its id, and not HEAD again.
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
	steps := e.steps
	if steps == nil {
		tools, err := resolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = fastSteps(root, tools)
	}
	ok := true
	report := func(name string, passed bool, detail string) {
		verdict := "ok  "
		if !passed {
			verdict, ok = "FAIL", false
		}
		say(e.stdout, "%s  %s\n%s", verdict, name, detail)
	}
	for _, s := range steps {
		out, err := s.run(ctx, root)
		detail := ""
		if err != nil {
			detail = fmt.Sprintf("%s%v\n", out, err)
		}
		report(s.name, err == nil, detail)
	}
	if hook {
		again, err := judge()
		if err == nil && again != head {
			err = fmt.Errorf("HEAD is now %s", again)
		}
		if err != nil {
			return false, fmt.Errorf("the working tree changed while the steps ran, so they may not have tested commit %s; push again once nothing changes it:\n%w", head, err)
		}
	}

	// Equivalent mutant, accepted: reading "HEAD" here instead of head
	// changes nothing in a sequential run, because the re-judge above
	// forces HEAD == head and no step runs between it and this call.
	// hygiene itself is pinned to its argument (TestJudgedCommitIsPinned).
	findings, err := hygiene(ctx, repo, head)
	if err != nil {
		return false, err
	}
	report("hygiene", len(findings) == 0, lines(findings))

	if hook {
		findings, checked, err := pushedRange(ctx, repo, args[0], push, head)
		if err != nil {
			return false, err
		}
		if checked || len(findings) > 0 {
			report("pushed range", len(findings) == 0, lines(findings))
		}
		if !checked {
			say(e.stdout, "--    pushed range: not checked against names, no denylist entry at HEAD or at the remote's default branch\n")
		}
	}
	return ok, nil
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
// go.work.sum file, a vendor directory at the root, a mise
// configuration (a toml file whose name starts with "mise" or ".mise",
// or that lies below a "mise" or ".mise" directory: every place where
// "mise config ls" of mise 2026.10.3 finds one), a .tool-versions file,
// or a nested repository, which git lists as a directory ending in "/"
// while "go list" still reads the packages inside it.
func isGateInput(path string) bool {
	base := filepath.Base(filepath.FromSlash(path))
	switch {
	case strings.HasSuffix(path, "/"),
		strings.HasSuffix(base, ".go"),
		base == "go.work", base == "go.work.sum", base == ".tool-versions",
		strings.HasPrefix(path, "vendor/"):
		return true
	case strings.HasSuffix(base, ".toml"):
		parts := strings.Split(path, "/")
		for _, dir := range parts[:len(parts)-1] {
			if dir == "mise" || dir == ".mise" {
				return true
			}
		}
		return strings.HasPrefix(base, "mise") || strings.HasPrefix(base, ".mise")
	}
	return false
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
	return fmt.Errorf("fast checks one module, and a go.mod below its root takes a directory out of it; remove these files:\n%s",
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
// Without an entry in either there is nothing to match names with, and
// checked is false: it means "names were matched", and the prologue
// rule ran all the same.
func pushedRange(ctx context.Context, repo git.Repo, remote string, push pushed.Push, head string) (findings []finding, checked bool, err error) {
	// A list that is missing or empty at head is hygiene's finding.
	atHead, _, err := loadDenylist(ctx, repo, head)
	if err != nil {
		return nil, false, err
	}
	atDefault, err := defaultBranchDenylist(ctx, repo, remote)
	if err != nil {
		return nil, false, err
	}
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
	return findings, list.Len() > 0, err
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
