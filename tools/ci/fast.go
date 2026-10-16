// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

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
func fastSteps(root string, tools check.Tools) []check.Step {
	before, unit := fastPhases(root, tools)
	return append(before, unit)
}

// fastPhases returns the steps of fastSteps in two parts: the ones
// that run none of the change's tests (generated runs the tools/ci of
// the change, which the ask-first review of tools/ci covers), and
// unit, the first that runs them. all puts its own steps that judge
// the tree between the two (allSteps).
func fastPhases(root string, tools check.Tools) ([]check.Step, check.Step) {
	goCmd := filepath.Join(tools.GoDir, "go")
	unit := []string{goCmd, "test", "-count=1"}
	for _, dir := range []string{"internal", "tools"} {
		if info, err := os.Stat(filepath.Join(root, dir)); err == nil && info.IsDir() {
			unit = append(unit, "./"+dir+"/...")
		}
	}
	steps := []check.Step{
		{Name: "format", Argv: []string{filepath.Join(tools.GoDir, "gofmt"), "-l", "."}, Quiet: true, Environ: check.StepEnv(tools.GoDir)},
		{Name: "vet", Argv: []string{goCmd, "vet", "./..."}, Environ: check.StepEnv(tools.GoDir)},
		{Name: "generated", Argv: []string{goCmd, "run", "./tools/ci", "generated"}, Environ: check.StepEnv(tools.GoDir)},
	}
	for _, target := range lintTargets {
		steps = append(steps, check.Step{
			Name:    "lint " + target.String(),
			Argv:    lintArgv(tools.Linter),
			Environ: lintEnv(target, tools.GoDir),
		})
	}
	return steps, check.Step{Name: "unit", Argv: unit, Environ: check.StepEnv(tools.GoDir)}
}

// lintArgs are the arguments of every golangci-lint run, the one list
// that the lint steps and the tests that run the linter hold:
//
//   - "--config" names .golangci.yml, so that no other .golangci.* file
//     takes its place.
//   - "-j 1" (golangci-lint v2.14.0, pkg/commands/flagsets.go:
//     --concurrency) limits golangci-lint's own Go threads to one
//     (GOMAXPROCS, pkg/commands/run.go); child processes such as "go
//     list" and the compiler are not capped. It does not fix the lock
//     failure below. A cold run takes about 14s against about 7s by
//     default, and warm runs are about the same. It stays to limit CPU
//     use when runs overlap.
//   - "--allow-parallel-runners" (same file) turns off the file lock
//     that a run takes on golangci-lint.lock in the temporary directory
//     (pkg/commands/run.go, acquireFileLock). Without it, a run fails
//     with "parallel golangci-lint is running" after 5 seconds whenever
//     any other run on the machine, of any repository, holds the lock.
//     The lock is no check: it only keeps runs from overlapping.
var lintArgs = []string{"run", "--config", ".golangci.yml", "-j", "1", "--allow-parallel-runners"}

// lintArgv returns the command line of a golangci-lint run: the path of
// the linter, then lintArgs.
func lintArgv(linter string) []string {
	return append([]string{linter}, lintArgs...)
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

// lintEnv is the environment of the linter for the target: stepEnv,
// then the target, the same for every machine, with cgo off.
func lintEnv(target lintTarget, goDir string) []string {
	return append(check.StepEnv(goDir),
		"GOOS="+target.goos,
		"GOARCH="+target.goarch,
		"CGO_ENABLED=0",
	)
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
func runFast(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 && len(args) != 2 {
		return false, fmt.Errorf("fast takes no argument, or the two of a pre-push hook\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	hook := len(args) == 2
	var push pushed.Push
	if hook {
		if push, err = pushed.Parse(e.Stdin); err != nil {
			return false, err
		}
	}
	// repo is what every git command of the judging and of the checks
	// reads with. In a pre-push run it has no git.LocalEnv variable,
	// so a GIT_DIR or an object directory that git exported to the
	// hook, or that the caller set, changes none of what is read
	// (nestedModules too: with the variables cleared it finds the same
	// repository from root, which is what it wants).
	repo := git.Repo{Dir: root, Env: e.GitEnv}
	if hook {
		repo.Env = git.CleanEnv(e.GitEnv)
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
	steps := e.Steps
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
	c := newChecks(e.Stdout)
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
	check.Say(c.out, "%s  %s\n%s", verdict, name, detail)
}

// commitChecks judges commit with hygiene, workflows and reqs, from one
// read of its tree with every blob in it. fast and all call it before they
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
func commitChecks(ctx context.Context, repo git.Repo, commit string) ([]check.Verdict, error) {
	files, err := check.HeadTree(ctx, repo, commit)
	if err != nil {
		return nil, err
	}
	findings, err := hygieneOf(files)
	reqs, deferred, reqsErr := reqsOf(files)
	return []check.Verdict{
		{Name: "hygiene", Findings: findings, Err: err},
		{Name: "workflows", Findings: workflowsOf(files)},
		{Name: "reqs", Findings: reqs, Err: reqsErr, Info: reqDeferredInfo(deferred)},
	}, nil
}

// reqDeferredInfo is the line under the reqs verdict of fast and all
// when ids rest on deferred tasks alone: the count, and where the ids
// are listed (12 12.11).
func reqDeferredInfo(deferred []string) string {
	if len(deferred) == 0 {
		return ""
	}
	return fmt.Sprintf("      %d id(s) cited only by deferred tasks; go run ./tools/ci reqs lists them\n", len(deferred))
}

// verdict reports one verdict of commitChecks: one that could not
// judge fails with its reason, and one with a note prints it with
// "--".
func (c *checks) verdict(v check.Verdict) {
	switch {
	case v.Note != "":
		check.Say(c.out, "--    %s: %s\n", v.Name, v.Note)
	case v.Err != nil:
		c.report(v.Name, false, git.SafeLines(v.Err.Error())+"\n")
	default:
		c.report(v.Name, len(v.Findings) == 0, check.Lines(v.Findings)+v.Info)
	}
}

// moduleLookupOff is what the go command prints when a step needs a
// module that the module cache does not hold: the steps run with
// GOPROXY=off (stepEnv).
const moduleLookupOff = "module lookup disabled by GOPROXY=off"

// run runs each step in root and reports it. A step that failed for a
// module missing from the module cache names the command that fills
// the cache.
func (c *checks) run(ctx context.Context, root string, steps []check.Step) {
	for _, s := range steps {
		if s.Skip != "" {
			check.Say(c.out, "--    %s: %s\n", s.Name, s.Skip)
			continue
		}
		out, err := s.Run(ctx, root)
		detail := ""
		if err != nil {
			// out is the step's own output, which is printed as it is, for
			// readability; the unit step in particular lets the change print
			// anything. A quiet step is the exception: gofmt -l prints paths
			// of the repository and its parse errors about them, so escaping
			// its output costs no readability and is defense in depth.
			// err, the error of the run, may hold a path and is made safe
			// too.
			text := string(out)
			if s.Quiet {
				text = git.SafeLines(text)
			}
			detail = fmt.Sprintf("%s%s\n", text, git.SafeLines(err.Error()))
			if bytes.Contains(out, []byte(moduleLookupOff)) {
				detail += "the module cache lacks a module of go.sum, and no step fetches one (GOPROXY=off): run \"go run ./tools/ci setup\", which runs \"go mod download\", then run this again\n"
			}
		}
		c.report(s.Name, err == nil, detail)
	}
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
	head, err := check.CommitOf(ctx, repo, "HEAD")
	if err != nil {
		return "", err
	}
	var others []string
	for _, tip := range push.Tips() {
		commit, err := check.CommitOf(ctx, repo, tip)
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
	head, err := check.CommitOf(ctx, repo, "HEAD")
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
func pushedRange(ctx context.Context, repo git.Repo, remote string, push pushed.Push, head string) ([]check.Verdict, error) {
	const name = "pushed range"
	atHead, _, err := loadDenylist(ctx, repo, head)
	if err != nil {
		return []check.Verdict{{Name: name, Err: err}}, nil
	}
	atDefault, err := defaultBranchDenylist(ctx, repo, remote)
	if err != nil {
		return nil, err
	}
	var findings []check.Finding
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
			findings = append(findings, check.Finding{Where: readingAt(r) + "added path " + where, Rule: "prologue", Msg: prologueMsg})
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
			findings = append(findings, check.Finding{Where: "pushed ref " + strconv.Itoa(r.Line), Rule: "name", Msg: "the remote ref name holds " + listed})
		case pushed.FieldMessage:
			findings = append(findings, check.Finding{Where: of + "message line " + strconv.Itoa(r.Line), Rule: "name", Msg: "the line holds " + listed})
		case pushed.FieldAuthor, pushed.FieldCommitter, pushed.FieldTagger:
			findings = append(findings, check.Finding{Where: of + r.Field, Rule: "name", Msg: "the name and email hold " + listed})
		case pushed.FieldTagName:
			findings = append(findings, check.Finding{Where: of + r.Field, Rule: "name", Msg: "the tag's name holds " + listed})
		case pushed.FieldPath:
			findings = append(findings, check.Finding{Where: of + "added path (withheld)", Rule: "name", Msg: "the path holds " + listed})
		case pushed.FieldLine:
			path := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				path = "(path withheld)"
			}
			findings = append(findings, check.Finding{Where: of + path + ":" + strconv.Itoa(r.Line), Rule: "name", Msg: "the added line holds " + listed})
		}
	})
	if err != nil {
		return nil, err
	}
	checked := list.Len() > 0
	var verdicts []check.Verdict
	if checked || len(findings) > 0 {
		verdicts = append(verdicts, check.Verdict{Name: name, Findings: findings})
	}
	if !checked {
		verdicts = append(verdicts, check.Verdict{Name: name, Note: "not checked against names, no denylist entry at HEAD or at the remote's default branch"})
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
	commit, err := check.CommitOf(ctx, repo, "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return nil, fmt.Errorf("the default branch of %s is not known here, and its denylist judges the push; run \"git fetch %s\" (or \"git remote set-head %s --auto\") and push again; if the remote has no commit yet, no fetch can succeed, and the operator makes the first push of a new repository with --no-verify: %w", remote, remote, remote, err)
	}
	list, _, err := denylistAt(ctx, repo, commit)
	if err != nil {
		return nil, fmt.Errorf("%s at the default branch of %s: %w", names.Path, remote, err)
	}
	return list, nil
}
