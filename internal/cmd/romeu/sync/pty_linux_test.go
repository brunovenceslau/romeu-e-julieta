// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sync

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal pair through /dev/ptmx: the test
// holds the master, and the code under test gets the terminal. It is
// the helper of the tests of internal/gate, which a test of another
// package cannot import (10 10.1, REQ-0790: no pty dependency).
func openPTY(t *testing.T) (master, terminal *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err, "open /dev/ptmx")
	closeAtEnd(t, master)
	fd := int(master.Fd())
	require.NoError(t, unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0), "unlock the terminal")
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	require.NoError(t, err, "read the terminal's number")
	terminal, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err, "open the terminal")
	closeAtEnd(t, terminal)
	return master, terminal
}
