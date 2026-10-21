// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux

package hooks_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// openPTY opens a pseudo-terminal and returns its controlling side and
// its terminal side, which a process reads as its terminal.
func openPTY(t *testing.T) (control, term *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	require.NoError(t, err)
	control = os.NewFile(uintptr(fd), "/dev/ptmx")
	require.NoError(t, unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0))
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	require.NoError(t, err)
	term, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = term.Close(); _ = control.Close() })
	return control, term
}

// TestRunATerminalOnStdinIsHandedOn: a terminal never ends, so the
// dispatcher does not copy it: it goes, as it is, to the first hook that
// runs, as git-p4 gives its p4-* hooks the user's terminal, and the
// later hooks get none. The run returns while the terminal stays open.
func TestRunATerminalOnStdinIsHandedOn(t *testing.T) {
	control, term := openPTY(t)
	r := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks", "p4-pre-submit"),
		[]byte("#!/bin/sh\nread line\necho \"tracked got $line\" >>"+r.log+"\n"), 0o755))
	r.hook(".git/hooks/p4-pre-submit", 0o755, "")
	_, err := control.WriteString("yes\n")
	require.NoError(t, err)

	type result struct {
		status int
		err    error
	}
	done := make(chan result, 1)
	go func() {
		var out bytes.Buffer
		status, err := hooks.Run(t.Context(), hooks.Options{Hook: "p4-pre-submit", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
			Stdin: term, Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
		done <- result{status, err}
	}()
	select {
	case res := <-done:
		require.NoError(t, res.err)
		assert.Equal(t, 0, res.status)
	case <-time.After(10 * time.Second):
		_ = control.Close() // ends a read of the terminal, so the run returns
		<-done
		require.FailNow(t, "the dispatcher waited for the end of a terminal")
	}
	assert.Equal(t, []string{"tracked got yes", ".git/hooks/p4-pre-submit args=", ".git/hooks/p4-pre-submit stdin="}, r.logged())
}
