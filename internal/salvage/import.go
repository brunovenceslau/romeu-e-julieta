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
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// importBundle runs step 2 of the host half (REQ-0715) for the bundle b
// of the manifest m of the repo r, the k-th nested one when b.Nested is
// set, in this order, each step only once the one before passed:
//
//  1. every ref m names in b passes CheckRef, and each salvage ref is
//     below refs/salvage/<generation>/ of this generation, so a salvage
//     ref of an earlier generation is not taken for this one's (I17);
//  2. b is copied out of the salvage dir through the root into the
//     work dir, its size and sha256 checked on the way (payload-mismatch);
//  3. a quarantine repository is made in the work dir, which borrows the
//     clone's objects and lends it none (quarantine);
//  4. git bundle verify reads the copy there; when it fails and the
//     repo has an origin, origin's branches are fetched into
//     refs/romeu/origin/<dir>/ of the clone, once per repo, and the
//     copy is verified again;
//  5. git bundle unbundle writes its objects into the quarantine, and
//     the heads it carries must pass step 1 and be the refs m names in
//     b, name for name and object for object (payload-mismatch);
//  6. git fsck --strict runs there on each head's object, since
//     transfer.fsckObjects does not cover an unbundle (05 5.1);
//  7. only then are the objects unbundled into the clone, and each head
//     becomes a create-only ref (CreateRef, I6) below HostRefPrefix:
//     <dir>/<ref without refs/> for the repo's bundle,
//     <dir>/nested-<k>/<ref without refs/> for a nested one's.
//
// A failure of steps 1, 4, 5 (but a refs mismatch) or 6 is the item
// fsck-failed, and nothing of the bundle reaches the clone: a malformed
// object stays in the quarantine, where it cannot fail every later fsck
// of the clone. The copy and the quarantine stay in the work dir, which
// the caller removes. An error is a payload path refused
// (salvage-path), a read or a git call that failed for another reason,
// or a ref that already exists; the refs created before it are
// returned with it.
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
	got, ok, err := v.quarantined(ctx, r, b, copied, want)
	if err != nil || !ok {
		return nil, err
	}
	if _, err := v.c.Git.Unbundle(ctx, r.Clone, copied); err != nil {
		return nil, err
	}
	base := v.prefix + r.Dir + "/"
	if b.Nested != "" {
		base += "nested-" + strconv.Itoa(k) + "/"
	}
	imported := make([]gitsafe.Head, 0, len(got))
	for _, name := range slices.Sorted(maps.Keys(got)) {
		ref := base + strings.TrimPrefix(name, "refs/")
		if err := v.c.Git.CreateRef(ctx, r.Clone, ref, got[name]); err != nil {
			return imported, fmt.Errorf("the ref %s: %w", termsafe.Escape(ref), err)
		}
		imported = append(imported, gitsafe.Head{Ref: ref, OID: got[name]})
	}
	return imported, nil
}

// quarantined runs steps 3 to 6 of importBundle for the copy of b, and
// returns the bundle's heads, ref to object, and true when they passed;
// false when the bundle is a skipped item, which it records.
func (v *verifier) quarantined(ctx context.Context, r VerifyRepo, b Bundle, copied string, want map[string]string) (map[string]string, bool, error) {
	fail := func(reason string, err error) (map[string]string, bool, error) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}
		v.skip(r.Dir, b.File, reason, err)
		return nil, false, nil
	}
	q, err := v.quarantine(ctx, r.Clone)
	if err != nil {
		return nil, false, err
	}
	if err := v.verifyBundle(ctx, r, b, q, copied); err != nil {
		return fail(ReasonFsckFailed, err)
	}
	heads, err := v.c.Git.Unbundle(ctx, q, copied)
	if err != nil {
		return fail(ReasonFsckFailed, err)
	}
	got := map[string]string{}
	for _, h := range heads {
		if err := v.checkBundleRef(h.Ref); err != nil {
			return fail(ReasonFsckFailed, err)
		}
		got[h.Ref] = h.OID
	}
	if !maps.Equal(got, want) {
		return fail(ReasonPayloadMismatch, errors.New("the bundle's refs are not the refs the manifest names"))
	}
	for _, oid := range slices.Compact(slices.Sorted(maps.Values(got))) {
		if err := v.c.Git.Fsck(ctx, q, oid); err != nil {
			return fail(ReasonFsckFailed, err)
		}
	}
	return got, true, nil
}

// checkBundleRef returns an error for a ref name a salvage bundle may
// not carry: one CheckRef refuses, or a salvage ref (refs/salvage/)
// outside refs/salvage/<generation>/ of this generation, the salvage
// refs the bundle carries (REQ-0704): this salvage's and those of an
// earlier salvage of the same generation.
func (v *verifier) checkBundleRef(name string) error {
	if err := CheckRef(name); err != nil {
		return err
	}
	if prefix := "refs/salvage/" + v.c.Generation + "/"; strings.HasPrefix(name, "refs/salvage/") && !strings.HasPrefix(name, prefix) {
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

// verifyBundle runs git bundle verify on the copy of b in the
// quarantine q; when it fails and b is the repo's own bundle of a repo
// with an origin, it fetches origin's branches into the clone's
// refs/romeu/origin/<dir>/, where a missing prerequisite would be, once
// per repo, and verifies again (REQ-0715). A nested repository's bundle
// has no prerequisite, so it has no fetch.
func (v *verifier) verifyBundle(ctx context.Context, r VerifyRepo, b Bundle, q, copied string) error {
	err := v.c.Git.BundleVerify(ctx, q, copied)
	if err == nil || b.Nested != "" || r.Origin == "" || v.fetched[r.Dir] {
		return err
	}
	if v.fetched == nil {
		v.fetched = map[string]bool{}
	}
	v.fetched[r.Dir] = true
	spec := "+refs/heads/*:refs/romeu/origin/" + r.Dir + "/*"
	if ferr := v.c.Git.Fetch(ctx, r.Clone, r.Origin, []string{spec}); ferr != nil {
		return errors.Join(err, fmt.Errorf("the fetch of origin: %w", ferr))
	}
	return v.c.Git.BundleVerify(ctx, q, copied)
}

// quarantine makes a new repository in the work dir whose objects/info/
// alternates names the object dir of the clone: git reads the clone's
// objects there, so a bundle's prerequisites resolve and fsck follows
// its history into them, while what an unbundle writes stays in the
// quarantine. It is a bare repository of the three entries git needs,
// HEAD, objects and refs, with no config, so no setting of the clone
// applies; a SHA-1 one, the format of every origin of v1 (GitHub serves
// no SHA-256 repository), so a SHA-256 bundle fails its verify there.
func (v *verifier) quarantine(ctx context.Context, clone string) (string, error) {
	common, err := v.c.Git.CommonDir(ctx, clone)
	if err != nil {
		return "", err
	}
	q, err := os.MkdirTemp(v.c.Work, "quarantine-")
	if err != nil {
		return "", err
	}
	for _, d := range []string{"objects", "objects/info", "refs"} {
		if err := os.Mkdir(filepath.Join(q, d), 0o700); err != nil {
			return "", err
		}
	}
	files := map[string]string{
		"HEAD":                    "ref: refs/heads/main\n",
		"objects/info/alternates": filepath.Join(common, "objects") + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(q, name), []byte(content), 0o600); err != nil {
			return "", err
		}
	}
	return q, nil
}
