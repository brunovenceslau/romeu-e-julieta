// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"bytes"
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// Floor is the lowest sbx version romeu runs against (12 12.1,
// REQ-1056), as sbx prints it without the leading v: the lowest one the
// parsers and the sessions model. Before v0.48.0 sbx ls --json was the
// object {"sandboxes": [...]} and the approval of a plan was
// --auto-approve, so nothing here reads them. A test holds it to the
// page and to the lowest recorded version.
const Floor = "0.48.0"

// RecordedVersions are the sbx versions with a session set under
// e2e/testdata/sbx/<version>/: the tested window (10 10.3, REQ-0932),
// which every parser is written for and runs over (REQ-0930). A test
// holds the list to the directories.
var RecordedVersions = []string{"0.48.0"}

// maxVersionOutput is the most output of sbx version a parser reads: a
// version line is a few bytes, and a larger output is not one.
const maxVersionOutput = 4 << 10

// Release is the number of an sbx version: major.minor.patch and an
// optional prerelease.
type Release struct {
	Major, Minor, Patch int
	// Pre is the prerelease, such as "rc.1"; empty for a release.
	Pre string
}

// String returns the version as the toolchain record writes it
// (03 3.8): numbers and prerelease, with no leading v and no build
// metadata.
func (v Release) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Compare orders versions by their numbers; a prerelease comes before
// its release, and two prereleases of one release are equal, since
// romeu has no use for their order.
func (v Release) Compare(w Release) int {
	return cmp.Or(
		cmp.Compare(v.Major, w.Major),
		cmp.Compare(v.Minor, w.Minor),
		cmp.Compare(v.Patch, w.Patch),
		cmp.Compare(boolInt(v.Pre == ""), boolInt(w.Pre == "")),
	)
}

// boolInt is 1 for true.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// versionLine is a version as sbx prints it: an optional label (the
// "sbx version " of sbx v0.49.2, or the "Client Version:" that sbx -D
// puts before it), the version with an optional v and no leading zero in
// a number, an optional prerelease and optional build metadata, and an
// optional commit of hex digits, and nothing else.
var versionLine = regexp.MustCompile(`^(?:sbx version |Client Version:\s*)?v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z][0-9A-Za-z.-]*))?(?:\+[0-9A-Za-z.-]+)?(?: [0-9a-f]{7,64})?$`)

// ParseVersion reads the standard output of sbx version: the first
// non-empty line is the version. Any other output is the error
// upstream-shape (RJ-310) and the zero Release.
//
// The shape is documented by Docker only as a command with a --format
// flag (https://docs.docker.com/reference/cli/sbx/version/). The form
// sbx v0.49.2 printed on 2026-10-22 is "sbx version v0.49.2 <commit>"; a
// bare version is read too, and probe A1 records what the pinned sbx
// prints.
func ParseVersion(out []byte) (Release, error) {
	if len(out) > maxVersionOutput {
		return Release{}, upstreamShape("version", "the output is longer than a version line can be")
	}
	for line := range bytes.Lines(out) {
		line := strings.TrimSpace(string(line))
		if line == "" {
			continue
		}
		m := versionLine.FindStringSubmatch(line)
		if m == nil {
			return Release{}, upstreamShape("version", "the first line is not a version")
		}
		major, err1 := strconv.Atoi(m[1])
		minor, err2 := strconv.Atoi(m[2])
		patch, err3 := strconv.Atoi(m[3])
		if err1 != nil || err2 != nil || err3 != nil {
			return Release{}, upstreamShape("version", "a number of the version does not fit")
		}
		return Release{Major: major, Minor: minor, Patch: patch, Pre: m[4]}, nil
	}
	return Release{}, upstreamShape("version", "the output is empty")
}

// floorVersion is Floor as a Release.
var floorVersion = func() Release {
	v, err := ParseVersion([]byte(Floor))
	if err != nil {
		panic("sbxdrv: the floor is not a version: " + err.Error())
	}
	return v
}()

// CheckFloor is the sbx floor check before gate 1 (REQ-0441): a
// version below Floor is the error sbx-floor (RJ-343, exit 2).
func CheckFloor(v Release) error {
	if v.Compare(floorVersion) >= 0 {
		return nil
	}
	return SbxFloor.New(fmt.Sprintf("sbx %s is below the floor %s", v, Floor),
		cli.Detail{Key: "found", Value: v.String()},
		cli.Detail{Key: "floor", Value: Floor})
}

// upstreamShape returns the error upstream-shape for the output of the
// sbx call what: it names sbx and the recorded versions (REQ-0064). A
// caller that knows the live version adds it with withLive.
func upstreamShape(what, why string) *cli.Error {
	return cli.UpstreamShape.New("the output of sbx "+what+" is not a shape romeu reads: "+why,
		cli.Detail{Key: "upstream", Value: "sbx"},
		cli.Detail{Key: "recorded sbx versions", Value: strings.Join(RecordedVersions, ", ")})
}
