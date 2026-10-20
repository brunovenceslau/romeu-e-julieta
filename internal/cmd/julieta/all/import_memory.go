// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/memory"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// MemoryReport is the --json document of julieta memory add, list,
// show and edit, the format julieta-memory.v1. It and MemoryEntry are
// here so that tools/ci, which reaches a command package through this
// one only (REQ-0971), registers their schemas.
type MemoryReport = memory.Report

// MemoryEntry is the front matter of a memory entry, the format
// memory-entry.v1 (03 3.9).
type MemoryEntry = memstore.Entry
