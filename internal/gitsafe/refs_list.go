// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
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
// generation of refs/salvage/ and one repo dir of refs/romeu/origin/
// per call (checkListNamespaces).
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
// refs/heads/, refs/tags/, refs/notes/, refs/salvage/<generation>/ for
// one generation per call, and refs/romeu/origin/<dir>/ for one repo
// dir per call (decided on 2026-10-29, T183), each named once; git
// reads each as a plain prefix, since none holds a pattern character.
// A name checkDisplayRef refuses is left out of the refs and returned
// in refused, clipped and escaped, so a caller keeps every other ref
// (decided while building T060). A name outside the namespaces asked
// for, an id that is not a full object id and more than maxBranches
// names, the refused ones counted, fail the whole list, and git's
// output is not read further; a record over maxRefRecord bytes is one
// refused name, counted in More (decided while building T060).
func (r *Runner) ListRefs(ctx context.Context, dir string, namespaces []string) (refs []Ref, refused Refused, err error) {
	if err := checkRepo(dir); err != nil {
		return nil, Refused{}, err
	}
	if err := checkListNamespaces(namespaces); err != nil {
		return nil, Refused{}, err
	}
	full := func() error {
		if len(refs)+len(refused.Names)+refused.More == maxBranches {
			return fmt.Errorf("%w: more than %d refs", ErrRefused, maxBranches)
		}
		return nil
	}
	w := &refRecords{limit: maxRefRecord, overlong: func() error {
		if err := full(); err != nil {
			return err
		}
		refused.More++
		return nil
	}, each: func(line string) error {
		if err := full(); err != nil {
			return err
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
	err = r.read(ctx, dir, nil, lines{w}, args...)
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

// refRecords splits ListRefs' output into NUL-ended records, as
// records does, with two differences: a record over limit bytes is
// not kept and calls overlong, as one name refused, and the next
// record is read; and an error of each or overlong fails the write, so
// git's output is not read further.
type refRecords struct {
	partial  bytes.Buffer
	over     bool
	limit    int
	each     func(string) error
	overlong func() error
	err      error
}

// Write takes the next part of the output.
func (w *refRecords) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, 0)
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		if !w.over && w.partial.Len()+len(chunk) > w.limit {
			w.over = true
			w.partial.Reset()
		}
		if !w.over {
			w.partial.Write(chunk)
		}
		if i < 0 {
			break
		}
		p = p[i+1:]
		if w.over {
			w.over = false
			w.err = w.overlong()
		} else {
			w.err = w.each(w.partial.String())
			w.partial.Reset()
		}
		if w.err != nil {
			return 0, w.err
		}
	}
	return n, nil
}

// close returns the error of the records, and ErrRefused for output that
// ends inside one.
func (w *refRecords) close() error {
	if w.err == nil && (w.partial.Len() > 0 || w.over) {
		w.err = fmt.Errorf("%w: git's output ends inside a record", ErrRefused)
	}
	return w.err
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
// more than one generation of refs/salvage/ or repo dir of
// refs/romeu/origin/.
func checkListNamespaces(namespaces []string) error {
	if len(namespaces) == 0 {
		return fmt.Errorf("%w: a list of refs names no namespace", ErrRefused)
	}
	generations, dirs := 0, 0
	for i, ns := range namespaces {
		switch {
		case slices.Contains(namespaces[:i], ns):
			return fmt.Errorf("%w: namespace %q named twice", ErrRefused, ns)
		case slices.Contains(listNamespaces, ns):
		case isSegmentNamespace(ns, salvagePrefix):
			generations++
		case isSegmentNamespace(ns, originPrefix):
			dirs++
		default:
			return fmt.Errorf("%w: namespace %q is not one of %s, %s<generation>/ or %s<dir>/", ErrRefused, ns, strings.Join(listNamespaces, ", "), salvagePrefix, originPrefix)
		}
	}
	if generations > 1 {
		return fmt.Errorf("%w: more than one generation of %s in one list", ErrRefused, salvagePrefix)
	}
	if dirs > 1 {
		return fmt.Errorf("%w: more than one repo dir of %s in one list", ErrRefused, originPrefix)
	}
	return nil
}

// isSegmentNamespace reports whether ns is prefix, one segment with no
// / or *, and a /, and a ref name may start with it.
func isSegmentNamespace(ns, prefix string) bool {
	segment, ok := strings.CutPrefix(ns, prefix)
	segment, slash := strings.CutSuffix(segment, "/")
	return ok && slash && segment != "" && !strings.ContainsAny(segment, "/*") && checkRefName(ns+"x", false) == nil
}
