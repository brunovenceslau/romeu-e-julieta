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

// pipes carries a command's output to its writers through pipes Run
// owns. Handed a writer that is not a file, os/exec makes the pipe
// itself and Wait returns only once the pipe is drained, up to
// WaitDelay, with no moment in between at which Run could kill the
// group; with the pipes here, Wait returns when the leader ends, Run
// kills the group, and drain then reads what is left.
type pipes struct {
	writers []*os.File // the ends the command writes; closed once it started
	readers []*os.File // the ends the copies read
	done    chan error // one result per reader
}

// attach sets cmd's Stdout and Stderr: nil stays nil, which os/exec
// opens on the null device; a file goes to the command as it is; any
// other writer gets a pipe and a copy. The same writer for both gets
// one pipe, so the order of the output is kept.
func attach(cmd *exec.Cmd, stdout, stderr io.Writer) (*pipes, error) {
	p := &pipes{done: make(chan error, 2)}
	var err error
	if cmd.Stdout, err = p.end(stdout); err != nil {
		return nil, err
	}
	if sameWriter(stdout, stderr) {
		cmd.Stderr = cmd.Stdout
		return p, nil
	}
	if cmd.Stderr, err = p.end(stderr); err != nil {
		p.abort()
		return nil, err
	}
	return p, nil
}

// end returns what the command writes for w.
func (p *pipes) end(w io.Writer) (io.Writer, error) {
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
	p.writers = append(p.writers, wf)
	go func() {
		_, err := io.Copy(w, r)
		p.done <- err
	}()
	return wf, nil
}

// started closes this process's copy of the ends the command writes,
// so a pipe reads EOF once every process of the command has closed it.
func (p *pipes) started() {
	for _, w := range p.writers {
		_ = w.Close()
	}
}

// abort closes every end and waits for the copies, for a command that
// did not start.
func (p *pipes) abort() {
	p.started()
	_ = p.drain(0)
}

// drain waits for the copies to read EOF, for at most bound, then
// stops the ones still reading and closes the read ends. It returns
// the first error of a copy, such as a writer that failed; the stop at
// the bound is not one (Run's doc says why).
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
