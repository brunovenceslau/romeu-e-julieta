// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package version

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// maxMacOS is the most bytes of a macOS version the reader takes.
const maxMacOS = 32

// parseMacOS holds the product version of macOS, as the kernel reports
// it, to its form: major.minor or major.minor.patch, each a decimal
// number without a leading zero, such as 15.1 or 14.6.1. The numbers are
// read by golang.org/x/mod/semver (ADR 0038), which takes v15.1 as the
// shorthand of v15.1.0; a prerelease, a build suffix and a bare major
// are refused here. A value that does not parse is the error internal
// (RJ-348): no row of 04 4.4 names that state, which no macOS romeu
// supports reaches, since every one answers kern.osproductversion; the
// value is quoted escaped and cut, so it reaches the report as data.
func parseMacOS(s string) (string, error) {
	v := "v" + s
	if len(s) <= maxMacOS && strings.Contains(s, ".") && semver.IsValid(v) &&
		semver.Prerelease(v) == "" && semver.Build(v) == "" {
		return s, nil
	}
	if len(s) > maxMacOS {
		s = s[:maxMacOS]
	}
	return "", cli.Internal.New(fmt.Sprintf("the macOS version %+q is not major.minor or major.minor.patch", s))
}
