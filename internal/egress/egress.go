// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package egress derives the domains a sandbox may reach (07 7.5,
// REQ-0057): the catalog's domains for each tool the locks of the repos
// name, the hosts of their artifact URLs, the meta and base sets, the
// spec's egress.extra and egress.tools, and the domains the kits declare,
// split into those that are allowed at once (auto) and those that wait for
// an approval (gated). A domain that can take data out, or that the
// catalog has no upload entry for, is always gated (I10).
//
// The input is the mise.lock of each repo at the commit romeu pinned
// (Reader, GitReader): git cat-file under the file cap, never a work tree,
// and no mise runs on the host (07 7.1, I9). 10 10.7 keeps this package, a
// package of romeu, from importing internal/mise, a package of julieta, so
// it reads the lock with a reader of its own (ParseLock), which a test
// holds to the fixture and to the limits of the one in internal/mise.
//
// The lock, not mise.toml, is the input (07 7.5, deliberate deviation).
// The artifact hosts are read from the url members of the platforms; the
// url_api member, the address of the GitHub API that an aqua download falls
// back to, is not an artifact URL, and api.github.com is in the base set.
// For an http: tool the url of mise.toml beats the url of the lock in
// locked mode, so the host mise contacts can differ from the host Derive
// allows; Derive's comment says why that fails closed.
//
// Everything fails closed and no error comes with a partial result: an
// unknown tool, a lock that is not the pinned layout and a host that is
// not an egress domain are errors of the table (RJ-325, RJ-326, RJ-327,
// RJ-340), and a domain is never allowed because it was not recognised.
package egress
