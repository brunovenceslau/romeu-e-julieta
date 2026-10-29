// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The process's standard input and the file descriptor of its standard
// error, which the CLI writer env.Stderr writes to: gate 2 asks only
// when both are a terminal. A test replaces them.
var (
	stdin    = func() io.Reader { return os.Stdin }
	stderrFD = func() int { return syscall.Stderr }
)

// errApprovalRequired is the interim error of a sync without a terminal
// whose widening digest differs from the approval (01 1.6, REQ-1210):
// it exits 1, as an error of no row, until project-approval-required
// (RJ-301, exit 3) is declared, with the slice that links sync into the
// binary.
var errApprovalRequired = errors.New("gate 2 needs a terminal to approve a changed widening; run romeu sync again on one")

// gateAndPromote is step 8 of 04 4.2 (REQ-0343): gate 2 and the
// candidate machine of 01 1.6, for each target in order.
//
//   - A widening digest equal to the approval's is promoted with no
//     prompt and no terminal (REQ-1207).
//   - Otherwise, on a terminal, the gate 2 diff and its prompt are
//     written to stderr and the answer read from stdin: y
//     approves, and the candidate is promoted (REQ-1208, REQ-1211); any
//     other answer declines, which deletes the candidate dir and leaves
//     the live files as they are (REQ-1209, I7).
//   - Without a terminal the candidate is declined, its dir deleted,
//     its widening digest printed, and the command fails once every
//     target is done (REQ-1210), recording no approval.
//
// gate.Confirm writes the diff and its prompt through the CLI writer
// of stderr, once both stdin and the file descriptor of stderr are a
// terminal, and asks as gate.Ask does: what was typed before the prompt
// is dropped, and y or Y approves. gate.Ask needs the *os.File of
// stderr, which no command holds, since all output goes through the
// CLI writer (04 4.1, Output). The diff is escaped already, and
// escaping it again changes nothing.
//
// A promotion runs the promotion commit (render.Live.Promote), then
// the project record is written through Store.UpdateProject: the
// approval, when this run gave one, and lastRunAt. The approval is
// written once the promotion commit is done, so a command killed
// before it asks again on the next sync (REQ-0082).
//
// On an error each candidate not yet promoted or declined is declined,
// so a command that returns leaves no candidate dir; an approved one
// whose promotion failed has no event that deletes it, and the next
// sync's render deletes its dir (REQ-0082). The workspace
// files are not written yet: render.Live takes them, and they need
// every clone of host state.
func gateAndPromote(ctx context.Context, r *run) error {
	var refused error
	for i, p := range r.targets {
		err := gateOne(ctx, r, p, r.trees[i].env, &r.candidates[i])
		switch {
		case errors.Is(err, errApprovalRequired):
			refused = err
		case err != nil:
			return errors.Join(err, decline(rendered(r.candidates)))
		}
	}
	return refused
}

// rendered returns the candidates of cs still in the state rendered:
// those the decline of an error deletes.
func rendered(cs []candidate) []candidate {
	var out []candidate
	for _, c := range cs {
		if c.dir.State() == state.CandidateRendered {
			out = append(out, c)
		}
	}
	return out
}

// gateOne is step 8 for the target p, whose env dir is env, and its
// candidate c.
func gateOne(ctx context.Context, r *run, p spec.Project, env string, c *candidate) error {
	if a := c.project.Approval; a != nil && a.WideningDigest == c.widening {
		return promote(r, p, env, c, nil)
	}
	d, err := diff(r, env, c)
	if err != nil {
		return err
	}
	var text strings.Builder
	if _, err := d.WriteTo(&text); err != nil {
		return err
	}
	ok, err := gate.Confirm(stdin(), stderrFD(), r.env.Stderr, text.String())
	if errors.Is(err, gate.ErrNoTerminal) {
		if _, err := c.dir.Apply(state.CandidateDeclineNoTTY, nil, state.Approval{}); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(r.env.Stdout, "gate 2: %s needs an approval of the widening digest %s; the candidate is deleted\n",
			termsafe.Escape(p.Name), c.widening); err != nil {
			return err
		}
		return errApprovalRequired
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ok {
		if _, err := c.dir.Apply(state.CandidateDecline, nil, state.Approval{}); err != nil {
			return err
		}
		_, err := fmt.Fprintf(r.env.Stdout, "gate 2: declined for %s; the candidate is deleted and the live files are unchanged\n", termsafe.Escape(p.Name))
		return err
	}
	a := state.Approval{
		WideningDigest: c.widening, Set: c.set, RenderDigest: c.set.RenderDigest,
		SpecCommit: r.specCommit, CandidateID: c.dir.ID(), ApprovedAt: seams.Now(),
	}
	if _, err := c.dir.Apply(state.CandidateApprove, c.project, a); err != nil {
		return err
	}
	return promote(r, p, env, c, &a)
}

// promote runs the promotion commit of the candidate c of p, whose env
// dir is env, then reports it to the candidate machine, as "widening
// digest equals the approval" for a candidate still rendered and as
// "promotion commit completes" for one approved, and writes the project
// record: the approval a, when this run gave one, and lastRunAt. A
// promotion that fails leaves a rendered candidate rendered, for the
// decline of the error.
func promote(r *run, p spec.Project, env string, c *candidate, a *state.Approval) error {
	live := render.Live{Root: r.settings.Root, EnvDir: env, Bin: r.bin}
	if err := live.Promote(render.Candidate{Dir: c.dir.Path()}, c.widening); err != nil {
		return err
	}
	event := state.CandidateDigestEqual
	if c.dir.State() == state.CandidateApproved {
		event = state.CandidatePromotionDone
	}
	if _, err := c.dir.Apply(event, nil, state.Approval{}); err != nil {
		return err
	}
	if err := r.store.UpdateProject(r.lock, p.Name, func(rec *state.Project) error {
		if a != nil {
			rec.Approval = a
		}
		rec.LastRunAt = seams.Now()
		return nil
	}); err != nil {
		return err
	}
	_, err := fmt.Fprintf(r.env.Stdout, "promoted  %s\n", termsafe.Escape(p.Name))
	return err
}

// diff returns the gate 2 diff of the candidate c, whose env dir is env,
// from the approved side read from the live files (approvedSide). When
// the project has no approval, or the live files are not the approved
// render, the diff is the one of a first approval, which shows the
// whole candidate as added, never less than its set holds; a line on
// stderr says why.
func diff(r *run, env string, c *candidate) (gate.Diff, error) {
	a := c.project.Approval
	if a == nil {
		return gate.NewDiff(nil, c.side)
	}
	old, err := approvedSide(env, a)
	if err == nil {
		d, derr := gate.NewDiff(old, c.side)
		if derr == nil {
			return d, nil
		}
		err = derr
	}
	if _, werr := fmt.Fprintf(r.env.Stderr, "gate 2: the live files are not the approved render (%s); the diff shows the candidate in full\n",
		termsafe.Escape(err.Error())); werr != nil {
		return gate.Diff{}, werr
	}
	return gate.NewDiff(nil, c.side)
}

// approvedSide returns the approved side of the gate 2 diff of the
// approval a: its set, the live sbxenv.yaml of env decoded, and the
// files of each live kit dir the set names. gate.NewDiff checks them
// against the set's digests.
func approvedSide(env string, a *state.Approval) (*gate.Side, error) {
	rt, err := os.OpenRoot(env)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rt.Close() }()
	data, err := rt.ReadFile(filepath.FromSlash(render.LiveEnvFile))
	if err != nil {
		return nil, err
	}
	var doc gate.Render
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("the live %s: %w", render.LiveEnvFile, err)
	}
	side := &gate.Side{Set: a.Set, Render: doc, KitFiles: map[string][]gate.File{}}
	if len(a.Set.Kits) == 0 {
		return side, nil
	}
	kits, err := rt.OpenRoot(filepath.FromSlash(render.LiveKitsDir))
	if err != nil {
		return nil, err
	}
	defer func() { _ = kits.Close() }()
	for _, k := range a.Set.Kits {
		files, err := kitFiles(kits, k.ID+"-"+k.ContentDigest.String()[:12])
		if err != nil {
			return nil, err
		}
		side.KitFiles[k.Source+":"+k.ID] = files
	}
	return side, nil
}
