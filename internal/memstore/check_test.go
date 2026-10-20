// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

import (
	"bytes"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// tempName is a temporary name of julieta, of the grammar .tmp-<ULID>.
const tempName = ".tmp-01j9z3k8m6t2q4w5e7r9y1v3k5"

// TestTopLevel is the allowlist of 08 8.1 (REQ-0645): its members are
// the four names of the specification, the one Go list that the
// reference renders.
func TestTopLevel(t *testing.T) {
	assert.Equal(t, []string{"entries", "handoff", "snapshot", "salvage"}, TopLevel)
}

// TestClassify is the allowlist table of I24 (05 5.2): each name and
// file type the layout of 08 8.1 refuses, and those it admits.
//
//romeu:invariant I24 test
func TestClassify(t *testing.T) {
	for _, c := range []struct {
		name string
		mode fs.FileMode
		top  bool
		want string
	}{
		{"entries", fs.ModeDir, true, ""},
		{"handoff", fs.ModeDir, true, ""},
		{"snapshot", fs.ModeDir, true, ""},
		{"salvage", fs.ModeDir, true, ""},
		{"notes", fs.ModeDir, true, RuleTopLevel},
		{"README.md", 0, true, RuleTopLevel},
		{"entries", 0, true, RuleNotDir},
		{"entries", fs.ModeSymlink, true, RuleSymlink},
		{"x.md", fs.ModeSymlink, false, RuleSymlink},
		{"x", fs.ModeNamedPipe, false, RuleFIFO},
		{"x", fs.ModeDevice, false, RuleDevice},
		{"x", fs.ModeDevice | fs.ModeCharDevice, false, RuleDevice},
		{"x", fs.ModeSocket, false, RuleSocket},
		{"x", fs.ModeIrregular, false, RuleIrregular},
		{".hidden", 0, false, RuleDotfile},
		{".hidden", fs.ModeDir, true, RuleDotfile},
		{".git", fs.ModeDir, true, RuleGit},
		{".git", 0, false, RuleGit},
		{"..", fs.ModeDir, false, RuleName},
		{".", fs.ModeDir, false, RuleName},
		{"", 0, false, RuleName},
		{"a/b", 0, false, RuleName},
		{"a\x00", 0, false, RuleName},
		{tempName, 0, false, ""},
		{tempName, 0, true, ""},
		{tempName, fs.ModeSymlink, false, RuleDotfile},
		{".tmp-x", 0, false, RuleDotfile},
		{"01j9z3k8m6t2q4w5e7r9y1v3k5.md", 0, false, ""},
		{"sub", fs.ModeDir, false, ""},
	} {
		assert.Equal(t, c.want, classify(c.name, c.mode, c.top), "%q %v top=%v", c.name, c.mode, c.top)
	}
}

// memDir returns a new memory dir with the given files, each a slash
// path with its content; a path that ends in a slash is a directory.
func memDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			require.NoError(t, os.MkdirAll(full, 0o700))
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
	return dir
}

// check runs CheckDir on dir.
func check(t *testing.T, dir string) Report {
	t.Helper()
	rep, err := CheckDir(dir)
	require.NoError(t, err)
	return rep
}

// TestCheckAValidDir finds nothing in a dir of the four directories,
// with an entry, a handoff, a snapshot and a nested salvage payload,
// and warns about a temporary file.
func TestCheckAValidDir(t *testing.T) {
	dir := memDir(t, map[string]string{
		"entries/01j9z3k8m6t2q4w5e7r9y1v3k5.md":       "x",
		"entries/" + tempName:                         "x",
		"handoff/01j9z3k8m6t2q4w5e7r9y1v3k5-clear.md": "x",
		"snapshot/heads.bundle":                       "x",
		"salvage/01j9zc2q8x7w4t6y3r5e1v9k0p/01j9zc2q8x7w4t6y3r5e1v9k0q/manifest.json": "x",
	})
	rep := check(t, dir)
	assert.Empty(t, rep.Findings)
	assert.Equal(t, []Finding{{Path: "entries/" + tempName, Rule: RuleTemp}}, rep.Warnings)
}

// TestCheckRefuses finds, through os.Root and Lstat, each node of the
// layout of 08 8.1 refuses, planted on disk: a symlink, a FIFO, a
// socket, a dotfile, a .git directory, a top-level name outside the
// four, a top-level file, a stray name in entries/ and a file over the
// cap (I12, I24 of 05 5.2). A symlink that points outside is reported,
// never followed.
//
//romeu:invariant I12 test
func TestCheckRefuses(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600))
	dir := memDir(t, map[string]string{
		"entries/notes.txt":   "x",
		"entries/sub/":        "",
		"handoff/.hidden":     "x",
		"handoff/big.md":      strings.Repeat("x", MaxFileSize+1),
		"snapshot/.git/HEAD":  "x",
		"other/":              "",
		"README.md":           "x",
		"salvage/ok/data.bin": "x",
	})
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "salvage", "link")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret"), filepath.Join(dir, "entries", "01j9z3k8m6t2q4w5e7r9y1v3k5.md")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "snapshot", "fifo"), 0o600))
	sock := filepath.Join(dir, "salvage", "s")
	if len(sock) < 100 {
		ln, err := new(net.ListenConfig).Listen(t.Context(), "unix", sock)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })
	}
	rep := check(t, dir)
	want := []Finding{
		{Path: "README.md", Rule: RuleTopLevel},
		{Path: "entries/01j9z3k8m6t2q4w5e7r9y1v3k5.md", Rule: RuleSymlink},
		{Path: "entries/notes.txt", Rule: RuleEntryName},
		{Path: "entries/sub", Rule: RuleEntryName},
		{Path: "handoff/.hidden", Rule: RuleDotfile},
		{Path: "handoff/big.md", Rule: RuleFileSize},
		{Path: "other", Rule: RuleTopLevel},
		{Path: "salvage/link", Rule: RuleSymlink},
		{Path: "snapshot/.git", Rule: RuleGit},
		{Path: "snapshot/fifo", Rule: RuleFIFO},
	}
	if len(sock) < 100 {
		want = append(want[:8], append([]Finding{{Path: "salvage/s", Rule: RuleSocket}}, want[8:]...)...)
	}
	assert.Equal(t, want, rep.Findings)
	assert.Empty(t, rep.Warnings)
}

// TestCheckCaps finds an entries/ over its cap and warns about a
// handoff/ over its own, which refuses nothing (REQ-0648, REQ-0650).
func TestCheckCaps(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{EntriesDir, HandoffDir} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, sub), 0o700))
	}
	src := canon.NewULIDSource(bytes.NewReader(make([]byte, 10)))
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for range MaxEntries + 1 {
		u, err := canon.NewULID(at, src)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, EntriesDir, EntryName(u)), nil, 0o600))
	}
	for i := range MaxHandoffFiles + 1 {
		require.NoError(t, os.WriteFile(filepath.Join(dir, HandoffDir, fmt.Sprintf("%04d.md", i)), nil, 0o600))
	}
	rep := check(t, dir)
	assert.Equal(t, []Finding{{Path: EntriesDir, Rule: RuleEntriesCap}}, rep.Findings)
	assert.Equal(t, []Finding{{Path: HandoffDir, Rule: RuleHandoffCap}}, rep.Warnings)
}

// TestCheckDir is exit 1 of julieta memory check: an absent dir, a
// file, a symlink and a dir julieta cannot write are each an error, and
// a path that is not absolute and clean is refused.
func TestCheckDir(t *testing.T) {
	base := t.TempDir()
	file := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(t.TempDir(), link))
	for _, p := range []string{filepath.Join(base, "absent"), file, link} {
		_, err := CheckDir(p)
		require.ErrorIs(t, err, ErrUnmounted, p)
	}
	for _, p := range []string{"relative", base + "/../x", ""} {
		_, err := CheckDir(p)
		require.Error(t, err, p)
	}
	if os.Geteuid() != 0 {
		ro := filepath.Join(base, "ro")
		require.NoError(t, os.Mkdir(ro, 0o500))
		_, err := CheckDir(ro)
		require.ErrorIs(t, err, ErrUnwritable)
	}
}

// TestCheckNodesCap finds a tree with more nodes than the walk reads,
// rather than judging part of it.
func TestCheckNodesCap(t *testing.T) {
	prev := maxNodes
	maxNodes = 3
	t.Cleanup(func() { maxNodes = prev })
	dir := memDir(t, map[string]string{"entries/": "", "handoff/a": "", "handoff/b": ""})
	assert.Equal(t, []Finding{{Path: ".", Rule: RuleNodesCap}}, check(t, dir).Findings)
	dir = memDir(t, map[string]string{"entries/": "", "handoff/a": ""})
	assert.Empty(t, check(t, dir).Findings)
}

// TestCheckTempSize applies the file cap to a temporary file too.
func TestCheckTempSize(t *testing.T) {
	dir := memDir(t, map[string]string{"entries/" + tempName: strings.Repeat("x", MaxFileSize+1)})
	rep := check(t, dir)
	assert.Equal(t, []Finding{{Path: "entries/" + tempName, Rule: RuleFileSize}}, rep.Findings)
	assert.Equal(t, []Finding{{Path: "entries/" + tempName, Rule: RuleTemp}}, rep.Warnings)
}

// TestFindingString escapes the path, so a name that holds a line feed
// or an escape sequence cannot forge a line of the output (I29).
func TestFindingString(t *testing.T) {
	f := Finding{Path: "entries/x\nfinding ok \x1b[2Jy", Rule: RuleEntryName}
	got := f.String()
	assert.NotContains(t, got, "\n")
	assert.NotContains(t, got, "\x1b")
	assert.Equal(t, "entry-name entries/x\\nfinding ok \\u001b[2Jy", got)
}
