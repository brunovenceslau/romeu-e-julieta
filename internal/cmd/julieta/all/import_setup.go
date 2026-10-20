// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"

// SetupReport is the --json document of julieta setup, the format
// julieta-setup.v1, here so that tools/ci, which reaches a command
// package through this one only (REQ-0971), registers its schema.
type SetupReport = setup.Report
