// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// A session is a JSON Lines file under e2e/testdata/sbx/<version>/
// (10 10.3): the header on the first line, then one call of sbx on
// each line, in the order it was made.
//
//	{"session":"lifecycle","sbxVersion":"0.48.0","capturedAt":"2026-10-22","synthetic":true,"source":"...","description":"..."}
//	{"argv":["version"],"exit":0,"stdout":"v0.48.0\n","stderr":""}
//	{"argv":["env","run","-d","--clone","<dir>"],"exit":0,"stdout":"","stderr":"","effects":{"gitRemote":{...}}}
//	{"argv":["env","exec","<dir>","--","<command...>"],"hang":true}
//
// argv is the shape of the call: the words sbx was given, with a
// placeholder token in place of each name, path or domain (match.go). A
// call that changes something (mutates, below) moves the session on;
// a call that only reads is answered by the entries between two
// changes. exit, stdout and stderr are what sbx answered; hang says the
// call never answered, which the timeout of its class ends; effects are
// what sbx wrote outside its answer.
type header struct {
	Session     string `json:"session"`
	SbxVersion  string `json:"sbxVersion"`
	CapturedAt  string `json:"capturedAt"`
	Synthetic   bool   `json:"synthetic"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// entry is one recorded call.
type entry struct {
	Argv    []string `json:"argv"`
	Exit    *int     `json:"exit"`
	Stdout  string   `json:"stdout"`
	Stderr  string   `json:"stderr"`
	Hang    bool     `json:"hang"`
	Effects *effects `json:"effects"`
}

// effects are the writes of a call outside its answer.
type effects struct {
	// GitRemote is the remote stanza sbx writes into the config of the
	// primary clone at a create (01 1.2, REQ-0929).
	GitRemote *gitRemote `json:"gitRemote"`
}

// gitRemote is the stanza [remote "<Name>"] with a url and a fetch
// refspec. <name> in a field stands for the name of the sandbox, and
// <git-url> for the git URL of the sandbox daemon the test serves.
type gitRemote struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Fetch string `json:"fetch"`
}

// session is a session read.
type session struct {
	Header  header
	Entries []entry
}

// mutatingVerbs are the verbs of the calls that change something, by
// their first words: they move a session on (REQ-0928). env exec is a
// read: what it runs is the sandbox's own work.
var mutatingVerbs = [][]string{
	{"env", "run"}, {"env", "create"}, {"env", "rm"}, {"stop"}, {"rm"},
	{"policy", "allow"}, {"policy", "deny"}, {"policy", "rm"},
	{"secret", "set"}, {"secret", "rm"}, {"settings", "set"}, {"settings", "unset"},
}

// mutates reports whether the argv, or the shape, is a call that
// changes something.
func mutates(argv []string) bool {
	return slices.ContainsFunc(mutatingVerbs, func(v []string) bool {
		return len(argv) >= len(v) && slices.Equal(argv[:len(v)], v)
	})
}

// loadSession reads and checks the session at path.
func loadSession(path string) (*session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s session
	for i, line := range slices.Collect(bytes.Lines(data)) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if i == 0 {
			if err := jsonv2.Unmarshal(line, &s.Header); err != nil {
				return nil, fmt.Errorf("%s line 1: %w", path, err)
			}
			continue
		}
		var e entry
		if err := jsonv2.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, i+1, err)
		}
		if len(e.Argv) == 0 {
			return nil, fmt.Errorf("%s line %d: an entry has no argv", path, i+1)
		}
		if e.Hang == (e.Exit != nil) {
			return nil, fmt.Errorf("%s line %d: an entry has an exit or hangs, and not both", path, i+1)
		}
		s.Entries = append(s.Entries, e)
	}
	if s.Header.Session == "" || s.Header.SbxVersion == "" {
		return nil, errors.New(path + ": the first line is not a session header")
	}
	if len(s.Entries) == 0 {
		return nil, errors.New(path + ": the session has no entry")
	}
	for _, e := range s.Entries {
		if e.Exit != nil && *e.Exit >= exitUnrecorded && *e.Exit <= exitGuard {
			return nil, fmt.Errorf("%s: an entry records the exit %d, which the fake keeps for itself", path, *e.Exit)
		}
		for _, w := range e.Argv {
			if strings.HasPrefix(w, "<") && typeMatchers[w] == nil && w != commandToken {
				return nil, fmt.Errorf("%s: the placeholder %s is not a type of the fake", path, w)
			}
		}
	}
	return &s, nil
}
