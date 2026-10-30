// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// settingsFile is the host settings file as doctor checks it before it
// runs anything the file names: its problem, when it is not a regular
// file of mode 0600 that this user owns, and otherwise the settings
// decoded from the same descriptor that was checked.
type settingsFile struct {
	problem  string
	settings spec.HostSettings
}

// checkedSettings opens the host settings file once, without following a
// link, judges that descriptor (a regular file, mode exactly 0600, owned
// by the effective user) and, when it passes, reads and decodes the
// settings from the same descriptor, so what settings-mode judged is what
// secret-argv runs. hostpaths.LoadSettings opens the path again and
// follows a link, so a file swapped after it read could run argv no
// check saw; this loader closes that window for the one row that runs
// what the settings name. A file it cannot open or read, or that does
// not decode, is the error.
func (h *host) checkedSettings() (settingsFile, error) {
	return h.settingsFile.get(func() (settingsFile, error) {
		f, err := os.OpenFile(h.settingsPath, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if errors.Is(err, syscall.ELOOP) {
			return settingsFile{problem: "is a link, not the file romeu wrote"}, nil
		}
		if err != nil {
			return settingsFile{}, err
		}
		defer func() { _ = f.Close() }()
		info, err := f.Stat()
		if err != nil {
			return settingsFile{}, err
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		switch {
		case !info.Mode().IsRegular():
			return settingsFile{problem: fmt.Sprintf("is %v, not a regular file", info.Mode())}, nil
		case info.Mode() != state.FileMode:
			return settingsFile{problem: fmt.Sprintf("is %v, not %v; chmod %04o %s", info.Mode(), state.FileMode, state.FileMode, h.settingsPath)}, nil
		case !ok || int(st.Uid) != os.Geteuid():
			return settingsFile{problem: "is not owned by this user"}, nil
		}
		data, err := io.ReadAll(io.LimitReader(f, spec.MaxFileSize+1))
		if err != nil {
			return settingsFile{}, err
		}
		s, err := spec.DecodeHostSettings(data)
		if err != nil {
			return settingsFile{}, err
		}
		return settingsFile{settings: s}, nil
	})
}
