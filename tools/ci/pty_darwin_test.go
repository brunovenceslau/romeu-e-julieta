// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// getTermios is the ioctl that reads a terminal's settings.
const getTermios = unix.TIOCGETA

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
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatalf("grant the terminal: %v", err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatalf("unlock the terminal: %v", err)
	}
	// TIOCPTYGNAME fills a buffer of 128 bytes with the terminal's path.
	// x/sys/unix exports no ioctl that fills a buffer the caller owns
	// on darwin, so this one call goes through the standard library.
	var name [128]byte
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		t.Fatalf("read the terminal's name: %v", errno)
	}
	path, _, _ := bytes.Cut(name[:], []byte{0})
	terminal, err = os.OpenFile(string(path), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open the terminal: %v", err)
	}
	closeAtEnd(t, terminal)
	return master, terminal
}
