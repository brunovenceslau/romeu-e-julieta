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
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The verdicts of a mutant.
const (
	killed   = "killed"
	survived = "survived"
	invalid  = "invalid"
)

// passThrough lists the variables of this process that reach the go
// command, when they are set: its caches, its module cache and its
// temporary files. With PATH, which goEnv sets, it is the part of the
// list of tools/ci (its passThrough) that a go test needs.
var passThrough = []string{
	"HOME", "TMPDIR", "XDG_CACHE_HOME",
	"GOPATH", "GOCACHE", "GOMODCACHE",
}

// fixedGoEnv holds the variables that decide what the go command builds
// and runs, each with a fixed value, as the steps of tools/ci set them:
// no "go env -w" file, no other toolchain, no go.work file, go.mod and
// the module cache only, and no module fetched.
var fixedGoEnv = []string{
	"GOENV=off",
	"GOTOOLCHAIN=local",
	"GOWORK=off",
	"GOFLAGS=-mod=readonly",
	"GOPROXY=off",
}

// runner holds what one run works with.
type runner struct {
	e      env
	tmp    string // the temporary directory: the clone and the test binaries
	clone  git.Repo
	root   *os.Root // the working tree of the clone
	commit string   // the commit HEAD of the source, which the clone holds
	goBin  string
	goEnv  []string
}

// mutate runs the mutants of the file at mutantsPath and reports
// whether every one was killed. An error means the run could not
// proceed.
func mutate(ctx context.Context, e env, mutantsPath string, worktree bool) (ok bool, err error) {
	if !filepath.IsAbs(mutantsPath) {
		mutantsPath = filepath.Join(e.dir, mutantsPath)
	}
	data, err := os.ReadFile(mutantsPath)
	if err != nil {
		return false, err
	}
	mutants, err := parseMutants(data)
	if err != nil {
		return false, fmt.Errorf("%s: %w", mutantsPath, err)
	}
	r, err := newRunner(ctx, e, worktree)
	if err != nil {
		return false, err
	}
	defer func() {
		if rmErr := r.close(); rmErr != nil && err == nil {
			err = rmErr
		}
	}()
	if err := r.baseline(ctx, targets(mutants)); err != nil {
		return false, err
	}
	counts := map[string]int{}
	for _, m := range mutants {
		verdict, reason, err := r.mutant(ctx, m)
		if err != nil {
			return false, err
		}
		counts[verdict]++
		say(e.stdout, "%s %s %s\n", m.ID, verdict, reason)
	}
	say(e.stdout, "mutate: %d killed, %d survived, %d invalid, of %d mutants at %s\n",
		counts[killed], counts[survived], counts[invalid], len(mutants), r.commit)
	return counts[killed] == len(mutants), nil
}

// newRunner finds the go command, then clones the commit HEAD of the
// repository that e.dir lies in into a new temporary directory, and
// checks that the clone is where it should be and holds that commit.
// With worktree, the clone then moves to a snapshot of the working tree
// (snapshot).
func newRunner(ctx context.Context, e env, worktree bool) (r *runner, err error) {
	source := git.Repo{Dir: e.dir, Env: e.gitEnv}
	top, err := gitLine(ctx, source, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	source.Dir = top
	commit, err := gitLine(ctx, source, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%s has no commit HEAD to run: %w", top, err)
	}
	goBin := e.goBin
	if goBin == "" {
		if goBin, err = resolveGo(ctx, top); err != nil {
			return nil, err
		}
	}
	if !worktree {
		if err := warnDirty(ctx, e, source, commit); err != nil {
			return nil, err
		}
	}
	tmp, err := os.MkdirTemp(e.tmpDir, "mutate-")
	if err != nil {
		return nil, fmt.Errorf("make the temporary directory: %w", err)
	}
	r = &runner{e: e, tmp: tmp, commit: commit, goBin: goBin, goEnv: goEnv(goBin)}
	defer func() {
		if err != nil {
			err = errors.Join(err, r.close())
			r = nil
		}
	}()
	dest := filepath.Join(tmp, "clone")
	if _, err := source.Run(ctx, nil, "clone", "--no-local", "--quiet", "--", top, dest); err != nil {
		return r, err
	}
	r.clone = git.Repo{Dir: dest, Env: e.gitEnv}
	gitDir, err := gitLine(ctx, r.clone, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return r, err
	}
	if !inside(tmp, gitDir) {
		return r, fmt.Errorf("the clone's git directory %s is not inside %s", gitDir, tmp)
	}
	head, err := gitLine(ctx, r.clone, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return r, err
	}
	if head != commit {
		return r, fmt.Errorf("the clone holds the commit %s, not %s, the commit HEAD of %s", head, commit, top)
	}
	if worktree {
		if r.commit, err = snapshot(ctx, source, r.clone, commit, tmp); err != nil {
			return r, err
		}
	}
	r.root, err = os.OpenRoot(dest)
	return r, err
}

// close removes the temporary directory.
func (r *runner) close() error {
	if r.root != nil {
		_ = r.root.Close()
	}
	if err := os.RemoveAll(r.tmp); err != nil {
		return fmt.Errorf("remove the clone: %w", err)
	}
	return nil
}

// warnDirty says on standard error that the working tree has changes
// the run does not see, as it runs the commit HEAD only.
func warnDirty(ctx context.Context, e env, source git.Repo, commit string) error {
	// --no-optional-locks: status does not refresh the real index.
	out, err := source.Run(ctx, nil, "--no-optional-locks", "status", "--porcelain")
	if err != nil {
		return err
	}
	if n := bytes.Count(out, []byte("\n")); n > 0 {
		say(e.stderr, "mutate: the working tree has uncommitted changes (git status: %d); the run uses the commit %s without them; -worktree runs them too\n", n, commit)
	}
	return nil
}

// baseline runs each target with no mutant. A target that fails, or
// that runs no test, is an error: no mutant can be killed by a test
// that already fails or that does not run.
func (r *runner) baseline(ctx context.Context, ts []target) error {
	for _, t := range ts {
		failed, out, err := r.goTest(ctx, t)
		if err != nil {
			return err
		}
		if failed {
			return fmt.Errorf("the baseline of %s -run %s fails with no mutant, so nothing can be killed:\n%s", t.pkg, t.run, out)
		}
		if !ranTests(out) {
			return fmt.Errorf("%s -run %s runs no test, so nothing can be killed:\n%s", t.pkg, t.run, out)
		}
	}
	return nil
}

// ranTests reports whether the output of a "go test" that passed shows
// a package where a test ran: an "ok" line that does not end in "[no
// tests to run]" (cmd/go reports a package whose -run matched nothing
// that way, and one with no test file with "?").
func ranTests(out string) bool {
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ok ") && !strings.HasSuffix(line, "[no tests to run]") {
			return true
		}
	}
	return false
}

// mutant applies m in the clone, runs its target, writes the file back
// from its blob, and returns the verdict and its reason. An error means
// the run cannot go on.
func (r *runner) mutant(ctx context.Context, m mutant) (verdict, reason string, err error) {
	blob, err := r.clone.Run(ctx, nil, "cat-file", "blob", r.commit+":"+m.File)
	if err != nil {
		if ctx.Err() != nil {
			return "", "", errInterrupted
		}
		return invalid, fmt.Sprintf("%s is not a file of the commit", git.Printable(m.File)), nil
	}
	onDisk, err := r.root.ReadFile(filepath.FromSlash(m.File))
	if err != nil || !bytes.Equal(onDisk, blob) {
		return invalid, fmt.Sprintf("%s is not checked out as its blob (a link, or a filter of .gitattributes)", git.Printable(m.File)), nil
	}
	if n := bytes.Count(blob, []byte(m.Old)); n != 1 {
		return invalid, fmt.Sprintf("old occurs %d times in %s, not once", n, git.Printable(m.File)), nil
	}
	pkgs, err := r.packages(ctx, m)
	if err != nil {
		return "", "", err
	}
	mutated := bytes.Replace(blob, []byte(m.Old), []byte(m.New), 1)
	defer func() {
		if restoreErr := r.restore(m.File, blob); restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()
	if err := r.root.WriteFile(filepath.FromSlash(m.File), mutated, 0o644); err != nil {
		return "", "", err
	}
	built, out, err := r.build(ctx, pkgs)
	if err != nil {
		return "", "", err
	}
	if !built {
		say(r.e.stderr, "%s: the build fails:\n%s\n", m.ID, git.SafeLines(strings.TrimSpace(out)))
		return invalid, "the mutated code does not build", nil
	}
	failed, _, err := r.goTest(ctx, m.target())
	if err != nil {
		return "", "", err
	}
	if failed {
		return killed, fmt.Sprintf("%s -run %s fails", m.Package, m.Run), nil
	}
	return survived, fmt.Sprintf("%s -run %s passes", m.Package, m.Run), nil
}

// restore writes the blob back to the file and checks that the clone
// then has no change to a tracked file, so the next mutant starts from
// the commit.
func (r *runner) restore(file string, blob []byte) error {
	if err := r.root.WriteFile(filepath.FromSlash(file), blob, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", git.Printable(file), err)
	}
	// Not the context of the run: an interrupted run still checks.
	out, err := r.clone.Run(context.Background(), nil, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if len(out) > 0 {
		return fmt.Errorf("the clone still has changes after %s was restored:\n%s", git.Printable(file), out)
	}
	return nil
}

// packages returns the packages a mutant of m builds: its target and,
// for a Go file, the package of the file when go lists one there, as it
// does not in a testdata directory. It runs on the commit, before the
// mutant is written, so a mutant that breaks the package clause does
// not hide the package.
func (r *runner) packages(ctx context.Context, m mutant) ([]string, error) {
	pkgs := []string{m.Package}
	if !strings.HasSuffix(m.File, ".go") {
		return pkgs, nil
	}
	dir := "./" + path.Dir(m.File)
	if dir == m.Package {
		return pkgs, nil
	}
	notListed, _, err := r.goCmd(ctx, "list", "-find", dir)
	if err != nil {
		return nil, err
	}
	if !notListed {
		pkgs = append(pkgs, dir)
	}
	return pkgs, nil
}

// build builds the test binaries of pkgs and reports whether they
// build.
func (r *runner) build(ctx context.Context, pkgs []string) (bool, string, error) {
	bin := filepath.Join(r.tmp, "bin") + string(filepath.Separator)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return false, "", err
	}
	failed, out, err := r.goCmd(ctx, slices.Concat([]string{"test", "-c", "-vet=off", "-o", bin}, pkgs)...)
	return !failed, out, err
}

// goTest runs the target in the clone and reports whether it failed.
// A package pattern starts with "./" (mutant.check), so the go command
// never reads one as a flag.
func (r *runner) goTest(ctx context.Context, t target) (failed bool, out string, err error) {
	return r.goCmd(ctx, "test", "-count=1", "-vet=off", "-run="+t.run, t.pkg)
}

// goCmd runs the go command in the clone. It reports a non-zero exit as
// failed with the output; an error means the command did not run or
// the run was interrupted.
func (r *runner) goCmd(ctx context.Context, args ...string) (failed bool, out string, err error) {
	cmd := exec.CommandContext(ctx, r.goBin, args...)
	cmd.Dir = r.clone.Dir
	cmd.Env = r.goEnv
	killWithGroup(cmd)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return false, "", errInterrupted
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return true, string(output), nil
	}
	if err != nil {
		return false, "", fmt.Errorf("%s %s: %w", r.goBin, strings.Join(args, " "), err)
	}
	return false, string(output), nil
}

// resolveGo returns the go command of the GOROOT that "go env GOROOT"
// names at the top of the source tree, where the mise shim finds the
// pinned go, and checks that it is the version this tool was built
// with.
func resolveGo(ctx context.Context, top string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "GOROOT")
	cmd.Dir = top
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT in %s: %w", top, err)
	}
	goBin := filepath.Join(strings.TrimSpace(string(out)), "bin", "go")
	if err := checkGoVersion(ctx, goBin); err != nil {
		return "", err
	}
	return goBin, nil
}

// checkGoVersion fails unless goBin is the version of the go that built
// this tool.
func checkGoVersion(ctx context.Context, goBin string) error {
	cmd := exec.CommandContext(ctx, goBin, "env", "GOVERSION")
	cmd.Env = goEnv(goBin)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s env GOVERSION: %w", goBin, err)
	}
	if got := strings.TrimSpace(string(out)); got != runtime.Version() {
		return fmt.Errorf("%s is %s, not %s, the go this tool was built with", goBin, got, runtime.Version())
	}
	return nil
}

// goEnv is the environment of the go command: the variables of
// passThrough that are set, with the directory of goBin first on PATH,
// so a go that a test starts is the same one, then fixedGoEnv.
func goEnv(goBin string) []string {
	env := []string{"PATH=" + filepath.Dir(goBin) + string(os.PathListSeparator) + os.Getenv("PATH")}
	for _, key := range passThrough {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return append(env, fixedGoEnv...)
}

// gitLine runs one git command and returns its output without the final
// newline.
func gitLine(ctx context.Context, repo git.Repo, args ...string) (string, error) {
	out, err := repo.Run(ctx, nil, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// inside reports whether path lies below dir, once the symbolic links
// of both are resolved.
func inside(dir, path string) bool {
	d, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(d, p)
	return err == nil && filepath.IsLocal(rel)
}
