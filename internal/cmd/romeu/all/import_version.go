// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/version"

// VersionReport is the --json document of romeu version, the format
// romeu-version.v1, here so that tools/ci, which reaches a command
// package through this one only (REQ-0971), registers its schema. The
// import also links the command into romeu.
type VersionReport = version.Report
