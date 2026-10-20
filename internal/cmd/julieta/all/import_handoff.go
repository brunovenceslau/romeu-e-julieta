// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// HandoffReport is the --json document of julieta handoff write and
// list, the format julieta-handoff.v1. It and HandoffFront are here so
// that tools/ci, which reaches a command package through this one only
// (REQ-0971), registers their schemas.
type HandoffReport = handoff.Report

// HandoffFront is the front matter of a handoff file, the format
// handoff.v1 (03 3.10).
type HandoffFront = hf.Front
