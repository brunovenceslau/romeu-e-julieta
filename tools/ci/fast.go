// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// stepTimeout bounds one command step. The unit tests are the longest,
// and a cold build cache is the slow case this leaves room for.
const stepTimeout = 15 * time.Minute

// step is one check of fast that is a command.
type step struct {
	name string
	argv []string
	// quiet marks a command whose output is its finding: it passes
	// when it prints nothing, as "gofmt -l" does.
	quiet bool
}

// fastSteps returns the command steps of fast, from the In fast column
// of 10 10.2: format, vet and unit. The other rows of that column join
// with the code they check. The unit step names the two directories
// the column names, and leaves out one that does not exist yet, which
// "go test" would take as an error.
func fastSteps(root string) []step {
	unit := []string{"go", "test"}
	for _, dir := range []string{"internal", "tools"} {
		if info, err := os.Stat(filepath.Join(root, dir)); err == nil && info.IsDir() {
			unit = append(unit, "./"+dir+"/...")
		}
	}
	return []step{
		{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true},
		{name: "vet", argv: []string{"go", "vet", "./..."}},
		{name: "unit", argv: unit},
	}
}

// run runs the step in dir and returns its output. A nil env is the
// environment of this process.
func (s step) run(ctx context.Context, dir string, env []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, stepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.argv[0], s.argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil && s.quiet && len(bytes.TrimSpace(out)) > 0 {
		err = errors.New("the command printed what it found")
	}
	return out, err
}

// runFast runs the fast checks: the command steps, then hygiene. Called
// with the two arguments git gives a pre-push hook, it also reads the
// pushed range from standard input. Every check runs, so one run shows
// everything there is to fix.
func runFast(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 && len(args) != 2 {
		return false, fmt.Errorf("fast takes no argument, or the two of a pre-push hook\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	steps := e.steps
	if steps == nil {
		steps = fastSteps(root)
	}
	ok := true
	report := func(name string, passed bool, detail string) {
		verdict := "ok  "
		if !passed {
			verdict, ok = "FAIL", false
		}
		say(e.stdout, "%s  %s\n%s", verdict, name, detail)
	}
	for _, s := range steps {
		out, err := s.run(ctx, root, nil)
		detail := ""
		if err != nil {
			detail = fmt.Sprintf("%s%v\n", out, err)
		}
		report(s.name, err == nil, detail)
	}

	findings, err := hygiene(ctx, e.repo())
	if err != nil {
		return false, err
	}
	report("hygiene", len(findings) == 0, lines(findings))

	if len(args) == 2 {
		findings, checked, err := pushedRange(ctx, e, args[0])
		switch {
		case err != nil:
			return false, err
		case !checked:
			say(e.stdout, "--    pushed range: not checked, no denylist entry at HEAD or at a pushed tip\n")
		default:
			report("pushed range", len(findings) == 0, lines(findings))
		}
	}
	return ok, nil
}

func lines(findings []finding) string {
	var b bytes.Buffer
	for _, f := range findings {
		fmt.Fprintln(&b, f)
	}
	return b.String()
}

// pushedRange applies the name matcher to what a push would publish:
// the remote ref names, the text of each pushed annotated tag and the
// four readings of each pushed commit.
//
// The push is judged by the denylist at HEAD together with the one at
// each pushed tip. A push need not come from the branch that is checked
// out, and the entries a pushed branch adds are part of what it
// publishes, so they hold for it. A tip without a denylist adds
// nothing; a tip whose denylist cannot be read stops the push. A
// finding says which of the two lists holds the name. Without an entry
// in any of them there is nothing to match with, and it reports that it
// checked nothing.
func pushedRange(ctx context.Context, e env, remote string) (findings []finding, checked bool, err error) {
	push, err := pushed.Parse(e.stdin)
	if err != nil {
		return nil, false, err
	}
	// A list that is missing or empty at HEAD is hygiene's finding.
	head, _, err := loadDenylist(ctx, e.repo())
	if err != nil {
		return nil, false, err
	}
	list := &names.List{}
	list.Merge(head)
	for _, tip := range push.Tips() {
		at, _, err := denylistAt(ctx, e.repo(), tip)
		if err != nil {
			return nil, false, fmt.Errorf("%s at pushed tip %s: %w", names.Path, tip, err)
		}
		list.Merge(at)
	}
	if list.Len() == 0 {
		return nil, false, nil
	}
	err = pushed.Walk(ctx, e.repo(), remote, push, func(r pushed.Reading) {
		if !list.Match(r.Text) {
			return
		}
		listed := "a name listed at HEAD"
		if !head.Match(r.Text) {
			listed = "a name listed at a pushed tip"
		}
		of := "commit " + r.Commit + ": "
		if r.Tag != "" {
			of = "tag " + r.Tag + ": "
		}
		if r.Mergetag > 0 {
			of += "mergetag " + strconv.Itoa(r.Mergetag) + " "
		}
		switch r.Field {
		case pushed.FieldRef:
			findings = append(findings, finding{"pushed ref " + strconv.Itoa(r.Line), "name", "the remote ref name holds " + listed})
		case pushed.FieldMessage:
			findings = append(findings, finding{of + "message line " + strconv.Itoa(r.Line), "name", "the line holds " + listed})
		case pushed.FieldAuthor, pushed.FieldCommitter, pushed.FieldTagger:
			findings = append(findings, finding{of + r.Field, "name", "the name and email hold " + listed})
		case pushed.FieldTagName:
			findings = append(findings, finding{of + r.Field, "name", "the tag's name holds " + listed})
		case pushed.FieldPath:
			findings = append(findings, finding{of + "added path (withheld)", "name", "the path holds " + listed})
		case pushed.FieldLine:
			path := git.Printable(r.Path)
			if list.Match([]byte(r.Path)) {
				path = "(path withheld)"
			}
			findings = append(findings, finding{of + path + ":" + strconv.Itoa(r.Line), "name", "the added line holds " + listed})
		}
	})
	return findings, true, err
}
