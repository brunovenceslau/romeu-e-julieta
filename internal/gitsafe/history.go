// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// historySettings are the extra settings of History and ReadDir: those
// of the reads of fsck.go, so no replace ref swaps an object; a log
// that checks no signature, which would run gpg.program from the
// repository's own config, and follows no rename; and no commit-graph
// file, whose parents could disagree with the commits'.
var historySettings = slices.Concat(readSettings, []setting{
	{"log.showSignature", "false"},
	{"log.follow", "false"},
	{"core.commitGraph", "false"},
})

// historyEnv is what History and ReadDir add to the environment of the
// mode: no lazy fetch, so a promisor remote in the repository's config
// never sends git to the network for a missing object, past CheckURL
// and the gitHosts; and no graft file, so the history is the commits'
// own. GIT_NO_LAZY_FETCH came with git 2.45.0 and --end-of-options,
// which both verbs pass, with 2.24.0 (their release notes), both below
// the floor of floor.go (2.45.4).
var historyEnv = []string{"GIT_NO_LAZY_FETCH=1", "GIT_GRAFT_FILE=/dev/null"}

// The bounds of the history reads, each far above the plan's: the
// commits of one history and the bytes of one subject, so a history
// holds at most about 22 MiB; the entries of one directory, directories
// included, and the files among them; the bytes of one file and of all
// of them. They are variables so the tests can lower them.
var (
	maxHistoryCommits = 20_000
	maxSubjectBytes   = 1 << 10
	maxDirEntries     = 10_000
	maxDirFiles       = 10_000
	maxFileBytes      = 1 << 20
	maxDirBytes       = 64 << 20
)

// maxHistoryRecord is the longest "<oid> <time> <subject>" record: a
// SHA-256 id, a 64-bit time and a subject at its bound.
func maxHistoryRecord() int { return 64 + 1 + 20 + 1 + maxSubjectBytes }

// Commit is one commit of a History: its full object id, its committer
// date and its subject, the first line of its message as git log %s
// gives it. The subject is repository content: valid UTF-8, at most
// maxSubjectBytes, with no character unsafeRune refuses in a name.
type Commit struct {
	OID     string
	Time    time.Time
	Subject string
}

// History lists the commits that touched path on the first-parent
// history of rev in the repository in dir, newest first, as git log
// --first-parent lists them, under the flags of the mode. It reads and
// writes nothing else. rev is a full object id, HEAD or a full ref name
// under refs/; path is a file or a directory from the root of the
// tree, taken literally (no pathspec magic), and holds no name the tree
// walk refuses. A path no commit touched gives no commit and no error.
// A shallow repository is refused, since its history is cut short, and
// so is a subject over maxSubjectBytes or one that holds a character
// unsafeRune refuses; a history over maxHistoryCommits is refused too.
func (r *Runner) History(ctx context.Context, dir, rev, path string) ([]Commit, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	if err := checkRev(rev); err != nil {
		return nil, err
	}
	if _, err := checkTreePath(path); err != nil {
		return nil, err
	}
	shallow := limitedBuffer{limit: 64}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, nil, &shallow, "rev-parse", "--is-shallow-repository"); err != nil {
		return nil, err
	}
	if answer := strings.TrimSpace(shallow.String()); answer != "false" {
		return nil, fmt.Errorf("%w: %q is a shallow repository (rev-parse answered %q)", ErrRefused, dir, answer)
	}
	var commits []Commit
	w := &records{limit: maxHistoryRecord(), each: func(rec string) error {
		if len(commits) == maxHistoryCommits {
			return fmt.Errorf("%w: the history holds more than %d commits", ErrRefused, maxHistoryCommits)
		}
		c, err := parseCommit(rec)
		if err != nil {
			return err
		}
		commits = append(commits, c)
		return nil
	}}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, nil, w, "log", "--first-parent", "-z", "--format=%H %ct %s", "--end-of-options", rev, "--", ":(literal)"+path); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return commits, nil
}

// parseCommit reads one "<oid> <committer time> <subject>" record.
func parseCommit(rec string) (Commit, error) {
	oid, rest, ok1 := strings.Cut(rec, " ")
	secs, subject, ok2 := strings.Cut(rest, " ")
	if !ok1 || !ok2 {
		return Commit{}, fmt.Errorf("%w: log record %q", ErrRefused, rec)
	}
	if err := checkObjectID(oid); err != nil {
		return Commit{}, err
	}
	n, err := strconv.ParseInt(secs, 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("%w: commit %s has the date %q", ErrRefused, oid, secs)
	}
	if !utf8.ValidString(subject) {
		return Commit{}, fmt.Errorf("%w: the subject of commit %s is not valid UTF-8", ErrRefused, oid)
	}
	if len(subject) > maxSubjectBytes {
		return Commit{}, fmt.Errorf("%w: the subject of commit %s is longer than %d bytes", ErrRefused, oid, maxSubjectBytes)
	}
	if i := strings.IndexFunc(subject, unsafeRune); i >= 0 {
		r, _ := utf8.DecodeRuneInString(subject[i:])
		return Commit{}, fmt.Errorf("%w: the subject of commit %s holds the character %U", ErrRefused, oid, r)
	}
	return Commit{OID: oid, Time: time.Unix(n, 0).UTC(), Subject: subject}, nil
}

// checkRev returns ErrRefused for a revision that is not a full object
// id, HEAD or a full ref name checkRefName accepts.
func checkRev(rev string) error {
	if rev == "HEAD" || checkObjectID(rev) == nil {
		return nil
	}
	return checkRefName(rev, false)
}

// File is one regular file ReadDir read: its name in the directory, its
// mode, modeFile or modeExecutable, and its bytes.
type File struct {
	Name string
	Mode string
	Data []byte
}

// ReadDir reads the regular files directly under the directory path of
// commit in the repository in dir, in the order git lists them, under
// the flags of the mode, with one git ls-tree and one git cat-file
// --batch. A directory below path is skipped; a symbolic link, a
// gitlink or any other mode is refused, and so is a name the tree walk
// refuses (WalkTree), two names that are one under its comparison, a
// file over 1 MiB, more than 10000 entries, more than 10000 files or
// more than 64 MiB in all, each with ErrRefused; so is a blob the
// repository does not hold, which git is not allowed to fetch. A path the commit does not hold gives no file
// and no error. commit is a full object id; path is taken as History
// takes it.
func (r *Runner) ReadDir(ctx context.Context, dir, commit, path string) ([]File, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	if err := checkObjectID(commit); err != nil {
		return nil, err
	}
	if _, err := checkTreePath(path); err != nil {
		return nil, err
	}
	l := &dirLister{prefix: path + "/", seen: map[string]string{}}
	w := &records{limit: maxTreePath + 128, each: l.entry}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, nil, w, "ls-tree", "-l", "-z", "--full-tree", "--end-of-options", commit, "--", ":(literal)"+path+"/"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	if len(l.files) == 0 {
		return nil, nil
	}
	var stdin strings.Builder
	for _, b := range l.blobs {
		stdin.WriteString(b.oid + "\n")
	}
	br := &batchReader{want: l.blobs}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, strings.NewReader(stdin.String()), br, "cat-file", "--batch"); err != nil {
		return nil, err
	}
	if br.err == nil && (len(br.want) > 0 || br.partial.Len() > 0) {
		br.err = fmt.Errorf("%w: cat-file output ends inside an object", ErrRefused)
	}
	if br.err != nil {
		return nil, br.err
	}
	for i := range l.files {
		l.files[i].Data = br.data[i]
	}
	return l.files, nil
}

// blobRef is one blob ReadDir asks cat-file for, with the size ls-tree
// gave it.
type blobRef struct {
	oid  string
	size int
}

// dirLister checks the entries of ls-tree -l under one directory: four
// fields, "<mode> <type> <oid> <size>", then a tab and the path.
type dirLister struct {
	prefix  string
	entries int
	seen    map[string]string // folded name -> the name that holds it
	files   []File
	blobs   []blobRef
	total   int
}

// entry checks one entry and keeps it when it is a regular file.
func (l *dirLister) entry(line string) error {
	l.entries++
	if l.entries > maxDirEntries {
		return fmt.Errorf("%w: %q holds more than %d entries", ErrRefused, l.prefix, maxDirEntries)
	}
	meta, path, ok := strings.Cut(line, "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 4 {
		return fmt.Errorf("%w: ls-tree entry %q", ErrRefused, line)
	}
	mode, kind, oid, size := fields[0], fields[1], fields[2], fields[3]
	if err := checkObjectID(oid); err != nil {
		return err
	}
	name, ok := strings.CutPrefix(path, l.prefix)
	if !ok || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("%w: ls-tree entry %q is not directly under %q", ErrRefused, path, l.prefix)
	}
	folded, err := checkTreePath(path)
	if err != nil {
		return err
	}
	if other, dup := l.seen[folded]; dup {
		return fmt.Errorf("%w: %q and %q are one path under case-insensitive comparison or Unicode normalization", ErrRefused, other, path)
	}
	l.seen[folded] = path
	switch {
	case (mode == modeFile || mode == modeExecutable) && kind == "blob":
		// A regular file: on to the size checks below.
	case mode == modeTree && kind == "tree":
		return nil
	case mode == "120000":
		return fmt.Errorf("%w: %q is a symbolic link", ErrRefused, path)
	case mode == "160000":
		return fmt.Errorf("%w: %q is a gitlink", ErrRefused, path)
	default:
		return fmt.Errorf("%w: %q has the mode %s %s", ErrRefused, path, mode, kind)
	}
	n, err := strconv.Atoi(size)
	if err != nil || n < 0 {
		return fmt.Errorf("%w: %q has the size %q", ErrRefused, path, size)
	}
	switch {
	case n > maxFileBytes:
		return fmt.Errorf("%w: %q holds more than %d bytes", ErrRefused, path, maxFileBytes)
	case len(l.files) == maxDirFiles:
		return fmt.Errorf("%w: %q holds more than %d files", ErrRefused, l.prefix, maxDirFiles)
	case l.total+n > maxDirBytes:
		return fmt.Errorf("%w: the files of %q hold more than %d bytes", ErrRefused, l.prefix, maxDirBytes)
	}
	l.total += n
	l.files = append(l.files, File{Name: name, Mode: mode})
	l.blobs = append(l.blobs, blobRef{oid: oid, size: n})
	return nil
}

// batchReader reads the answers of cat-file --batch to the blobs of
// want, in order: "<oid> blob <size>\n", the bytes, then "\n". Each
// answer must name the blob asked for, with the size ls-tree gave it.
// After the first refusal it keeps reading and drops the rest, so git
// never blocks on a full pipe.
type batchReader struct {
	want    []blobRef
	partial bytes.Buffer // the header read so far, or the body and its newline
	inBody  bool
	data    [][]byte
	err     error
}

// Write takes the next part of the output.
func (b *batchReader) Write(p []byte) (int, error) {
	if b.err != nil {
		return len(p), nil
	}
	rest := p
	for len(rest) > 0 && b.err == nil {
		if len(b.want) == 0 {
			b.err = fmt.Errorf("%w: cat-file wrote more than it was asked for", ErrRefused)
			break
		}
		if !b.inBody {
			i := bytes.IndexByte(rest, '\n')
			if i < 0 {
				b.partial.Write(rest)
				if b.partial.Len() > 256 {
					b.err = fmt.Errorf("%w: a cat-file header longer than 256 bytes", ErrRefused)
				}
				break
			}
			b.partial.Write(rest[:i])
			rest = rest[i+1:]
			b.err = b.header(b.partial.String())
			b.partial.Reset()
			b.inBody = true
			continue
		}
		need := b.want[0].size + 1 - b.partial.Len()
		take := min(need, len(rest))
		b.partial.Write(rest[:take])
		rest = rest[take:]
		if take < need {
			break
		}
		body := b.partial.Bytes()
		if body[len(body)-1] != '\n' {
			b.err = fmt.Errorf("%w: cat-file object %s does not end where its size says", ErrRefused, b.want[0].oid)
			break
		}
		b.data = append(b.data, bytes.Clone(body[:len(body)-1]))
		b.partial.Reset()
		b.inBody = false
		b.want = b.want[1:]
	}
	return len(p), nil
}

// header checks one answer header against the next blob asked for.
func (b *batchReader) header(line string) error {
	want := b.want[0]
	if line != want.oid+" blob "+strconv.Itoa(want.size) {
		return fmt.Errorf("%w: cat-file answered %q for the blob %s of %d bytes", ErrRefused, line, want.oid, want.size)
	}
	return nil
}

// records splits NUL-terminated output into records and passes each to
// each. After the first refusal it keeps reading and drops the rest, so
// git never blocks on a full pipe.
type records struct {
	partial bytes.Buffer
	limit   int
	each    func(string) error
	err     error
}

// Write takes the next part of the output.
func (w *records) Write(p []byte) (int, error) {
	if w.err != nil {
		return len(p), nil
	}
	rest := p
	for len(rest) > 0 {
		i := bytes.IndexByte(rest, 0)
		end := i
		if i < 0 {
			end = len(rest)
		}
		if w.partial.Len()+end > w.limit {
			w.err = fmt.Errorf("%w: a record of git's output longer than %d bytes", ErrRefused, w.limit)
			return len(p), nil
		}
		if i < 0 {
			w.partial.Write(rest)
			break
		}
		w.partial.Write(rest[:i])
		rest = rest[i+1:]
		w.err = w.each(w.partial.String())
		w.partial.Reset()
		if w.err != nil {
			break
		}
	}
	return len(p), nil
}

// close returns the first refusal, or ErrRefused when the output ends
// inside a record.
func (w *records) close() error {
	if w.err == nil && w.partial.Len() > 0 {
		w.err = fmt.Errorf("%w: git's output ends inside a record", ErrRefused)
	}
	return w.err
}
