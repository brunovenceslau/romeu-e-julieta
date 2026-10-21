// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// Plan is what a snapshot bundles: the pairs, each a ref name and the
// object the bundle gives it, sorted by name, and the inputs the
// snapshot left out.
type Plan struct {
	Pairs   []gitsafe.Ref
	Skipped []Skip
}

// Skip is one input a snapshot left out: what it is (a ref name gitsafe
// refused, a namespace, "worktrees", a worktree, the reflog of a ref,
// a reflog commit git could not walk, or the reflog commits over the
// cap) and why. Whole says it is a namespace or a whole source of
// reflog commits, whose loss a snapshot must not write over an earlier
// one with (decided while building T060).
type Skip struct {
	Item  string
	Err   error
	Whole bool
}

// Whole reports whether p left out a namespace or a whole source.
func (p Plan) Whole() bool {
	return slices.ContainsFunc(p.Skipped, func(s Skip) bool { return s.Whole })
}

// Refs returns the pairs of p as the refs of snapshot/heads.json.
func (p Plan) Refs() map[string]string {
	m := make(map[string]string, len(p.Pairs))
	for _, r := range p.Pairs {
		m[r.Name] = r.OID
	}
	return m
}

// worktreePrefix is the namespace of the names of a worktree's HEAD and
// of its HEAD reflog's commits, below ReflogPrefix:
// worktrees/<k>/HEAD and worktrees/<k>/<n>, with k the worktree's place
// in git worktree list, the main one 0.
const worktreePrefix = "worktrees/"

// MaxReflogCommits is the most reflog-only commits one snapshot names
// (decided while building T060): the first of them in source order
// (the branches by name, then the worktrees, each newest first) are
// named, and the rest left out and recorded. A variable, so a test can
// lower it.
var MaxReflogCommits = 1000

// Pairs returns what a snapshot of the repository in dir bundles (08
// 8.4, REQ-0688), read through gitsafe: the local branches, tags and
// notes and the refs of refs/salvage/<generation>/ (gitsafe.ListRefs,
// one namespace per call, so no earlier generation's); each worktree
// HEAD that is detached, or that points elsewhere than its branch, as
// refs/salvage-reflog/worktrees/<k>/HEAD (gitsafe.Worktrees); and each
// commit of a branch's reflog, or of a worktree's HEAD reflog
// (gitsafe.Reflog), that no bundled ref reaches, as
// refs/salvage-reflog/<branch>/<n> or
// refs/salvage-reflog/worktrees/<k>/<n>, n the entry's index as git
// numbers it, newest 0. A commit two entries hold is named once, by the
// first source and the newest entry; the sources are the branches by
// name, then the worktrees in order.
//
// Which reflog commits no bundled ref reaches is one question to git
// (gitsafe.Unreached), with the bundled refs and the bases excluded:
// the bases bound git's walk, and a commit they reach is in no bundle.
// A reflog commit another reflog commit reaches needs no name of its
// own. At most MaxReflogCommits are named.
//
// What the snapshot cannot read is left out and recorded in Skipped,
// and the rest is bundled (decided on 2026-10-21): a ref name gitsafe
// refuses, alone; a namespace gitsafe refuses whole (more refs than it
// lists); the worktrees when git cannot list them; a worktree whose
// common git dir is not the repository's own; a reflog that cannot be
// read. A name a branch already takes gets the suffix -<k>, the first
// that is free. A cancelled context and any other error of git fail
// the whole plan.
func Pairs(ctx context.Context, git *gitsafe.Runner, dir, generation string, bases []string) (Plan, error) {
	var p Plan
	// skip records err as the reason item is left out, and returns the
	// error that fails the plan instead: the context's.
	skip := func(item string, err error) error {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		p.Skipped = append(p.Skipped, Skip{Item: item, Err: err, Whole: true})
		return nil
	}
	names := newNameSet()
	add := func(name, oid string) {
		p.Pairs = append(p.Pairs, gitsafe.Ref{Name: name, OID: oid})
		names.add(name)
	}
	var branches []gitsafe.Ref
	for _, ns := range []string{"refs/heads/", "refs/tags/", "refs/notes/", "refs/salvage/" + generation + "/"} {
		refs, refused, err := git.ListRefs(ctx, dir, []string{ns})
		if errors.Is(err, gitsafe.ErrRefused) {
			if err := skip(ns, err); err != nil {
				return Plan{}, err
			}
			continue
		}
		if err != nil {
			return Plan{}, err
		}
		for _, name := range refused.Names {
			p.Skipped = append(p.Skipped, Skip{Item: name, Err: fmt.Errorf("%w: a ref name gitsafe does not read", gitsafe.ErrRefused)})
		}
		if refused.More > 0 {
			p.Skipped = append(p.Skipped, Skip{Item: strconv.Itoa(refused.More) + " more ref names of " + ns,
				Err: fmt.Errorf("%w: ref names gitsafe does not read", gitsafe.ErrRefused)})
		}
		for _, r := range refs {
			add(r.Name, r.OID)
		}
		if ns == "refs/heads/" {
			branches = refs
		}
	}
	type source struct {
		dir, ref, prefix string
	}
	var sources []source
	for _, b := range branches {
		sources = append(sources, source{dir, b.Name, ReflogPrefix + strings.TrimPrefix(b.Name, "refs/heads/") + "/"})
	}
	worktrees, err := git.Worktrees(ctx, dir)
	if err != nil {
		if err := skip("worktrees", err); err != nil {
			return Plan{}, err
		}
	}
	common := ""
	if len(worktrees) > 0 {
		if common, err = git.CommonDir(ctx, dir); err != nil {
			if err := skip("the reflogs of the worktrees", err); err != nil {
				return Plan{}, err
			}
		}
	}
	for k, w := range worktrees {
		if w.Bare || w.OID == "" {
			continue
		}
		prefix := ReflogPrefix + worktreePrefix + strconv.Itoa(k) + "/"
		if !slices.Contains(branches, gitsafe.Ref{Name: w.Branch, OID: w.OID}) {
			add(prefix+"HEAD", w.OID)
		}
		if w.Prunable || common == "" {
			continue
		}
		switch c, err := git.CommonDir(ctx, w.Path); {
		case err != nil:
			if err := skip("worktree "+strconv.Itoa(k), err); err != nil {
				return Plan{}, err
			}
		case !sameDir(c, common):
			p.Skipped = append(p.Skipped, Skip{Item: "worktree " + strconv.Itoa(k), Whole: true,
				Err: fmt.Errorf("%w: its common git dir %s is not the repository's %s", gitsafe.ErrRefused, c, common)})
		default:
			sources = append(sources, source{w.Path, "HEAD", prefix})
		}
	}

	// The reflog commits no bundled ref is, in source order, each once,
	// with the name of its newest entry.
	type candidate struct{ prefix, n, item string }
	seen := map[string]bool{}
	tips := make([]string, 0, len(p.Pairs))
	for _, r := range p.Pairs {
		seen[r.OID] = true
		tips = append(tips, r.OID)
	}
	var oids []string
	named := map[string]candidate{}
	for _, s := range sources {
		item := "the reflog of " + s.prefix
		entries, err := git.Reflog(ctx, s.dir, s.ref)
		if err != nil {
			if err := skip(item, err); err != nil {
				return Plan{}, err
			}
			continue
		}
		for i, oid := range slices.Backward(entries) {
			if seen[oid] {
				continue
			}
			seen[oid] = true
			oids = append(oids, oid)
			named[oid] = candidate{s.prefix, strconv.Itoa(len(entries) - 1 - i), item}
		}
	}
	exclude := slices.Concat(tips, bases)
	unreached, err := git.Unreached(ctx, dir, oids, exclude)
	if err != nil {
		// A walk git cannot finish for the whole set is asked again for
		// each commit alone; a commit that fails alone is left out, and
		// with it the whole of its source is incomplete.
		if cerr := ctx.Err(); cerr != nil {
			return Plan{}, cerr
		}
		unreached = nil
		for _, oid := range oids {
			one, err := git.Unreached(ctx, dir, []string{oid}, exclude)
			if err != nil {
				if err := skip(named[oid].item+": the commit "+oid, err); err != nil {
					return Plan{}, err
				}
				continue
			}
			unreached = append(unreached, one...)
		}
	}
	if over := len(unreached) - MaxReflogCommits; over > 0 {
		p.Skipped = append(p.Skipped, Skip{Item: strconv.Itoa(over) + " reflog commits",
			Err: fmt.Errorf("%w: a snapshot names at most %d reflog commits", gitsafe.ErrRefused, MaxReflogCommits)})
		unreached = unreached[:MaxReflogCommits]
	}
	for _, oid := range unreached {
		c := named[oid]
		name, ok := names.free(c.prefix + c.n)
		if !ok {
			p.Skipped = append(p.Skipped, Skip{Item: c.prefix + c.n, Err: fmt.Errorf("%w: the name and its suffixes are taken", gitsafe.ErrRefused)})
			continue
		}
		add(name, oid)
	}
	slices.SortFunc(p.Pairs, func(a, b gitsafe.Ref) int { return strings.Compare(a.Name, b.Name) })
	return p, nil
}

// sameDir reports whether a and b name one directory, their symbolic
// links resolved when they can be.
func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ra == rb
}

// nameSet holds the ref names of a plan and every directory above them,
// so a new name that is one of them, or that is a directory of one or
// below one, gets another before git refuses the bundle whole.
type nameSet struct {
	names, dirs map[string]bool
}

func newNameSet() *nameSet { return &nameSet{names: map[string]bool{}, dirs: map[string]bool{}} }

// add adds name and its directories.
func (s *nameSet) add(name string) {
	s.names[name] = true
	for i := strings.LastIndexByte(name, '/'); i > 0; i = strings.LastIndexByte(name[:i], '/') {
		s.dirs[name[:i]] = true
	}
}

// maxSuffix is the last suffix -<k> free tries.
const maxSuffix = 100

// free returns name, or name-<k> with the first k from 1 that is free,
// when name cannot join the set; false when none up to maxSuffix is, as
// when a name of the set is a directory of name.
func (s *nameSet) free(name string) (string, bool) {
	if !s.conflicts(name) {
		return name, true
	}
	for k := 1; k <= maxSuffix; k++ {
		if n := name + "-" + strconv.Itoa(k); !s.conflicts(n) {
			return n, true
		}
	}
	return "", false
}

// conflicts reports whether name cannot join the set: it is a name or a
// directory of the set, or a name of the set is one of its directories.
func (s *nameSet) conflicts(name string) bool {
	if s.names[name] || s.dirs[name] {
		return true
	}
	for i := strings.LastIndexByte(name, '/'); i > 0; i = strings.LastIndexByte(name[:i], '/') {
		if s.names[name[:i]] {
			return true
		}
	}
	return false
}
