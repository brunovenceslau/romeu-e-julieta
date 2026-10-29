// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
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
// shows the prompt, and returns what the step wrote to stderr and its
// error. A step that has not ended by promptWait fails the test; the
// terminal is closed at the end of the test, which ends its read.
func (f *gateFixture) gateOnPTY(t *testing.T, answer string) (string, error) {
	t.Helper()
	master, terminal := openPTY(t)
	withTerminal(t, terminal, int(terminal.Fd()))
	typing := &typingBuffer{lockedBuffer: f.stderr, master: master, answer: answer}
	f.r.env.Stderr = cli.NewWriter(typing)
	done := make(chan error, 1)
	go func() { done <- gateAndPromote(t.Context(), f.r) }()
	select {
	case err := <-done:
		require.NoError(t, typing.err, "type the answer")
		return f.stderr.String(), err
	case <-time.After(promptWait):
		t.Fatalf("step 8 did not end in %s; stderr: %q", promptWait, f.stderr.String())
		return "", nil
	}
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
// the candidate dir and leaves the live files byte-identical (01 1.6,
// REQ-1208, REQ-1209, REQ-1211; I7): the cases "sync approves on y and
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

			require.NoError(t, err)
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

// TestGateAndPromoteDeclinesWithoutATerminalUntilRJ301: a widening
// digest that differs from the approval, with no terminal on stdin or
// on stderr, deletes the candidate dir, prints the widening digest and
// fails, recording no approval and promoting nothing (01 1.6, REQ-1210;
// T045 folded). The exit is 1, an error of no row, until
// project-approval-required (RJ-301, exit 3) is declared with the slice
// that links sync into the binary; this test is the one to move to
// RJ-301 then. The prompt is written to stderr and asked from stdin, so
// either one that is not a terminal declines.
func TestGateAndPromoteDeclinesWithoutATerminalUntilRJ301(t *testing.T) {
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

			require.ErrorIs(t, err, errApprovalRequired)
			assert.Equal(t, 1, cli.ExitCode(err), "an error of no row, exit 1, until RJ-301")
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

	require.NoError(t, gateAndPromote(t.Context(), f.r))

	assert.Equal(t, state.CandidatePromoted, f.r.candidates[0].dir.State())
	assert.Empty(t, f.candidates(t))
	assert.Equal(t, first.Approval, f.record(t).Approval, "the approval stays as it was")
	assert.False(t, f.record(t).LastRunAt.Before(first.LastRunAt))
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
	f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
	f.rerender(t)

	shown, err := f.gateOnPTY(t, "n\n")

	require.NoError(t, err)
	assert.Equal(t, before, f.outsideCandidates(t), "N leaves the promoted live files byte-identical (I7)")
	assert.Empty(t, f.candidates(t))
	assert.NotContains(t, f.stderr.String(), "not the approved render", "the approved side is read from the live files")
	assert.Contains(t, shown, "product:plain")
	assert.Contains(t, shown, "withcaps")
}

// TestGateAndPromoteShowsTheWholeCandidateWhenTheLiveFilesAreNotTheApproval:
// a live sbxenv.yaml that is not the approved render (here, edited and
// let through by --overwrite-drift) cannot stand as the approved side,
// so the diff is the one of a first approval, and a line says why.
func TestGateAndPromoteShowsTheWholeCandidateWhenTheLiveFilesAreNotTheApproval(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	require.NoError(t, os.WriteFile(f.in(render.LiveEnvFile), []byte("edited: true\n"), 0o600))
	f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
	f.rerender(t)

	shown, err := f.gateOnPTY(t, "n\n")

	require.NoError(t, err)
	assert.Contains(t, f.stderr.String(), "the live files are not the approved render")
	assert.Contains(t, shown, gate.PromptLine)
}

// TestGateAndPromoteDeclinesTheRestOnAnError: a promotion that fails
// stops the step and declines every candidate still rendered, so no
// candidate dir is left; the project record is not written.
func TestGateAndPromoteDeclinesTheRestOnAnError(t *testing.T) {
	f := gateRun(t)
	f.approve(t)
	f.rerender(t)
	noTerminal(t)
	f.r.bin = render.Bin{} // not the julieta the candidate's record names

	err := gateAndPromote(t.Context(), f.r)

	require.Error(t, err)
	assert.Empty(t, f.candidates(t))
	assert.Equal(t, state.CandidateDeclined, f.r.candidates[0].dir.State())
}
