// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import "context"

// The check record-unreadable (04 4.2, REQ-0391): a record of a format
// romeu knows that does not decode, a project record or toolchain.json
// of the state directory, named with its reason. The hint is the one of
// RJ-334: restore the record from a backup, saving the work first.
var _ = register(row{slug: "record-unreadable", run: checkRecordUnreadable})

func checkRecordUnreadable(_ context.Context, h *host) Check {
	const slug = "record-unreadable"
	r, err := h.loadRecords()
	if err != nil {
		return errorOf(slug, err)
	}
	if len(r.unreadable) > 0 {
		return fail(slug, joined(r.unreadable)+"; restore each from a backup, after romeu salvage --from-host <name> saves the work")
	}
	return pass(slug, "")
}
