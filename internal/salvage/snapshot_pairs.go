// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// Plan is what a snapshot bundles: the pairs, each a ref name and the
// object the bundle gives it, sorted by name, and the inputs gitsafe
// refused whole and the snapshot left out.
type Plan struct {
	Pairs   []gitsafe.Ref
	Skipped []Skip
}

// Skip is one input a snapshot left out: what it is (a namespace such
// as refs/tags/, "worktrees", the reflog of a ref, or a name the
// snapshot could not give) and why.
type Skip struct {
	Item string
	Err  error
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
// Whether a bundled ref reaches a commit is asked of git one ref at a
// time (gitsafe.AheadBehind), its own branch or worktree first; a
// commit found unreached is bundled and reaches the older ones in turn.
// An answer git cannot give for one ref, such as for a tag of a tree,
// counts as not reached, so the commit is bundled.
//
// An input gitsafe refuses whole, with gitsafe.ErrRefused (a ref name
// it refuses, more refs or worktrees than it lists, a reflog over its
// bound), is left out and recorded in Skipped, and the rest is
// bundled; so is a name a branch already takes. Any other error of git
// fails the whole plan. A worktree whose directory is gone has its
// HEAD bundled and its HEAD reflog not read.
func Pairs(ctx context.Context, git *gitsafe.Runner, dir, generation string) (Plan, error) {
	var p Plan
	skip := func(item string, err error) bool {
		if errors.Is(err, gitsafe.ErrRefused) {
			p.Skipped = append(p.Skipped, Skip{Item: item, Err: err})
			return true
		}
		return false
	}
	names := newNameSet()
	add := func(name, oid string) {
		p.Pairs = append(p.Pairs, gitsafe.Ref{Name: name, OID: oid})
		names.add(name)
	}
	var branches []gitsafe.Ref
	for _, ns := range []string{"refs/heads/", "refs/tags/", "refs/notes/", "refs/salvage/" + generation + "/"} {
		refs, err := git.ListRefs(ctx, dir, []string{ns})
		if skip(ns, err) {
			continue
		}
		if err != nil {
			return Plan{}, err
		}
		for _, r := range refs {
			add(r.Name, r.OID)
		}
		if ns == "refs/heads/" {
			branches = refs
		}
	}
	// The tips a reflog commit is asked against: every bundled ref, then
	// each worktree HEAD and each reflog commit bundled.
	var tips []string
	for _, r := range p.Pairs {
		tips = append(tips, r.OID)
	}
	type source struct {
		dir, ref, prefix, tip string
	}
	var sources []source
	for _, b := range branches {
		sources = append(sources, source{dir, b.Name, ReflogPrefix + strings.TrimPrefix(b.Name, "refs/heads/") + "/", b.OID})
	}
	worktrees, err := git.Worktrees(ctx, dir)
	if !skip("worktrees", err) && err != nil {
		return Plan{}, err
	}
	for k, w := range worktrees {
		if w.Bare || w.OID == "" {
			continue
		}
		prefix := ReflogPrefix + worktreePrefix + strconv.Itoa(k) + "/"
		if !slices.Contains(branches, gitsafe.Ref{Name: w.Branch, OID: w.OID}) {
			add(prefix+"HEAD", w.OID)
			tips = append(tips, w.OID)
		}
		if !w.Prunable {
			sources = append(sources, source{w.Path, "HEAD", prefix, w.OID})
		}
	}
	seen := map[string]bool{}
	for _, tip := range tips {
		seen[tip] = true
	}
	for _, s := range sources {
		oids, err := git.Reflog(ctx, s.dir, s.ref)
		if skip("the reflog of "+s.prefix, err) {
			continue
		}
		if err != nil {
			return Plan{}, err
		}
		for i, oid := range slices.Backward(oids) {
			if seen[oid] {
				continue
			}
			seen[oid] = true
			reached, err := reachedFrom(ctx, git, dir, oid, s.tip, tips)
			if err != nil {
				return Plan{}, err
			}
			if reached {
				continue
			}
			name := s.prefix + strconv.Itoa(len(oids)-1-i)
			if names.conflicts(name) {
				p.Skipped = append(p.Skipped, Skip{Item: name, Err: fmt.Errorf("%w: the name %q is taken", gitsafe.ErrRefused, name)})
				continue
			}
			add(name, oid)
			tips = append(tips, oid)
		}
	}
	slices.SortFunc(p.Pairs, func(a, b gitsafe.Ref) int { return strings.Compare(a.Name, b.Name) })
	return p, nil
}

// reachedFrom reports whether one of tips reaches oid in the
// repository in dir, asking first, then the others in order.
func reachedFrom(ctx context.Context, git *gitsafe.Runner, dir, oid, first string, tips []string) (bool, error) {
	for _, tip := range append([]string{first}, tips...) {
		ahead, _, err := git.AheadBehind(ctx, dir, oid, tip)
		if cerr := ctx.Err(); cerr != nil {
			return false, cerr
		}
		if err == nil && ahead == 0 {
			return true, nil
		}
	}
	return false, nil
}

// nameSet holds the ref names of a plan and every directory above them,
// so a new name that is one of them, or that is a directory of one or
// below one, is told apart before git refuses the bundle whole.
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
