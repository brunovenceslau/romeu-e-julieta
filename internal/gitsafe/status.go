// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// statusEnv is what the reads of a working tree add to the environment
// of the mode: that of History, and GIT_OPTIONAL_LOCKS=0, so no read
// refreshes the index on disk or takes index.lock (10 10.2 runs git
// status the same way).
var statusEnv = slices.Concat(historyEnv, []string{"GIT_OPTIONAL_LOCKS=0"})

// The bounds of the reads of a working tree, each far above what a
// sandbox holds: the local branches, the stashes and the worktrees of
// one repository, and the bytes of one record of git's output. They
// are variables so the tests can lower them.
var (
	maxBranches     = 10_000
	maxStashes      = 10_000
	maxWorktrees    = 1_000
	maxStatusRecord = 64 << 10
)

// The bounds of the short answers: a one-line answer of git, such as a
// ref name or two counts, and the value of core.hooksPath.
const (
	maxLineBytes      = 4 << 10
	maxHooksPathBytes = 8 << 10
)

// read runs one read of a working tree in dir: the settings of History,
// statusEnv and env added to the environment, and stdout as git's
// standard output.
func (r *Runner) read(ctx context.Context, dir string, env []string, stdout io.Writer, args ...string) error {
	return r.runInput(ctx, dir, historySettings, slices.Concat(statusEnv, env), nil, stdout, args...)
}

// readLine runs a read whose answer is one short line and returns it
// without its newline.
func (r *Runner) readLine(ctx context.Context, dir string, args ...string) (string, error) {
	out := limitedBuffer{limit: maxLineBytes}
	if err := r.read(ctx, dir, nil, &out, args...); err != nil {
		return "", err
	}
	if out.truncated {
		return "", fmt.Errorf("%w: git %s wrote more than %d bytes", ErrRefused, args[0], out.limit)
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// exitedWith reports whether err is that of a git that exited with
// code, which some reads use to answer "none".
func exitedWith(err error, code int) bool {
	e, ok := errors.AsType[*procgroup.ExitError](err)
	return ok && e.Code == code
}

// Status is the state of HEAD and of the working tree of a repository.
type Status struct {
	// Branch is the full ref name HEAD points at, under refs/heads/
	// for a branch, born or unborn; empty when HEAD is detached.
	Branch string
	// OID is the commit HEAD names; empty on an unborn branch.
	OID string
	// Staged: the index differs from HEAD.
	Staged bool
	// Unstaged: a tracked file of the working tree differs from the
	// index.
	Unstaged bool
	// Untracked: the working tree holds a file that is neither tracked
	// nor ignored.
	Untracked bool
	// Unmerged: the index holds a conflict.
	Unmerged bool
	// Hidden: an index entry is marked assume-unchanged or
	// skip-worktree, so git status does not compare it with the
	// working tree and a change to it would not show. A sparse
	// checkout marks every file outside its cone skip-worktree.
	Hidden bool
}

// Clean reports whether the working tree holds no change: nothing
// staged, unstaged, untracked, unmerged or hidden. It does not cover
// ignored files, a mode change git does not see under
// core.fileMode=false, or the working tree of a submodule
// (--ignore-submodules=dirty).
func (s Status) Clean() bool {
	return !s.Staged && !s.Unstaged && !s.Untracked && !s.Unmerged && !s.Hidden
}

// Status reads HEAD and the state of the working tree of the repository
// in dir, under the flags of the mode, with four git commands: git
// config lists the filter drivers (below), git symbolic-ref reads HEAD,
// git status --porcelain=v2 reads the changes, and git ls-files -v
// reads the assume-unchanged and skip-worktree flags. A submodule
// counts by the commit it records, not by the state of its own working
// tree, so git never runs in it. HEAD that moves between symbolic-ref
// and status, so the two disagree on the branch, is an error.
//
// No clean filter runs: git status hashes a file whose stat data does
// not match the index, and a racy one, through the filter its
// attributes name, and the attributes come from the tree, from
// .git/info/attributes and from the config, so they cannot all be
// switched off. Status instead lists the filter drivers the config
// defines (git config --get-regexp, one more call) and runs git status
// with each driver's clean and process commands empty and required
// false, which git reads as no filter. A file a filter would change,
// such as one under git-lfs, then reads as Unstaged when its stat data
// no longer matches the index (a changed modification time is enough),
// until something refreshes the index; Status may so report a change
// git status would not. A driver defined between the calls is not
// neutralized; only a process that can already write the repository's
// config can do that.
func (r *Runner) Status(ctx context.Context, dir string) (Status, error) {
	if err := checkRepo(dir); err != nil {
		return Status{}, err
	}
	noFilters, err := r.filterOverrides(ctx, dir)
	if err != nil {
		return Status{}, err
	}
	var s Status
	branch, err := r.readLine(ctx, dir, "symbolic-ref", "-q", "HEAD")
	switch {
	case exitedWith(err, 1):
		// symbolic-ref -q exits 1, silently, on a detached HEAD.
	case err != nil:
		return Status{}, err
	default:
		if !strings.HasPrefix(branch, "refs/heads/") {
			return Status{}, fmt.Errorf("%w: HEAD points at %q, outside refs/heads/", ErrRefused, branch)
		}
		if err := checkDisplayRef(branch); err != nil {
			return Status{}, err
		}
		s.Branch = branch
	}
	p := &porcelain{status: &s}
	w := &records{limit: maxStatusRecord, each: p.entry}
	if err := r.read(ctx, dir, noFilters, w, "status", "--porcelain=v2", "-z", "--branch", "--no-ahead-behind",
		"--untracked-files=normal", "--ignore-submodules=dirty", "--no-renames"); err != nil {
		return Status{}, err
	}
	if err := w.close(); err != nil {
		return Status{}, err
	}
	if !p.sawOID {
		return Status{}, fmt.Errorf("%w: git status named no branch.oid", ErrRefused)
	}
	head := "(detached)"
	if s.Branch != "" {
		head = strings.TrimPrefix(s.Branch, "refs/heads/")
	}
	if p.head != head {
		return Status{}, fmt.Errorf("%w: HEAD moved during the read: symbolic-ref named %q, status %q", ErrRefused, s.Branch, p.head)
	}
	flags := &records{limit: maxStatusRecord, each: func(rec string) error {
		if rec == "" {
			return fmt.Errorf("%w: an empty ls-files record", ErrRefused)
		}
		if tag := rec[0]; tag == 'S' || (tag >= 'a' && tag <= 'z') {
			s.Hidden = true
		}
		return nil
	}}
	if err := r.read(ctx, dir, nil, flags, "ls-files", "-v", "-z"); err != nil {
		return Status{}, err
	}
	if err := flags.close(); err != nil {
		return Status{}, err
	}
	return s, nil
}

// checkDisplayRef returns ErrRefused for a ref name that checkRefName
// refuses, or that holds a character unsafeRune refuses: a ref name
// that reaches a typed result is shown to a person.
func checkDisplayRef(name string) error {
	if err := checkRefName(name, false); err != nil {
		return err
	}
	if i := strings.IndexFunc(name, unsafeRune); i >= 0 {
		c, _ := utf8.DecodeRuneInString(name[i:])
		return fmt.Errorf("%w: ref name %q holds the character %U", ErrRefused, name, c)
	}
	return nil
}

// filterOverrides returns the GIT_CONFIG_COUNT environment that empties
// the clean and process commands of every filter driver the config of
// the repository in dir defines, and makes none required; nil when it
// defines none.
func (r *Runner) filterOverrides(ctx context.Context, dir string) ([]string, error) {
	out := limitedBuffer{limit: outputLimit}
	err := r.read(ctx, dir, nil, &out, "config", "-z", "--name-only", "--get-regexp", `^filter\.`)
	switch {
	case exitedWith(err, 1):
		return nil, nil // no key matched
	case err != nil:
		return nil, err
	case out.truncated:
		return nil, fmt.Errorf("%w: the filter keys of the config are longer than %d bytes", ErrRefused, outputLimit)
	}
	var names []string
	for key := range strings.SplitSeq(strings.TrimSuffix(out.String(), "\x00"), "\x00") {
		rest, _ := strings.CutPrefix(key, "filter.")
		if i := strings.LastIndexByte(rest, '.'); i > 0 && !slices.Contains(names, rest[:i]) {
			names = append(names, rest[:i])
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	env := []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(3*len(names))}
	for i, name := range names {
		for j, kv := range [][2]string{{"clean", ""}, {"process", ""}, {"required", "false"}} {
			n := strconv.Itoa(3*i + j)
			env = append(env, "GIT_CONFIG_KEY_"+n+"=filter."+name+"."+kv[0], "GIT_CONFIG_VALUE_"+n+"="+kv[1])
		}
	}
	return env, nil
}

// porcelain reads the records of git status --porcelain=v2 -z --branch
// --no-renames into a Status.
type porcelain struct {
	status *Status
	sawOID bool
	head   string // the branch.head header
}

// entry reads one record: a header, an ordinary change, a conflict or
// an untracked path.
func (p *porcelain) entry(rec string) error {
	switch {
	case strings.HasPrefix(rec, "# branch.oid "):
		oid := strings.TrimPrefix(rec, "# branch.oid ")
		p.sawOID = true
		if oid == "(initial)" {
			return nil
		}
		if err := checkObjectID(oid); err != nil {
			return err
		}
		p.status.OID = oid
	case strings.HasPrefix(rec, "# branch.head "):
		p.head = strings.TrimPrefix(rec, "# branch.head ")
	case strings.HasPrefix(rec, "# "):
		// Any header a later git adds.
	case strings.HasPrefix(rec, "1 ") && len(rec) > 4:
		p.status.Staged = p.status.Staged || rec[2] != '.'
		p.status.Unstaged = p.status.Unstaged || rec[3] != '.'
	case strings.HasPrefix(rec, "u "):
		p.status.Unmerged = true
	case strings.HasPrefix(rec, "? "):
		p.status.Untracked = true
	default:
		return fmt.Errorf("%w: git status record %q", ErrRefused, rec)
	}
	return nil
}

// Branch is one local branch.
type Branch struct {
	// Name is the full ref name, under refs/heads/.
	Name string
	// OID is the commit it points at.
	OID string
	// Upstream is the full ref name of its upstream, as the config sets
	// it, whether or not that ref exists; empty when it has none.
	Upstream string
	// Current: HEAD points at it.
	Current bool
}

// Branches lists the local branches of the repository in dir, sorted
// by name, under the flags of the mode, with git for-each-ref. A name
// or an upstream checkDisplayRef refuses is refused, and fails the
// whole list, and so are more than maxBranches branches.
func (r *Runner) Branches(ctx context.Context, dir string) ([]Branch, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	var branches []Branch
	w := &records{limit: maxStatusRecord, each: func(line string) error {
		if len(branches) == maxBranches {
			return fmt.Errorf("%w: more than %d branches", ErrRefused, maxBranches)
		}
		b, err := parseBranch(line)
		if err != nil {
			return err
		}
		branches = append(branches, b)
		return nil
	}}
	if err := r.read(ctx, dir, nil, lines{w}, "for-each-ref", "--format=%(HEAD)%(objectname) %(refname) %(upstream)", "refs/heads/"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return branches, nil
}

// parseBranch reads one "<HEAD mark><oid> <ref> <upstream>" line; the
// mark is * for the current branch and a space for the others.
func parseBranch(line string) (Branch, error) {
	if line == "" || (line[0] != '*' && line[0] != ' ') {
		return Branch{}, fmt.Errorf("%w: for-each-ref line %q", ErrRefused, line)
	}
	fields := strings.Split(line[1:], " ")
	if len(fields) != 3 {
		return Branch{}, fmt.Errorf("%w: for-each-ref line %q", ErrRefused, line)
	}
	b := Branch{Current: line[0] == '*', OID: fields[0], Name: fields[1], Upstream: fields[2]}
	if err := checkObjectID(b.OID); err != nil {
		return Branch{}, err
	}
	if !strings.HasPrefix(b.Name, "refs/heads/") {
		return Branch{}, fmt.Errorf("%w: branch %q is not under refs/heads/", ErrRefused, b.Name)
	}
	if err := checkDisplayRef(b.Name); err != nil {
		return Branch{}, err
	}
	if b.Upstream != "" {
		if err := checkDisplayRef(b.Upstream); err != nil {
			return Branch{}, err
		}
	}
	return b, nil
}

// lines passes git's output on with each newline turned into a NUL, so
// records splits it into lines. None of the outputs read this way holds
// a NUL; one that did would fail to parse.
type lines struct{ w io.Writer }

// Write passes p on.
func (l lines) Write(p []byte) (int, error) {
	if _, err := l.w.Write(bytes.ReplaceAll(p, []byte{'\n'}, []byte{0})); err != nil {
		return 0, err
	}
	return len(p), nil
}

// AheadBehind counts the commits ref has that base does not (ahead) and
// those base has that ref does not (behind), in the repository in dir,
// under the flags of the mode, with git rev-list --left-right --count.
// ref and base are each a full object id, HEAD or a full ref name: a
// branch and its upstream, or a local default branch and
// refs/remotes/origin/<default>. A ref that does not exist, such as the
// upstream of a branch that has none, is git's error, never a count of
// zero.
func (r *Runner) AheadBehind(ctx context.Context, dir, ref, base string) (ahead, behind int, err error) {
	if err := checkRepo(dir); err != nil {
		return 0, 0, err
	}
	for _, rev := range []string{ref, base} {
		if err := checkRev(rev); err != nil {
			return 0, 0, err
		}
	}
	out, err := r.readLine(ctx, dir, "rev-list", "--left-right", "--count", "--end-of-options", ref+"..."+base, "--")
	if err != nil {
		return 0, 0, err
	}
	left, right, ok := strings.Cut(out, "\t")
	ahead, errA := strconv.Atoi(left)
	behind, errB := strconv.Atoi(right)
	if !ok || errA != nil || errB != nil || ahead < 0 || behind < 0 {
		return 0, 0, fmt.Errorf("%w: rev-list answered %q", ErrRefused, out)
	}
	return ahead, behind, nil
}

// originHead is the ref the default branch of origin is read from.
const originHead = "refs/remotes/origin/HEAD"

// DefaultBranch returns the name of the default branch of origin in the
// repository in dir, such as main: the branch refs/remotes/origin/HEAD
// points at, as git clone and git remote set-head write it, under the
// flags of the mode, with git for-each-ref. It returns "" when that ref
// does not exist, is not a symbolic ref, or points at a ref that does
// not exist (a dangling origin/HEAD): there is then no default branch
// to fast-forward, and the caller reports that. A target outside
// refs/remotes/origin/, or one checkDisplayRef refuses, is refused.
func (r *Runner) DefaultBranch(ctx context.Context, dir string) (string, error) {
	if err := checkRepo(dir); err != nil {
		return "", err
	}
	var target string
	w := &records{limit: maxStatusRecord, each: func(line string) error {
		name, symref, ok := strings.Cut(line, " ")
		if !ok {
			return fmt.Errorf("%w: for-each-ref line %q", ErrRefused, line)
		}
		if name == originHead {
			target = symref
		}
		return nil
	}}
	if err := r.read(ctx, dir, nil, lines{w}, "for-each-ref", "--format=%(refname) %(symref)", originHead); err != nil {
		return "", err
	}
	if err := w.close(); err != nil {
		return "", err
	}
	if target == "" {
		return "", nil
	}
	name, ok := strings.CutPrefix(target, "refs/remotes/origin/")
	if err := checkDisplayRef(target); err != nil {
		return "", err
	}
	if !ok || name == "HEAD" {
		return "", fmt.Errorf("%w: %s points at %q", ErrRefused, originHead, target)
	}
	return name, nil
}

// Stashes lists the stashes of the repository in dir, newest first, so
// the one at index i is stash@{i}, under the flags of the mode, with
// git stash list. Each is a Commit whose Subject is the stash's
// message. The message is free text, often a copy of a commit subject,
// so it is not refused: it is escaped with termsafe.Escape, which also
// replaces bytes that are not UTF-8, and cut to maxSubjectBytes. The
// id and the time are checked as History checks them; a record over
// maxStatusRecord bytes, or more than maxStashes stashes, is refused.
func (r *Runner) Stashes(ctx context.Context, dir string) ([]Commit, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	var stashes []Commit
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		if len(stashes) == maxStashes {
			return fmt.Errorf("%w: more than %d stashes", ErrRefused, maxStashes)
		}
		c, err := parseStash(rec)
		if err != nil {
			return err
		}
		stashes = append(stashes, c)
		return nil
	}}
	if err := r.read(ctx, dir, nil, w, "stash", "list", "-z", "--format=%H %ct %gs"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return stashes, nil
}

// parseStash reads one "<oid> <committer time> <message>" record.
func parseStash(rec string) (Commit, error) {
	oid, rest, ok1 := strings.Cut(rec, " ")
	secs, msg, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 {
		return Commit{}, fmt.Errorf("%w: stash record %q", ErrRefused, termsafe.Escape(rec))
	}
	if err := checkObjectID(oid); err != nil {
		return Commit{}, err
	}
	n, err := strconv.ParseInt(secs, 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("%w: stash %s has the date %q", ErrRefused, oid, secs)
	}
	msg = termsafe.Escape(msg)
	if len(msg) > maxSubjectBytes {
		cut := maxSubjectBytes
		for cut > 0 && !utf8.RuneStart(msg[cut]) {
			cut--
		}
		msg = msg[:cut]
	}
	return Commit{OID: oid, Time: time.Unix(n, 0).UTC(), Subject: msg}, nil
}

// RefOID returns the object id the full ref name ref points at in the
// repository in dir, and whether it exists, under the flags of the
// mode, with git for-each-ref: such as the tip of
// refs/remotes/origin/<default> before and after a fast-forward. A
// missing ref gives false and no error; a name checkDisplayRef refuses
// is refused before git starts.
func (r *Runner) RefOID(ctx context.Context, dir, ref string) (string, bool, error) {
	if err := checkRepo(dir); err != nil {
		return "", false, err
	}
	if err := checkDisplayRef(ref); err != nil {
		return "", false, err
	}
	var oid string
	w := &records{limit: maxStatusRecord, each: func(line string) error {
		name, id, ok := strings.Cut(line, " ")
		if !ok {
			return fmt.Errorf("%w: for-each-ref line %q", ErrRefused, line)
		}
		if name != ref {
			return nil // a ref below ref, which then does not exist
		}
		if err := checkObjectID(id); err != nil {
			return err
		}
		oid = id
		return nil
	}}
	// A ref and a ref below it cannot both exist, so the first match
	// is either ref itself or proof that it is missing.
	if err := r.read(ctx, dir, nil, lines{w}, "for-each-ref", "--count=1", "--format=%(refname) %(objectname)", "--end-of-options", ref); err != nil {
		return "", false, err
	}
	if err := w.close(); err != nil {
		return "", false, err
	}
	return oid, oid != "", nil
}

// Worktree is one worktree of a repository.
type Worktree struct {
	// Path is the absolute path of its root, as git records it.
	Path string
	// OID is the commit its HEAD names; empty for a bare repository
	// and on an unborn branch.
	OID string
	// Branch is the full ref name its HEAD points at; empty when it is
	// detached or bare.
	Branch string
	// Bare, Detached, Locked and Prunable are git's flags: a prunable
	// worktree is one whose directory is gone.
	Bare, Detached, Locked, Prunable bool
}

// zeroOID is the id git worktree list gives the HEAD of an unborn
// branch, in either hash.
func zeroOID(oid string) bool {
	return strings.Trim(oid, "0") == ""
}

// Worktrees lists the worktrees of the repository in dir, the main one
// first, under the flags of the mode, with git worktree list
// --porcelain -z. A branch checkDisplayRef refuses fails the whole
// list, and so does a path that is not absolute, not valid UTF-8 or
// that holds a character unsafeRune refuses, and so do more than
// maxWorktrees worktrees. The reasons git gives for a lock or a prune
// are dropped.
func (r *Runner) Worktrees(ctx context.Context, dir string) ([]Worktree, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	var worktrees []Worktree
	open := false
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		if rec == "" {
			open = false
			return nil
		}
		key, value, _ := strings.Cut(rec, " ")
		if key == "worktree" {
			if open {
				return fmt.Errorf("%w: worktree record %q inside another", ErrRefused, rec)
			}
			if len(worktrees) == maxWorktrees {
				return fmt.Errorf("%w: more than %d worktrees", ErrRefused, maxWorktrees)
			}
			if err := checkWorktreePath(value); err != nil {
				return err
			}
			worktrees = append(worktrees, Worktree{Path: value})
			open = true
			return nil
		}
		if !open {
			return fmt.Errorf("%w: worktree attribute %q outside a worktree", ErrRefused, rec)
		}
		return parseWorktreeAttr(&worktrees[len(worktrees)-1], key, value)
	}}
	if err := r.read(ctx, dir, nil, w, "worktree", "list", "--porcelain", "-z"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return worktrees, nil
}

// parseWorktreeAttr reads one attribute of a worktree.
func parseWorktreeAttr(wt *Worktree, key, value string) error {
	switch key {
	case "HEAD":
		if err := checkObjectID(value); err != nil {
			return err
		}
		if !zeroOID(value) {
			wt.OID = value
		}
	case "branch":
		if err := checkDisplayRef(value); err != nil {
			return err
		}
		wt.Branch = value
	case "bare":
		wt.Bare = true
	case "detached":
		wt.Detached = true
	case "locked":
		wt.Locked = true
	case "prunable":
		wt.Prunable = true
	default:
		// An attribute a later git adds.
	}
	return nil
}

// checkWorktreePath returns ErrRefused for a worktree path that is not
// absolute, not valid UTF-8, or that holds a character unsafeRune
// refuses.
func checkWorktreePath(path string) error {
	switch {
	case !filepath.IsAbs(path):
		return fmt.Errorf("%w: worktree path %q is not absolute", ErrRefused, path)
	case !utf8.ValidString(path):
		return fmt.Errorf("%w: worktree path %q is not valid UTF-8", ErrRefused, path)
	}
	if i := strings.IndexFunc(path, unsafeRune); i >= 0 {
		c, _ := utf8.DecodeRuneInString(path[i:])
		return fmt.Errorf("%w: worktree path %q holds the character %U", ErrRefused, path, c)
	}
	return nil
}

// HooksPath returns the core.hooksPath the repository's own config file
// sets, and whether it sets one, under the flags of the mode, with git
// config --local --no-includes: neither the global or system config,
// nor the -c core.hooksPath=/dev/null that both modes pass, nor an
// include is read. In a linked worktree --local reads the config the
// worktrees share; a value in that worktree's own config.worktree
// (extensions.worktreeConfig) is not read, though git honours it in
// that worktree. The value is returned raw, as the file holds it: a
// leading ~ is not expanded and a relative path is not resolved. A key
// with no value, which git config -z answers as an empty value, is
// refused with the empty value, and so is a value that is not valid
// UTF-8 or that holds a character unsafeRune refuses.
func (r *Runner) HooksPath(ctx context.Context, dir string) (string, bool, error) {
	if err := checkRepo(dir); err != nil {
		return "", false, err
	}
	out := limitedBuffer{limit: maxHooksPathBytes}
	err := r.read(ctx, dir, nil, &out, "config", "--local", "--no-includes", "-z", "--get", "core.hooksPath")
	switch {
	case exitedWith(err, 1):
		return "", false, nil // the key is not set
	case err != nil:
		return "", false, err
	case out.truncated:
		return "", false, fmt.Errorf("%w: core.hooksPath is longer than %d bytes", ErrRefused, out.limit)
	}
	value, ok := strings.CutSuffix(out.String(), "\x00")
	if value == "" {
		return "", false, fmt.Errorf("%w: core.hooksPath is set with no value or an empty one", ErrRefused)
	}
	if !ok || !utf8.ValidString(value) {
		return "", false, fmt.Errorf("%w: git config answered %q", ErrRefused, out.String())
	}
	if i := strings.IndexFunc(value, unsafeRune); i >= 0 {
		c, _ := utf8.DecodeRuneInString(value[i:])
		return "", false, fmt.Errorf("%w: core.hooksPath holds the character %U", ErrRefused, c)
	}
	return value, true, nil
}
