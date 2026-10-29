// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Repo is one repo of the manifest a salvage covers.
type Repo struct {
	// Dir is the repo's dir in the project.
	Dir string
	// Path is the root of its main worktree in the sandbox.
	Path string
	// Memory is its memory dir, an absolute and clean path.
	Memory string
	// Base are the origin SHAs the host already has (repos[].base):
	// the bundle leaves out what they reach.
	Base []string
	// Primary says that it is the primary repo, whose manifest holds
	// what concerns the whole sandbox: agentStop and the agent's state.
	Primary bool
}

// AgentProfile is what salvage knows of the agent (08 8.5 steps 1 and
// 5): cmd/julieta builds it from the agent's package, which this
// package, linked by romeu too, does not import (REQ-0699).
type AgentProfile struct {
	// Name is the profile's name, which names its archive in agent/.
	Name string
	// Paths are its state paths below $HOME.
	Paths []AgentPath
	// Transcript reports whether the file at rel, a slash path below
	// $HOME, is a transcript, which is kept only with
	// --include-transcripts.
	Transcript func(rel string) bool
}

// AgentPath is one state path of the agent.
type AgentPath struct {
	// Rel is the path relative to $HOME, clean and slash-separated.
	Rel string
	// Required says that the pinned agent always has it, so a missing
	// one is agent-path-missing; a missing optional one is no item.
	Required bool
}

// Config is one salvage of the sandbox half (08 8.5 steps 2 to 6): what
// it reads, where it writes, and what step 1 found.
type Config struct {
	// Git is julieta's runner, in sandbox mode (REQ-0694).
	Git *gitsafe.Runner
	// Home is $HOME, below which the agent's state paths are.
	Home string
	// Work is a directory of julieta state that this salvage alone
	// uses, for the temporary indexes and the bundles it builds; it is
	// made 0700 and removed at the end.
	Work string
	// Repos are the repos of the manifest, in its order.
	Repos []Repo
	// Generation and SalvageID name the salvage dirs and the refs.
	Generation, SalvageID string
	// CapBytes is salvage.capBytes, the bytes of untracked files,
	// ignored files and agent state one salvage keeps.
	CapBytes int64
	// ExcludeIgnored is salvage.excludeIgnored.
	ExcludeIgnored []string
	// Profile is the agent profile, or nil when the manifest's agent
	// names none.
	Profile *AgentProfile
	// IncludeTranscripts keeps the transcripts.
	IncludeTranscripts bool
	// AgentStop is what --stop-agents did; nil without it.
	AgentStop *AgentStop
	// AgentNotMatched says that --stop-agents matched no process while
	// a session is open (REQ-0696).
	AgentNotMatched bool
	// Now is the clock and ULIDs the source of every ULID (10 10.4).
	Now   func() time.Time
	ULIDs *canon.ULIDSource
}

// Written is the manifest a salvage wrote for one repo.
type Written struct {
	// Repo is the repo's dir.
	Repo string
	// Path is the manifest's path.
	Path string
	// Manifest is what it holds.
	Manifest Manifest
}

// Result is what a salvage wrote and read.
type Result struct {
	// Manifests are the manifests, one per repo, in the manifest's
	// order.
	Manifests []Written
	// Fingerprint is the fingerprint of the reads (REQ-0417).
	Fingerprint Fingerprint
	// SecretBearing says that a payload holds ignored files or agent
	// state, which can hold credentials (REQ-0710).
	SecretBearing bool
}

// The identity of the capture commits (REQ-0700).
const (
	CaptureName  = "julieta-salvage"
	CaptureEmail = "julieta-salvage@localhost"
)

// The labels of the refs of a salvage, below RefPrefix: the capture of
// worktree n and stash n.
const (
	worktreeRef = "worktree-"
	stashRef    = "stash-"
)

// RefPrefix returns the namespace of the refs of a salvage,
// refs/salvage/<generation>/<salvage-id>/ (REQ-0700, REQ-0701).
func RefPrefix(generation, salvageID string) string {
	return "refs/salvage/" + generation + "/" + salvageID + "/"
}

// afterCapture is the test seam of the recheck of step 2 (10 10.4):
// called with the worktree's root after its capture commit and before
// it is read again; nil outside tests.
var afterCapture func(dir string)

// salvager is one run of Salvage.
type salvager struct {
	c      Config
	now    time.Time
	prefix string
	budget int64
}

// repoRun is the salvage of one repo of the manifest: its salvage dir,
// the manifest it fills and the ignored files step 4 archives.
type repoRun struct {
	repo    Repo
	root    *os.Root
	dir     string // the salvage dir, relative to root
	m       Manifest
	ignored []archived
	fp      FingerprintRepo
	nested  int
}

// archived is one ignored entry of a worktree to archive under label.
type archived struct {
	worktree string // the absolute path of the worktree
	label    string
	entry    ignoredEntry
}

// unit is one worktree the capture reads.
type unit struct {
	repo   string // the repository the refs go in
	path   string // the root of the worktree
	label  string // Worktree.Path
	nested string // Worktree.Nested
	ref    string // the name of its capture ref
}

// Salvage runs steps 2 to 6 of the sandbox half (08 8.5) for every repo
// of c: it makes each salvage dir memory/<dir>/salvage/<generation>/
// <salvage-id>/, 0700, which must not exist; captures each worktree,
// its nested repositories' too, and the stashes into refs under
// RefPrefix; writes one bundle per repository; archives the ignored
// files and the agent's state within the salvage cap; and writes
// manifest.json last, through a temporary file and a rename. What
// cannot be captured is a skipped item of the manifest; an error is a
// salvage julieta cannot write, and leaves a salvage dir without its
// manifest, which verification ignores and keeps.
func Salvage(ctx context.Context, c Config) (res Result, err error) {
	if c.Now == nil || c.ULIDs == nil || c.Git == nil {
		return Result{}, errors.New("salvage: a config needs a runner, a clock and a ULID source")
	}
	if _, err := canon.ParseULID(c.Generation); err != nil {
		return Result{}, fmt.Errorf("the generation: %w", err)
	}
	if _, err := canon.ParseULID(c.SalvageID); err != nil {
		return Result{}, fmt.Errorf("the salvage id: %w", err)
	}
	s := &salvager{c: c, now: c.Now().UTC(), prefix: RefPrefix(c.Generation, c.SalvageID), budget: max(c.CapBytes, 0)}
	if err := os.MkdirAll(c.Work, memstore.DirMode); err != nil {
		return Result{}, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(c.Work)) }()
	runs := make([]*repoRun, 0, len(c.Repos))
	defer func() {
		for _, r := range runs {
			err = errors.Join(err, r.root.Close())
		}
	}()
	for _, r := range c.Repos {
		run, err := s.open(r)
		if err != nil {
			return Result{}, fmt.Errorf("the salvage dir of the repo %s: %w", r.Dir, err)
		}
		runs = append(runs, run)
	}
	res.Fingerprint.Repos = []FingerprintRepo{}
	for _, run := range runs {
		if err := s.captureRepo(ctx, run); err != nil {
			return Result{}, fmt.Errorf("the repo %s: %w", run.repo.Dir, err)
		}
		res.Fingerprint.Repos = append(res.Fingerprint.Repos, run.fp)
	}
	for _, run := range runs {
		stored, err := s.archiveIgnored(ctx, run)
		if err != nil {
			return Result{}, fmt.Errorf("the ignored files of the repo %s: %w", run.repo.Dir, err)
		}
		res.SecretBearing = res.SecretBearing || stored
	}
	for _, run := range runs {
		if !run.repo.Primary {
			continue
		}
		run.m.AgentStop = c.AgentStop
		stored, err := s.archiveAgent(ctx, run)
		if err != nil {
			return Result{}, fmt.Errorf("the agent's state: %w", err)
		}
		res.SecretBearing = res.SecretBearing || stored
	}
	for _, run := range runs {
		w, err := s.writeManifest(run)
		if err != nil {
			return Result{}, fmt.Errorf("the manifest of the repo %s: %w", run.repo.Dir, err)
		}
		res.Manifests = append(res.Manifests, w)
	}
	return res, nil
}

// open makes the salvage dir of r and returns its run. The directories
// above it are made 0700 when absent; the salvage dir itself must be
// new, so a second salvage with the same id fails before it writes.
func (s *salvager) open(r Repo) (*repoRun, error) {
	if !filepath.IsAbs(r.Memory) || filepath.Clean(r.Memory) != r.Memory {
		return nil, fmt.Errorf("the memory dir %q is not an absolute, clean path", r.Memory)
	}
	root, err := os.OpenRoot(r.Memory)
	if err != nil {
		return nil, err
	}
	gen := path.Join(memstore.SalvageDir, s.c.Generation)
	dir := path.Join(gen, s.c.SalvageID)
	for _, d := range []string{memstore.SalvageDir, gen} {
		if err := mkdirIn(root, d); err != nil {
			return nil, errors.Join(err, root.Close())
		}
	}
	if err := root.Mkdir(dir, memstore.DirMode); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &repoRun{
		repo: r, root: root, dir: dir,
		m:  Manifest{Generation: s.c.Generation, Repo: r.Dir, SalvageID: s.c.SalvageID},
		fp: FingerprintRepo{Dir: r.Dir, Worktrees: []FingerprintWorktree{}},
	}, nil
}

// mkdirIn makes the directory dir of root 0700 when it is absent; a
// node there that is no directory is an error.
func mkdirIn(root *os.Root, dir string) error {
	err := root.Mkdir(dir, memstore.DirMode)
	if err == nil || !errors.Is(err, fs.ErrExist) {
		return err
	}
	info, err := root.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", memstore.ErrLayout, dir)
	}
	return nil
}

// skip records an item the salvage left out of the repo's manifest.
func (r *repoRun) skip(item, reason string, err error) {
	s := Skipped{Item: termsafe.Escape(item), Reason: reason}
	if err != nil {
		s.Detail = termsafe.Escape(err.Error())
	}
	r.m.Skipped = append(r.m.Skipped, s)
}

// captureRepo runs steps 2 and 3 for the repo of run, and the reads of
// step 4: each worktree in label order, each nested repository right
// after the worktree that holds it, then the stashes and the bundle of
// each repository.
func (s *salvager) captureRepo(ctx context.Context, run *repoRun) error {
	set, err := worktreeSet(ctx, s.c.Git, run.repo.Path)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// No worktree list: the repo is not one git reads, such as a
		// secondary not cloned, so nothing else of it can be read.
		run.skip("worktrees", ReasonUnreadable, err)
		return nil
	}
	for i, w := range set {
		u := unit{repo: run.repo.Path, path: w.Path, label: label(i), ref: s.prefix + worktreeRef + strconv.Itoa(i)}
		if err := s.captureUnit(ctx, run, u, 0); err != nil {
			return err
		}
	}
	return s.repoRefs(ctx, run, run.repo.Path, "", run.repo.Base)
}

// captureUnit captures the worktree u, reads its ignored files, and
// then captures each nested repository found in it.
func (s *salvager) captureUnit(ctx context.Context, run *repoRun, u unit, depth int) error {
	fw := FingerprintWorktree{Path: u.label}
	read, ok := s.captureWorktree(ctx, run, u)
	if err := ctx.Err(); err != nil {
		return err
	}
	var nested []string
	if ok {
		fw.Head, fw.Status = read.changes.OID, read.status
		nested = nestedOf(u.path, read.changes)
	}
	ign, err := readIgnored(ctx, s.c.Git, u.path, s.c.ExcludeIgnored)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		run.skip(u.label+" ignored files", ReasonUnreadable, err)
	} else {
		fw.Ignored = ign.digest
		nested = append(nested, ign.nested...)
		for _, e := range ign.excluded {
			run.skip(u.label+"/"+e, ReasonExcludedBySpec, nil)
		}
		for _, e := range ign.entries {
			run.ignored = append(run.ignored, archived{worktree: u.path, label: u.label, entry: e})
		}
	}
	if links, err := s.c.Git.Gitlinks(ctx, u.path); err == nil {
		nested = append(nested, existingRepos(u.path, links)...)
	}
	run.fp.Worktrees = append(run.fp.Worktrees, fw)
	if depth >= MaxNestedDepth {
		return nil
	}
	for _, n := range dedupe(nested) {
		dir := path.Join(u.path, n)
		nu := unit{repo: dir, path: dir, label: u.label + "/" + n, ref: s.prefix + worktreeRef + "0"}
		nu.nested = nu.label
		if err := s.captureUnit(ctx, run, nu, depth+1); err != nil {
			return err
		}
		if err := s.repoRefs(ctx, run, dir, nu.nested, nil); err != nil {
			return err
		}
	}
	return nil
}

// captureWorktree runs step 2 for the worktree u (REQ-0700, REQ-0702):
// it reads HEAD and the status, records the working tree in a capture
// commit, and reads both again; when either moved, the capture repeats
// once, and then the worktree is skipped as worktree-changing. An
// operation in progress skips it as in-progress-operation. The
// untracked files the cap leaves room for go in the commit, and the
// rest are over-cap. It returns the last read, and false when there is
// none.
func (s *salvager) captureWorktree(ctx context.Context, run *repoRun, u unit) (worktreeRead, bool) {
	rec := Worktree{Path: u.label, Location: u.path, Nested: u.nested}
	defer func() { run.m.Worktrees = append(run.m.Worktrees, rec) }()
	var last worktreeRead
	for attempt := 0; attempt < 2; attempt++ {
		before, err := readWorktree(ctx, s.c.Git, u.path)
		if err != nil {
			run.skip(u.label, ReasonUnreadable, err)
			return last, attempt > 0
		}
		last = before
		c := before.changes
		rec.Head, rec.Branch, rec.Detached = c.OID, c.Branch, c.Branch == ""
		rec.Dirty = len(c.Tracked) > 0 || len(c.Untracked) > 0
		if c.Operation != "" {
			run.skip(u.label, ReasonInProgressOperation, fmt.Errorf("a %s is in progress", c.Operation))
			return last, true
		}
		add, over, budget := s.untracked(u.path, c)
		oid := ""
		if rec.Dirty {
			oid, err = s.commit(ctx, u.path, c.OID, add)
			if err != nil {
				run.skip(u.label, failReason(err), err)
				return last, true
			}
		}
		if afterCapture != nil {
			afterCapture(u.path)
		}
		after, err := readWorktree(ctx, s.c.Git, u.path)
		if err != nil {
			run.skip(u.label, ReasonUnreadable, err)
			return last, true
		}
		last = after
		if !after.same(before) {
			continue
		}
		s.budget = budget
		for _, p := range over {
			run.skip(u.label+"/"+p, ReasonOverCap, nil)
		}
		if oid != "" {
			if err := s.c.Git.CreateRef(ctx, u.repo, u.ref, oid); err != nil {
				run.skip(u.label, ReasonUnreadable, err)
				return last, true
			}
			rec.Capture = u.ref
		}
		return last, true
	}
	run.skip(u.label, ReasonWorktreeChanging, errors.New("HEAD or the status moved during the capture, twice"))
	return last, true
}

// untracked returns the paths a capture of c adds: the files the index
// adds, and the untracked files in git's order while the salvage cap
// leaves room for them (REQ-0703); the untracked files beyond it, and
// the budget left. A nested repository is not a file of the capture.
func (s *salvager) untracked(dir string, c gitsafe.Changes) (add, over []string, budget int64) {
	budget = s.budget
	add = slices.Clone(c.Added)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return add, nil, budget
	}
	defer func() { _ = root.Close() }()
	for _, p := range c.Untracked {
		if strings.HasSuffix(p, "/") {
			continue
		}
		info, err := root.Lstat(p)
		if err != nil {
			continue // gone since git listed it: the recheck sees it
		}
		if info.Size() > budget {
			over = append(over, p)
			continue
		}
		budget -= info.Size()
		add = append(add, p)
	}
	return add, over, budget
}

// commit makes the capture commit of the worktree dir through a
// temporary index in the salvage's work dir.
func (s *salvager) commit(ctx context.Context, dir, parent string, add []string) (string, error) {
	u, err := canon.NewULID(s.now, s.c.ULIDs)
	if err != nil {
		return "", err
	}
	return s.c.Git.CaptureCommit(ctx, dir, gitsafe.Capture{
		Index: filepath.Join(s.c.Work, "index-"+u.String()), Parent: parent, Add: add,
		Name: CaptureName, Email: CaptureEmail, When: s.now,
		Message: "julieta salvage " + s.c.SalvageID + " of generation " + s.c.Generation,
	})
}

// failReason is the skipped reason of a capture git could not write:
// disk-full when the disk is full (REQ-0703), unreadable otherwise.
func failReason(err error) string {
	if errors.Is(err, syscall.ENOSPC) || strings.Contains(err.Error(), "No space left on device") {
		return ReasonDiskFull
	}
	return ReasonUnreadable
}

// repoRefs runs the rest of steps 2 and 3 for the repository dir: each
// stash becomes a ref (REQ-0701), then the bundle of the repository
// (REQ-0704), its refs and what it left out go in the manifest.
func (s *salvager) repoRefs(ctx context.Context, run *repoRun, dir, nested string, bases []string) error {
	what := "the repo"
	if nested != "" {
		what = nested
	}
	stashes, err := s.c.Git.Stashes(ctx, dir)
	if err != nil {
		run.skip("the stashes of "+what, failReason(err), err)
	}
	for i, st := range stashes {
		if err := s.c.Git.CreateRef(ctx, dir, s.prefix+stashRef+strconv.Itoa(i), st.OID); err != nil {
			run.skip("stash@{"+strconv.Itoa(i)+"} of "+what, failReason(err), err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.bundle(ctx, run, dir, nested, bases)
}
