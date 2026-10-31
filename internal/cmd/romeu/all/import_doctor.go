// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/doctor"

// DoctorReport is the --json document of romeu doctor, the format
// romeu-doctor.v1, and DoctorResults the values its result admits, here
// so that tools/ci, which reaches a command package through this one
// only (REQ-0971), registers its schema.
type DoctorReport = doctor.Report

// DoctorResults are the values of the result of a check.
var DoctorResults = doctor.Results
