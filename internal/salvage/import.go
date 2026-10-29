// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// importBundle runs step 2 of the host half (REQ-0715) for the bundle b
// of the manifest m of the repo r, the k-th nested one when b.Nested is
// set, in this order, each step only once the one before passed:
//
//  1. every ref m names in b passes CheckRef, and each salvage ref is
//     below the prefix of this generation and salvage id, so a salvage
//     ref of an earlier generation is not taken for this one's;
//  2. b is copied out of the salvage dir through the root into the
//     work dir, its size and sha256 checked on the way (payload-mismatch);
//  3. git bundle verify reads the copy; when it fails and the repo has
//     an origin, origin's branches are fetched into
//     refs/romeu/origin/<dir>/ and the copy is verified again;
//  4. git bundle unbundle writes its objects, and the heads it carries
//     must be the refs m names in b, name for name and object for
//     object (payload-mismatch);
//  5. git fsck --strict runs on each head's object, since
//     transfer.fsckObjects does not cover an unbundle (05 5.1);
//  6. each head becomes a create-only ref (CreateRef, I6) below
//     HostRefPrefix: <dir>/<ref without refs/> for the repo's bundle,
//     <dir>/nested-<k>/<ref without refs/> for a nested one's.
//
// A failure of steps 1, 3 or 5 is the item fsck-failed, and nothing of
// the bundle is imported. The copy stays in the work dir, which the
// caller removes. An error is a payload path refused (salvage-path), a
// read or a git call that failed for another reason, or a ref that
// already exists.
func (v *verifier) importBundle(ctx context.Context, root *os.Root, r VerifyRepo, m Manifest, b Bundle, k int) ([]gitsafe.Head, error) {
	want := map[string]string{}
	for _, ref := range m.Refs {
		if ref.Nested != b.Nested {
			continue
		}
		if err := v.checkBundleRef(ref.Ref); err != nil {
			v.skip(r.Dir, b.File, ReasonFsckFailed, err)
			return nil, nil
		}
		if _, dup := want[ref.Ref]; dup {
			v.skip(r.Dir, b.File, ReasonPayloadMismatch, fmt.Errorf("the manifest names the ref %q twice", clip(ref.Ref)))
			return nil, nil
		}
		want[ref.Ref] = ref.SHA
	}
	copied, err := v.copyOut(root, b)
	if errors.Is(err, errMismatch) {
		v.skip(r.Dir, b.File, ReasonPayloadMismatch, err)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := v.verifyBundle(ctx, r, b, copied); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		v.skip(r.Dir, b.File, ReasonFsckFailed, err)
		return nil, nil
	}
	heads, err := v.c.Git.Unbundle(ctx, r.Clone, copied)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		v.skip(r.Dir, b.File, ReasonFsckFailed, err)
		return nil, nil
	}
	got := map[string]string{}
	for _, h := range heads {
		if err := v.checkBundleRef(h.Ref); err != nil {
			v.skip(r.Dir, b.File, ReasonFsckFailed, err)
			return nil, nil
		}
		got[h.Ref] = h.OID
	}
	if !maps.Equal(got, want) {
		v.skip(r.Dir, b.File, ReasonPayloadMismatch, errors.New("the bundle's refs are not the refs the manifest names"))
		return nil, nil
	}
	oids := slices.Compact(slices.Sorted(maps.Values(got)))
	for _, oid := range oids {
		if err := v.c.Git.Fsck(ctx, r.Clone, oid); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			v.skip(r.Dir, b.File, ReasonFsckFailed, err)
			return nil, nil
		}
	}
	base := v.prefix + r.Dir + "/"
	if b.Nested != "" {
		base += "nested-" + strconv.Itoa(k) + "/"
	}
	imported := make([]gitsafe.Head, 0, len(heads))
	for _, name := range slices.Sorted(maps.Keys(got)) {
		ref := base + strings.TrimPrefix(name, "refs/")
		if err := v.c.Git.CreateRef(ctx, r.Clone, ref, got[name]); err != nil {
			return nil, fmt.Errorf("the ref %s: %w", ref, err)
		}
		imported = append(imported, gitsafe.Head{Ref: ref, OID: got[name]})
	}
	return imported, nil
}

// checkBundleRef returns an error for a ref name a salvage bundle may
// not carry: one CheckRef refuses, or a salvage ref (refs/salvage/)
// outside RefPrefix of this generation and salvage id (REQ-0704).
func (v *verifier) checkBundleRef(name string) error {
	if err := CheckRef(name); err != nil {
		return err
	}
	if prefix := RefPrefix(v.c.Generation, v.c.SalvageID); strings.HasPrefix(name, "refs/salvage/") && !strings.HasPrefix(name, prefix) {
		return fmt.Errorf("the salvage ref %q is not below %s", clip(name), prefix)
	}
	return nil
}

// copyOut copies the bundle b out of the salvage dir root into a new
// file of the work dir, 0600, checking its size and sha256 on the way
// (readPayload), and returns the copy's path. git then reads the copy,
// which the sandbox cannot change, rather than the file of the mount.
func (v *verifier) copyOut(root *os.Root, b Bundle) (string, error) {
	f, err := os.CreateTemp(v.c.Work, "bundle-*.bundle")
	if err != nil {
		return "", err
	}
	err = readPayload(root, b.File, b.Size, b.SHA256, f)
	return f.Name(), errors.Join(err, f.Close())
}

// verifyBundle runs git bundle verify on the copy of b; when it fails
// and b is the repo's own bundle of a repo with an origin, it fetches
// origin's branches into refs/romeu/origin/<dir>/, where a missing
// prerequisite would be, and verifies again (REQ-0715). A nested
// repository's bundle has no prerequisite, so it has no fetch.
func (v *verifier) verifyBundle(ctx context.Context, r VerifyRepo, b Bundle, copied string) error {
	err := v.c.Git.BundleVerify(ctx, r.Clone, copied)
	if err == nil || b.Nested != "" || r.Origin == "" {
		return err
	}
	spec := "+refs/heads/*:refs/romeu/origin/" + r.Dir + "/*"
	if ferr := v.c.Git.Fetch(ctx, r.Clone, r.Origin, []string{spec}); ferr != nil {
		return errors.Join(err, fmt.Errorf("the fetch of origin: %w", ferr))
	}
	return v.c.Git.BundleVerify(ctx, r.Clone, copied)
}
