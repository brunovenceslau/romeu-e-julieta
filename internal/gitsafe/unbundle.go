// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"strings"
)

// Head is one ref a bundle carries: its full name and the object it
// points at.
type Head struct {
	Ref, OID string
}

// Unbundle writes the objects of the bundle at file into the repository
// in dir, under the flags of the mode, and returns the refs the bundle
// carries, each a full ref name under refs/ and a full object id. It
// changes no ref: the caller creates the ones it keeps (CreateRef).
// transfer.fsckObjects does not cover unbundle, so the trees the caller
// materializes from these objects go through Fsck and WalkTree (05 5.1,
// REQ-0465). The heads are what the bundle says: a head may name any
// object, so the caller checks that one is a commit before it runs Fsck
// or WalkTree on it, and keeps its own namespace for the refs it
// creates rather than a name built from a head. file is an absolute
// path.
func (r *Runner) Unbundle(ctx context.Context, dir, file string) ([]Head, error) {
	if err := checkBundle(dir, file); err != nil {
		return nil, err
	}
	out, err := r.output(ctx, dir, nil, "bundle", "unbundle", file)
	if err != nil {
		return nil, err
	}
	return parseHeads(out)
}

// parseHeads reads the "<oid> <ref>" lines git bundle unbundle prints.
// A line of another shape, a ref outside refs/ such as HEAD, or a ref
// named twice is refused, so no name the caller would act on is taken
// from the bundle unchecked.
func parseHeads(out string) ([]Head, error) {
	var heads []Head
	seen := map[string]bool{}
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		oid, ref, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("%w: bundle head line %q", ErrRefused, line)
		}
		if err := checkObjectID(oid); err != nil {
			return nil, err
		}
		if err := checkRefName(ref, false); err != nil {
			return nil, err
		}
		if seen[ref] {
			return nil, fmt.Errorf("%w: the bundle names %q twice", ErrRefused, ref)
		}
		seen[ref] = true
		heads = append(heads, Head{Ref: ref, OID: oid})
	}
	return heads, nil
}
