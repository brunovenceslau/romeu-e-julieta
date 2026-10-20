// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import (
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// createdIn watches dir and returns a function that lists the names
// created in it since: the kernel queues an event at each creation, so
// a file made and removed again within the write still shows.
func createdIn(t *testing.T, dir string) func() []string {
	t.Helper()
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	require.NoError(t, err)
	t.Cleanup(func() { _ = unix.Close(fd) })
	_, err = unix.InotifyAddWatch(fd, dir, unix.IN_CREATE|unix.IN_MOVED_TO)
	require.NoError(t, err)
	return func() []string {
		names := []string{}
		buf := make([]byte, 4096)
		for {
			n, err := unix.Read(fd, buf)
			if err != nil || n <= 0 {
				return names
			}
			for off := 0; off < n; {
				ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
				name := string(buf[off+unix.SizeofInotifyEvent : off+unix.SizeofInotifyEvent+int(ev.Len)])
				names = append(names, strings.TrimRight(name, "\x00"))
				off += unix.SizeofInotifyEvent + int(ev.Len)
			}
		}
	}
}

// TestWriteFileNeverTouchesTmpdir checks that the write never creates a
// file in $TMPDIR, not even the probe the library makes there when it
// is not given a directory.
func TestWriteFileNeverTouchesTmpdir(t *testing.T) {
	target := filepath.Join(t.TempDir(), "denylist")
	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)
	created := createdIn(t, tmpdir)

	require.NoError(t, writeFile(target, []byte("new")))

	assert.Empty(t, created(), "files created in $TMPDIR")
}
