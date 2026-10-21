// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// ErrNoRefs is the error of a BundleOf given no pair. It is not
// ErrRefused and wraps no error of git, so the caller tells a snapshot
// with nothing to bundle apart from a bundle that failed.
var ErrNoRefs = errors.New("gitsafe: a bundle of no ref")

// bundleNamespaces are the namespaces of the names BundleOf bundles:
// the user's branches, tags and notes, and the salvage refs and
// salvage reflog refs the snapshot names (REQ-0688); a worktree's HEAD
// is named by the caller under one of them.
var bundleNamespaces = []string{"refs/heads/", "refs/tags/", "refs/notes/", "refs/salvage/", "refs/salvage-reflog/"}

// The object formats git names with rev-parse --show-object-format.
var objectFormats = []string{"sha1", "sha256"}

// BundleOf writes to file a bundle of the pairs, each a ref name and
// the object id the bundle gives it, minus the objects reachable from
// the bases, which the bundle then needs as prerequisites, from the
// objects of the repository in dir: the root of a work tree, a linked
// worktree's included, or a bare repository. An object id need not be
// the tip of any ref: a reflog entry, or a commit no ref reaches, is
// bundled under the name the caller gives it, which git bundle create
// alone cannot do. Each name passes checkRefName, is under one of
// bundleNamespaces and is named once; each id and base is a full object
// id; file is an absolute path.
//
// No ref is written into the user's repository. BundleOf reads the
// object format and the common git dir of dir, makes a temporary bare
// repository of that format in the directory of file, named
// .gitsafe-bundle-<random>, which borrows the objects of the common git
// dir, creates the refs there in one transaction (git update-ref
// --stdin, which fails on an object that does not exist), runs git
// bundle create there (BundleCreate), and removes the temporary
// repository on every path, a failure and a cancelled context included.
// The temporary repository stores its refs in a reftable
// (--ref-format=reftable, git 2.45.0, below the floor), so no ref name
// the caller gives becomes a file or directory name. A process killed
// before it removes the temporary repository leaves it behind: a caller
// that scans the directory of its bundles ignores .gitsafe-bundle-*.
//
// The temporary repository borrows the objects through its own
// objects/info/alternates file, not GIT_ALTERNATE_OBJECT_DIRECTORIES
// (decided while building T153, 2026-10-21): the file belongs to the
// temporary repository and goes with it, so the user's repository,
// whose own alternates git still follows, is never written; every git
// call keeps the environment of its mode, which in sandbox mode passes
// no other GIT_* variable (05 5.1); and the file needs no list
// separator, where the variable splits paths at a colon. The objects
// directory reaches the file only if it is absolute, valid UTF-8 and
// holds no character unsafeRune refuses, so it is one line git reads
// as it is.
//
// The temporary repository has no shallow file, so a shallow
// repository's history is git's error. No pair is ErrNoRefs; a bundle
// every pair of which a base reaches is git's error ("Refusing to
// create empty bundle").
func (r *Runner) BundleOf(ctx context.Context, dir, file string, pairs []Ref, bases []string) (err error) {
	if err := checkBundle(dir, file); err != nil {
		return err
	}
	if len(pairs) == 0 {
		return ErrNoRefs
	}
	names := make([]string, 0, len(pairs))
	var stdin strings.Builder
	for _, p := range pairs {
		if err := checkRefName(p.Name, false); err != nil {
			return err
		}
		if !slices.ContainsFunc(bundleNamespaces, func(ns string) bool { return strings.HasPrefix(p.Name, ns) && len(p.Name) > len(ns) }) {
			return fmt.Errorf("%w: ref %q is outside %s", ErrRefused, p.Name, strings.Join(bundleNamespaces, ", "))
		}
		if slices.Contains(names, p.Name) {
			return fmt.Errorf("%w: ref %q named twice", ErrRefused, p.Name)
		}
		if err := checkObjectID(p.OID); err != nil {
			return err
		}
		names = append(names, p.Name)
		stdin.WriteString("create " + p.Name + "\x00" + p.OID + "\x00")
	}
	for _, base := range bases {
		if err := checkObjectID(base); err != nil {
			return err
		}
	}
	format, objects, err := r.objectsOf(ctx, dir)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(file), ".gitsafe-bundle-")
	if err != nil {
		return fmt.Errorf("gitsafe: the temporary repository of a bundle: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmp); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("gitsafe: remove the temporary repository of a bundle: %w", rmErr))
		}
	}()
	if err := r.run(ctx, tmp, nil, "init", "--bare", "--quiet", "--template=", "--object-format="+format, "--ref-format=reftable"); err != nil {
		return err
	}
	info := filepath.Join(tmp, "objects", "info")
	if err := os.MkdirAll(info, 0o700); err != nil {
		return fmt.Errorf("gitsafe: the temporary repository of a bundle: %w", err)
	}
	if err := os.WriteFile(filepath.Join(info, "alternates"), []byte(objects+"\n"), 0o600); err != nil {
		return fmt.Errorf("gitsafe: the temporary repository of a bundle: %w", err)
	}
	if err := r.runInput(ctx, tmp, nil, nil, strings.NewReader(stdin.String()), nil, "update-ref", "--stdin", "-z"); err != nil {
		return err
	}
	return r.BundleCreate(ctx, tmp, file, names, bases)
}

// objectsOf returns the object format of the repository in dir and the
// objects directory of its common git dir, which a linked worktree
// shares with the main one, with one git rev-parse.
func (r *Runner) objectsOf(ctx context.Context, dir string) (format, objects string, err error) {
	out := limitedBuffer{limit: maxHooksPathBytes}
	if err := r.read(ctx, dir, nil, &out, "rev-parse", "--show-object-format", "--path-format=absolute", "--git-common-dir"); err != nil {
		return "", "", err
	}
	answer := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if out.truncated || len(answer) != 2 || !slices.Contains(objectFormats, answer[0]) {
		return "", "", fmt.Errorf("%w: rev-parse answered %q", ErrRefused, out.String())
	}
	common := answer[1]
	switch {
	case !filepath.IsAbs(common) || !utf8.ValidString(common) || strings.ContainsFunc(common, unsafeRune):
		return "", "", fmt.Errorf("%w: the common git dir %q is not an absolute path gitsafe writes", ErrRefused, common)
	}
	objects = filepath.Join(common, "objects")
	if info, err := os.Stat(objects); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%w: %s is not an objects directory", ErrRefused, objects)
	}
	return answer[0], objects, nil
}
