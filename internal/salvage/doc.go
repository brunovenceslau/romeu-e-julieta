// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package salvage holds what both binaries know of the bundles that
// carry a sandbox's unpushed work to the host (08 8.4 and 8.5): the
// refs a snapshot bundles (Pairs, REQ-0688) and the format of
// snapshot/heads.json, its writer's encoder and romeu's strict reader
// (EncodeHeads, DecodeHeads; 03 3.13, REQ-0261); and the sandbox half
// of a salvage (08 8.5 steps 2 to 6): the capture of every worktree
// and stash into refs, the bundles, the archives of the ignored files
// and of the agent's state, and the salvage manifest of each repo,
// salvage.v1 (Salvage, Manifest), with the fingerprint of the same
// reads (ReadFingerprint, Fingerprint).
//
// This package is linked by romeu too (10 10.7), so it imports no
// julieta-only package: julieta's snapshot writer, which takes the
// store lock, is internal/hooks/actions (snapshot.go), and julieta
// salvage passes the agent profile and the store lock's function
// (AgentProfile, LockFunc). A salvage writes its own files, each new in
// a new salvage dir, the manifest last through a rename.
package salvage
