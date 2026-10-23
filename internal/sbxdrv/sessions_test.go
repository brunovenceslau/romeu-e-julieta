// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionsDir is the root of the sessions, from this package.
const sessionsDir = "../../e2e/testdata/sbx"

// recordedDir is the directory of the sessions the tests replay by name:
// the lowest recorded version, which is the floor.
var recordedDir = RecordedVersions[0]

// sessionHeader is the first line of a session file (10 10.3).
type sessionHeader struct {
	Session     string `json:"session"`
	SbxVersion  string `json:"sbxVersion"`
	CapturedAt  string `json:"capturedAt"`
	Synthetic   bool   `json:"synthetic"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// sessionEntry is one call of a session: the argv shape, the answer,
// and the way the answer is given.
type sessionEntry struct {
	Argv   []string `json:"argv"`
	Exit   *int     `json:"exit"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Hang   bool     `json:"hang"`
}

// session is a session file read.
type session struct {
	Path    string
	Version string
	Header  sessionHeader
	Entries []sessionEntry
}

// loadSessions reads every session of every recorded version.
func loadSessions(t *testing.T) []session {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(sessionsDir, "*", "*.jsonl"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no session under %s", sessionsDir)
	var out []session
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		s := session{Path: f, Version: filepath.Base(filepath.Dir(f))}
		for i, line := range slices.Collect(bytes.Lines(data)) {
			if i == 0 {
				require.NoError(t, jsonv2.Unmarshal(line, &s.Header), f)
				continue
			}
			var e sessionEntry
			require.NoError(t, jsonv2.Unmarshal(line, &e), "%s line %d", f, i+1)
			s.Entries = append(s.Entries, e)
		}
		out = append(out, s)
	}
	return out
}

// shapeKey joins an argv shape into one comparable string.
func shapeKey(shape []string) string { return strings.Join(shape, "\x00") }

// TestSessionsAreWellFormed holds each session to the format of 10 10.3:
// a header that names the session, the version and the date, marks it
// synthetic, and says what it was written from; entries that hold only
// the placeholder tokens of this package between angle brackets.
func TestSessionsAreWellFormed(t *testing.T) {
	date := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	tokens := PlaceholderTokens()
	for _, s := range loadSessions(t) {
		t.Run(filepath.Base(s.Path), func(t *testing.T) {
			assert.Equal(t, strings.TrimSuffix(filepath.Base(s.Path), ".jsonl"), s.Header.Session)
			assert.Equal(t, s.Version, s.Header.SbxVersion)
			assert.Regexp(t, date, s.Header.CapturedAt)
			assert.True(t, s.Header.Synthetic, "a session written by hand is marked synthetic: true until block A records it")
			assert.NotEmpty(t, s.Header.Source)
			assert.NotEmpty(t, s.Header.Description)
			require.NotEmpty(t, s.Entries)
			for i, e := range s.Entries {
				require.NotEmpty(t, e.Argv, "entry %d", i+1)
				for _, w := range e.Argv {
					if strings.HasPrefix(w, "<") {
						assert.Contains(t, tokens, w, "entry %d", i+1)
					}
				}
				if e.Hang {
					assert.Nil(t, e.Exit, "entry %d hangs, so it has no exit", i+1)
				} else {
					assert.NotNil(t, e.Exit, "entry %d", i+1)
				}
				for _, text := range []string{e.Stdout, e.Stderr} {
					assert.True(t, utf8.ValidString(text), "entry %d", i+1)
					assert.NotContains(t, text, string(rune(0x2014)), "entry %d: a recording holds no em dash (10 10.3)", i+1)
				}
			}
		})
	}
}

// TestBuildersAndSessionsAgree holds the two rules of 04 4.1 and 11.1:
// each argv builder produces a shape present in a session, and each
// shape in a session has a builder (REQ-0996).
func TestBuildersAndSessionsAgree(t *testing.T) {
	built := map[string]string{} // shape -> builder name
	for _, c := range examples(defaultFixture) {
		built[shapeKey(c.Shape)] = c.Name
	}
	recorded := map[string]string{} // shape -> session file
	for _, s := range loadSessions(t) {
		for _, e := range s.Entries {
			recorded[shapeKey(e.Argv)] = filepath.Base(s.Path)
		}
	}
	for shape, name := range built {
		assert.Contains(t, recorded, shape, "the builder %s gives a shape no session holds", name)
	}
	for shape, file := range recorded {
		assert.Contains(t, built, shape, "%s holds a shape no builder gives: %q", file, strings.Split(shape, "\x00"))
	}
}
