// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/status"

// StatusReport is the --json document of romeu status, the format
// romeu-status.v1, and StatusRecordStates and StatusToolchainStates the
// values its two closed members admit, here so that tools/ci, which
// reaches a command package through this one only (REQ-0971), registers
// its schema. The import also links the command into romeu.
type StatusReport = status.Report

// StatusRecordStates are the values of the record of a project's block.
var StatusRecordStates = status.RecordStates

// StatusToolchainStates are the values of the state of the toolchain.
var StatusToolchainStates = status.ToolchainStates
