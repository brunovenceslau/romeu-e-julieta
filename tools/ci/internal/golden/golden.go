// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package golden is the golden governance check of 10 10.2 and 10 10.4
// (REQ-0835, REQ-0941, REQ-0942): a golden file changes only through
// the -update flag of its test, run locally, so no CI invocation passes
// that flag, and every golden a change touches is listed for review, in
// the job summary on a runner.
//
// A golden, for this check, is a file under a directory the Golden row
// of the test levels table of 10 10.1 names; the check reads that row
// from the spec at the commit, so the list is written once.
package golden

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Name is the name of the check.
const Name = "golden"

// SpecPath is the spec file whose test levels table holds the Golden
// row (10 10.1).
const SpecPath = "docs/spec/10-testing-style.md"

// workflowsDir holds the workflows, the invocations of CI.
const workflowsDir = ".github/workflows/"

// toolsCIDir holds tools/ci, whose steps are the commands CI runs.
const toolsCIDir = "tools/ci/"

// updateWord finds the flag in a workflow line: -update or --update,
// alone or with a value, as a word of a command.
var updateWord = regexp.MustCompile(`(?:^|[\s'"])--?update(?:[=\s'"]|$)`)

// updateLiteral finds the flag as a Go string literal in the source of
// tools/ci, an argument of a step it runs.
var updateLiteral = regexp.MustCompile(`"--?update(?:=[^"]*)?"`)

// bases are the refs whose merge base with the commit the changed
// goldens are listed against, the first that resolves.
var bases = []string{"refs/remotes/origin/HEAD", "refs/remotes/origin/main"}

// Run is "tools/ci golden": the check over the commit at HEAD.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("golden takes no argument\n%s", check.Usage())
	}
	repo := e.Repo()
	id, err := check.CommitOf(ctx, repo, "HEAD")
	if err != nil {
		return false, err
	}
	files, err := check.HeadTree(ctx, repo, id)
	if err != nil {
		return false, err
	}
	vs, err := Judge(ctx, check.Commit{Repo: repo, ID: id, Files: files})
	if err != nil {
		return false, err
	}
	v := vs[0]
	if v.Err != nil {
		return false, v.Err
	}
	check.Say(e.Stdout, "%s%s%s\n", check.Lines(v.Findings), v.Info, check.Summary(Name, len(v.Findings)))
	return len(v.Findings) == 0, nil
}

// Judge is the golden check of the commit fast and all read before any
// step: a finding for each CI invocation that passes -update, and, as
// Info, the goldens the change touches. On a runner the list is also
// written to the job summary.
func Judge(ctx context.Context, c check.Commit) ([]check.Verdict, error) {
	v := check.Verdict{Name: Name, Findings: UpdateFindings(c.Files)}
	if _, found, _ := check.FileIn(c.Files, SpecPath); !found {
		// A tree without the spec, a fixture of another check, has no
		// golden to list; the invocations are judged all the same.
		v.Info = Info(nil, "the changed goldens are not listed: "+SpecPath+" is not in the tree")
		return []check.Verdict{v}, nil
	}
	dirs, err := Dirs(c.Files)
	if err != nil {
		v.Err = err
		return []check.Verdict{v}, nil
	}
	changed, note, err := Changed(ctx, c.Repo, c.ID, dirs)
	if err != nil {
		v.Err = err
		return []check.Verdict{v}, nil
	}
	v.Info = Info(changed, note)
	if err := writeSummary(os.Getenv("GITHUB_STEP_SUMMARY"), changed, note); err != nil {
		v.Err = err
	}
	return []check.Verdict{v}, nil
}

// Dirs returns the directories the Golden row of the test levels table
// names (10 10.1), each with its final slash, from the spec in files.
func Dirs(files []check.TreeFile) ([]string, error) {
	data, found, err := check.FileIn(files, SpecPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("golden: %s is not in the tree", SpecPath)
	}
	for line := range strings.Lines(string(data)) {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.TrimSpace(cells[1]) != "Golden" {
			continue
		}
		var dirs []string
		for _, m := range regexp.MustCompile("`([^`]+/)`").FindAllStringSubmatch(cells[2], -1) {
			dirs = append(dirs, m[1])
		}
		if len(dirs) == 0 {
			return nil, fmt.Errorf("golden: the Golden row of %s names no directory", SpecPath)
		}
		return dirs, nil
	}
	return nil, fmt.Errorf("golden: %s has no Golden row", SpecPath)
}

// UpdateFindings returns a finding for each line of a workflow, and each
// string literal of the source of tools/ci outside its tests, that
// passes -update: CI never rewrites a golden (REQ-0942).
func UpdateFindings(files []check.TreeFile) []check.Finding {
	var out []check.Finding
	for _, f := range files {
		var re *regexp.Regexp
		switch {
		case strings.HasPrefix(f.Path, workflowsDir) && (strings.HasSuffix(f.Path, ".yml") || strings.HasSuffix(f.Path, ".yaml")):
			re = updateWord
		case strings.HasPrefix(f.Path, toolsCIDir) && strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, "_test.go"):
			re = updateLiteral
		default:
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(f.Content))
		sc.Buffer(nil, len(f.Content)+1)
		for n := 1; sc.Scan(); n++ {
			if re.Match(sc.Bytes()) {
				out = append(out, check.Finding{
					Where: f.Path + ":" + strconv.Itoa(n),
					Rule:  "update-in-ci",
					Msg:   "a CI invocation passes the flag that rewrites the goldens; they change only locally (REQ-0942)",
				})
			}
		}
	}
	return out
}

// Change is one golden a change touches: git's status letter and the
// path.
type Change struct {
	Status string
	Path   string
}

// Changed returns the files under dirs that differ between the commit
// and its base: the merge base with the remote's default branch, or
// else the commit's parent. With neither in the clone, a shallow one,
// it returns a note that says so and lists nothing.
func Changed(ctx context.Context, repo git.Repo, commit string, dirs []string) ([]Change, string, error) {
	base := ""
	for _, ref := range bases {
		out, err := repo.Run(ctx, nil, "merge-base", "--end-of-options", ref, commit)
		if err == nil {
			base = strings.TrimSpace(string(out))
			break
		}
	}
	if base == "" {
		out, err := repo.Run(ctx, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", commit+"^1^{commit}")
		if err != nil {
			return nil, "the changed goldens are not listed: this clone holds neither a merge base with origin nor the parent of the commit", nil
		}
		base = strings.TrimSpace(string(out))
	}
	args := []string{"diff", "--name-status", "-z", "--no-renames", base, commit, "--"}
	for _, d := range dirs {
		args = append(args, ":(top,literal)"+d)
	}
	out, err := repo.Run(ctx, nil, args...)
	if err != nil {
		return nil, "", err
	}
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(fields) == 1 && fields[0] == "" {
		return nil, "", nil
	}
	if len(fields)%2 != 0 {
		return nil, "", errors.New("golden: git diff wrote an odd number of fields")
	}
	var changes []Change
	for i := 0; i < len(fields); i += 2 {
		changes = append(changes, Change{Status: fields[i], Path: fields[i+1]})
	}
	return changes, "", nil
}

// Info returns the lines that list the changed goldens, or the note.
func Info(changes []Change, note string) string {
	if note != "" {
		return note + "\n"
	}
	if len(changes) == 0 {
		return "no golden changed\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d changed golden(s), each to be listed under Evidence and reviewed:\n", len(changes))
	for _, c := range changes {
		fmt.Fprintf(&b, "  %s %s\n", c.Status, git.Printable(c.Path))
	}
	return b.String()
}

// writeSummary appends the list of changed goldens to the job summary
// at path, the file GITHUB_STEP_SUMMARY names; an empty path, off a
// runner, writes nothing.
func writeSummary(path string, changes []Change, note string) error {
	if path == "" {
		return nil
	}
	var b strings.Builder
	b.WriteString("### Changed goldens\n\n")
	switch {
	case note != "":
		b.WriteString(git.SafeLines(note) + "\n")
	case len(changes) == 0:
		b.WriteString("None.\n")
	default:
		for _, c := range changes {
			fmt.Fprintf(&b, "- %s `%s`\n", c.Status, strings.ReplaceAll(git.SafeLines(git.Printable(c.Path)), "`", "'"))
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("golden: the job summary: %w", err)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		return fmt.Errorf("golden: the job summary: %w", err)
	}
	return f.Close()
}
