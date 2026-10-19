// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// versionTag is the form of a release tag (10 10.2, Release and
// bootstrap): v<major>.<minor>.<patch>, and the same with a suffix for a
// prerelease.
var versionTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)

// versionFile is the form of the committed VERSION file (REQ-1249):
// <major>.<minor>.<patch> and a newline.
var versionFile = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\n$`)

// CheckVersion checks the --version of a build against the VERSION file
// (REQ-1249): version is a release tag or a prerelease tag, and its
// <major>.<minor>.<patch> is the one VERSION names. It returns whether
// the tag is a prerelease.
func CheckVersion(version string, file []byte) (prerelease bool, err error) {
	m := versionTag.FindStringSubmatch(version)
	if m == nil {
		return false, fmt.Errorf("version %q is not v<major>.<minor>.<patch> with an optional suffix", version)
	}
	if !versionFile.Match(file) {
		return false, errors.New("VERSION holds no <major>.<minor>.<patch> and a newline (REQ-1249)")
	}
	want := string(bytes.TrimSuffix(file, []byte("\n")))
	if got := strings.Join(m[1:4], "."); got != want {
		return false, fmt.Errorf("version %q is not a build of VERSION %s", version, want)
	}
	return m[4] != "", nil
}
