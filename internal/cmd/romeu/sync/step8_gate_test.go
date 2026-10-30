// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The unit tests of steps 7 and 8 render on the fixture of step 6, then
// gate and promote into its live env dir. The terminal is a /dev/ptmx
// pair (openPTY), in place of the process's stdin and of the file
// descriptor of its stderr (stdin, stderrFD); without one, a pipe stands
// for them.

// closeAtEnd closes f when the test ends.
func closeAtEnd(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			require.NoError(t, err, "close %s", f.Name())
		}
	})
}

// lockedBuffer is a buffer a test reads while the command writes it.
type lockedBuffer struct {
	mu gosync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// String returns what was written so far.
func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// gateFixture is the fixture of step 6 with the CLI writers of the
// command, which a test reads.
type gateFixture struct {
	*renderFixture
	stdout, stderr *lockedBuffer
}

// gateRun returns the fixture with steps 1 to 6 done.
func gateRun(t *testing.T) *gateFixture {
	t.Helper()
	f := &gateFixture{renderFixture: renderRun(t), stdout: &lockedBuffer{}, stderr: &lockedBuffer{}}
	f.r.env = &cli.Env{Stdout: cli.NewWriter(f.stdout), Stderr: cli.NewWriter(f.stderr)}
	require.NoError(t, renderCandidates(t.Context(), f.r))
	return f
}

// rerender readies the fixture for one more sync and runs step 6.
func (f *gateFixture) rerender(t *testing.T) {
	t.Helper()
	f.again()
	f.r.overwritten = nil
	require.NoError(t, renderCandidates(t.Context(), f.r))
}

// withTerminal makes in the standard input of the command, and errFD
// the file descriptor of its standard error, until the test ends.
func withTerminal(t *testing.T, in io.Reader, errFD int) {
	t.Helper()
	oldIn, oldErr := stdin, stderrFD
	stdin = func() io.Reader { return in }
	stderrFD = func() int { return errFD }
	t.Cleanup(func() { stdin, stderrFD = oldIn, oldErr })
}

// pipe returns the two ends of a pipe, closed when the test ends.
func pipe(t *testing.T) (r, w *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, r)
	closeAtEnd(t, w)
	return r, w
}

// noTerminal makes a pipe the standard input and standard error of the
// command until the test ends.
func noTerminal(t *testing.T) {
	t.Helper()
	r, w := pipe(t)
	withTerminal(t, r, int(w.Fd()))
}

// typingBuffer is the stderr under the CLI writer: it records what the
// command writes and, in the Write that shows the prompt, types the
// answer on the terminal through master before it returns, as the
// fastest operator would.
type typingBuffer struct {
	*lockedBuffer
	master *os.File
	answer string
	err    error
}

// Write records p and types the answer when p holds the prompt.
func (b *typingBuffer) Write(p []byte) (int, error) {
	n, err := b.lockedBuffer.Write(p)
	if strings.Contains(string(p), gate.PromptLine) {
		_, b.err = io.WriteString(b.master, b.answer)
	}
	return n, err
}

// promptWait is how long a test waits for step 8 to end.
const promptWait = 10 * time.Second

// gateOnPTY runs step 8 with stdin and the file descriptor of stderr on
// the terminal of a /dev/ptmx pair, types answer in the write that
// shows the prompt, and returns what this step wrote to stderr and its
// error. A step that has not ended by promptWait fails the test; the
// terminal is closed at the end of the test, which ends its read.
func (f *gateFixture) gateOnPTY(t *testing.T, answer string) (string, error) {
	t.Helper()
	master, terminal := openPTY(t)
	withTerminal(t, terminal, int(terminal.Fd()))
	start := len(f.stderr.String())
	typing := &typingBuffer{lockedBuffer: f.stderr, master: master, answer: answer}
	f.r.env.Stderr = cli.NewWriter(typing)
	done := make(chan error, 1)
	go func() { done <- gateAndPromote(t.Context(), f.r) }()
	select {
	case err := <-done:
		require.NoError(t, typing.err, "type the answer")
		return f.stderr.String()[start:], err
	case <-time.After(promptWait):
		t.Fatalf("step 8 did not end in %s; stderr: %q", promptWait, f.stderr.String())
		return "", nil
	}
}

// later makes the clock of the package read at until the test ends.
func later(t *testing.T, at time.Time) {
	t.Helper()
	old := seams.Now
	seams.Now = func() time.Time { return at }
	t.Cleanup(func() { seams.Now = old })
}

// warned returns what the warnings of the run print.
func warned(r *run) string {
	var b lockedBuffer
	w := cli.NewWriter(&b)
	for _, warn := range r.warnings {
		warn(w)
	}
	return b.String()
}

// liveEnv reads the live sbxenv.yaml of shop.
func (f *gateFixture) liveEnv(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(f.in(render.LiveEnvFile))
	require.NoError(t, err)
	return data
}

// record reads the project record of shop.
func (f *gateFixture) record(t *testing.T) *state.Project {
	t.Helper()
	p, err := f.r.store.LoadProject("shop")
	require.NoError(t, err)
	return p
}

// approve runs a first sync of shop on a terminal answered y.
func (f *gateFixture) approve(t *testing.T) {
	t.Helper()
	_, err := f.gateOnPTY(t, "y\n")
	require.NoError(t, err)
}

// TestGateAndPromoteOnATerminal is gate 2 on a first sync with stdin
// and stderr on a terminal: the diff and the prompt are written to
// stderr, y approves and promotes, and any other line declines, deletes
// the candidate dir, leaves the live files byte-identical and exits 1
// with candidate-declined naming the project (01 1.6, REQ-1208,
// REQ-1209, REQ-1211, REQ-1276; I7): the cases "sync approves on y and
// promotes" and "sync declines on N and deletes the candidate".
//
//romeu:invariant I7 test
func TestGateAndPromoteOnATerminal(t *testing.T) {
	t.Run("sync approves on y and promotes", func(t *testing.T) {
		f := gateRun(t)
		c := f.r.candidates[0]
		want, err := os.ReadFile(filepath.Join(c.dir.Path(), render.CandidateEnvFile))
		require.NoError(t, err)

		shown, err := f.gateOnPTY(t, "y\n")

		require.NoError(t, err)
		assert.Contains(t, shown, "gate 2: widening of the project shop", "the diff is written to stderr")
		assert.Equal(t, want, f.liveEnv(t), "the candidate's sbxenv.yaml is live")
		assert.FileExists(t, f.in(c.kits[0].LivePath()+"/files/run.sh"))
		assert.Empty(t, f.candidates(t), "the promotion deletes the candidate dir")
		assert.Equal(t, state.CandidatePromoted, c.dir.State())
		rec := f.record(t)
		require.NotNil(t, rec.Approval)
		assert.Equal(t, c.widening, rec.Approval.WideningDigest)
		assert.Equal(t, c.dir.ID(), rec.Approval.CandidateID, "the approval binds to the candidate")
		assert.Equal(t, f.r.specCommit, rec.Approval.SpecCommit)
		assert.False(t, rec.LastRunAt.IsZero(), "lastRunAt is recorded")
		assert.Contains(t, f.stdout.String(), "promoted  shop")
		drifted, err := render.CheckDrift(f.env)
		require.NoError(t, err)
		assert.Empty(t, drifted, "the live files are those render.json records")
	})
	for _, answer := range []string{"N\n", "n\n", "yes\n", "\n"} {
		t.Run("sync declines on "+strings.TrimSpace(answer)+" and deletes the candidate", func(t *testing.T) {
			f := gateRun(t)
			before := f.outsideCandidates(t)

			shown, err := f.gateOnPTY(t, answer)

			requireDeclined(t, err, "shop")
			assert.Contains(t, shown, gate.PromptLine)
			assert.Empty(t, f.candidates(t), "no candidate dir remains")
			assert.Equal(t, before, f.outsideCandidates(t), "the live files are byte-identical (I7)")
			assert.NoFileExists(t, f.in(render.LiveEnvFile))
			_, err = f.r.store.LoadProject("shop")
			require.ErrorIs(t, err, fs.ErrNotExist, "no approval is recorded")
			assert.Contains(t, f.stdout.String(), "declined for shop")
		})
	}
}

// requireDeclined requires err to be candidate-declined (RJ-358, exit
// 1) naming the projects names, its hint their sync.
func requireDeclined(t *testing.T, err error, names ...string) {
	t.Helper()
	e := cli.Of(err)
	require.Equal(t, CandidateDeclined.ID, e.ID, "%v", err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "gate 2 was declined for "+strings.Join(names, ", "), e.Message)
	assert.Equal(t, "romeu sync "+strings.Join(names, " "), e.Hint())
}

// TestGateAndPromoteRefusesWithoutATerminal: a widening digest that
// differs from the approval, with no terminal on stdin or on stderr,
// deletes the candidate dir, prints the widening digest and exits 3
// with project-approval-required (RJ-301), recording no approval and
// promoting nothing (01 1.6, REQ-1210; T045 folded). The prompt is
// written to stderr and asked from stdin, so either one that is not a
// terminal declines.
func TestGateAndPromoteRefusesWithoutATerminal(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"a pipe on both", noTerminal},
		{"a reader that is no file on stdin", func(t *testing.T) {
			t.Helper()
			_, terminal := openPTY(t)
			withTerminal(t, strings.NewReader("y\n"), int(terminal.Fd()))
		}},
		{"a pipe on stdin", func(t *testing.T) {
			t.Helper()
			r, _ := pipe(t)
			_, terminal := openPTY(t)
			withTerminal(t, r, int(terminal.Fd()))
		}},
		{"a pipe on stderr", func(t *testing.T) {
			t.Helper()
			_, w := pipe(t)
			_, terminal := openPTY(t)
			withTerminal(t, terminal, int(w.Fd()))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := gateRun(t)
			c := f.r.candidates[0]
			before := f.outsideCandidates(t)
			tt.setup(t)

			err := gateAndPromote(t.Context(), f.r)

			e := cli.Of(err)
			require.Equal(t, ProjectApprovalRequired.ID, e.ID, "%v", err)
			assert.Equal(t, 3, cli.ExitCode(err))
			assert.Equal(t, "romeu sync shop", e.Hint())
			assert.Contains(t, f.stdout.String(), c.widening.String(), "the widening digest is printed")
			assert.Equal(t, state.CandidateDeclined, c.dir.State())
			assert.Empty(t, f.candidates(t), "the candidate dir is deleted")
			assert.Equal(t, before, f.outsideCandidates(t), "nothing is promoted")
			_, err = f.r.store.LoadProject("shop")
			require.ErrorIs(t, err, fs.ErrNotExist, "no awaiting candidate or approval is recorded")
		})
	}
}

// TestGateAndPromotePromotesAnEqualDigestWithNoTerminal: once approved,
// a sync whose widening digest equals the approval promotes with no
// prompt and no terminal (REQ-1207), records lastRunAt again, and
// leaves at most one dir, none, under .romeu/candidates/ (REQ-0084).
func TestGateAndPromotePromotesAnEqualDigestWithNoTerminal(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	first := f.record(t)
	f.rerender(t)
	require.Len(t, f.candidates(t), 1, "one candidate at a time")
	noTerminal(t)
	later(t, first.LastRunAt.Add(time.Hour))

	require.NoError(t, gateAndPromote(t.Context(), f.r))

	assert.Equal(t, state.CandidatePromoted, f.r.candidates[0].dir.State())
	assert.Empty(t, f.candidates(t))
	assert.Equal(t, first.Approval, f.record(t).Approval, "the approval stays as it was")
	assert.True(t, f.record(t).LastRunAt.After(first.LastRunAt), "lastRunAt is recorded again")
}

// TestGateAndPromoteAgainChangesNothing: a second sync on unchanged
// inputs leaves every file of the env dir, content and mode, as it was
// (04 4.1, Idempotency, REQ-0277), and the project record but for
// lastRunAt.
func TestGateAndPromoteAgainChangesNothing(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	before, rec := snapshot(t, f.env), f.record(t)
	f.rerender(t)
	noTerminal(t)

	require.NoError(t, gateAndPromote(t.Context(), f.r))

	assert.Equal(t, before, snapshot(t, f.env))
	again := f.record(t)
	again.LastRunAt = rec.LastRunAt
	assert.Equal(t, rec, again)
}

// TestGateAndPromoteShowsTheChangeFromTheApproval: a changed widening
// is shown against the approved side read from the live files, so the
// diff names the change, not the whole candidate as added; N then
// leaves the live files of the approval byte-identical (I7).
//
//romeu:invariant I7 test
func TestGateAndPromoteShowsTheChangeFromTheApproval(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	before := f.outsideCandidates(t)
	f.r.targets[0].Kits = []spec.Kit{{Product: "plain"}, {Product: "withcaps"}}
	f.rerender(t)

	shown, err := f.gateOnPTY(t, "n\n")

	requireDeclined(t, err, "shop")
	assert.Equal(t, before, f.outsideCandidates(t), "N leaves the promoted live files byte-identical (I7)")
	assert.Empty(t, f.candidates(t))
	assert.NotContains(t, f.stderr.String(), "not the approved render", "the approved side is read from the live files")
	assert.NotContains(t, shown, "kit content product:plain:", "the approved kit is not shown as a change")
	assert.Contains(t, shown, "product:plain (unchanged)")
	assert.Contains(t, shown, "kit content product:withcaps: 0 file(s) changed, 1 added", "the new kit is shown as added")
}

// TestGateAndPromoteShowsTheWholeCandidateWhenTheLiveFilesAreNotTheApproval:
// a live sbxenv.yaml that is not the approved render (here, edited and
// let through by --overwrite-drift) cannot stand as the approved side,
// so the diff is the one of a first approval, and a line says why.
func TestGateAndPromoteShowsTheWholeCandidateWhenTheLiveFilesAreNotTheApproval(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	require.NoError(t, os.WriteFile(f.in(render.LiveEnvFile), []byte("edited: true\n"), 0o600))
	f.r.targets[0].Kits = []spec.Kit{{Product: "plain"}, {Product: "withcaps"}}
	f.rerender(t)

	shown, err := f.gateOnPTY(t, "n\n")

	requireDeclined(t, err, "shop")
	assert.Contains(t, f.stderr.String(), "the live files are not the approved render")
	assert.Contains(t, shown, "kit content product:plain: 0 file(s) changed, 2 added", "the approved kit is shown as added too")
	assert.Contains(t, shown, "product:plain (added)")
	assert.Contains(t, shown, "kit content product:withcaps: 0 file(s) changed, 1 added")
}

// twoTargets returns the fixture of steps 7 and 8 on the targets shop
// and blog, both approved on a terminal and promoted, and rendered
// again.
func twoTargets(t *testing.T) *gateFixture {
	t.Helper()
	f := &gateFixture{
		renderFixture: renderRunOf(t, func(h *hostTree) { h.addTarget(t, "blog", "blog-site") }),
		stdout:        &lockedBuffer{}, stderr: &lockedBuffer{},
	}
	f.r.env = &cli.Env{Stdout: cli.NewWriter(f.stdout), Stderr: cli.NewWriter(f.stderr)}
	require.NoError(t, renderCandidates(t.Context(), f.r))
	f.approve(t)
	f.rerender(t)
	return f
}

// editKit edits a file of the live kit dir of the target i: the next
// promotion of a candidate that names that dir fails (ErrKitDirDiffers).
func (f *gateFixture) editKit(t *testing.T, i int) {
	t.Helper()
	p := filepath.Join(f.r.trees[i].env, filepath.FromSlash(f.r.candidates[i].kits[0].LivePath()), "files", "run.sh")
	require.NoError(t, os.Chmod(p, 0o755))
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\necho edited\n"), 0o755))
}

// TestGateAndPromoteDeclinesTheRestOnAnError: a promotion that fails
// stops the step and declines every candidate still rendered, so no
// candidate dir is left; a target promoted before it stays promoted and
// recorded, and the record of the failed one is not written.
func TestGateAndPromoteDeclinesTheRestOnAnError(t *testing.T) {
	f := twoTargets(t)
	blogBefore, err := f.r.store.LoadProject("blog")
	require.NoError(t, err)
	shopBefore := f.record(t)
	f.editKit(t, 1)
	noTerminal(t)
	later(t, shopBefore.LastRunAt.Add(time.Hour))

	err = gateAndPromote(t.Context(), f.r)

	require.ErrorIs(t, err, render.ErrKitDirDiffers)
	assert.Equal(t, state.CandidatePromoted, f.r.candidates[0].dir.State())
	assert.True(t, f.record(t).LastRunAt.After(shopBefore.LastRunAt), "shop is promoted and recorded")
	assert.Equal(t, state.CandidateDeclined, f.r.candidates[1].dir.State())
	assert.NoDirExists(t, f.r.candidates[1].dir.Path(), "blog's candidate dir is gone")
	blogAfter, err := f.r.store.LoadProject("blog")
	require.NoError(t, err)
	assert.Equal(t, blogBefore, blogAfter, "the record of the failed target is not written")
}

// TestGateAndPromoteOfTwoTargetsWithoutATerminal: without a terminal, a
// target whose digest equals its approval promotes, a later one whose
// digest differs is deleted, and project-approval-required names it;
// when a later target then fails, its error stops the step, and the
// refusal is the line printed for it.
func TestGateAndPromoteOfTwoTargetsWithoutATerminal(t *testing.T) {
	t.Run("the equal one promotes and the other is refused", func(t *testing.T) {
		f := twoTargets(t)
		f.r.targets[1].Kits = []spec.Kit{{Product: "withcaps"}}
		f.rerender(t)
		noTerminal(t)

		err := gateAndPromote(t.Context(), f.r)

		e := cli.Of(err)
		require.Equal(t, ProjectApprovalRequired.ID, e.ID, "%v", err)
		assert.Equal(t, "romeu sync blog", e.Hint())
		assert.Equal(t, state.CandidatePromoted, f.r.candidates[0].dir.State())
		assert.Equal(t, state.CandidateDeclined, f.r.candidates[1].dir.State())
		assert.NoDirExists(t, f.r.candidates[1].dir.Path())
		assert.Contains(t, f.stdout.String(), "promoted  shop")
		assert.Contains(t, f.stdout.String(), f.r.candidates[1].widening.String())
	})
	t.Run("a refusal and a later error", func(t *testing.T) {
		f := twoTargets(t)
		f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
		f.rerender(t)
		f.editKit(t, 1)
		noTerminal(t)

		err := gateAndPromote(t.Context(), f.r)

		require.ErrorIs(t, err, render.ErrKitDirDiffers)
		assert.Contains(t, f.stdout.String(), "gate 2: shop needs an approval of the widening digest "+f.r.candidates[0].widening.String())
		assert.Empty(t, f.candidates(t))
	})
}

// TestGateAndPromoteGoesOnAfterADecline: N to gate 2 of the first
// target does not stop the second, which promotes; the command then
// exits 1 with candidate-declined naming the first alone (REQ-1276).
// With N to both, the error names both, in the order of the targets.
func TestGateAndPromoteGoesOnAfterADecline(t *testing.T) {
	t.Run("the next target promotes", func(t *testing.T) {
		f := twoTargets(t)
		f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
		f.rerender(t)

		_, err := f.gateOnPTY(t, "N\n")

		requireDeclined(t, err, "shop")
		assert.Equal(t, state.CandidateDeclined, f.r.candidates[0].dir.State())
		assert.Equal(t, state.CandidatePromoted, f.r.candidates[1].dir.State())
		assert.Contains(t, f.stdout.String(), "promoted  blog")
	})
	t.Run("both are declined", func(t *testing.T) {
		f := twoTargets(t)
		f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
		f.r.targets[1].Kits = []spec.Kit{{Product: "withcaps"}}
		f.rerender(t)

		_, err := f.gateOnPTY(t, "N\n")

		requireDeclined(t, err, "shop", "blog")
		assert.Empty(t, f.candidates(t))
	})
}

// workspaceFolders returns the folders of the workspace file name in
// the root of f.
func (f *gateFixture) workspaceFolders(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, name))
	require.NoError(t, err)
	var doc struct {
		Folders []struct {
			Path string `json:"path"`
		} `json:"folders"`
	}
	require.NoError(t, jsonv2.Unmarshal(data, &doc))
	out := []string{}
	for _, d := range doc.Folders {
		out = append(out, d.Path)
	}
	return out
}

// TestGateAndPromoteWritesTheWorkspaceFiles: after the last promotion
// of a run, dev.code-workspace lists every host clone of every
// project's record, the projects promoted before this run among them,
// and review.code-workspace lists no review checkout, since host state
// records none yet (REQ-0048, REQ-0137).
func TestGateAndPromoteWritesTheWorkspaceFiles(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	assert.Equal(t, []string{"shop-env/shop-api", "shop-env/shop-web"}, f.workspaceFolders(t, render.DevWorkspaceFile))
	assert.Empty(t, f.workspaceFolders(t, render.ReviewWorkspaceFile))

	g := twoTargets(t)
	g.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
	g.rerender(t)
	require.NoError(t, os.Remove(filepath.Join(g.root, render.DevWorkspaceFile)))

	_, err := g.gateOnPTY(t, "N\n")

	requireDeclined(t, err, "shop")
	assert.Equal(t, []string{"blog-env/blog-site", "shop-env/shop-api", "shop-env/shop-web"}, g.workspaceFolders(t, render.DevWorkspaceFile),
		"a project declined in this run is listed from its record")
}

// TestGateAndPromoteLeavesTheWorkspaceFilesWhenNothingIsPromoted: a run
// that promotes nothing, on a decline or without a terminal, leaves the
// workspace files as they were, byte for byte.
func TestGateAndPromoteLeavesTheWorkspaceFilesWhenNothingIsPromoted(t *testing.T) {
	for _, tt := range []struct {
		name string
		gate func(t *testing.T, f *gateFixture) error
	}{
		{"declined", func(t *testing.T, f *gateFixture) error {
			t.Helper()
			_, err := f.gateOnPTY(t, "N\n")
			return err
		}},
		{"refused", func(t *testing.T, f *gateFixture) error {
			t.Helper()
			noTerminal(t)
			return gateAndPromote(t.Context(), f.r)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := gateRun(t)
			for _, name := range []string{render.DevWorkspaceFile, render.ReviewWorkspaceFile} {
				require.NoError(t, os.WriteFile(filepath.Join(f.root, name), []byte("by hand\n"), 0o644))
			}

			require.Error(t, tt.gate(t, f))

			for _, name := range []string{render.DevWorkspaceFile, render.ReviewWorkspaceFile} {
				data, err := os.ReadFile(filepath.Join(f.root, name))
				require.NoError(t, err)
				assert.Equal(t, "by hand\n", string(data), name)
				assert.Equal(t, fs.FileMode(0o644), mode(t, filepath.Join(f.root, name)), name)
			}
		})
	}
}

// TestGateStepDefersADecline: a decline of gate 2 does not stop the
// run, so step 10 still reports, and the run then returns
// candidate-declined; another error of step 8 stops the run there.
func TestGateStepDefersADecline(t *testing.T) {
	f := gateRun(t)
	master, terminal := openPTY(t)
	withTerminal(t, terminal, int(terminal.Fd()))
	f.r.env.Stderr = cli.NewWriter(&typingBuffer{lockedBuffer: f.stderr, master: master, answer: "N\n"})
	reported := false
	list := []step{
		{8, "gate 2 and promote", gateStep},
		{10, "report orphans", func(context.Context, *run) error { reported = true; return nil }},
	}

	err := runSteps(t.Context(), f.r, list)

	requireDeclined(t, err, "shop")
	assert.True(t, reported, "step 10 ran after the decline")
}

// TestGateAndPromoteLeavesOutARecordItCannotRead: the record of another
// project that does not decode is left out of the workspace files with
// a line among the warnings, and the sync that promoted does not fail.
func TestGateAndPromoteLeavesOutARecordItCannotRead(t *testing.T) {
	f := gateRun(t)
	require.NoError(t, os.MkdirAll(filepath.Join(f.root, "blog-env"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(f.r.store.Dir(), "projects"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(f.r.store.Dir(), "projects", "blog.json"), []byte("{not json"), 0o600))

	f.approve(t)

	assert.Equal(t, []string{"shop-env/shop-api", "shop-env/shop-web"}, f.workspaceFolders(t, render.DevWorkspaceFile))
	assert.Contains(t, warned(f.r), "the workspace files leave out blog")
}

// TestGateAndPromoteRefusesACandidateChangedAfterTheRender: a candidate
// dir rewritten after step 6, its sbxenv.yaml and its render.json
// consistent with each other, is not the one the diff showed: it is
// declined before an approval or a promotion, on y and on an equal
// digest, and the live files are unchanged.
func TestGateAndPromoteRefusesACandidateChangedAfterTheRender(t *testing.T) {
	tamper := func(t *testing.T, c candidate) {
		t.Helper()
		env := []byte("tampered: true\n")
		require.NoError(t, os.WriteFile(filepath.Join(c.dir.Path(), render.CandidateEnvFile), env, 0o600))
		data, err := os.ReadFile(filepath.Join(c.dir.Path(), render.CandidateRecord))
		require.NoError(t, err)
		rec, err := render.DecodeRecord(data)
		require.NoError(t, err)
		rec.Files[render.LiveEnvFile] = plainSum(env)
		require.NoError(t, render.WriteRecord(render.Candidate{Dir: c.dir.Path()}, rec))
	}
	t.Run("answered y", func(t *testing.T) {
		f := gateRun(t)
		tamper(t, f.r.candidates[0])
		before := f.outsideCandidates(t)

		_, err := f.gateOnPTY(t, "y\n")

		require.ErrorIs(t, err, errCandidateChanged)
		assert.Equal(t, state.CandidateDeclined, f.r.candidates[0].dir.State())
		assert.Empty(t, f.candidates(t))
		assert.Equal(t, before, f.outsideCandidates(t), "nothing is promoted")
		_, err = f.r.store.LoadProject("shop")
		require.ErrorIs(t, err, fs.ErrNotExist, "no approval is recorded")
	})
	t.Run("an equal digest", func(t *testing.T) {
		f := gateRun(t)
		f.approve(t)
		f.rerender(t)
		tamper(t, f.r.candidates[0])
		before := f.outsideCandidates(t)
		noTerminal(t)

		err := gateAndPromote(t.Context(), f.r)

		require.ErrorIs(t, err, errCandidateChanged)
		assert.Empty(t, f.candidates(t))
		assert.Equal(t, before, f.outsideCandidates(t), "nothing is promoted")
	})
}

// TestGateAndPromoteSaysTheRecordWasNotWritten: a project record that
// cannot be written after the promotion commit fails the step with an
// error that says the promotion happened.
func TestGateAndPromoteSaysTheRecordWasNotWritten(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	f.rerender(t)
	noTerminal(t)
	f.r.lock.Unlock() // UpdateProject refuses a write without the lock

	err := gateAndPromote(t.Context(), f.r)

	require.ErrorIs(t, err, state.ErrNotLocked)
	assert.Contains(t, err.Error(), "shop was promoted, and its project record was not written")
	assert.Equal(t, state.CandidatePromoted, f.r.candidates[0].dir.State())
}

// TestGateAndPromoteLeavesAnApprovedCandidateWhosePromotionFailedUntilAnApprovedToDeclinedRow
// pins today's behavior: a candidate approved on y whose promotion
// fails stays approved, the candidate machine having no row from
// approved to declined, so its dir is left for the next sync's render
// to delete (REQ-0082); the approval is not recorded and the live files
// are unchanged. A row from approved to declined would delete it.
func TestGateAndPromoteLeavesAnApprovedCandidateWhosePromotionFailedUntilAnApprovedToDeclinedRow(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	approved := f.record(t).Approval
	f.r.targets[0].Kits = []spec.Kit{{Product: "plain"}, {Product: "withcaps"}}
	f.rerender(t)
	f.editKit(t, 0)
	before := f.outsideCandidates(t)

	_, err := f.gateOnPTY(t, "y\n")

	require.ErrorIs(t, err, render.ErrKitDirDiffers)
	assert.Equal(t, state.CandidateApproved, f.r.candidates[0].dir.State())
	assert.DirExists(t, f.r.candidates[0].dir.Path(), "the approved candidate's dir is left")
	assert.Equal(t, approved, f.record(t).Approval, "the new approval is not recorded")
	assert.Equal(t, before, f.outsideCandidates(t), "the live files are unchanged")
}
