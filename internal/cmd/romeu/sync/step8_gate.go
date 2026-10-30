// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
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

// errCandidateChanged is the refusal of a candidate dir whose files
// are no longer those step 6 rendered, which the gate 2 diff showed: it
// is declined, and nothing of it is promoted.
var errCandidateChanged = errors.New("the candidate dir changed after it was rendered, so it is not the one the gate showed; it is declined")

// outcome is what gate 2 did with one target's candidate.
type outcome int

const (
	// gatePromoted: the candidate was promoted.
	gatePromoted outcome = iota + 1
	// gateDeclined: the operator declined it on a terminal.
	gateDeclined
	// gateRefused: its widening digest differs and no terminal can ask.
	gateRefused
)

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
//     and its widening digest printed (REQ-1210), recording no approval.
//
// The targets after a declined or refused one still meet gate 2. Once
// every target is done, a refusal without a terminal fails the command
// with project-approval-required (RJ-301, exit 3), and a decline on a
// terminal with candidate-declined (RJ-358, exit 1), as gate 1's
// decline does (REQ-1276); each names the projects, and its hint is
// their sync.
//
// gate.Confirm writes the diff and its prompt through the CLI writer
// of stderr, once both stdin and the file descriptor of stderr are a
// terminal: what was typed before the prompt is dropped, and y or Y
// approves. The diff is escaped already, and escaping it again changes
// nothing.
//
// A promotion runs the promotion commit (render.Live.Promote), then
// the project record is written through Store.UpdateProject: the
// approval, when this run gave one, and lastRunAt. The approval is
// written once the promotion commit is done, so a command killed
// before it asks again on the next sync (REQ-0082). Right before a
// candidate is approved or promoted, the digest of its dir is computed
// again and must be the one step 6 rendered, so the bytes promoted are
// those the diff showed; one that differs is declined
// (errCandidateChanged). A live file that --overwrite-drift let through
// (step 7) is added to the warnings, once its project is promoted, as
// replaced, its edit lost.
//
// After the last target, when this run promoted any, the workspace
// files are written once from every project's record (writeWorkspaces);
// a run that promotes nothing leaves them as they were. The promotion
// commit itself is given none to write.
//
// On an error each candidate still rendered is declined, which deletes
// its dir, and the error stops the step and is the one returned: a
// target declined or refused before it is reported by the line printed
// for it alone, by choice: joined to it, the row of the decline would be
// the one cli.Main reports, and the text of the error that stopped the
// step would be lost. The workspace files are still written when a
// target before it was promoted. An approved candidate whose promotion
// failed is the one exception to the decline: the candidate machine has
// no row from approved to declined, so its dir is left, and the next
// sync's render deletes it (REQ-0082).
func gateAndPromote(ctx context.Context, r *run) error {
	names := map[outcome][]string{}
	for i, p := range r.targets {
		got, err := gateOne(ctx, r, p, r.trees[i].env, &r.candidates[i])
		if err != nil {
			var ws error
			if len(names[gatePromoted]) > 0 {
				ws = writeWorkspaces(r)
			}
			return errors.Join(err, decline(rendered(r.candidates)), ws)
		}
		names[got] = append(names[got], p.Name)
	}
	if len(names[gatePromoted]) > 0 {
		if err := writeWorkspaces(r); err != nil {
			return err
		}
	}
	var errs []error
	if n := names[gateRefused]; len(n) > 0 {
		errs = append(errs, named(ProjectApprovalRequired, n, "gate 2 needs a terminal to approve the changed widening of "))
	}
	if n := names[gateDeclined]; len(n) > 0 {
		errs = append(errs, named(CandidateDeclined, n, "gate 2 was declined for "))
	}
	return errors.Join(errs...)
}

// named returns the error of row for the projects names, its message
// prefix followed by the names and its hint the sync of them all.
func named(row cli.Row, names []string, prefix string) error {
	escaped := make([]string, len(names))
	for i, n := range names {
		escaped[i] = termsafe.Escape(n)
	}
	e := row.New(prefix + strings.Join(escaped, ", "))
	e.Fill = map[string]string{"<name>": strings.Join(escaped, " ")}
	return e
}

// writeWorkspaces writes dev.code-workspace and review.code-workspace
// into the root (REQ-0048) from every project's record: the repos of
// the approved widening set of each <name>-env/ under the root whose
// record holds an approval, as host clones. A dir whose name is no
// project name, or has no record, is no project of host state; a record
// that cannot be read is left out with a line among the warnings, so a
// damaged record of another project does not fail a sync that promoted.
// Host state records no review checkout until romeu pull does (T069),
// so review.code-workspace lists none; T069 passes them here.
func writeWorkspaces(r *run) error {
	names, err := envDirs(r.settings.Root)
	if err != nil {
		return fmt.Errorf("the workspace files: %w", err)
	}
	var clones []render.Clone
	for _, name := range names {
		rec, err := r.store.LoadProject(name)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, state.ErrInvalidName) {
			continue
		}
		if err != nil {
			line := "the workspace files leave out " + termsafe.Escape(name) + ": " + termsafe.Escape(err.Error())
			r.warnings = append(r.warnings, func(w *cli.Writer) { _, _ = fmt.Fprintln(w, line) })
			continue
		}
		if rec.Approval == nil {
			continue
		}
		for _, repo := range rec.Approval.Set.Repos {
			clones = append(clones, render.Clone{Project: name, Dir: repo.Dir})
		}
	}
	if err := render.WriteWorkspaces(r.settings.Root, clones, nil); err != nil {
		return fmt.Errorf("the workspace files: %w", err)
	}
	return nil
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
// candidate c: it returns what gate 2 did with it.
func gateOne(ctx context.Context, r *run, p spec.Project, env string, c *candidate) (outcome, error) {
	if a := c.project.Approval; a != nil && a.WideningDigest == c.widening {
		if err := unchanged(c); err != nil {
			return 0, err
		}
		return gatePromoted, promote(r, p, env, c, nil)
	}
	d, err := diff(r, env, c)
	if err != nil {
		return 0, err
	}
	var text strings.Builder
	if _, err := d.WriteTo(&text); err != nil {
		return 0, err
	}
	ok, err := gate.Confirm(stdin(), stderrFD(), r.env.Stderr, text.String())
	if errors.Is(err, gate.ErrNoTerminal) {
		if _, err := c.dir.Apply(state.CandidateDeclineNoTTY, nil, state.Approval{}); err != nil {
			return 0, err
		}
		_, err := fmt.Fprintf(r.env.Stdout, "gate 2: %s needs an approval of the widening digest %s; the candidate is deleted\n",
			termsafe.Escape(p.Name), c.widening)
		return gateRefused, err
	}
	if err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !ok {
		if _, err := c.dir.Apply(state.CandidateDecline, nil, state.Approval{}); err != nil {
			return 0, err
		}
		_, err := fmt.Fprintf(r.env.Stdout, "gate 2: declined for %s; the candidate is deleted and the live files are unchanged\n", termsafe.Escape(p.Name))
		return gateDeclined, err
	}
	if err := unchanged(c); err != nil {
		return 0, err
	}
	a := state.Approval{
		WideningDigest: c.widening, Set: c.set, RenderDigest: c.set.RenderDigest,
		SpecCommit: r.specCommit, CandidateID: c.dir.ID(), ApprovedAt: seams.Now(),
	}
	if _, err := c.dir.Apply(state.CandidateApprove, c.project, a); err != nil {
		return 0, err
	}
	return gatePromoted, promote(r, p, env, c, &a)
}

// unchanged checks that the dir of the rendered candidate c holds the
// files step 6 rendered, and declines it when it does not. The check
// runs right before the approval and the promotion commit, which reads
// the dir again; render.Live.Promote checks the widening digest against
// the candidate's own render.json alone.
func unchanged(c *candidate) error {
	sum, err := render.CandidateFilesDigest(c.dir.Path())
	if err == nil && sum == c.filesSum {
		return nil
	}
	if _, derr := c.dir.Apply(state.CandidateDecline, nil, state.Approval{}); derr != nil {
		return errors.Join(errCandidateChanged, err, derr)
	}
	return errors.Join(errCandidateChanged, err)
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
		return fmt.Errorf("%s was promoted, and its project record was not written: %w", termsafe.Escape(p.Name), err)
	}
	for _, e := range r.overwritten {
		if e.project != p.Name {
			continue
		}
		line := "replaced the edited " + termsafe.Escape(e.path) + " of " + termsafe.Escape(p.Name) + "; its edit is lost"
		r.warnings = append(r.warnings, func(w *cli.Writer) { _, _ = fmt.Fprintln(w, line) })
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
	full, err := gate.NewDiff(nil, c.side)
	a := c.project.Approval
	if err != nil || a == nil {
		return full, err
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
	return full, nil
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
