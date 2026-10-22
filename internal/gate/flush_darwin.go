// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import "golang.org/x/sys/unix"

// fread is FREAD of <sys/fcntl.h>, the flag of TIOCFLUSH that names the
// input queue; x/sys/unix does not export it.
const fread = 0x1

// flushInput drops what the terminal fd holds in its input queue, as
// tcflush(fd, TCIFLUSH) does: on darwin, the ioctl TIOCFLUSH takes a
// pointer to FREAD (tty(4)).
func flushInput(fd int) error {
	return unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, fread)
}
