// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package fix is the fix step of 10 10.2: "go fix -diff" over the whole
// module, the go command that mise resolves, once per lint target and
// with the tags of the tagged suites, as the lint step reads the module
// (lint.Targets, lint.BuildTags). The fixers are those of the pinned
// toolchain ("go tool fix help"). The step reports the changes they
// would make and never makes them: the fix is to run the command the
// failure names and commit what it writes.
//
// A file builds for one target at least, with those tags (the test of
// the lint package over Targets holds that), so a file that go fix
// would change for one platform alone fails the step of that platform
// on every machine, whatever machine runs it.
package fix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
)

// MaxLines is the number of lines of the diff a failing step shows; a
// longer diff is clipped, and the step then lists every file the diff
// changes.
const MaxLines = 200

// Steps returns one fix step per lint target.
func Steps(_ context.Context, p check.Plan) []check.Step {
	var steps []check.Step
	for _, target := range lint.Targets {
		steps = append(steps, check.Step{
			Name:    "fix " + target.String(),
			Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "fix", "-diff", "-tags", lint.BuildTags, "./..."},
			Environ: lint.Env(target, p.Tools.GoDir),
			Around: func() ([]string, check.Read, error) {
				return nil, func(out []byte, runErr error) ([]byte, error) { return Read(out, runErr, target) }, nil
			},
		})
	}
	return steps
}

// Hint is the error of a step whose run printed a diff of n files for
// the target: it names the command that applies the fixes.
func Hint(target lint.Target, n int) string {
	return fmt.Sprintf("go fix would change %d file(s) for %s; run \"GOOS=%s GOARCH=%s CGO_ENABLED=0 go fix -tags %s ./...\" and commit what it writes",
		n, target, target.GOOS, target.GOARCH, lint.BuildTags)
}

// Read makes the verdict of one run of "go fix -diff" for the target.
// The go command prints the diff and its own messages (a "# <package>"
// line and the errors below it, a line of go or of the fix tool) to one
// stream, so Read takes every line that is no line of a diff out of the
// diff and prints those lines first, whole. A run that printed a diff
// fails, whatever its exit status, with those lines, the diff sorted by
// file and clipped to MaxLines, and Hint; when the go command printed
// other lines and failed too, the error says so, so a package that does
// not build is not hidden behind the diff. A run without a diff keeps
// its output and its error. go fix prints the files in the order its
// packages end, which changes from run to run, so the sort makes the
// output of one tree the same on every run.
func Read(out []byte, runErr error, target lint.Target) ([]byte, error) {
	other, files := split(out)
	if len(files) == 0 {
		return out, runErr
	}
	slices.SortStableFunc(files, func(a, b fileDiff) int { return strings.Compare(a.name, b.name) })
	var lines [][]byte
	for _, f := range files {
		lines = append(lines, f.lines...)
	}
	var b bytes.Buffer
	for _, l := range other {
		b.Write(l)
	}
	if len(other) > 0 && !bytes.HasSuffix(b.Bytes(), []byte("\n")) {
		b.WriteByte('\n')
	}
	b.Write(clip(lines, files))
	hint := Hint(target, len(files))
	if len(other) > 0 && runErr != nil {
		return b.Bytes(), fmt.Errorf("%s; go also printed the lines above the diff and failed: %w", hint, runErr)
	}
	return b.Bytes(), errors.New(hint)
}

// fileDiff is the part of the diff of one file: its name, and its
// lines from its "--- " line on, each with its newline.
type fileDiff struct {
	name  string
	lines [][]byte
}

// split returns the lines that belong to no diff, and the diff of each
// file in the order printed. A file starts at a line "--- <path> (old)"
// followed by "+++ <path> (new)", the header go fix writes, so a
// removed line that starts with "-- " starts none. A line of the diff
// of a file is its "+++ " header or a line of a hunk, which starts with
// "@@ ", " ", "+", "-" or "\\" (a line with no newline at the end of
// the file); any other line belongs to no diff.
func split(out []byte) (other [][]byte, files []fileDiff) {
	lines := slices.Collect(bytes.Lines(out))
	for i, line := range lines {
		if name, ok := header(lines, i); ok {
			files = append(files, fileDiff{name: name})
		}
		if n := len(files); n > 0 && inDiff(line) {
			files[n-1].lines = append(files[n-1].lines, line)
		} else {
			other = append(other, line)
		}
	}
	return other, files
}

// inDiff reports whether the line can be a line of the diff of a file.
func inDiff(line []byte) bool {
	for _, prefix := range []string{"@@ ", " ", "+", "-", "\\"} {
		if bytes.HasPrefix(line, []byte(prefix)) {
			return true
		}
	}
	return false
}

// header reports whether lines[i] starts the diff of a file, and the
// file's path.
func header(lines [][]byte, i int) (string, bool) {
	old, ok := bytes.CutPrefix(bytes.TrimRight(lines[i], "\n"), []byte("--- "))
	if !ok || i+1 >= len(lines) {
		return "", false
	}
	name, ok := bytes.CutSuffix(old, []byte(" (old)"))
	if !ok {
		return "", false
	}
	want := "+++ " + string(name) + " (new)"
	return string(name), string(bytes.TrimRight(lines[i+1], "\n")) == want
}

// clip joins the lines of the diff, the first MaxLines of them when
// there are more, followed by a line that says how many were left out
// and the files the diff changes, one per line.
func clip(lines [][]byte, files []fileDiff) []byte {
	var b bytes.Buffer
	for _, l := range lines[:min(len(lines), MaxLines)] {
		b.Write(l)
	}
	if len(lines) <= MaxLines {
		return b.Bytes()
	}
	if !bytes.HasSuffix(b.Bytes(), []byte("\n")) {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "(clipped: %d more lines of the diff; it changes these %d files)\n", len(lines)-MaxLines, len(files))
	for _, f := range files {
		b.WriteString(f.name + "\n")
	}
	return b.Bytes()
}
