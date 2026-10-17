// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
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
	childEnds []*os.File   // the ends the command holds; closed here once it started
	readers   []*os.File   // the ends the output copies read
	done      chan error   // one result per output copy
	stdin     *os.File     // the end the input copy writes; nil when there is none
	moved     atomic.Int64 // counts each read and each write of the output copies
	writing   atomic.Int32 // the output copies inside a write to the caller's writer
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
		err := p.copy(w, r)
		_ = r.Close()
		p.done <- err
	}()
	return wf, nil
}

// copy copies r to w as io.Copy does, and records its progress for
// drain: each read that returns bytes and each write moves the count,
// and the copy is counted as writing while it waits on w, so a slow
// writer is not taken for a silent pipe.
func (p *pipes) copy(w io.Writer, r io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			p.moved.Add(1)
			p.writing.Add(1)
			_, werr := w.Write(buf[:n])
			p.writing.Add(-1)
			p.moved.Add(1)
			if werr != nil {
				return werr
			}
		}
		if errors.Is(rerr, io.EOF) {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
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
// its next write. It waits for the output copies, so the caller has its
// writers back once Run returns.
func (p *pipes) abort() {
	p.started()
	p.leaderEnded()
	_ = p.drain(context.Background(), WaitDelay)
}

// drain waits for the output copies to read EOF. It stops the ones
// still reading, and closes the read ends, once no copy has moved for
// idle: no byte read and no write under way. So what the command wrote
// before it ended is read whole however late the reader is, and a
// holder of a pipe that left the group and writes nothing ends the
// drain after idle. It also stops when ctx ends, and then returns
// ctx's cause: the output may be cut, so the run must not read as a
// success. Otherwise it returns the first error of a copy, such as a
// writer that failed; the stop after idle is not one (Run's doc says
// why).
func (p *pipes) drain(ctx context.Context, idle time.Duration) error {
	tick := time.NewTicker(idle / 10)
	defer tick.Stop()
	seen, quietSince := p.moved.Load(), time.Now()
	var first, cut error
	stop := func(err error) {
		if cut != nil {
			return
		}
		cut = err
		for _, r := range p.readers {
			_ = r.SetReadDeadline(time.Now())
		}
	}
	for range p.readers {
		var err error
	wait:
		for {
			select {
			case err = <-p.done:
				break wait
			case <-ctx.Done():
				stop(context.Cause(ctx))
			case <-tick.C:
				if m := p.moved.Load(); m != seen || p.writing.Load() > 0 {
					seen, quietSince = m, time.Now()
				} else if time.Since(quietSince) >= idle {
					stop(errIdle)
				}
			}
		}
		if first == nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			first = err
		}
	}
	for _, r := range p.readers {
		_ = r.Close()
	}
	if first == nil && cut != nil && !errors.Is(cut, errIdle) {
		first = cut
	}
	return first
}

// errIdle marks a drain that stopped after its idle bound, which is not
// an error of the run.
var errIdle = errors.New("procgroup: the output pipes were idle")

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
