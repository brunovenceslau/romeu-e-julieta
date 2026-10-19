// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// commitFile writes content to the file name of the work tree in dir
// and commits it at the committer date, returning the commit.
func commitFile(t *testing.T, dir, name, content, msg, date string) string {
	t.Helper()
	file := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o700))
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
	plainGit(t, dir, "add", "--", name)
	_, err := plainGitErr(t, dir, []string{"GIT_COMMITTER_DATE=" + date}, "commit", "-q", "-m", msg)
	require.NoError(t, err)
	return plainGit(t, dir, "rev-parse", "HEAD")
}

// TestHistoryRealGit lists the commits that touched a directory on the
// first-parent history, newest first, with their committer dates and
// subjects: a commit elsewhere is left out, and so is a commit of a
// merged branch, whose merge stands for it.
func TestHistoryRealGit(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			first := commitFile(t, dir, "docs/plan/tasks/T001.md", "one\n", "plan: first", "2026-02-01T00:00:00Z")
			commitFile(t, dir, "README", "x\n", "elsewhere", "2026-02-02T00:00:00Z")
			plainGit(t, dir, "checkout", "-q", "-b", "side")
			commitFile(t, dir, "docs/plan/tasks/T002.md", "two\n", "plan: on a side branch", "2026-02-03T00:00:00Z")
			plainGit(t, dir, "checkout", "-q", "main")
			_, err := plainGitErr(t, dir, []string{"GIT_COMMITTER_DATE=2026-02-04T00:00:00Z"}, "merge", "-q", "--no-ff", "-m", "merge side", "side")
			require.NoError(t, err)
			merge := plainGit(t, dir, "rev-parse", "HEAD")
			last := commitFile(t, dir, "docs/plan/tasks/T001.md", "one again\n", "plan: édit", "2026-02-05T00:00:00Z")

			r := newRunner(t, mode)
			got, err := r.History(t.Context(), dir, "HEAD", "docs/plan/tasks")
			require.NoError(t, err)
			assert.Equal(t, []Commit{
				{last, time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), "plan: édit"},
				{merge, time.Date(2026, 2, 4, 0, 0, 0, 0, time.UTC), "merge side"},
				{first, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), "plan: first"},
			}, got)

			byRef, err := r.History(t.Context(), dir, "refs/heads/main", "docs/plan/tasks")
			require.NoError(t, err)
			assert.Equal(t, got, byRef)
			byOID, err := r.History(t.Context(), dir, first, "docs/plan/tasks/T001.md")
			require.NoError(t, err)
			assert.Equal(t, []Commit{got[2]}, byOID)

			never, err := r.History(t.Context(), dir, "HEAD", "docs/never")
			require.NoError(t, err)
			assert.Empty(t, never)
			literal, err := r.History(t.Context(), dir, "HEAD", "docs/*")
			require.NoError(t, err)
			assert.Empty(t, literal, "the path is taken literally, not as a glob")
		})
	}
}

// TestHistoryRefuses: a revision, a path or a directory gitsafe does
// not accept is refused before git starts.
func TestHistoryRefuses(t *testing.T) {
	var rec recorder
	r := newRunner(t, Host)
	rec.install(r)
	ctx := t.Context()
	for _, rev := range []string{"", "-p", "--all", "main", "HEAD~1", "refs/heads/a..b", "refs/heads/x:y", "abc123"} {
		_, err := r.History(ctx, stubRepo, rev, "docs")
		require.ErrorIs(t, err, ErrRefused, "rev %q", rev)
	}
	for _, path := range []string{"", "/docs", "docs/", ".git/config", "a/../b", "./a", "a/.GIT/b", "a\x1bb", "a\xff"} {
		_, err := r.History(ctx, stubRepo, "HEAD", path)
		require.ErrorIs(t, err, ErrRefused, "path %q", path)
		_, err = r.ReadDir(ctx, stubRepo, strings.Repeat("a", 40), path)
		require.ErrorIs(t, err, ErrRefused, "path %q", path)
	}
	_, err := r.ReadDir(ctx, stubRepo, "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused, "ReadDir takes a full object id only")
	_, err = r.History(ctx, filepath.Join(stubRepo, "sub"), "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused)
	_, err = r.ReadDir(ctx, "relative", strings.Repeat("a", 40), "docs")
	require.ErrorIs(t, err, ErrRefused)
	assert.Empty(t, rec.commands)
}

// TestHistoryArgv pins what History and ReadDir run: a log that checks
// no signature, follows no rename and reads no replace ref, with the
// revision after --end-of-options and the path literal.
func TestHistoryArgv(t *testing.T) {
	r := newRunner(t, Host)
	var commands []procgroup.Command
	r.start = func(ctx context.Context, c procgroup.Command) error {
		commands = append(commands, c)
		return answer(nil)(ctx, c)
	}
	_, err := r.History(t.Context(), stubRepo, "HEAD", "docs/plan")
	require.NoError(t, err)
	oid := strings.Repeat("a", 40)
	_, err = r.ReadDir(t.Context(), stubRepo, oid, "docs/plan")
	require.NoError(t, err)
	require.Len(t, commands, 3)
	for _, c := range commands {
		for _, flag := range []string{"core.useReplaceRefs=false", "log.showSignature=false", "log.follow=false", "core.commitGraph=false", "core.hooksPath=/dev/null"} {
			assert.Contains(t, c.Argv, flag)
		}
		assert.Subset(t, c.Env, []string{"GIT_NO_LAZY_FETCH=1", "GIT_GRAFT_FILE=/dev/null", "GIT_CONFIG_NOSYSTEM=1"})
	}
	shallow := commands[0].Argv
	assert.Equal(t, []string{"rev-parse", "--is-shallow-repository"}, shallow[slices.Index(shallow, "rev-parse"):])
	log := commands[1].Argv
	assert.Equal(t, []string{"log", "--first-parent", "-z", "--format=%H %ct %s", "--end-of-options", "HEAD", "--", ":(literal)docs/plan"}, log[slices.Index(log, "log"):])
	ls := commands[2].Argv
	assert.Equal(t, []string{"ls-tree", "-l", "-z", "--full-tree", "--end-of-options", oid, "--", ":(literal)docs/plan/"}, ls[slices.Index(ls, "ls-tree"):])
}

// answer returns a start function that answers rev-parse
// --is-shallow-repository with false and each other subcommand with its
// output in outputs, nothing when it has none.
func answer(outputs map[string]string) func(context.Context, procgroup.Command) error {
	return func(_ context.Context, c procgroup.Command) error {
		out := ""
		switch {
		case slices.Contains(c.Argv, "rev-parse"):
			out = "false\n"
		case slices.Contains(c.Argv, "log"):
			out = outputs["log"]
		case slices.Contains(c.Argv, "ls-tree"):
			out = outputs["ls-tree"]
		case slices.Contains(c.Argv, "cat-file"):
			out = outputs["cat-file"]
		}
		_, err := io.WriteString(c.Stdout, out)
		return err
	}
}

// lower sets the bound *v to n for the test.
func lower(t *testing.T, v *int, n int) {
	t.Helper()
	old := *v
	*v = n
	t.Cleanup(func() { *v = old })
}

// tasksCommit stores a commit whose tree holds the entries under
// docs/tasks, and returns it.
func tasksCommit(t *testing.T, dir string, entries ...rawEntry) string {
	t.Helper()
	tasks := writeTree(t, dir, entries...)
	docs := writeTree(t, dir, rawEntry{"40000", "tasks", tasks})
	return commitTree(t, dir, writeTree(t, dir, rawEntry{"40000", "docs", docs}))
}

// TestReadDirRealGit reads the regular files directly under a
// directory, in git's order, with their bytes: a directory below it is
// skipped, and a directory the commit does not hold gives nothing.
func TestReadDirRealGit(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			one := writeBlob(t, dir, "# T001\n")
			two := writeBlob(t, dir, "")
			run := writeBlob(t, dir, "#!/bin/sh\n")
			nested := writeTree(t, dir, rawEntry{"100644", "deep.md", one})
			c := tasksCommit(t, dir,
				rawEntry{"100644", "T001.md", one},
				rawEntry{"100644", "T002.md", two},
				rawEntry{"40000", "old", nested},
				rawEntry{"100755", "run", run},
				rawEntry{"100644", "été.md", one},
			)
			r := newRunner(t, mode)
			got, err := r.ReadDir(t.Context(), dir, c, "docs/tasks")
			require.NoError(t, err)
			assert.Equal(t, []File{
				{"T001.md", modeFile, []byte("# T001\n")},
				{"T002.md", modeFile, []byte{}},
				{"run", modeExecutable, []byte("#!/bin/sh\n")},
				{"été.md", modeFile, []byte("# T001\n")},
			}, got)

			for _, path := range []string{"docs/missing", "docs/tasks/T001.md", "docs/tas"} {
				none, err := r.ReadDir(t.Context(), dir, c, path)
				require.NoError(t, err, path)
				assert.Empty(t, none, path)
			}
		})
	}
}

// TestReadDirRefusesHostileDirectories: each directory is refused with
// ErrRefused, by the mode, name and size rules.
func TestReadDirRefusesHostileDirectories(t *testing.T) {
	dir, head := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	large := writeBlob(t, dir, strings.Repeat("x", maxFileBytes+1))
	file := func(name string) rawEntry { return rawEntry{"100644", name, blob} }
	tests := []struct {
		name    string
		entries []rawEntry
		want    string
	}{
		{"symlink", []rawEntry{{"120000", "link", blob}}, `"docs/tasks/link" is a symbolic link`},
		{"gitlink", []rawEntry{{"160000", "module", head}}, `"docs/tasks/module" is a gitlink`},
		{".GIT", []rawEntry{file(".GIT")}, `holds the name ".GIT"`},
		{".git directory", []rawEntry{{"40000", ".git", writeTree(t, dir, file("config"))}}, `holds the name ".git"`},
		{"escape character", []rawEntry{file("a\x1b[2J")}, `holds the character U+001B`},
		{"not UTF-8", []rawEntry{file("a\xff")}, `is not valid UTF-8`},
		{"case collision", []rawEntry{file("T001.md"), file("t001.md")}, `"docs/tasks/T001.md" and "docs/tasks/t001.md" are one path`},
		{"normalization collision", []rawEntry{file("café"), file("café")}, `are one path`},
		{"a blob over the bound", []rawEntry{file("a"), {"100644", "big", large}}, `"docs/tasks/big" holds more than 1048576 bytes`},
	}
	r := newRunner(t, Host)
	for _, tt := range tests {
		got, err := r.ReadDir(t.Context(), dir, tasksCommit(t, dir, tt.entries...), "docs/tasks")
		require.ErrorIs(t, err, ErrRefused, tt.name)
		assert.Contains(t, err.Error(), tt.want, tt.name)
		assert.Nil(t, got, tt.name)
	}
}

// TestReadDirRefusesAWrongAnswer: an answer of cat-file that does not
// match what ls-tree listed, or that ends early, is refused.
func TestReadDirRefusesAWrongAnswer(t *testing.T) {
	oid := strings.Repeat("b", 40)
	listing := "100644 blob " + oid + "       3\tdocs/a\x00"
	for name, reply := range map[string]string{
		"another size":    oid + " blob 4\nabcd\n",
		"another object":  strings.Repeat("c", 40) + " blob 3\nabc\n",
		"missing":         oid + " missing\n",
		"ends early":      oid + " blob 3\nab",
		"no newline":      oid + " blob 3\nabcd",
		"more than asked": oid + " blob 3\nabc\n" + oid + " blob 3\nabc\n",
	} {
		r := newRunner(t, Host)
		r.start = answer(map[string]string{"ls-tree": listing, "cat-file": reply})
		_, err := r.ReadDir(t.Context(), stubRepo, strings.Repeat("a", 40), "docs")
		require.ErrorIs(t, err, ErrRefused, name)
	}
}

// TestHistoryRefusesABadRecord: a log record gitsafe cannot read, or an
// output that ends inside one, is refused.
func TestHistoryRefusesABadRecord(t *testing.T) {
	oid := strings.Repeat("a", 40)
	for name, out := range map[string]string{
		"no subject field": oid + " 1\x00",
		"a bad oid":        "abc 1 s\x00",
		"a bad date":       oid + " x s\x00",
		"not UTF-8":        oid + " 1 \xff\x00",
		"ends inside":      oid + " 1 s",
		"too long":         oid + " 1 " + strings.Repeat("s", maxHistoryRecord()) + "\x00",
		"a long subject":   oid + " 1 " + strings.Repeat("s", maxSubjectBytes+1) + "\x00",
		"an escape":        oid + " 1 a\x1b[2Jb\x00",
		"a bell":           oid + " 1 a\ab\x00",
		"an override":      oid + " 1 a\u202eb\x00",
		"a C1 control":     oid + " 1 a\u009bb\x00",
	} {
		r := newRunner(t, Host)
		r.start = answer(map[string]string{"log": out})
		_, err := r.History(t.Context(), stubRepo, "HEAD", "docs")
		require.ErrorIs(t, err, ErrRefused, name)
	}
	r := newRunner(t, Host)
	r.start = answer(map[string]string{"log": oid + " 1 " + strings.Repeat("s", maxSubjectBytes) + "\x00"})
	got, err := r.History(t.Context(), stubRepo, "HEAD", "docs")
	require.NoError(t, err)
	assert.Len(t, got[0].Subject, maxSubjectBytes, "a subject at the bound is kept")
}

// TestHistoryRefusesTooManyCommits: a history over maxHistoryCommits
// is refused, and one at it is kept.
func TestHistoryRefusesTooManyCommits(t *testing.T) {
	lower(t, &maxHistoryCommits, 2)
	rec := strings.Repeat("a", 40) + " 1 s\x00"
	r := newRunner(t, Host)
	r.start = answer(map[string]string{"log": rec + rec})
	got, err := r.History(t.Context(), stubRepo, "HEAD", "docs")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	r.start = answer(map[string]string{"log": rec + rec + rec})
	_, err = r.History(t.Context(), stubRepo, "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "more than 2 commits")
}

// TestHistoryRefusesAShallowRepository: a shallow clone, whose history
// is cut short, is refused, and so is an answer of rev-parse that is
// neither true nor false.
func TestHistoryRefusesAShallowRepository(t *testing.T) {
	src, _ := newRepo(t)
	commitFile(t, src, "docs/a", "1\n", "one", "2026-02-01T00:00:00Z")
	commitFile(t, src, "docs/a", "2\n", "two", "2026-02-02T00:00:00Z")
	dst := filepath.Join(t.TempDir(), "shallow")
	plainGit(t, "", "clone", "-q", "--depth", "1", "file://"+src, dst)
	_, err := newRunner(t, Host).History(t.Context(), dst, "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "is a shallow repository")

	r := newRunner(t, Host)
	r.start = func(_ context.Context, c procgroup.Command) error {
		_, err := io.WriteString(c.Stdout, "maybe\n")
		return err
	}
	_, err = r.History(t.Context(), stubRepo, "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused)
}

// TestHistoryIgnoresGrafts: a graft file in the repository that cuts
// the history short changes nothing, and plain git log follows it (the
// positive control).
func TestHistoryIgnoresGrafts(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "docs/a", "1\n", "one", "2026-02-01T00:00:00Z")
	last := commitFile(t, dir, "docs/a", "2\n", "two", "2026-02-02T00:00:00Z")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "info", "grafts"), []byte(last+"\n"), 0o600))
	got, err := newRunner(t, Host).History(t.Context(), dir, "HEAD", "docs")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "two", plainGit(t, dir, "log", "--format=%s", "HEAD", "--", "docs"), "the graft is live: plain git follows it")
}

// TestHistoryRefusesAControlCharacterInASubjectRealGit: a subject that
// holds an escape is refused as a name that holds one is.
func TestHistoryRefusesAControlCharacterInASubjectRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "docs/a", "1\n", "a \x1b[2J b", "2026-02-01T00:00:00Z")
	_, err := newRunner(t, Host).History(t.Context(), dir, "HEAD", "docs")
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "holds the character U+001B")
}

// TestHistoryErrorsEscapeGitOutput: what git writes to its standard
// error reaches the error of History and ReadDir only escaped (I29).
func TestHistoryErrorsEscapeGitOutput(t *testing.T) {
	git := fakeGit(t, `printf 'fatal: \033[2J\033]0;title\007bad\n' >&2; exit 128`+"\n")
	r, err := New(Config{Mode: Host, Git: git, Home: t.TempDir()})
	require.NoError(t, err)
	_, histErr := r.History(t.Context(), stubRepo, "HEAD", "docs")
	_, dirErr := r.ReadDir(t.Context(), stubRepo, strings.Repeat("a", 40), "docs")
	for _, err := range []error{histErr, dirErr} {
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "\x1b")
		assert.NotContains(t, err.Error(), "\a")
		assert.Contains(t, err.Error(), `fatal: \u001b[2J`)
	}
}

// TestHistoryRunsNoSignatureProgram plants log.showSignature and a
// gpg.program in the repository's local config over a signed commit:
// History runs no program, and plain git log with the same config runs
// it (the positive control).
//
//romeu:invariant I3 test
func TestHistoryRunsNoSignatureProgram(t *testing.T) {
	dir, _ := newRepo(t)
	marker := filepath.Join(t.TempDir(), "marker")
	script := filepath.Join(t.TempDir(), "gpg.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\n"), 0o700))
	tree := writeTree(t, dir, rawEntry{"40000", "docs", writeTree(t, dir, rawEntry{"100644", "a", writeBlob(t, dir, "x\n")})})
	body := "tree " + tree + "\nauthor a <a@example.invalid> 1 +0000\ncommitter a <a@example.invalid> 1 +0000\n" +
		"gpgsig -----BEGIN PGP SIGNATURE-----\n x\n -----END PGP SIGNATURE-----\n\nsigned\n"
	file := filepath.Join(t.TempDir(), "commit")
	require.NoError(t, os.WriteFile(file, []byte(body), 0o600))
	signed := plainGit(t, dir, "hash-object", "-w", "-t", "commit", file)
	plainGit(t, dir, "config", "log.showSignature", "true")
	plainGit(t, dir, "config", "gpg.program", script)

	got, err := newRunner(t, Host).History(t.Context(), dir, signed, "docs")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "signed", got[0].Subject)
	assert.NoFileExists(t, marker)

	plainGit(t, dir, "log", "--format=%s", signed, "--", "docs")
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}

// TestReadDirRefusesByItsBounds: a directory over each bound of
// ReadDir, lowered for the test, is refused, and one at it is read.
// maxDirEntries counts directories too, so a directory of many
// directories is refused before its names are compared.
func TestReadDirRefusesByItsBounds(t *testing.T) {
	dir, _ := newRepo(t)
	blob := writeBlob(t, dir, "abc")
	sub := writeTree(t, dir, rawEntry{"100644", "x", blob})
	file := func(name string) rawEntry { return rawEntry{"100644", name, blob} }
	tree := func(name string) rawEntry { return rawEntry{"40000", name, sub} }
	tests := []struct {
		name     string
		bound    *int
		n        int
		at, over []rawEntry
		want     string
	}{
		{"entries", &maxDirEntries, 2, []rawEntry{file("a"), tree("b")}, []rawEntry{file("a"), tree("b"), tree("c")}, "more than 2 entries"},
		{"files", &maxDirFiles, 2, []rawEntry{file("a"), file("b"), tree("c")}, []rawEntry{file("a"), file("b"), file("c")}, "more than 2 files"},
		{"bytes", &maxDirBytes, 6, []rawEntry{file("a"), file("b")}, []rawEntry{file("a"), file("b"), file("c")}, "hold more than 6 bytes"},
		{"file bytes", &maxFileBytes, 3, []rawEntry{file("a")}, []rawEntry{{"100644", "big", writeBlob(t, dir, "abcd")}}, "holds more than 3 bytes"},
	}
	r := newRunner(t, Host)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower(t, tt.bound, tt.n)
			_, err := r.ReadDir(t.Context(), dir, tasksCommit(t, dir, tt.at...), "docs/tasks")
			require.NoError(t, err)
			_, err = r.ReadDir(t.Context(), dir, tasksCommit(t, dir, tt.over...), "docs/tasks")
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestReadDirRefusesABadListing: an ls-tree entry that is not directly
// under the directory, or whose size is not a count of bytes, is
// refused before cat-file runs.
func TestReadDirRefusesABadListing(t *testing.T) {
	oid := strings.Repeat("b", 40)
	for name, listing := range map[string]string{
		"below the directory": "100644 blob " + oid + " 3\tdocs/sub/a\x00",
		"outside it":          "100644 blob " + oid + " 3\tother/a\x00",
		"the directory":       "040000 tree " + oid + " -\tdocs/\x00",
		"a size of -":         "100644 blob " + oid + " -\tdocs/a\x00",
		"a negative size":     "100644 blob " + oid + " -5\tdocs/a\x00",
		"three fields":        "100644 blob " + oid + "\tdocs/a\x00",
		"a bad oid":           "100644 blob abc 3\tdocs/a\x00",
	} {
		r := newRunner(t, Host)
		var catFile bool
		r.start = func(ctx context.Context, c procgroup.Command) error {
			catFile = catFile || slices.Contains(c.Argv, "cat-file")
			return answer(map[string]string{"ls-tree": listing})(ctx, c)
		}
		_, err := r.ReadDir(t.Context(), stubRepo, strings.Repeat("a", 40), "docs")
		require.ErrorIs(t, err, ErrRefused, name)
		assert.False(t, catFile, name)
	}
}

// TestReadDirRefusesAMissingBlobRealGit: a tree that names a blob the
// repository does not hold is refused.
func TestReadDirRefusesAMissingBlobRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	c := tasksCommit(t, dir, rawEntry{"100644", "gone", strings.Repeat("de", 20)})
	_, err := newRunner(t, Host).ReadDir(t.Context(), dir, c, "docs/tasks")
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), `"docs/tasks/gone" has the size "BAD"`)
}

// TestHistoryNeverFetchesLazily plants a promisor remote on a listener
// of 127.0.0.1 and deletes a blob: ReadDir and History refuse or answer
// from what the repository holds and never connect, and plain git with
// the same config connects (the positive control).
func TestHistoryNeverFetchesLazily(t *testing.T) {
	var connections atomic.Int32
	url := listen(t, func(_ context.Context, _ net.Conn) { connections.Add(1) }) + "r.git"
	dir, _ := newRepo(t)
	commitFile(t, dir, "docs/tasks/T001.md", "gone\n", "plan", "2026-02-01T00:00:00Z")
	blob := plainGit(t, dir, "rev-parse", "HEAD:docs/tasks/T001.md")
	head := plainGit(t, dir, "rev-parse", "HEAD")
	require.NoError(t, os.Remove(filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])))
	for _, kv := range [][2]string{
		{"core.repositoryFormatVersion", "1"},
		{"extensions.partialClone", "origin"},
		{"remote.origin.url", url},
		{"remote.origin.promisor", "true"},
	} {
		plainGit(t, dir, "config", kv[0], kv[1])
	}
	r := newRunner(t, Host)
	_, err := r.ReadDir(t.Context(), dir, head, "docs/tasks")
	require.ErrorIs(t, err, ErrRefused)
	got, err := r.History(t.Context(), dir, "HEAD", "docs/tasks")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Zero(t, connections.Load(), "gitsafe connected to the promisor remote")

	_, err = plainGitErr(t, dir, []string{"GIT_TERMINAL_PROMPT=0"}, "cat-file", "-s", blob)
	require.Error(t, err)
	assert.NotZero(t, connections.Load(), "the plant is live: plain git fetches the blob")
}

// feedChunks writes out to w in chunks of n bytes.
func feedChunks(w io.Writer, out string, n int) {
	for len(out) > 0 {
		k := min(n, len(out))
		_, _ = w.Write([]byte(out[:k]))
		out = out[k:]
	}
}

// TestBatchReaderInChunks: the answer of cat-file read in chunks of
// every size from one byte up gives what the whole answer gives, with
// bodies that hold a NUL, blank lines and a line shaped like a header.
func TestBatchReaderInChunks(t *testing.T) {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	bodies := []string{"", "\n\n", "x\x00y", a + " missing\n" + b + " blob 3\n", "\n"}
	oids := []string{a, b, c, a, b}
	var want []blobRef
	var out strings.Builder
	for i, body := range bodies {
		want = append(want, blobRef{oid: oids[i], size: len(body)})
		out.WriteString(oids[i] + " blob " + strconv.Itoa(len(body)) + "\n" + body + "\n")
	}
	wantData := make([][]byte, len(bodies))
	for i, body := range bodies {
		wantData[i] = []byte(body)
	}
	for n := 1; n <= out.Len(); n++ {
		br := &batchReader{want: slices.Clone(want)}
		feedChunks(br, out.String(), n)
		require.NoError(t, br.err, "chunks of %d", n)
		assert.Empty(t, br.want, "chunks of %d", n)
		assert.Equal(t, wantData, br.data, "chunks of %d", n)
	}
	for n := 1; n <= 8; n++ {
		br := &batchReader{want: []blobRef{{oid: a, size: 1}}}
		feedChunks(br, a+" missing\n", n)
		require.ErrorIs(t, br.err, ErrRefused, "chunks of %d", n)
	}
}

// TestRecordsInChunks: NUL-terminated output read in chunks of every
// size from one byte up gives the records the whole output gives.
func TestRecordsInChunks(t *testing.T) {
	out := "one\x00\x00two words\x00" + strings.Repeat("x", 30) + "\x00"
	whole := []string{"one", "", "two words", strings.Repeat("x", 30)}
	for n := 1; n <= len(out); n++ {
		var got []string
		w := &records{limit: 64, each: func(r string) error { got = append(got, r); return nil }}
		feedChunks(w, out, n)
		require.NoError(t, w.close(), "chunks of %d", n)
		assert.Equal(t, whole, got, "chunks of %d", n)
	}
	for n := 1; n <= 8; n++ {
		w := &records{limit: 4, each: func(string) error { return nil }}
		feedChunks(w, "abcd\x00abcde\x00", n)
		require.ErrorIs(t, w.close(), ErrRefused, "chunks of %d", n)
	}
}
