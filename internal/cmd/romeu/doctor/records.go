// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// records are the records of the state directory, read once: each
// project record that decodes, by name, and the records of a format
// romeu knows that do not (record-unreadable, RJ-334), each with the
// reason, which doctor reports without that error's exit (03, opening).
type records struct {
	// names are the projects whose record decodes, in name order.
	names    []string
	projects map[string]*state.Project
	// unreadable names each record that does not decode, with why.
	unreadable []string
}

// loadRecords reads the project records and toolchain.json. A state
// directory that does not exist holds none. A read that fails for
// another reason than the record's content, or a record of a newer
// format, is the error of every check that reads the records.
func (h *host) loadRecords() (records, error) {
	return h.records.get(func() (records, error) {
		r := records{projects: map[string]*state.Project{}}
		if h.stateErr != nil || h.store == nil {
			return r, h.stateErr
		}
		names, err := h.store.ProjectNames()
		if err != nil {
			return r, err
		}
		for _, name := range names {
			p, err := h.store.LoadProject(name)
			switch {
			case err == nil:
				r.names = append(r.names, name)
				r.projects[name] = p
			case isUnreadable(err):
				r.unreadable = append(r.unreadable, filepath.Join("projects", name+".json")+": "+err.Error())
			case !errors.Is(err, fs.ErrNotExist):
				return r, err
			}
		}
		_, err = h.store.LoadToolchain()
		switch {
		case isUnreadable(err):
			r.unreadable = append(r.unreadable, "toolchain.json: "+err.Error())
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return r, err
		}
		return r, nil
	})
}

// isUnreadable reports whether err is record-unreadable (RJ-334).
func isUnreadable(err error) bool {
	e, ok := errors.AsType[*cli.Error](err)
	return ok && e.ID == state.RecordUnreadable.ID
}
