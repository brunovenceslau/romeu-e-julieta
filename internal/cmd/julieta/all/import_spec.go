// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/spec"

// SpecReport is the --json document of julieta spec validate, the
// format julieta-spec.v1, here so that tools/ci, which reaches a command
// package through this one only (REQ-0971), registers its schema.
type SpecReport = spec.Report
