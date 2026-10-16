// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/vulnerabilities"
)

func init() {
	check.Register(check.Check{
		Name:    "vulnerabilities",
		Guards:  "no known vulnerability of the Go vulnerability database reaches the module",
		Row:     "vulnerabilities",
		Req:     "REQ-0831",
		Usage:   "govulncheck ./...",
		All:     true,
		Order:   50,
		Package: "tools/ci/internal/vulnerabilities",
		Steps:   vulnerabilities.Steps,
	})
}
