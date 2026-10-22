// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/google/renameio/v2"
	"golang.org/x/sys/unix"
)

// state is the progress of a replay, kept in the state file between the
// calls of a test, each a process of its own.
type state struct {
	// Mutations is the number of calls that changed something so far
	// (REQ-0928): the session is in the window after that many.
	Mutations int `json:"mutations"`
	// Reads counts the answers given to each read, by the window and the
	// first entry that answers it, so that a read recorded twice in one
	// window is answered in order, and the last answer stands.
	Reads map[string]int `json:"reads"`
	// Calls is every argv the fake was given, in order, for a test to
	// read; the fake adds the call that failed to it too.
	Calls [][]string `json:"calls"`
	// Failures are the calls the replay refused, with the reason.
	Failures []failure `json:"failures"`
}

// failure is a call the fake refused to replay.
type failure struct {
	Argv   []string `json:"argv"`
	Reason string   `json:"reason"`
}

// stateFile is the open state of a replay under its lock.
type stateFile struct {
	path string
	lock *os.File
	st   state
}

// openState takes the lock beside the state file, which serializes the
// calls of a test that run at once, and reads the state; a file that is
// not there is a replay that has not started.
func openState(path string) (*stateFile, error) {
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, err
	}
	sf := &stateFile{path: path, lock: lock, st: state{Reads: map[string]int{}}}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		sf.close()
		return nil, err
	default:
		if err := jsonv2.Unmarshal(data, &sf.st); err != nil {
			sf.close()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if sf.st.Reads == nil {
			sf.st.Reads = map[string]int{}
		}
	}
	return sf, nil
}

// save writes the state atomically.
func (sf *stateFile) save() error {
	data, err := jsonv2.Marshal(sf.st)
	if err != nil {
		return err
	}
	return renameio.WriteFile(sf.path, data, 0o600)
}

// close releases the lock.
func (sf *stateFile) close() {
	_ = unix.Flock(int(sf.lock.Fd()), unix.LOCK_UN)
	_ = sf.lock.Close()
}

// windowOf returns, for each entry, the window it belongs to: the
// number of changes recorded before it.
func windowOf(entries []entry) []int {
	windows := make([]int, len(entries))
	n := 0
	for i, e := range entries {
		windows[i] = n
		if mutates(e.Argv) {
			n++
		}
	}
	return windows
}

// answer picks the entry that answers argv in the state st, and moves
// the state on. A call the session cannot answer is an error with the
// reason: an argv shape absent from the recording, or a call out of the
// recorded order (REQ-0927, REQ-0928).
func answer(s *session, st *state, argv []string) (int, error) {
	var candidates []int
	for i, e := range s.Entries {
		if matchShape(e.Argv, argv) {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no recorded argv shape matches %q in the session %s", argv, s.Header.Session)
	}
	windows := windowOf(s.Entries)
	if mutates(argv) {
		var changes []int
		for i, e := range s.Entries {
			if mutates(e.Argv) {
				changes = append(changes, i)
			}
		}
		if st.Mutations >= len(changes) {
			return 0, fmt.Errorf("%q changes something, and the session %s has recorded %d changes, all made", argv, s.Header.Session, len(changes))
		}
		next := changes[st.Mutations]
		if !slices.Contains(candidates, next) {
			return 0, fmt.Errorf("%q is out of the recorded order: change %d of the session %s is %q", argv, st.Mutations+1, s.Header.Session, s.Entries[next].Argv)
		}
		st.Mutations++
		return next, nil
	}
	inWindow := slices.DeleteFunc(slices.Clone(candidates), func(i int) bool { return windows[i] != st.Mutations })
	if len(inWindow) == 0 {
		return 0, fmt.Errorf("%q is out of the recorded order: the session %s records it after change %d, and %d have been made", argv, s.Header.Session, windows[candidates[0]], st.Mutations)
	}
	key := strconv.Itoa(st.Mutations) + ":" + strconv.Itoa(inWindow[0])
	n := st.Reads[key]
	st.Reads[key] = n + 1
	return inWindow[min(n, len(inWindow)-1)], nil
}
