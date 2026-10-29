// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import (
	cmdsalvage "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/salvage"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// The formats of julieta salvage, here so that tools/ci, which reaches
// a command package through this one only (REQ-0971), registers their
// schemas: the salvage manifest, salvage.v1 (03 3.11), and the --json
// document, julieta-salvage.v1.
type (
	// SalvageManifest is the format salvage.v1.
	SalvageManifest = salvage.Manifest
	// SalvageReport is the format julieta-salvage.v1.
	SalvageReport = cmdsalvage.Report
)
