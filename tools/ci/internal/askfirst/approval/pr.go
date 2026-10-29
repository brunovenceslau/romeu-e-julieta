// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
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
	var obj map[string]jsontext.Value
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
func field(obj map[string]jsontext.Value, path string, to *string, nullable bool) error {
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

// ReadArgs reads the pull request of a local run of pr (10 10.2, 12
// 12.4) from its arguments: one saved payload file, or, before the pull
// request exists, --title and --body over the local range from --base to
// HEAD. usage is the text a wrong set of arguments is answered with.
func ReadArgs(ctx context.Context, r git.Repo, args []string, usage string) (PR, error) {
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "")
	bodyFile := fs.String("body", "", "")
	base := fs.String("base", localBase, "")
	if err := fs.Parse(args); err != nil {
		return PR{}, fmt.Errorf("pr: %w\n%s", err, usage)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case len(set) == 0 && fs.NArg() == 1:
		return ReadPayload(fs.Arg(0))
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
	return PR{}, fmt.Errorf("pr takes a payload file, or --title and --body with an optional --base\n%s", usage)
}

// ReadPayload reads a saved payload file with ParsePayload.
func ReadPayload(name string) (PR, error) {
	src, err := os.ReadFile(name)
	if err != nil {
		return PR{}, err
	}
	return ParsePayload(src)
}

// Judge reads from git what the approval rules need and judges the
// pull request: the walk from base.sha to head.sha finds a commit
// (REQ-0819), and the body carries the approval lines its diff needs.
// With fetch set, which only the pr job sets (cmd/pr), a head.sha the
// clone lacks is fetched first (fetchHead); without it, a missing
// commit is an error, so a local run reads no network (REQ-0816).
func Judge(ctx context.Context, r git.Repo, pr PR, fetch bool) ([]Finding, error) {
	base, err := commit(ctx, r, pr.BaseSHA)
	if err != nil {
		return nil, err
	}
	if fetch {
		if err := fetchHead(ctx, r, pr.HeadSHA); err != nil {
			return nil, err
		}
	}
	head, err := commit(ctx, r, pr.HeadSHA)
	if err != nil {
		return nil, err
	}
	if base != pr.BaseSHA || head != pr.HeadSHA {
		return nil, fmt.Errorf("base.sha and head.sha name the commits %s and %s, and not themselves", base, head)
	}
	out, err := read(ctx, r, "rev-list", "--max-count=1", base+".."+head)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return []Finding{{Where: "pull request", Msg: "the walk from base.sha to head.sha finds no commit"}}, nil
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
	var findings []Finding
	if _, err := readGrants(ctx, r, head); err != nil {
		findings = append(findings, Finding{Where: GrantsPath, Msg: "at the head: " + err.Error()})
	}
	rv := review{body: pr.Body, base: baseList, head: headList, changed: changed, grants: grants}
	return append(findings, rv.findings()...), nil
}

// read runs one git command on objects of the pull request, with hooks
// off (REQ-1152); the head is read as objects and never checked out.
// Every git command of pr goes through it, and so through runGit, which
// the test of the hostile repository records (TestPRHostileRepository).
func read(ctx context.Context, r git.Repo, args ...string) ([]byte, error) {
	return runGit(ctx, r, append([]string{"-c", "core.hooksPath=/dev/null"}, args...))
}

// runGit runs one git command. It is a variable so that a test can see
// each argument list pr passes.
var runGit = func(ctx context.Context, r git.Repo, args []string) ([]byte, error) {
	return r.Run(ctx, nil, args...)
}

// fetchRemote is the remote the pr job fetches a missing head from: the
// one actions/checkout names, the repository of the default branch,
// which serves the head of a pull request from a fork too.
const fetchRemote = "origin"

// noCredentials are the settings the fetch of the head runs with, so it
// sends no credential whatever the configuration holds (REQ-1152): no
// credential helper and no askpass program (an empty value resets each
// list), no extra header, which is where actions/checkout keeps the token
// when it persists it, and no redirect, so the fetch reads only the URL of
// the remote.
var noCredentials = []string{
	"-c", "credential.helper=",
	"-c", "core.askPass=",
	"-c", "http.extraHeader=",
	"-c", "http.followRedirects=false",
}

// fetchHead fetches the commit sha from fetchRemote when the clone lacks
// it (REQ-1152): without credentials, without submodules, tags, a
// FETCH_HEAD or the maintenance a fetch starts, and only that commit and
// the history it needs. The repository is public (10 10.2), so the read
// needs no token. A commit the clone holds is not fetched, so a run on
// such a clone reads no network.
func fetchHead(ctx context.Context, r git.Repo, sha string) error {
	if _, err := commit(ctx, r, sha); err == nil {
		return nil
	}
	r.Env = append(r.Environ(), "GIT_TERMINAL_PROMPT=0")
	args := slices.Concat(noCredentials, []string{"fetch", "--no-recurse-submodules", "--no-tags",
		"--no-write-fetch-head", "--no-auto-maintenance", "--quiet", "--end-of-options", fetchRemote, sha})
	if _, err := read(ctx, r, args...); err != nil {
		return fmt.Errorf("fetching head.sha %s: %w", sha, err)
	}
	return nil
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
