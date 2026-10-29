// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package askfirst

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// PR is what pr reads of a pull request (REQ-0817): the seven fields of
// the pull request object. The rest comes from git.
type PR struct {
	Title, Body                     string
	HeadRef, HeadSHA                string
	BaseSHA, BaseRef, DefaultBranch string
}

// tasksDir is the directory of the task files of the plan, whose names
// start with the task id.
const tasksDir = "docs/plan/tasks"

// taskFile is the name of a task file under tasksDir: its id, then a
// slug. The plan numbers tasks with three digits (docs/plan.md, "Where
// the tasks are"); a fourth digit would need this pattern changed.
var taskFile = regexp.MustCompile(`^(T[0-9]{3})-[^/]*\.md$`)

// objectID is a full commit id, SHA-1 or SHA-256, as a payload carries
// it: a payload names commits by id, not by revision.
var objectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// localBase is the base of a local run with --title and --body when
// --base names none: the default branch as the last fetch saw it.
const localBase = "origin/main"

// ParsePayload reads a saved pull request event (REQ-0817): the pull
// request object, or the event whose pull_request member is that
// object. A file with neither shape, or without one of the seven fields,
// fails. GitHub writes a pull request with an empty body as "body":
// null, which reads as an empty body.
func ParsePayload(src []byte) (PR, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(src, &obj); err != nil {
		return PR{}, fmt.Errorf("the payload is not a JSON object: %w", err)
	}
	if inner, ok := obj["pull_request"]; ok {
		obj = nil
		if err := json.Unmarshal(inner, &obj); err != nil || obj == nil {
			return PR{}, errors.New("the payload's pull_request member is not an object")
		}
	}
	var pr PR
	var errs []error
	for _, f := range []struct {
		path     string
		to       *string
		nullable bool
	}{
		{"title", &pr.Title, false},
		{"body", &pr.Body, true},
		{"head.ref", &pr.HeadRef, false},
		{"head.sha", &pr.HeadSHA, false},
		{"base.sha", &pr.BaseSHA, false},
		{"base.ref", &pr.BaseRef, false},
		{"base.repo.default_branch", &pr.DefaultBranch, false},
	} {
		if err := field(obj, f.path, f.to, f.nullable); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return PR{}, fmt.Errorf("the payload is neither a pull request nor its event: %w", errors.Join(errs...))
	}
	for _, id := range []struct{ name, value string }{{"head.sha", pr.HeadSHA}, {"base.sha", pr.BaseSHA}} {
		if !objectID.MatchString(id.value) {
			return PR{}, fmt.Errorf("the payload's %s is not a full commit id: %q", id.name, id.value)
		}
	}
	return pr, nil
}

// field reads the string at the dotted path of obj into to. A null is
// read as "" when nullable is set, and is a missing field otherwise.
func field(obj map[string]json.RawMessage, path string, to *string, nullable bool) error {
	keys := strings.Split(path, ".")
	for _, k := range keys[:len(keys)-1] {
		raw, ok := obj[k]
		obj = nil
		if !ok || json.Unmarshal(raw, &obj) != nil || obj == nil {
			return fmt.Errorf("no %s", path)
		}
	}
	raw, ok := obj[keys[len(keys)-1]]
	if !ok {
		return fmt.Errorf("no %s", path)
	}
	if nullable && string(bytes.TrimSpace(raw)) == "null" {
		*to = ""
		return nil
	}
	var s *string
	if err := json.Unmarshal(raw, &s); err != nil || s == nil {
		return fmt.Errorf("%s is not a string", path)
	}
	*to = *s
	return nil
}

// RunPR is the pr subcommand (10 10.2, 12 12.4): it judges the approval
// lines of a pull request, read from a saved payload file, or, before the
// pull request exists, from --title and --body over the local range from
// --base to HEAD. It reads only local objects: a commit the clone lacks
// is an error, not a fetch (REQ-0816).
func RunPR(ctx context.Context, e check.Env, args []string) (bool, error) {
	pr, err := prInput(ctx, e.Repo(), args)
	if err != nil {
		return false, err
	}
	findings, err := judgePR(ctx, e.Repo(), pr)
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("pr", len(findings)))
	return len(findings) == 0, nil
}

// prInput reads the pull request from the arguments: one payload file,
// or --title, --body and an optional --base for a local run.
func prInput(ctx context.Context, r git.Repo, args []string) (PR, error) {
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "")
	bodyFile := fs.String("body", "", "")
	base := fs.String("base", localBase, "")
	if err := fs.Parse(args); err != nil {
		return PR{}, fmt.Errorf("pr: %w\n%s", err, check.Usage())
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case len(set) == 0 && fs.NArg() == 1:
		src, err := os.ReadFile(fs.Arg(0))
		if err != nil {
			return PR{}, err
		}
		return ParsePayload(src)
	case set["title"] && set["body"] && fs.NArg() == 0:
		body, err := os.ReadFile(*bodyFile)
		if err != nil {
			return PR{}, err
		}
		baseSHA, err := commit(ctx, r, *base)
		if err != nil {
			return PR{}, err
		}
		headSHA, err := commit(ctx, r, "HEAD")
		if err != nil {
			return PR{}, err
		}
		return PR{Title: *title, Body: string(body), BaseSHA: baseSHA, HeadSHA: headSHA}, nil
	}
	return PR{}, fmt.Errorf("pr takes a payload file, or --title and --body with an optional --base\n%s", check.Usage())
}

// judgePR reads from git what the approval rules need and judges the
// pull request: the walk from base.sha to head.sha finds a commit
// (REQ-0819), and the body carries the approval lines its diff needs.
func judgePR(ctx context.Context, r git.Repo, pr PR) ([]check.Finding, error) {
	base, err := commit(ctx, r, pr.BaseSHA)
	if err != nil {
		return nil, err
	}
	head, err := commit(ctx, r, pr.HeadSHA)
	if err != nil {
		return nil, err
	}
	out, err := read(ctx, r, "rev-list", "--max-count=1", base+".."+head)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return []check.Finding{{Where: "pull request", Rule: "pr", Msg: "the walk from base.sha to head.sha finds no commit"}}, nil
	}
	changed, err := changedPaths(ctx, r, base, head)
	if err != nil {
		return nil, err
	}
	baseList, err := readList(ctx, r, base)
	if err != nil {
		return nil, err
	}
	if baseList == nil {
		return nil, fmt.Errorf("%s is missing at the base commit %s", Path, base)
	}
	headList, err := readList(ctx, r, head)
	if err != nil {
		return nil, err
	}
	// The grants of the base decide; a base whose file does not parse
	// holds no grant, so only a line that names one fails. The head's
	// file is checked, so a pull request cannot merge a file that breaks.
	grants, err := readGrants(ctx, r, base)
	if err != nil {
		grants = nil
	}
	var findings []check.Finding
	if _, err := readGrants(ctx, r, head); err != nil {
		findings = append(findings, check.Finding{Where: GrantsPath, Rule: "pr", Msg: "at the head: " + err.Error()})
	}
	rv := review{body: pr.Body, base: baseList, head: headList, changed: changed, grants: grants}
	for _, msg := range rv.findings() {
		findings = append(findings, check.Finding{Where: "pull request body", Rule: "pr", Msg: msg})
	}
	return findings, nil
}

// read runs one git command that reads objects of the pull request,
// with hooks off (REQ-1152); the head is read as objects and never
// checked out.
func read(ctx context.Context, r git.Repo, args ...string) ([]byte, error) {
	return r.Run(ctx, nil, append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
}

// commit returns the full id of the commit rev names, or an error that
// says how to fetch it when the clone lacks it.
func commit(ctx context.Context, r git.Repo, rev string) (string, error) {
	out, err := read(ctx, r, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("%s names no commit of this clone; fetch it, or %s", git.Printable(rev), git.FetchHint(rev))
	}
	return strings.TrimSpace(string(out)), nil
}

// changedPaths returns the paths that differ between the merge base of
// base and head, and head, both names of a rename included (REQ-0818):
// a merge of the default branch into the head adds none. A criss-cross
// history has more than one best merge base; the paths of each are
// joined, so a path that differs from any of them counts.
func changedPaths(ctx context.Context, r git.Repo, base, head string) ([]string, error) {
	out, err := read(ctx, r, "merge-base", "--all", base, head)
	if err != nil {
		return nil, fmt.Errorf("base.sha and head.sha have no merge base in this clone (a shallow clone needs a deeper fetch): %w", err)
	}
	var paths []string
	for mb := range strings.FieldsSeq(string(out)) {
		p, err := diffPaths(ctx, r, mb, head)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p...)
	}
	return paths, nil
}

// diffPaths returns the paths of the tree-to-tree diff from one to two.
// It passes no --ignore-submodules: that option drops a gitlink entry
// from the list (git 2.53.0, measured on 2026-10-29), and a diff of two
// trees opens no submodule.
func diffPaths(ctx context.Context, r git.Repo, one, two string) ([]string, error) {
	out, err := read(ctx, r, "diff", "--name-status", "-z", "-M", "--no-ext-diff", "--no-textconv", one, two)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	// Each entry is a status, then one path, or two for a rename or a
	// copy, each ended by NUL.
	var paths []string
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i < len(fields); {
		n := 1
		if strings.HasPrefix(fields[i], "R") || strings.HasPrefix(fields[i], "C") {
			n = 2
		}
		if i+n >= len(fields) {
			return nil, fmt.Errorf("git diff: the entry %q comes without its paths", fields[i])
		}
		paths = append(paths, fields[i+1:i+1+n]...)
		i += 1 + n
	}
	return paths, nil
}

// blob returns the content of path at commit, or nil when the commit's
// tree has no such file.
func blob(ctx context.Context, r git.Repo, commit, path string) ([]byte, error) {
	out, err := read(ctx, r, "ls-tree", "--name-only", commit, "--", path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}
	return read(ctx, r, "cat-file", "blob", commit+":"+path)
}

// readList reads the ask-first list at commit with Parse, the one
// reader the CODEOWNERS generator uses too; nil when the commit has
// none.
func readList(ctx context.Context, r git.Repo, commit string) (*List, error) {
	src, err := blob(ctx, r, commit, Path)
	if err != nil || src == nil {
		return nil, err
	}
	return Parse(src)
}

// readGrants reads the grants at commit with ParseGrants, against the
// task ids of the plan at that commit; none when the commit has no grant
// file.
func readGrants(ctx context.Context, r git.Repo, commit string) (*Grants, error) {
	src, err := blob(ctx, r, commit, GrantsPath)
	if err != nil || src == nil {
		return nil, err
	}
	out, err := read(ctx, r, "ls-tree", "--name-only", "-z", commit+":"+tasksDir)
	if err != nil {
		return nil, err
	}
	var tasks []string
	for name := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if m := taskFile.FindStringSubmatch(name); m != nil {
			tasks = append(tasks, m[1])
		}
	}
	return ParseGrants(src, tasks)
}
