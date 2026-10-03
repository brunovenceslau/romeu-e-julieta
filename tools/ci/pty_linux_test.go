// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

// getTermios is the ioctl that reads a terminal's settings.
const getTermios = unix.TCGETS

// openPTY opens a pseudo-terminal pair through /dev/ptmx: the test
// holds the master, and the code under test gets the terminal.
func openPTY(t *testing.T) (master, terminal *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open /dev/ptmx: %v", err)
	}
	closeAtEnd(t, master)
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlock the terminal: %v", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("read the terminal's number: %v", err)
	}
	terminal, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open the terminal: %v", err)
	}
	closeAtEnd(t, terminal)
	return master, terminal
}
