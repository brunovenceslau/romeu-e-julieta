// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package romeu

import (
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal pair through /dev/ptmx: the test
// holds the master, and romeu gets the terminal. Both are closed when
// the test ends.
func openPTY(tb testing.TB) (master, terminal *os.File) {
	tb.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		tb.Fatalf("open /dev/ptmx: %v", err)
	}
	tb.Cleanup(func() { _ = master.Close() })
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		tb.Fatalf("unlock the terminal: %v", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		tb.Fatalf("read the terminal's number: %v", err)
	}
	terminal, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		tb.Fatalf("open the terminal: %v", err)
	}
	tb.Cleanup(func() { _ = terminal.Close() })
	return master, terminal
}
