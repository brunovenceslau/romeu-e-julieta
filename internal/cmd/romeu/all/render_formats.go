// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/render"

// RenderRecord is the type of the format render.v1, render.json
// (03 3.7). It is here so that tools/ci, which reaches internal/
// through this package (REQ-0971), registers its schema.
type RenderRecord = render.Record
