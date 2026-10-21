// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Ref is a full ref name and the object it points at: one ref ListRefs
// lists, or one pair BundleOf bundles.
type Ref struct {
	Name, OID string
}

// listNamespaces are the namespaces ListRefs lists, besides one
// generation of refs/salvage/ per call (salvageNamespace).
var listNamespaces = []string{"refs/heads/", "refs/tags/", "refs/notes/"}

// salvagePrefix is the namespace of the salvage refs of a sandbox clone
// (08 8.5), one directory per generation.
const salvagePrefix = "refs/salvage/"

// ListRefs lists the refs of the repository in dir under the namespaces
// given, sorted by name, each with the object it points at (an
// annotated tag's own object, not its commit), under the flags of the
// mode, with git for-each-ref. The namespaces come from a closed list:
// refs/heads/, refs/tags/, refs/notes/, and refs/salvage/<generation>/
// for one generation per call, each named once; git reads each as a
// plain prefix, since none holds a pattern character. A name
// checkDisplayRef refuses is left out of the refs and returned in
// refused, escaped with termsafe, so a caller keeps every other ref
// (decided while building T060). A name outside the namespaces asked
// for, an id that is not a full object id, and more than maxBranches
// names, the refused ones counted, fail the whole list.
func (r *Runner) ListRefs(ctx context.Context, dir string, namespaces []string) (refs []Ref, refused []string, err error) {
	if err := checkRepo(dir); err != nil {
		return nil, nil, err
	}
	if err := checkListNamespaces(namespaces); err != nil {
		return nil, nil, err
	}
	w := &records{limit: maxStatusRecord, each: func(line string) error {
		if len(refs)+len(refused) == maxBranches {
			return fmt.Errorf("%w: more than %d refs", ErrRefused, maxBranches)
		}
		oid, name, ok := strings.Cut(line, " ")
		if !ok {
			return fmt.Errorf("%w: for-each-ref line %q", ErrRefused, line)
		}
		if err := checkObjectID(oid); err != nil {
			return err
		}
		if !slices.ContainsFunc(namespaces, func(ns string) bool { return strings.HasPrefix(name, ns) }) {
			return fmt.Errorf("%w: ref %q is outside %s", ErrRefused, name, strings.Join(namespaces, ", "))
		}
		if checkDisplayRef(name) != nil {
			refused = append(refused, termsafe.Escape(name))
			return nil
		}
		refs = append(refs, Ref{Name: name, OID: oid})
		return nil
	}}
	args := slices.Concat([]string{"for-each-ref", "--format=%(objectname) %(refname)", "--end-of-options"}, namespaces)
	if err := r.read(ctx, dir, nil, lines{w}, args...); err != nil {
		return nil, nil, err
	}
	if err := w.close(); err != nil {
		return nil, nil, err
	}
	slices.SortFunc(refs, func(a, b Ref) int { return strings.Compare(a.Name, b.Name) })
	return refs, refused, nil
}

// checkListNamespaces returns ErrRefused for a list ListRefs does not
// take: empty, a namespace outside the closed list or named twice, or
// more than one generation of refs/salvage/.
func checkListNamespaces(namespaces []string) error {
	if len(namespaces) == 0 {
		return fmt.Errorf("%w: a list of refs names no namespace", ErrRefused)
	}
	generations := 0
	for i, ns := range namespaces {
		if slices.Contains(namespaces[:i], ns) {
			return fmt.Errorf("%w: namespace %q named twice", ErrRefused, ns)
		}
		if slices.Contains(listNamespaces, ns) {
			continue
		}
		generation, ok := strings.CutPrefix(ns, salvagePrefix)
		generation, slash := strings.CutSuffix(generation, "/")
		if !ok || !slash || generation == "" || strings.ContainsAny(generation, "/*") || checkRefName(ns+"x", false) != nil {
			return fmt.Errorf("%w: namespace %q is not one of %s or %s<generation>/", ErrRefused, ns, strings.Join(listNamespaces, ", "), salvagePrefix)
		}
		generations++
	}
	if generations > 1 {
		return fmt.Errorf("%w: more than one generation of %s in one list", ErrRefused, salvagePrefix)
	}
	return nil
}
