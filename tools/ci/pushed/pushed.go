// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package pushed reads what a push would publish: the name each pushed
// ref gets on the remote and, of each pushed commit, its message, its
// two identities, the paths it adds or renames to, and the lines it
// adds (10 10.2, "Forbidden names"). It reads what each commit adds and
// not only the final tree, which is what catches a line that one commit
// adds and a later commit removes. Of a pushed annotated tag it reads
// the tag's own text as well: its name, its tagger and its message.
//
// The package finds and reads; what counts as a hit is the caller's
// business.
package pushed

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The fields of a Reading.
const (
	FieldRef       = "ref"       // the name a pushed ref gets on the remote
	FieldMessage   = "message"   // one line of a commit message or of a tag message
	FieldAuthor    = "author"    // the author's name and email
	FieldCommitter = "committer" // the committer's name and email
	FieldPath      = "path"      // a path the commit adds or renames to
	FieldLine      = "line"      // a line the commit adds
	FieldTagName   = "tag name"  // the name an annotated tag was created with
	FieldTagger    = "tagger"    // the tagger's name and email
)

// Reading is one line of text a push would publish, with where it is.
type Reading struct {
	// Commit is the commit id; empty for FieldRef and for a reading of
	// an annotated tag.
	Commit string
	// Tag is the id of the tag object, for a reading of an annotated
	// tag: FieldTagName, FieldTagger, and FieldMessage of its message.
	Tag string
	// Mergetag numbers, from 1, the mergetag header of commit Commit
	// that a reading comes from: a tag that merge embeds, read as
	// FieldTagName, FieldTagger and FieldMessage. It is 0 otherwise.
	Mergetag int
	// Field is one of the Field constants.
	Field string
	// Path is the file, for FieldPath and FieldLine.
	Path string
	// Line is the line of the hook's input for FieldRef, of the message
	// for FieldMessage, and of the file for FieldLine.
	Line int
	// Text is the line to check.
	Text []byte
}

// ref is one line of a pre-push hook's standard input.
type ref struct {
	localSHA, remoteRef, remoteSHA string
}

// Push is what one run of a pre-push hook is asked to publish: the
// lines git gives the hook on standard input, one per pushed ref.
type Push struct {
	refs []ref
}

// Parse reads the standard input of a pre-push hook. An error means
// the input is not a hook's, and the caller stops the push.
func Parse(stdin io.Reader) (Push, error) {
	refs, err := parseRefs(stdin)
	return Push{refs: refs}, err
}

// Tips returns the object each pushed ref points at locally, once
// each, in the order of the input. A line that deletes a ref names no
// object and adds none. A tip is a commit or, for an annotated tag, the
// tag object; git reads "<tip>:<path>" through either.
func (p Push) Tips() []string {
	var tips []string
	for _, r := range p.refs {
		if !isZero(r.localSHA) && !slices.Contains(tips, r.localSHA) {
			tips = append(tips, r.localSHA)
		}
	}
	return tips
}

// Walk calls visit for the remote name of each pushed ref, for the
// readings of each annotated tag a ref points at, and for the four
// readings of each commit the push would publish.
//
// remote is the hook's first argument: the remote's name, or the URL
// when the push names no configured remote. A line that deletes a ref
// adds no commit and is skipped, name included, so a ref with a
// forbidden name can be deleted. An error means the push could not be
// read, and the caller stops the push.
func Walk(ctx context.Context, repo git.Repo, remote string, p Push, visit func(Reading)) error {
	read := map[string]bool{} // tags and commits already read, when two refs share them
	for i, r := range p.refs {
		if isZero(r.localSHA) {
			continue
		}
		visit(Reading{Field: FieldRef, Line: i + 1, Text: []byte(r.remoteRef)})
		if err := wantCommit(ctx, repo, r.localSHA); err != nil {
			return err
		}
		if err := tagReadings(ctx, repo, r.localSHA, read, visit); err != nil {
			return err
		}
		commits, err := commitsOf(ctx, repo, remote, r)
		if err != nil {
			return err
		}
		for _, c := range commits {
			if read[c] {
				continue
			}
			read[c] = true
			if err := Readings(ctx, repo, c, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseRefs(stdin io.Reader) ([]ref, error) {
	var refs []ref
	sc := bufio.NewScanner(stdin)
	for n := 1; sc.Scan(); n++ {
		f := strings.Split(sc.Text(), " ")
		if len(f) != 4 {
			return nil, fmt.Errorf("pre-push input line %d: want 4 fields, got %d", n, len(f))
		}
		// The ids become arguments of git commands, so anything that is
		// not an object id is refused here.
		if !isObjectID(f[1]) || !isObjectID(f[3]) {
			return nil, fmt.Errorf("pre-push input line %d: a sha is not an object id", n)
		}
		refs = append(refs, ref{localSHA: f[1], remoteRef: f[2], remoteSHA: f[3]})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read pre-push input: %w", err)
	}
	return refs, nil
}

// commitsOf returns the commits one pushed ref would publish: those
// reachable from the local sha and not from the remote sha. When the
// remote has no such ref yet, or names a commit this repository does
// not hold, the commits no remote-tracking ref of that remote reaches
// stand in; a URL that is no configured remote has no such ref, and
// then every commit the local sha reaches is read.
func commitsOf(ctx context.Context, repo git.Repo, remote string, r ref) ([]string, error) {
	var exclude []string
	held := false
	if !isZero(r.remoteSHA) {
		var err error
		if held, err = holdsCommit(ctx, repo, r.remoteSHA); err != nil {
			return nil, err
		}
	}
	if held {
		exclude = []string{r.remoteSHA}
	} else {
		var err error
		if exclude, err = trackingTips(ctx, repo, remote); err != nil {
			return nil, err
		}
	}
	var in bytes.Buffer
	in.WriteString(r.localSHA + "\n")
	for _, e := range exclude {
		in.WriteString("^" + e + "\n")
	}
	out, err := repo.Run(ctx, in.Bytes(), "rev-list", "--stdin")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// wantCommit refuses a pushed ref that leads to no commit. A tag may
// point at a blob or a tree, and the walk reads commits: such a push
// would publish content that no reading covers.
func wantCommit(ctx context.Context, repo git.Repo, sha string) error {
	// "^{}" follows an annotated tag to what it tags.
	out, err := repo.Run(ctx, []byte(sha+"^{}\n"), "cat-file", "--batch-check")
	if err != nil {
		return err
	}
	f := strings.Fields(string(out))
	if len(f) != 3 {
		return errors.New("a pushed ref points at an object this repository does not hold")
	}
	if f[1] != "commit" {
		return fmt.Errorf("a pushed ref points at a %s, not at a commit; the check reads commits only", f[1])
	}
	return nil
}

// tagReadings calls visit for the readings of the annotated tag sha
// names, and of each tag that one tags in turn, down to the commit. A
// sha that names a commit has none. A push publishes a tag object with
// the commit it leads to, so its text is read like a commit's: the name
// it was created with, which need not be the name it is pushed under,
// its tagger and each line of its message. A signed tag carries its
// signature at the end of the message, and that is read as lines too.
func tagReadings(ctx context.Context, repo git.Repo, sha string, read map[string]bool, visit func(Reading)) error {
	for !read[sha] {
		out, err := repo.Run(ctx, []byte(sha+"\n"), "cat-file", "--batch-check")
		if err != nil {
			return err
		}
		// "<id> <type> <size>"
		if f := strings.Fields(string(out)); len(f) != 3 || f[1] != "tag" {
			return nil
		}
		read[sha] = true
		raw, err := repo.Run(ctx, nil, "cat-file", "tag", sha)
		if err != nil {
			return err
		}
		t, err := parseTag(raw)
		if err != nil {
			return fmt.Errorf("tag %s: %w", sha, err)
		}
		if !isObjectID(t.object) {
			return fmt.Errorf("tag %s: no object id in its header", sha)
		}
		visitTag(t, Reading{Tag: sha}, visit)
		sha = t.object
	}
	return nil
}

// visitTag calls visit for the three readings of a tag: its name, its
// tagger and each line of its message. at says where the tag is: a
// tag object, or a mergetag header of a commit.
func visitTag(t tag, at Reading, visit func(Reading)) {
	r := at
	r.Field, r.Text = FieldTagName, t.name
	visit(r)
	r.Field, r.Text = FieldTagger, t.tagger
	visit(r)
	for i, line := range bytes.Split(bytes.TrimRight(t.message, "\n"), []byte{'\n'}) {
		r.Field, r.Line, r.Text = FieldMessage, i+1, line
		visit(r)
	}
}

type tag struct {
	object       string // what the tag tags: a commit, or another tag
	name, tagger []byte // tagger is "Name <email>"
	message      []byte
}

// parseTag reads a raw tag object: header lines, a blank line, the
// message. An object, tag or tagger header that appears twice is an
// error (see once).
func parseTag(raw []byte) (tag, error) {
	var t tag
	head, message, _ := bytes.Cut(raw, []byte("\n\n"))
	t.message = message
	seen := map[string]bool{}
	for line := range bytes.SplitSeq(head, []byte{'\n'}) {
		key, value, _ := bytes.Cut(line, []byte{' '})
		if err := once(seen, string(key), "object", "tag", "tagger"); err != nil {
			return tag{}, err
		}
		switch string(key) {
		case "object":
			t.object = string(value)
		case "tag":
			t.name = value
		case "tagger":
			t.tagger = identity(value)
		}
	}
	return t, nil
}

// holdsCommit reports whether the repository has the commit. It asks
// with cat-file's batch mode, which answers "missing" with exit 0, so a
// commit that is absent is told apart from a git command that failed.
func holdsCommit(ctx context.Context, repo git.Repo, sha string) (bool, error) {
	out, err := repo.Run(ctx, []byte(sha+"^{commit}\n"), "cat-file", "--batch-check")
	if err != nil {
		return false, err
	}
	return !bytes.HasSuffix(bytes.TrimSpace(out), []byte(" missing")), nil
}

// trackingTips returns the commits the remote-tracking refs of remote
// point at, or nothing when remote is not a configured remote.
func trackingTips(ctx context.Context, repo git.Repo, remote string) ([]string, error) {
	out, err := repo.Run(ctx, nil, "remote")
	if err != nil {
		return nil, err
	}
	if !slices.Contains(strings.Split(strings.TrimSpace(string(out)), "\n"), remote) {
		return nil, nil
	}
	out, err = repo.Run(ctx, nil, "for-each-ref", "--format=%(objectname)", "refs/remotes/"+remote+"/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// Readings calls visit for the four readings of one commit. The commit
// is compared with its first parent, a merge commit too, and a commit
// without a parent with the empty tree.
func Readings(ctx context.Context, repo git.Repo, sha string, visit func(Reading)) error {
	if !isObjectID(sha) {
		return errors.New("readings: not an object id")
	}
	raw, err := repo.Run(ctx, nil, "cat-file", "commit", sha)
	if err != nil {
		return err
	}
	c, err := parseCommit(raw)
	if err != nil {
		return fmt.Errorf("commit %s: %w", sha, err)
	}
	for i, line := range bytes.Split(bytes.TrimRight(c.message, "\n"), []byte{'\n'}) {
		visit(Reading{Commit: sha, Field: FieldMessage, Line: i + 1, Text: line})
	}
	visit(Reading{Commit: sha, Field: FieldAuthor, Text: c.author})
	visit(Reading{Commit: sha, Field: FieldCommitter, Text: c.committer})
	for i, raw := range c.mergetags {
		t, err := parseTag(raw)
		if err == nil && !isObjectID(t.object) {
			err = errors.New("no object id in its header")
		}
		if err != nil {
			return fmt.Errorf("commit %s: mergetag %d: %w", sha, i+1, err)
		}
		visitTag(t, Reading{Commit: sha, Mergetag: i + 1}, visit)
	}

	base := c.firstParent
	if base == "" {
		out, err := repo.Run(ctx, nil, "hash-object", "-t", "tree", "--stdin")
		if err != nil {
			return err
		}
		base = strings.TrimSpace(string(out))
	}
	names, err := repo.Run(ctx, nil, "diff-tree", "-r", "-z", "-M", "--name-status", "--diff-filter=AR", base, sha)
	if err != nil {
		return err
	}
	for _, p := range addedPaths(names) {
		visit(Reading{Commit: sha, Field: FieldPath, Path: p, Text: []byte(p)})
	}
	// --text reads a binary file as lines too, as the check of the tree
	// does. The prefixes are spelled out so that no configuration
	// changes the shape the parser below expects.
	patch, err := repo.Run(ctx, nil, "diff-tree", "-r", "-p", "-M", "-U0", "--text", "--no-color",
		"--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/", base, sha)
	if err != nil {
		return err
	}
	err = addedLines(patch, func(path string, line int, text []byte) {
		visit(Reading{Commit: sha, Field: FieldLine, Path: path, Line: line, Text: text})
	})
	if err != nil {
		return fmt.Errorf("commit %s: %w", sha, err)
	}
	return nil
}

type commit struct {
	firstParent       string
	author, committer []byte   // "Name <email>"
	mergetags         [][]byte // each a raw tag object, unfolded
	message           []byte
}

// parseCommit reads a raw commit object: header lines, a blank line,
// the message. A header goes on over the lines after it that begin
// with one space; of those, a mergetag header is kept, unfolded: it is
// a whole tag object that "git merge" of a signed tag embeds. The other
// folded headers, such as a signature, are not read. An author or a
// committer header that appears twice is an error (see once).
func parseCommit(raw []byte) (commit, error) {
	var c commit
	head, message, _ := bytes.Cut(raw, []byte("\n\n"))
	c.message = message
	var key string
	seen := map[string]bool{}
	for line := range bytes.SplitSeq(head, []byte{'\n'}) {
		if more, ok := bytes.CutPrefix(line, []byte{' '}); ok {
			if key == "mergetag" {
				last := &c.mergetags[len(c.mergetags)-1]
				*last = append(append(*last, '\n'), more...)
			}
			continue
		}
		k, value, _ := bytes.Cut(line, []byte{' '})
		key = string(k)
		if err := once(seen, key, "author", "committer"); err != nil {
			return commit{}, err
		}
		switch key {
		case "mergetag":
			// A copy, so that the lines appended to it can never reach
			// raw, however the line it came from was cut.
			c.mergetags = append(c.mergetags, bytes.Clone(value))
		case "parent":
			if c.firstParent == "" {
				c.firstParent = string(value)
			}
		case "author":
			c.author = identity(value)
		case "committer":
			c.committer = identity(value)
		}
	}
	return c, nil
}

// once records key in seen and refuses a key of the read ones that
// was seen before. Git writes each of these headers once; with two,
// keeping either would leave the other unread. The error names the
// header and repeats nothing of its value.
func once(seen map[string]bool, key string, read ...string) error {
	if !slices.Contains(read, key) {
		return nil
	}
	if seen[key] {
		return fmt.Errorf("the %s header appears twice", key)
	}
	seen[key] = true
	return nil
}

// dateAndZone is what git writes after the email of an identity: the
// seconds since the epoch and the zone.
var dateAndZone = regexp.MustCompile(`^ [0-9]+ [+-][0-9]+$`)

// identity cuts the date off an author, committer or tagger header.
// Only a date in git's form is cut: any other text after the email is
// read with the rest.
func identity(value []byte) []byte {
	if i := bytes.LastIndexByte(value, '>'); i >= 0 && dateAndZone.Match(value[i+1:]) {
		return value[:i+1]
	}
	return value
}

// addedPaths reads "diff-tree -z --name-status" output limited to added
// and renamed files: a status, then one path, or two for a rename, of
// which the second is the new name.
func addedPaths(out []byte) []string {
	var paths []string
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		if strings.HasPrefix(fields[i], "R") {
			i++
		}
		if i+1 < len(fields) {
			paths = append(paths, fields[i+1])
		}
	}
	return paths
}

// addedLines reads a patch made with -U0 and calls add for each added
// line. It counts the lines each hunk header announces, so a line of
// content that looks like a header is read as content. An error names
// the hunk by its number in the patch and repeats nothing of it: the
// path and the lines are what the caller is checking.
func addedLines(patch []byte, add func(path string, line int, text []byte)) error {
	var path string
	var oldLeft, newLeft, lineNo, hunk int
	for len(patch) > 0 {
		line, rest, _ := bytes.Cut(patch, []byte{'\n'})
		patch = rest
		if oldLeft > 0 || newLeft > 0 {
			switch {
			case bytes.HasPrefix(line, []byte{'-'}) && oldLeft > 0:
				oldLeft--
			case bytes.HasPrefix(line, []byte{'+'}) && newLeft > 0:
				add(path, lineNo, line[1:])
				lineNo++
				newLeft--
			case bytes.HasPrefix(line, []byte{'\\'}):
				// "\ No newline at end of file"
			default:
				return fmt.Errorf("read patch: hunk %d does not have the lines its header announces", hunk)
			}
			continue
		}
		switch {
		case bytes.HasPrefix(line, []byte("+++ ")):
			var err error
			if path, err = patchPath(line[4:]); err != nil {
				return fmt.Errorf("read patch: the path header after hunk %d: %w", hunk, err)
			}
		case bytes.HasPrefix(line, []byte("@@ ")):
			hunk++
			var ok bool
			if oldLeft, lineNo, newLeft, ok = hunkHeader(string(line)); !ok {
				return fmt.Errorf("read patch: the header of hunk %d has no line ranges", hunk)
			}
		}
	}
	if oldLeft > 0 || newLeft > 0 {
		return fmt.Errorf("read patch: hunk %d does not have the lines its header announces", hunk)
	}
	return nil
}

// patchPath returns the path of a "+++ " header without its "b/". Git
// ends a path that holds a space with a tab, and writes a path that
// holds a byte outside printable ASCII, a double quote or a backslash
// in double quotes with C escapes; such a path is unquoted, so that it
// reads as the same bytes as the path of the tree. The error repeats
// nothing of the header.
func patchPath(header []byte) (string, error) {
	path := strings.TrimRight(string(header), "\t")
	if strings.HasPrefix(path, `"`) {
		var err error
		if path, err = strconv.Unquote(path); err != nil {
			return "", errors.New("a quoted path that does not unquote")
		}
	}
	return strings.TrimPrefix(path, "b/"), nil
}

// hunkHeader reads "@@ -a[,b] +c[,d] @@" and returns b, c and d; a
// count that is left out is 1.
func hunkHeader(line string) (oldCount, newStart, newCount int, ok bool) {
	f := strings.Fields(line)
	if len(f) < 4 || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return 0, 0, 0, false
	}
	_, oldCount, okOld := startCount(f[1][1:])
	newStart, newCount, okNew := startCount(f[2][1:])
	return oldCount, newStart, newCount, okOld && okNew
}

func startCount(s string) (start, count int, ok bool) {
	a, b, hasCount := strings.Cut(s, ",")
	start, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, false
	}
	count = 1
	if hasCount {
		if count, err = strconv.Atoi(b); err != nil {
			return 0, 0, false
		}
	}
	return start, count, true
}

// isObjectID reports whether s is a full sha1 or sha256 object id.
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	return strings.Trim(s, "0123456789abcdef") == ""
}

// isZero reports whether an object id is all zeros, git's way of
// saying "no object": a ref that is deleted or does not exist yet.
func isZero(s string) bool {
	return strings.Trim(s, "0") == ""
}
