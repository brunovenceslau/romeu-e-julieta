// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

// The kinds table of 03 3.12, closed for v1: one Go type per kind, and
// one typed constant of that type, the only value Digest accepts for
// it. A kind is in the type of a digest (Sum), so a digest of one kind
// does not compile where a digest of another goes. A new kind is a
// change to the table of 03 3.12 first, then to this file and to Kind.
type (
	// WideningKind is the kind of the widening set, which gate 2
	// compares.
	WideningKind string
	// RecreateKind is the kind of the fields tagged apply:"recreate",
	// which tell that a recreate is needed.
	RecreateKind string
	// RenderKind is the kind of the normalized render (01 1.4), the
	// widening set's renderDigest.
	RenderKind string
	// EgressKind is the kind of the sorted effective domain union,
	// written to render.json.
	EgressKind string
	// CatalogKind is the kind of the decoded catalog, which gate 1
	// compares.
	CatalogKind string
	// KitTreeKind is the kind of the sorted {path, mode, sha256} of a
	// kit, in the widening set and the kit directory's name.
	KitTreeKind string
	// CandidateFilesKind is the kind of the sorted {path, sha256} of a
	// candidate directory, which promotion recovery compares.
	CandidateFilesKind string
	// RunKind is the kind of the validated run layout, which stale
	// layout detection compares.
	RunKind string
	// LockSetKind is the kind of the sorted {dir, mise.lock sha256},
	// the skip rule of julieta install.
	LockSetKind string
	// SecretArgvKind is the kind of one secret's argv, in the widening
	// set.
	SecretArgvKind string
	// MemoryContentKind is the kind of a memory entry's title, body,
	// tags and status, the duplicate rule of julieta memory add.
	MemoryContentKind string
	// FingerprintKind is the kind of the fingerprint object of 03 3.8,
	// written with the state removing and compared on its rerun.
	FingerprintKind string
)

// The one value of each kind type, in the order of the table of 03
// 3.12.
const (
	Widening       WideningKind       = "widening"        // the widening set
	Recreate       RecreateKind       = "recreate"        // the fields tagged apply:"recreate"
	Render         RenderKind         = "render"          // the normalized render
	Egress         EgressKind         = "egress"          // the sorted effective domain union
	Catalog        CatalogKind        = "catalog"         // the decoded catalog
	KitTree        KitTreeKind        = "kit-tree"        // the sorted files of a kit
	CandidateFiles CandidateFilesKind = "candidate-files" // the sorted files of a candidate directory
	Run            RunKind            = "run"             // the validated run layout
	LockSet        LockSetKind        = "lock-set"        // the sorted mise.lock digests
	SecretArgv     SecretArgvKind     = "secret-argv"     // one secret's argv
	MemoryContent  MemoryContentKind  = "memory-content"  // a memory entry's content
	Fingerprint    FingerprintKind    = "fingerprint"     // the fingerprint object of 03 3.8
)

// Kind is the closed set of the kind types, the constraint of Digest
// and Sum.
type Kind interface {
	WideningKind | RecreateKind | RenderKind | EgressKind | CatalogKind |
		KitTreeKind | CandidateFilesKind | RunKind | LockSetKind |
		SecretArgvKind | MemoryContentKind | FingerprintKind
}

// Input is the constraint on the values of the kind K: a Go type
// declares its one kind with a CanonKind method, so a value of one kind
// does not compile as the input of a digest of another (03 3.12: each
// Go type has exactly one kind).
type Input[K Kind] interface {
	CanonKind() K
}

// kinds is the table of 03 3.12 in its order. A kind's entry is its
// constant, as an interface value, so a lookup matches the type and the
// value both.
var kinds = []any{
	Widening, Recreate, Render, Egress, Catalog, KitTree, CandidateFiles,
	Run, LockSet, SecretArgv, MemoryContent, Fingerprint,
}

// known reports whether k is the one value of its kind type.
func known[K Kind](k K) bool {
	for _, kind := range kinds {
		if kind == any(k) {
			return true
		}
	}
	return false
}
