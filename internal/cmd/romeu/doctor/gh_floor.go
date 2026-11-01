// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The check gh-floor (04 4.2, REQ-1331): tools.gh unset, so romeu
// upgrade cannot run (warn); set and not an absolute path to an
// executable file under the file rule of the credential helper, or a
// gh whose gh --version is below the floor of 12 12.1 (fail); a gh that
// does not run or prints a line the parser does not read (error). The
// one gh command doctor runs is gh --version (REQ-1320), with the
// scrubbed environment of REQ-0296.
var _ = register(row{slug: "gh-floor", run: checkGhFloor})

// GhFloor is the floor of gh (12 12.1, REQ-1058): the first release
// whose gh attestation verify has --source-digest,
// --deny-self-hosted-runners and --source-ref. TestGhFloorIsTheSpecs
// holds it to the number 12 12.1 states.
const GhFloor = "2.68.0"

// ghFloor is GhFloor as numbers.
var ghFloor = [3]int{2, 68, 0}

func checkGhFloor(ctx context.Context, h *host) Check {
	const slug = "gh-floor"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	gh := h.settings.Tools.Gh
	if gh == "" {
		return warn(slug, "tools.gh is unset, so romeu upgrade cannot run: install gh at or above "+GhFloor+" (the install of J1 step 2) and record its absolute path as tools.gh of the host settings")
	}
	// The settings validation holds tools.gh to an absolute clean path
	// (REQ-1319), so a relative one never reaches here: baseErr says so.
	if info, err := os.Stat(gh); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fail(slug, "tools.gh does not name an executable file: "+termsafe.Escape(gh))
	}
	if err := gitsafe.CheckCredentialHelperFile(gh); err != nil {
		return fail(slug, "tools.gh is refused: "+termsafe.Escape(err.Error()))
	}
	out, err := runGhVersion(ctx, gh, h.home)
	if _, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return errorOf(slug, cli.SubprocessTimeout.Wrap(err, "gh --version did not finish"))
	}
	if ctx.Err() != nil {
		return errorOf(slug, ctx.Err())
	}
	if err != nil {
		return errorOf(slug, fmt.Errorf("gh --version: %w", err))
	}
	v, err := parseGhVersion(out)
	if err != nil {
		return errorOf(slug, err)
	}
	if !atGhFloor(v) {
		return fail(slug, fmt.Sprintf("gh %s is below the floor %s of 12 12.1: romeu upgrade needs the attestation flags of that release", v, GhFloor))
	}
	return pass(slug, fmt.Sprintf("gh %s, floor %s", v, GhFloor))
}

// ghVersionLimit bounds what doctor keeps of gh --version: its first
// line is under 100 bytes.
const ghVersionLimit = 4096

// runGhVersion runs gh --version with the scrubbed environment of
// REQ-0296 (PATH=/usr/bin:/bin, HOME, LANG=C) and returns its standard
// output, cut to ghVersionLimit; the standard error is dropped. The
// call site is admitted by the table of I16 (REQ-1320).
func runGhVersion(ctx context.Context, gh, home string) ([]byte, error) {
	var out bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Gh,
		Argv:   []string{gh, "--version"},
		Dir:    home,
		Env:    []string{"HOME=" + home, secretPath, "LANG=C"},
		Stdout: &limitWriter{w: &out, left: ghVersionLimit},
	})
	return out.Bytes(), err
}

// limitWriter keeps the first left bytes it is given and drops the
// rest, reporting every write as whole.
type limitWriter struct {
	w    *bytes.Buffer
	left int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if n := min(len(p), l.left); n > 0 {
		l.w.Write(p[:n])
		l.left -= n
	}
	return len(p), nil
}

// errGhVersion is the error of an output that is not a gh version line.
var errGhVersion = errors.New("not a gh version line")

// ghVersion is a version of gh: its three numbers and whether a
// prerelease suffix followed them.
type ghVersion struct {
	nums       [3]int
	prerelease bool
}

// String writes the version as gh prints it, the suffix as "-pre".
func (v ghVersion) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.nums[0], v.nums[1], v.nums[2])
	if v.prerelease {
		s += "-pre"
	}
	return s
}

// lineLimit bounds the line an error of the parse quotes.
const lineLimit = 200

// parseGhVersion reads the first line of gh --version, "gh version
// <major>.<minor>.<patch> (<date>)", as gh prints it; a prerelease
// suffix after the patch, as in 2.70.0-pre.1, marks the version a
// prerelease, which is below the release of the same numbers (semver).
func parseGhVersion(out []byte) (ghVersion, error) {
	line, _, _ := strings.Cut(string(out), "\n")
	bad := func() error {
		if len(line) > lineLimit {
			line = line[:lineLimit] + "..."
		}
		return fmt.Errorf("%w: %q", errGhVersion, termsafe.Escape(line))
	}
	rest, ok := strings.CutPrefix(line, "gh version ")
	if !ok {
		return ghVersion{}, bad()
	}
	rest, _, _ = strings.Cut(rest, " ")
	rest, suffix, prerelease := strings.Cut(rest, "-")
	fields := strings.Split(rest, ".")
	if len(fields) != 3 || (prerelease && suffix == "") {
		return ghVersion{}, bad()
	}
	v := ghVersion{prerelease: prerelease}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || f == "" || f[0] == '+' {
			return ghVersion{}, bad()
		}
		v.nums[i] = n
	}
	return v, nil
}

// atGhFloor reports whether v is at or above ghFloor: a prerelease of
// the floor's own numbers is below it.
func atGhFloor(v ghVersion) bool {
	for i := range v.nums {
		if v.nums[i] != ghFloor[i] {
			return v.nums[i] > ghFloor[i]
		}
	}
	return !v.prerelease
}
