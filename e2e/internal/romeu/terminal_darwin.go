// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package romeu

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"

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
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		tb.Fatalf("grant the terminal: %v", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		tb.Fatalf("unlock the terminal: %v", err)
	}
	// TIOCPTYGNAME fills a buffer of 128 bytes with the terminal's path.
	// x/sys/unix exports no ioctl that fills a buffer the caller owns
	// on darwin, so this one call goes through the standard library.
	var name [128]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		tb.Fatalf("read the terminal's name: %v", errno)
	}
	path, _, _ := bytes.Cut(name[:], []byte{0})
	terminal, err = os.OpenFile(string(path), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		tb.Fatalf("open the terminal: %v", err)
	}
	tb.Cleanup(func() { _ = terminal.Close() })
	return master, terminal
}
