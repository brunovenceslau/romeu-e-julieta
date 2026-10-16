// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/generated"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// inFast and inAll say whether a check runs in fast and in all. A
// check of the pre-push run alone (check.Check.Hook) runs in fast when
// fast runs as the hook (commitChecks).
func inFast(c check.Check) bool { return c.Fast }
func inAll(c check.Check) bool  { return c.All }

// registered returns the registered checks that in holds, in the order
// of the registry.
func registered(in func(check.Check) bool) []check.Check {
	var out []check.Check
	for _, c := range check.Registered() {
		if in(c) {
			out = append(out, c)
		}
	}
	return out
}

// stepsOf returns the command steps of the checks that in holds, in the
// order a run starts them (check.RunOrder): every check that judges the
// tree, by order key, then every check that runs code of the change, so
// that no test of the change can rewrite a file before a check has read
// it.
//
// Every step runs a pinned tool by its path: gofmt and go from the
// directory of the go command that mise resolves, and the golangci-lint
// and govulncheck it resolves, directly and not through "mise exec",
// which would add the [env] table of a mise configuration to the
// environment. Each one runs in check.StepEnv, so the go command that
// vets and tests is the one the linter runs, whatever go is first on
// the search path.
func stepsOf(ctx context.Context, in func(check.Check) bool, p check.Plan) []check.Step {
	var steps []check.Step
	for _, c := range check.RunOrder(registered(in)) {
		steps = append(steps, c.Steps(ctx, p)...)
	}
	return steps
}

// fastSteps returns the command steps of fast, from the In fast column
// of 10 10.2: format, vet, generated, lint (once per lint target) and
// unit. The other rows of that column join with the code they check.
func fastSteps(ctx context.Context, root string, tools check.Tools) []check.Step {
	return stepsOf(ctx, inFast, check.Plan{Root: root, Tools: tools})
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
	var hooked *check.Hook
	if hook {
		hooked = &check.Hook{Remote: args[0], Push: push}
	}
	verdicts, err := commitChecks(ctx, repo, head, hooked, inFast)
	if err != nil {
		return false, err
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
		if err := mise.CheckVersion(ctx, root); err != nil {
			return false, err
		}
		tools, err := mise.ResolveLintTools(ctx, root)
		if err != nil {
			return false, err
		}
		steps = fastSteps(ctx, root, tools)
	}
	c := check.NewPrinter(e.Stdout)
	c.Run(ctx, root, steps)
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
		c.Verdict(v)
	}
	return c.OK, nil
}

// commitChecks judges commit with every check of the commit that in
// holds (check.Check.Judge), in the order of the registry, from one
// read of its tree with every blob in it; with hook, the push of a
// pre-push run, the checks of that run alone (check.Check.Hook) judge
// it too. fast and all call it before they start mise or any step, and
// report its verdicts after the steps (check.Printer.Verdict). The
// steps run code of the change under test, and that code could rewrite
// the working tree, the index, HEAD or an object of the store (git
// cat-file does not hash what it reads, so a rewritten loose object is
// read as the commit's: measured with git 2.53.0), so the checks judge
// what was read before any of it ran. A tree that cannot be read is an
// error; a check that cannot judge what was read is a verdict with its
// error, and one that fails otherwise, as the pushed range does for a
// remote that is not configured, is an error. git and the object store
// are trusted up to that read; what mise may run before it is the
// threat model of misefiles.go.
func commitChecks(ctx context.Context, repo git.Repo, commit string, hook *check.Hook, in func(check.Check) bool) ([]check.Verdict, error) {
	files, err := check.HeadTree(ctx, repo, commit)
	if err != nil {
		return nil, err
	}
	judged := check.Commit{Repo: repo, ID: commit, Files: files, Hook: hook}
	var verdicts []check.Verdict
	for _, c := range check.Registered() {
		judges := in(c) || (hook != nil && c.Hook)
		if c.Judge == nil || !judges {
			continue
		}
		got, err := c.Judge(ctx, judged)
		if err != nil {
			return nil, err
		}
		verdicts = append(verdicts, got...)
	}
	return verdicts, nil
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
	files, err := generated.Snapshot(ctx, repo, root)
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
// (mise.IsMiseFileAtAnyDepth), or a nested repository, which git lists as a
// directory ending in "/" while "go list" still reads the packages
// inside it.
func isGateInput(path string) bool {
	base := filepath.Base(filepath.FromSlash(path))
	return strings.HasSuffix(path, "/") ||
		strings.HasSuffix(base, ".go") ||
		base == "go.work" || base == "go.work.sum" ||
		strings.HasPrefix(path, "vendor/") ||
		mise.IsMiseFileAtAnyDepth(path)
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
