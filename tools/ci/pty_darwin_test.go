// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// getTermios is the ioctl that reads a terminal's settings.
const getTermios = unix.TIOCGETA

// openPTY opens a pseudo-terminal pair through /dev/ptmx: the test
// holds the master, and the code under test gets the terminal.
func openPTY(t *testing.T) (master, terminal *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err, "open /dev/ptmx")
	closeAtEnd(t, master)
	fd := int(master.Fd())
	require.NoError(t, unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0), "grant the terminal")
	require.NoError(t, unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0), "unlock the terminal")
	// TIOCPTYGNAME fills a buffer of 128 bytes with the terminal's path.
	// x/sys/unix exports no ioctl that fills a buffer the caller owns
	// on darwin, so this one call goes through the standard library.
	var name [128]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0])))
	require.Zero(t, errno, "read the terminal's name")
	path, _, _ := bytes.Cut(name[:], []byte{0})
	terminal, err = os.OpenFile(string(path), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err, "open the terminal")
	closeAtEnd(t, terminal)
	return master, terminal
}
