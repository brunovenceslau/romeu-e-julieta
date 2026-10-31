// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build darwin

package version

import (
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// macOSVersion returns the product version of macOS from the kernel,
// the sysctl kern.osproductversion, the value sw_vers prints as its
// ProductVersion. It starts no program, so romeu still runs sbx and git
// alone (04 4.1, Subprocesses, item 1, REQ-0296). A sysctl that fails
// is the error internal (RJ-348), as parseMacOS says of a value that
// does not parse.
func macOSVersion() (string, error) {
	s, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return "", cli.Internal.Wrap(err, "the sysctl kern.osproductversion cannot be read")
	}
	return parseMacOS(s)
}
