// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// version is a git release: major.minor.patch, and whether it is a
// release candidate of it.
type version struct {
	major, minor, patch int
	rc                  bool
}

// String returns the parsed version the floor compares: major.minor.patch,
// with ".rc" for a release candidate and no rc number or build suffix, so
// it is not always the text git prints (Version returns that).
func (v version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.rc {
		s += ".rc"
	}
	return s
}

// compare orders versions; a release candidate comes before its
// release.
func (v version) compare(w version) int {
	return cmp.Or(
		cmp.Compare(v.major, w.major),
		cmp.Compare(v.minor, w.minor),
		cmp.Compare(v.patch, w.patch),
		// rc false sorts after rc true.
		-cmp.Compare(boolInt(v.rc), boolInt(w.rc)),
	)
}

// boolInt is 1 for true.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// floor is the host git floor of 05 5.1: the latest patched point
// release of each supported series, with the initial floor 2.45.4
// below every series. Probe A1 verifies the exact per-series minimums
// and the CVEs they close, and T027 records them here, one row per
// series, each with its sources; a git whose series has no row must
// reach the initial floor.
//
// Source of the initial floor: 05 5.1 (REQ-0469).
var floor = struct {
	initial version
	series  []version // the minimum of each series, one per major.minor
}{
	initial: version{2, 45, 4, false},
	series:  []version{{2, 45, 4, false}},
}

// Floor returns the initial floor, as git writes a version.
func Floor() string {
	return floor.initial.String()
}

// atFloor reports whether v is at or above the floor of its series.
func atFloor(v version) bool {
	if v.compare(floor.initial) < 0 {
		return false
	}
	for _, least := range floor.series {
		if least.major == v.major && least.minor == v.minor {
			return v.compare(least) >= 0
		}
	}
	return true
}

// errVersion is the error of an output that is not a git version line.
var errVersion = errors.New("gitsafe: not a git version line")

// parseVersion reads the first line of git version: "git version
// 2.53.0", with an optional build suffix such as ".windows.1" or "
// (Apple Git-154)", or a release candidate such as "2.46.0.rc1".
func parseVersion(out string) (version, error) {
	line, _, _ := strings.Cut(out, "\n")
	rest, ok := strings.CutPrefix(line, "git version ")
	if !ok {
		return version{}, fmt.Errorf("%w: %q", errVersion, line)
	}
	rest, _, _ = strings.Cut(rest, " ")
	fields := strings.Split(rest, ".")
	if len(fields) < 3 {
		return version{}, fmt.Errorf("%w: %q", errVersion, line)
	}
	// A release candidate is 2.46.0.rc1, or 2.46.0-rc1.
	patch, suffix, _ := strings.Cut(fields[2], "-")
	fields[2] = patch
	var nums [3]int
	for i := range nums {
		n, err := strconv.Atoi(fields[i])
		if err != nil || strings.TrimLeft(fields[i], "0123456789") != "" {
			return version{}, fmt.Errorf("%w: %q", errVersion, line)
		}
		nums[i] = n
	}
	rc := strings.HasPrefix(suffix, "rc") || (len(fields) > 3 && strings.HasPrefix(fields[3], "rc"))
	return version{nums[0], nums[1], nums[2], rc}, nil
}

// foundLimit is the most bytes of git's version line a FloorError
// quotes.
const foundLimit = 200

// quoteVersion returns the first line of out, cut to foundLimit bytes
// and escaped with termsafe, for a version CheckFloor cannot read.
func quoteVersion(out string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	if len(line) > foundLimit {
		line = line[:foundLimit]
	}
	return termsafe.Escape(line)
}

// CheckFloor runs git version and returns the error git-floor
// (*FloorError, RJ-344) when git is below the floor of 05 5.1, or when
// its version cannot be read: a version gitsafe cannot read is not one
// it can trust. A command that starts git checks it before gate 1
// (04 4.2).
func (r *Runner) CheckFloor(ctx context.Context) error {
	v, _, err := r.readVersion(ctx)
	if err != nil {
		return err
	}
	if !atFloor(v) {
		return &FloorError{Found: v.String()}
	}
	return nil
}
