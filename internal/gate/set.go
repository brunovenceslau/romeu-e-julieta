// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package gate is gate 2 of 01 1.4, the per-project widening gate: the
// widening set and its digest, the recreate digest, the diff the
// operator reads before approving, and the prompt on a terminal.
//
// Which fields of a project spec are gated, and which apply only at
// create, is declared once as the gate and apply struct tags of
// internal/spec; "go generate ./..." writes the field lists of this
// package (fields_gen.go) and its I7 flip table (flip_gen_test.go)
// from them, and fails on a field that is neither tagged nor on the
// committed list of ungated fields (01 1.4, REQ-0067, I7).
//
// The gate fails closed: an input it cannot account for, such as a
// kit file whose content does not match the digest in the set, is an
// error, never a smaller diff.
package gate

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
)

// Set is the widening set of 01 1.4: what gate 2 compares with the
// approval record, by its digest (canon.Widening). The lists the table
// of 01 1.4 calls sorted are sorted by Digest, and the kits keep their
// order.
type Set struct {
	// Project is the project name.
	Project string `json:"project"`
	// Repos are the project's repositories, sorted by dir.
	Repos []Repo `json:"repos"`
	// Secrets are the project's secrets, sorted by name; each holds the
	// digest of the host-settings argv behind it.
	Secrets []Secret `json:"secrets"`
	// Workload is the workload image and its capabilities.
	Workload Workload `json:"workload"`
	// Kits are the kits of the sandbox, in order.
	Kits []Kit `json:"kits"`
	// EgressGated are the gated domains, sorted, each with its reason.
	EgressGated []GatedDomain `json:"egressGated"`
	// Ports are the published port specs, sorted.
	Ports []string `json:"ports"`
	// Agent is the agent handle.
	Agent string `json:"agent"`
	// SalvageExcludes is salvage.excludeIgnored, sorted: it defines
	// acknowledged loss.
	SalvageExcludes []string `json:"salvageExcludes"`
	// RenderDigest is the digest of the normalized render (RenderDigest).
	RenderDigest canon.Sum[canon.RenderKind] `json:"renderDigest"`
}

// CanonKind declares the one kind of a widening set.
func (Set) CanonKind() canon.WideningKind { return canon.Widening }

// Repo is one repository of the widening set.
type Repo struct {
	// Dir is the repository's directory in the project.
	Dir string `json:"dir"`
	// URL is the repository's origin.
	URL string `json:"url"`
	// Primary marks the primary repository.
	Primary bool `json:"primary"`
	// Ref is the branch egress derives from, empty for the origin's
	// HEAD.
	Ref string `json:"ref"`
}

// Secret is one secret of the widening set: its name, the sbx service
// and the digest of the host-settings argv that fetches its value.
type Secret struct {
	// Name is the secret's name in the spec.
	Name string `json:"name"`
	// Service is the sbx service name.
	Service string `json:"service"`
	// ArgvDigest is SecretArgvDigest of the argv.
	ArgvDigest canon.Sum[canon.SecretArgvKind] `json:"argvDigest"`
}

// Workload is the workload of the widening set.
type Workload struct {
	// Repository is the image repository, <registry>/<repository>.
	Repository string `json:"repository"`
	// Digest is the image digest, sha256:<hex>.
	Digest string `json:"digest"`
	// Capabilities is the normalized capability set read from the
	// workload descriptor.
	Capabilities oci.Capabilities `json:"capabilities"`
}

// The sources of a kit.
const (
	// SourceProduct is a product kit of the romeu build.
	SourceProduct = "product"
	// SourcePersonal is a kit of the config repo's kits/ directory.
	SourcePersonal = "personal"
)

// Kit is one kit of the widening set.
type Kit struct {
	// ID is the kit's name.
	ID string `json:"id"`
	// Source is SourceProduct or SourcePersonal.
	Source string `json:"source"`
	// ContentDigest is KitTreeDigest of the kit's files.
	ContentDigest canon.Sum[canon.KitTreeKind] `json:"contentDigest"`
	// Args are the kit's arguments.
	Args map[string]string `json:"args"`
	// Capabilities is the normalized capability set of the kit's
	// descriptor.
	Capabilities oci.Capabilities `json:"capabilities"`
}

// key is what names a kit in the set and in a diff: its source and id.
func (k Kit) key() string { return k.Source + ":" + k.ID }

// The reasons a domain is gated (01 1.4).
const (
	ReasonUpload           = "upload"
	ReasonNonCatalogLock   = "non-catalog lock host"
	ReasonSpecExtra        = "spec extra"
	ReasonSpecToolOverride = "spec tool override"
	ReasonKitDeclared      = "kit-declared"
)

// GatedDomain is one gated domain and why it is gated.
type GatedDomain struct {
	// Domain is the domain name.
	Domain string `json:"domain"`
	// Reason is one of the Reason constants.
	Reason string `json:"reason"`
}

// Digest returns the digest of the widening set s, canon.Widening over
// its normal form (normal). It does not change s.
func Digest(s Set) (canon.Sum[canon.WideningKind], error) {
	n, err := normal(s)
	if err != nil {
		return canon.Sum[canon.WideningKind]{}, err
	}
	return canon.Digest(canon.Widening, n)
}

// Normal returns a copy of s in its normal form (normal), the form
// Digest digests and the approval record holds.
func Normal(s Set) (Set, error) { return normal(s) }

// normal returns a copy of s in its one form: the sorted lists sorted,
// an absent list, map or capability set as an empty one, so that two
// sets that mean the same have one digest. It refuses a set that names
// one key twice (a repo dir, a secret name, a kit, a gated domain for
// one reason, a port, an exclude) or a kit of an unknown source: such a
// set has no one meaning to approve.
func normal(s Set) (Set, error) {
	n := s
	n.Repos = sortedCopy(s.Repos, func(a, b Repo) int { return cmp.Compare(a.Dir, b.Dir) })
	n.Secrets = sortedCopy(s.Secrets, func(a, b Secret) int { return cmp.Compare(a.Name, b.Name) })
	n.EgressGated = sortedCopy(s.EgressGated, func(a, b GatedDomain) int {
		return cmp.Or(cmp.Compare(a.Domain, b.Domain), cmp.Compare(a.Reason, b.Reason))
	})
	n.Ports = sortedCopy(s.Ports, cmp.Compare[string])
	n.SalvageExcludes = sortedCopy(s.SalvageExcludes, cmp.Compare[string])
	n.Kits = make([]Kit, len(s.Kits))
	for i, k := range s.Kits {
		if k.Source != SourceProduct && k.Source != SourcePersonal {
			return Set{}, fmt.Errorf("gate: the kit %q has the source %q; it is %q or %q", k.ID, k.Source, SourceProduct, SourcePersonal)
		}
		k.Args = mapCopy(k.Args)
		n.Kits[i] = k
	}
	dups := []struct {
		what string
		dup  string
	}{
		{"repo dir", duplicate(n.Repos, func(r Repo) string { return r.Dir })},
		{"secret", duplicate(n.Secrets, func(s Secret) string { return s.Name })},
		{"kit", duplicate(slices.SortedFunc(slices.Values(n.Kits), func(a, b Kit) int { return cmp.Compare(a.key(), b.key()) }), Kit.key)},
		{"gated domain", duplicate(n.EgressGated, func(g GatedDomain) string { return g.Domain + " (" + g.Reason + ")" })},
		{"port", duplicate(n.Ports, func(p string) string { return p })},
		{"salvage exclude", duplicate(n.SalvageExcludes, func(p string) string { return p })},
	}
	for _, d := range dups {
		if d.dup != "" {
			return Set{}, fmt.Errorf("gate: the widening set names the %s %q twice", d.what, d.dup)
		}
	}
	return n, nil
}

// sortedCopy returns a sorted copy of s, empty and not nil when s is.
func sortedCopy[E any](s []E, compare func(a, b E) int) []E {
	out := append(make([]E, 0, len(s)), s...)
	slices.SortStableFunc(out, compare)
	return out
}

// mapCopy returns a shallow copy of m, empty and not nil when m is.
func mapCopy[M ~map[string]V, V any](m M) M {
	out := make(M, len(m))
	maps.Copy(out, m)
	return out
}

// duplicate returns the first key of the sorted list s that two
// neighbours share, or "".
func duplicate[E any](s []E, key func(E) string) string {
	for i := 1; i < len(s); i++ {
		if key(s[i-1]) == key(s[i]) {
			return key(s[i])
		}
	}
	return ""
}

// The modes a kit file may have (I27): a regular file and an
// executable one, as git writes them.
const (
	ModeRegular    = "100644"
	ModeExecutable = "100755"
)

// File is one file of a kit: its slash-separated path in the kit, its
// git mode and its bytes.
type File struct {
	Path string
	Mode string
	Data []byte
}

// kitTreeEntry is one entry of the kit-tree digest's input.
type kitTreeEntry struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}

// kitTree is the input of the kit-tree digest, sorted by path.
type kitTree []kitTreeEntry

// CanonKind declares the one kind of a kit tree.
func (kitTree) CanonKind() canon.KitTreeKind { return canon.KitTree }

// errKitFile is the error of a kit file the tree cannot hold.
var errKitFile = errors.New("gate: a kit file")

// KitTreeDigest returns the digest of a kit's files (03 3.12): the
// kit-tree digest of the sorted {path, mode, sha256}. It refuses an
// empty path, one path twice and a mode other than ModeRegular and
// ModeExecutable, which the walk of I27 has refused before.
func KitTreeDigest(files []File) (canon.Sum[canon.KitTreeKind], error) {
	tree := make(kitTree, 0, len(files))
	for _, f := range files {
		if f.Path == "" {
			return canon.Sum[canon.KitTreeKind]{}, fmt.Errorf("%w has no path", errKitFile)
		}
		if f.Mode != ModeRegular && f.Mode != ModeExecutable {
			return canon.Sum[canon.KitTreeKind]{}, fmt.Errorf("%w %q has the mode %q; a kit holds regular files of the modes %s and %s only",
				errKitFile, f.Path, f.Mode, ModeRegular, ModeExecutable)
		}
		sum := sha256.Sum256(f.Data)
		tree = append(tree, kitTreeEntry{Path: f.Path, Mode: f.Mode, SHA256: hex.EncodeToString(sum[:])})
	}
	slices.SortFunc(tree, func(a, b kitTreeEntry) int { return cmp.Compare(a.Path, b.Path) })
	if d := duplicate(tree, func(e kitTreeEntry) string { return e.Path }); d != "" {
		return canon.Sum[canon.KitTreeKind]{}, fmt.Errorf("%w %q is listed twice", errKitFile, d)
	}
	return canon.Digest(canon.KitTree, tree)
}

// secretArgv is one secret's argv, the input of the secret-argv digest.
type secretArgv []string

// CanonKind declares the one kind of a secret's argv.
func (secretArgv) CanonKind() canon.SecretArgvKind { return canon.SecretArgv }

// SecretArgvDigest returns the digest of one secret's host-settings
// argv (03 3.12). An empty argv is refused: a secret has a command.
func SecretArgvDigest(argv []string) (canon.Sum[canon.SecretArgvKind], error) {
	if len(argv) == 0 {
		return canon.Sum[canon.SecretArgvKind]{}, errors.New("gate: a secret's argv is empty")
	}
	return canon.Digest(canon.SecretArgv, secretArgv(argv))
}
