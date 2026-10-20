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
		hint := fmt.Sprintf("go fix would change the files above for %s; run \"GOOS=%s GOARCH=%s CGO_ENABLED=0 go fix -tags %s ./...\" and commit what it writes",
			target, target.GOOS, target.GOARCH, lint.BuildTags)
		steps = append(steps, check.Step{
			Name:    "fix " + target.String(),
			Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "fix", "-diff", "-tags", lint.BuildTags, "./..."},
			Environ: lint.Env(target, p.Tools.GoDir),
			Around: func() ([]string, check.Read, error) {
				return nil, func(out []byte, runErr error) ([]byte, error) { return Read(out, runErr, hint) }, nil
			},
		})
	}
	return steps
}

// Read makes the verdict of one run of "go fix -diff": a run that
// printed a diff fails, whatever its exit status, with the diff sorted
// by file and clipped to MaxLines, and the hint, which names the
// command that applies the fixes; any other run keeps its output and
// its error, so a package that does not build fails with what the go
// command said. go fix prints the files in the order its packages end,
// which changes from run to run, so the sort makes the output of one
// tree the same on every run.
func Read(out []byte, runErr error, hint string) ([]byte, error) {
	head, files := split(out)
	if len(files) == 0 {
		return out, runErr
	}
	slices.SortStableFunc(files, func(a, b fileDiff) int { return strings.Compare(a.name, b.name) })
	var lines [][]byte
	lines = append(lines, head...)
	for _, f := range files {
		lines = append(lines, f.lines...)
	}
	return clip(lines, files), errors.New(hint)
}

// fileDiff is the part of the diff of one file: its name, and its
// lines from its "--- " line on, each with its newline.
type fileDiff struct {
	name  string
	lines [][]byte
}

// split returns the lines before the first file of the diff, and the
// diff of each file in the order printed. A file starts at a line
// "--- <path> (old)" followed by "+++ <path> (new)", the header go fix
// writes, so a removed line that starts with "-- " starts none.
func split(out []byte) (head [][]byte, files []fileDiff) {
	lines := slices.Collect(bytes.Lines(out))
	for i, line := range lines {
		if name, ok := header(lines, i); ok {
			files = append(files, fileDiff{name: name})
		}
		if n := len(files); n > 0 {
			files[n-1].lines = append(files[n-1].lines, line)
		} else {
			head = append(head, line)
		}
	}
	return head, files
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

// clip joins the lines, the first MaxLines of them when there are
// more, followed by a line that says how many were left out and the
// files the diff changes, one per line.
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
