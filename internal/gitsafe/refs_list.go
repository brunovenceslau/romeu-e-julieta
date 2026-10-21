// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

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

// Refused are the ref names a list left out (ListRefs): at most
// maxRefusedNames of them, each clipped to maxRefusedName bytes and
// escaped with termsafe, and the number of the others.
type Refused struct {
	Names []string
	More  int
}

// The bounds of the refused names and of one record of ListRefs.
const (
	maxRefusedNames = 100
	maxRefusedName  = 128
	// maxRefRecord is the longest "<oid> <name>" record ListRefs reads,
	// a SHA-256 id and a name of 4 KiB.
	maxRefRecord = 64 + 1 + 4<<10
)

// ListRefs lists the refs of the repository in dir under the namespaces
// given, sorted by name, each with the object it points at (an
// annotated tag's own object, not its commit), under the flags of the
// mode, with git for-each-ref. The namespaces come from a closed list:
// refs/heads/, refs/tags/, refs/notes/, and refs/salvage/<generation>/
// for one generation per call, each named once; git reads each as a
// plain prefix, since none holds a pattern character. A name
// checkDisplayRef refuses is left out of the refs and returned in
// refused, clipped and escaped, so a caller keeps every other ref
// (decided while building T060). A name outside the namespaces asked
// for, an id that is not a full object id, a record over maxRefRecord
// bytes and more than maxBranches names, the refused ones counted, fail
// the whole list.
func (r *Runner) ListRefs(ctx context.Context, dir string, namespaces []string) (refs []Ref, refused Refused, err error) {
	if err := checkRepo(dir); err != nil {
		return nil, Refused{}, err
	}
	if err := checkListNamespaces(namespaces); err != nil {
		return nil, Refused{}, err
	}
	w := &records{limit: maxRefRecord, each: func(line string) error {
		if len(refs)+len(refused.Names)+refused.More == maxBranches {
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
			if len(refused.Names) == maxRefusedNames {
				refused.More++
			} else {
				refused.Names = append(refused.Names, termsafe.Escape(clipName(name)))
			}
			return nil
		}
		refs = append(refs, Ref{Name: name, OID: oid})
		return nil
	}}
	args := slices.Concat([]string{"for-each-ref", "--format=%(objectname) %(refname)", "--end-of-options"}, namespaces)
	err = r.read(ctx, dir, nil, lines{stopOnRefusal{w}}, args...)
	if w.err != nil {
		return nil, Refused{}, w.err
	}
	if err != nil {
		return nil, Refused{}, err
	}
	if err := w.close(); err != nil {
		return nil, Refused{}, err
	}
	slices.SortFunc(refs, func(a, b Ref) int { return strings.Compare(a.Name, b.Name) })
	return refs, refused, nil
}

// stopOnRefusal passes git's output to w and fails the write once w
// has refused a record, so git's output is not read further.
type stopOnRefusal struct{ w *records }

// Write passes p on, and returns the refusal of w once there is one.
func (s stopOnRefusal) Write(p []byte) (int, error) {
	n, _ := s.w.Write(p)
	if s.w.err != nil {
		return 0, s.w.err
	}
	return n, nil
}

// clipName returns name cut to maxRefusedName bytes, on a character's
// start, with "..." when it was cut.
func clipName(name string) string {
	if len(name) <= maxRefusedName {
		return name
	}
	cut := maxRefusedName
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return name[:cut] + "..."
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
