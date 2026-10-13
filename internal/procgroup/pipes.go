// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

// pipes carries a command's standard streams between it and the
// caller's readers and writers through pipes Run owns. Handed a reader
// or a writer that is not a file, os/exec makes the pipe itself and
// Wait returns only once its copies are done, up to WaitDelay, with no
// moment in between at which Run could kill the group; with the pipes
// here, Wait returns when the leader ends, Run kills the group, and
// drain then reads what is left.
type pipes struct {
	childEnds []*os.File // the ends the command holds; closed here once it started
	readers   []*os.File // the ends the output copies read
	done      chan error // one result per output copy
	stdin     *os.File   // the end the input copy writes; nil when there is none
}

// attach sets cmd's standard streams. A nil reader or writer stays nil,
// which os/exec opens on the null device; a file goes to the command as
// it is; any other gets a pipe and a copy. The same writer for Stdout
// and Stderr gets one pipe, so the order of the output is kept.
func attach(cmd *exec.Cmd, stdin io.Reader, stdout, stderr io.Writer) (*pipes, error) {
	p := &pipes{done: make(chan error, 2)}
	var err error
	if cmd.Stdin, err = p.in(stdin); err != nil {
		return nil, err
	}
	if cmd.Stdout, err = p.out(stdout); err != nil {
		p.abort()
		return nil, err
	}
	if sameWriter(stdout, stderr) {
		cmd.Stderr = cmd.Stdout
		return p, nil
	}
	if cmd.Stderr, err = p.out(stderr); err != nil {
		p.abort()
		return nil, err
	}
	return p, nil
}

// in returns what the command reads for r. The copy ends when r ends,
// when r fails, or when Run closes the pipe at the end of the leader;
// a copy that blocks in r itself ends only when r returns, which Run
// does not wait for.
func (p *pipes) in(r io.Reader) (io.Reader, error) {
	if r == nil {
		return nil, nil
	}
	if f, ok := r.(*os.File); ok {
		return f, nil
	}
	rf, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.childEnds = append(p.childEnds, rf)
	p.stdin = w
	go func() {
		_, _ = io.Copy(w, r)
		_ = w.Close()
	}()
	return rf, nil
}

// out returns what the command writes for w. When the copy stops, at
// EOF or because w failed, it closes its end at once, so a command
// that goes on writing gets EPIPE or SIGPIPE instead of a full pipe it
// would wait on until its timeout.
func (p *pipes) out(w io.Writer) (io.Writer, error) {
	if w == nil {
		return nil, nil
	}
	if f, ok := w.(*os.File); ok {
		return f, nil
	}
	r, wf, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p.readers = append(p.readers, r)
	p.childEnds = append(p.childEnds, wf)
	go func() {
		_, err := io.Copy(w, r)
		_ = r.Close()
		p.done <- err
	}()
	return wf, nil
}

// started closes this process's copy of the ends the command holds, so
// an output pipe reads EOF once every process of the command has
// closed it.
func (p *pipes) started() {
	for _, f := range p.childEnds {
		_ = f.Close()
	}
}

// leaderEnded closes the input pipe, so its copy stops writing to a
// command that is gone.
func (p *pipes) leaderEnded() {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
}

// abort closes every end of a command that did not start: each output
// copy reads EOF and closes its own end, and the input copy stops at
// its next write.
func (p *pipes) abort() {
	p.started()
	p.leaderEnded()
}

// drain waits for the output copies to read EOF, for at most bound,
// then stops the ones still reading and closes the read ends. It
// returns the first error of a copy, such as a writer that failed; the
// stop at the bound is not one (Run's doc says why).
func (p *pipes) drain(bound time.Duration) error {
	timer := time.NewTimer(bound)
	defer timer.Stop()
	var first error
	for range p.readers {
		var err error
		select {
		case err = <-p.done:
		case <-timer.C:
			for _, r := range p.readers {
				_ = r.SetReadDeadline(time.Now())
			}
			err = <-p.done
		}
		if first == nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			first = err
		}
	}
	for _, r := range p.readers {
		_ = r.Close()
	}
	return first
}

// sameWriter reports whether a and b are the one writer. Comparing two
// interfaces whose dynamic type is not comparable panics, and such
// writers are not the same one here.
func sameWriter(a, b io.Writer) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return a == b
}
