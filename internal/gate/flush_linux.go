// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import "golang.org/x/sys/unix"

// flushInput drops what the terminal fd holds in its input queue, as
// tcflush(fd, TCIFLUSH) does: on Linux, the ioctl TCFLSH takes the
// queue selector by value (ioctl_tty(2)).
func flushInput(fd int) error {
	return unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH)
}
