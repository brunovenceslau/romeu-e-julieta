// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Unreached returns those of oids that neither one of exclude nor
// another of oids reaches, in the repository in dir, in the order of
// oids, each once, under the flags of the mode, with one git rev-list
// --parents --stdin: each id of oids, then ^<id> for each of exclude,
// each once, on its standard input, so no id is an argument. They are
// the commits a bundle must name so that, with exclude, every commit of
// oids is in it. Each id is a full object id; an id of exclude that the
// repository does not hold is git's error.
//
// git walks every commit oids reach and exclude does not. One of oids
// is reached by none of exclude when the walk lists it, and by another
// of oids when the walk lists it as the parent of a commit it walked,
// since that commit descends from some id of oids and every commit
// between is walked too. A caller bounds the walk by putting in exclude
// the commits the host has (the bases). An id that names no commit,
// such as a tree, is never returned, since rev-list lists commits only.
func (r *Runner) Unreached(ctx context.Context, dir string, oids, exclude []string) ([]string, error) {
	stdin, err := reachInput(dir, oids, exclude)
	if err != nil || len(oids) == 0 {
		return nil, err
	}
	want := make(map[string]bool, len(oids))
	for _, oid := range oids {
		want[oid] = true
	}
	walked, parent := map[string]bool{}, map[string]bool{}
	w := &records{limit: maxStatusRecord, each: func(line string) error {
		ids := strings.Fields(line)
		if len(ids) == 0 {
			return fmt.Errorf("%w: rev-list line %q", ErrRefused, line)
		}
		for i, id := range ids {
			if err := checkObjectID(id); err != nil {
				return err
			}
			if !want[id] {
				continue
			}
			if i == 0 {
				walked[id] = true
			} else {
				parent[id] = true
			}
		}
		return nil
	}}
	if err := r.runInput(ctx, dir, historySettings, statusEnv, strings.NewReader(stdin), lines{w}, "rev-list", "--parents", "--stdin"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	var out []string
	for _, oid := range oids {
		if walked[oid] && !parent[oid] {
			out = append(out, oid)
			delete(walked, oid)
		}
	}
	return out, nil
}

// errBeyond stops the output of Beyond's rev-list at its first line.
var errBeyond = errors.New("gitsafe: an object is beyond the exclusion")

// firstLine records whether a writer was given any byte, and refuses
// it, so git's output stops there.
type firstLine struct{ seen bool }

// Write records that p came and refuses it.
func (f *firstLine) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	f.seen = true
	return 0, errBeyond
}

// Beyond reports whether the objects oids reach, commits, trees, blobs
// and tags, hold one that none of exclude reaches, in the repository in
// dir, under the flags of the mode, with git rev-list --objects --stdin
// fed as Unreached feeds it. It reads git's output up to its first line
// and stops git there: whether a bundle of oids minus exclude would
// hold an object, which git bundle create refuses to write when none
// is (a tag object on a commit exclude reaches is one). No oid is
// false.
func (r *Runner) Beyond(ctx context.Context, dir string, oids, exclude []string) (bool, error) {
	stdin, err := reachInput(dir, oids, exclude)
	if err != nil || len(oids) == 0 {
		return false, err
	}
	out := &firstLine{}
	err = r.runInput(ctx, dir, historySettings, statusEnv, strings.NewReader(stdin), out, "rev-list", "--objects", "--stdin")
	if cerr := ctx.Err(); cerr != nil {
		return false, cerr
	}
	if out.seen {
		return true, nil
	}
	return false, err
}

// reachInput checks the repo and every id and returns the standard
// input of Unreached and Beyond: oids, then ^<id> for each of exclude,
// each once.
func reachInput(dir string, oids, exclude []string) (string, error) {
	if err := checkRepo(dir); err != nil {
		return "", err
	}
	var b strings.Builder
	seen := map[string]bool{}
	for _, list := range [][]string{oids, exclude} {
		for _, oid := range list {
			if err := checkObjectID(oid); err != nil {
				return "", err
			}
		}
	}
	for _, oid := range oids {
		if !seen[oid] {
			seen[oid] = true
			b.WriteString(oid + "\n")
		}
	}
	for _, oid := range slices.Compact(slices.Sorted(slices.Values(exclude))) {
		b.WriteString("^" + oid + "\n")
	}
	return b.String(), nil
}
