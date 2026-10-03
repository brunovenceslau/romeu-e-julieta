// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pushed

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

const zero = "0000000000000000000000000000000000000000"

// fixture is a repository with one remote, origin, that holds commit a
// on main. Locally main goes on to b and c, feature leaves b with d,
// and m merges feature into main.
//
//	a - b - c - m   main
//	     \     /
//	      d ---     feature
type fixture struct {
	repo          *gittest.Repo
	a, b, c, d, m string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	origin := gittest.NewBare(t)
	r := gittest.New(t)
	f := fixture{repo: r}
	r.Git("remote", "add", "origin", origin.Dir)
	r.Write("a.txt", "a\n")
	f.a = r.Commit("a")
	r.Git("push", "--quiet", "origin", "main")
	r.Write("b.txt", "b\n")
	f.b = r.Commit("b")
	r.Git("branch", "feature")
	r.Write("c.txt", "c\n")
	f.c = r.Commit("c")
	r.Git("checkout", "--quiet", "feature")
	r.Write("d.txt", "d one\nd two\n")
	f.d = r.Commit("d")
	r.Git("checkout", "--quiet", "main")
	r.Git("merge", "--quiet", "--no-ff", "--message", "m", "feature")
	f.m = r.Git("rev-parse", "HEAD")
	return f
}

// walk runs Walk and returns the commits it read, sorted, and the ref
// names it read, in order. A commit is read once, however many pushed
// refs reach it; the author reading comes once per read, so it counts
// them.
func walk(t *testing.T, f fixture, remote, stdin string) (commits, refs []string, err error) {
	t.Helper()
	reads := map[string]int{}
	push, err := Parse(strings.NewReader(stdin))
	if err != nil {
		return nil, nil, err
	}
	err = Walk(t.Context(), f.repo.Repo, remote, push, func(rd Reading) {
		switch rd.Field {
		case FieldRef:
			refs = append(refs, string(rd.Text))
		case FieldAuthor:
			reads[rd.Commit]++
		}
	})
	for c, n := range reads {
		if n != 1 {
			t.Errorf("commit %s was read %d times, want once", c, n)
		}
		commits = append(commits, c)
	}
	slices.Sort(commits)
	return commits, refs, err
}

func sorted(s ...string) []string {
	slices.Sort(s)
	return s
}

// TestWalkRanges is the fixture-repository table of 10 10.2: which
// commits a push is read for, case by case.
func TestWalkRanges(t *testing.T) {
	f := newFixture(t)
	missing := strings.Repeat("12", 20) // a commit id the fixture does not hold
	f.repo.Git("tag", "--annotate", "--message", "a tag", "v-test", f.c)
	tag := f.repo.Git("rev-parse", "refs/tags/v-test")
	tests := []struct {
		name    string
		remote  string
		stdin   string
		commits []string
		refs    []string
	}{
		{
			name:    "a ref the remote has: the commits after the remote sha",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, f.a),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a new ref: the commits no remote-tracking ref reaches",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.b, f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a remote sha the local repository lacks: the same fallback",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, missing),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a URL that is no remote: every commit the local sha reaches",
			remote:  "https://example.invalid/some/repository.git",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.a, f.b, f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a URL that is no remote, with a remote sha the repository holds",
			remote:  "https://example.invalid/some/repository.git",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, f.b),
			commits: sorted(f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a merge commit: the merge and both sides",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.m, f.a),
			commits: sorted(f.b, f.c, f.d, f.m),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a merge commit whose parents the remote has: the merge alone",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.m, f.c) + fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, f.d),
			commits: sorted(f.d, f.m),
			refs:    []string{"refs/heads/main", "refs/heads/feature"},
		},
		{
			name:    "a deleted ref: no commit and no name",
			remote:  "origin",
			stdin:   fmt.Sprintf("(delete) %s refs/heads/gone %s\n", zero, f.a),
			commits: nil,
			refs:    nil,
		},
		{
			name:    "two refs in one push",
			remote:  "origin",
			stdin:   fmt.Sprintf("(delete) %s refs/heads/gone %s\nrefs/heads/main %s refs/heads/renamed %s\n", zero, f.a, f.b, zero),
			commits: sorted(f.b),
			refs:    []string{"refs/heads/renamed"},
		},
		{
			name:   "two refs that share a commit: it is read once",
			remote: "origin",
			stdin: fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, f.a) +
				fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.b, f.c, f.d),
			refs:    []string{"refs/heads/main", "refs/heads/feature"},
		},
		{
			name:    "an annotated tag: the commits it leads to",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/tags/v-test %s refs/tags/v-test %s\n", tag, zero),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/tags/v-test"},
		},
		{
			name:    "nothing pushed",
			remote:  "origin",
			stdin:   "",
			commits: nil,
			refs:    nil,
		},
		{
			name:    "up to date",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.a, f.a),
			commits: nil,
			refs:    []string{"refs/heads/main"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commits, refs, err := walk(t, f, tt.remote, tt.stdin)
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if !reflect.DeepEqual(commits, tt.commits) {
				t.Errorf("commits = %v, want %v", commits, tt.commits)
			}
			if !reflect.DeepEqual(refs, tt.refs) {
				t.Errorf("refs = %v, want %v", refs, tt.refs)
			}
		})
	}
}

// TestWalkStops shows that input Walk cannot read, and a git command
// that fails, are errors: either one stops the push.
func TestWalkStops(t *testing.T) {
	f := newFixture(t)
	missing := strings.Repeat("12", 20)
	blob := f.repo.Git("rev-parse", f.c+":c.txt")
	tree := f.repo.Git("rev-parse", f.c+"^{tree}")
	tests := []struct {
		name  string
		stdin string
	}{
		{"a ref that points at a blob", "refs/tags/b " + blob + " refs/tags/b " + zero + "\n"},
		{"a ref that points at a tree", "refs/tags/t " + tree + " refs/tags/t " + zero + "\n"},
		{"three fields", "refs/heads/main " + f.c + " refs/heads/main\n"},
		{"a local sha that is no object id", "refs/heads/main --all refs/heads/main " + f.a + "\n"},
		{"a remote sha that is no object id", "refs/heads/main " + f.c + " refs/heads/main ^" + f.a[:39] + "\n"},
		{"a local sha the repository lacks", "refs/heads/main " + missing + " refs/heads/main " + f.a + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := walk(t, f, "origin", tt.stdin); err == nil {
				t.Error("Walk succeeded, want an error")
			}
		})
	}
}

// TestWalkTags covers the text of an annotated tag: the name it was
// created with, its tagger and its message are read, once per tag
// object, and a tag that tags a tag is followed down to the commit.
func TestWalkTags(t *testing.T) {
	f := newFixture(t)
	r := f.repo
	r.Git("tag", "--annotate", "--message", "first line\n\nthird line", "v-one", f.c)
	one := r.Git("rev-parse", "refs/tags/v-one")
	r.WithEnv("GIT_COMMITTER_NAME=Tag Maker").Git("tag", "--annotate", "--message", "outer", "v-two", one)
	two := r.Git("rev-parse", "refs/tags/v-two")
	r.Git("tag", "v-light", f.c)
	// A signed tag stores its signature at the end of its message, as
	// lines; a literal block stands in for one, no key needed.
	r.Git("tag", "--annotate", "--message", "signed\n-----BEGIN SSH SIGNATURE-----\nU1NIU0lHAAAA\n-----END SSH SIGNATURE-----", "v-signed", f.c)
	signed := r.Git("rev-parse", "refs/tags/v-signed")

	oneReadings := []string{
		one + "|tag name|0|v-one",
		one + "|tagger|0|Test Committer <committer@example.invalid>",
		one + "|message|1|first line",
		one + "|message|2|",
		one + "|message|3|third line",
	}
	twoReadings := []string{
		two + "|tag name|0|v-two",
		two + "|tagger|0|Tag Maker <committer@example.invalid>",
		two + "|message|1|outer",
	}
	push := func(sha, name string) string {
		return fmt.Sprintf("refs/tags/%s %s refs/tags/%s %s\n", name, sha, name, zero)
	}
	tests := []struct {
		name  string
		stdin string
		want  []string
	}{
		{"an annotated tag", push(one, "v-one"), oneReadings},
		{"a tag of a tag: both, the outer one first", push(two, "v-two"), slices.Concat(twoReadings, oneReadings)},
		{"two refs that share a tag: read once", push(one, "v-one") + push(two, "v-two") + push(one, "v-same"),
			slices.Concat(oneReadings, twoReadings)},
		{"a tag that is only a name for a commit", push(f.c, "v-light"), nil},
		{"a signed tag: the signature lines are read as message lines", push(signed, "v-signed"), []string{
			signed + "|tag name|0|v-signed",
			signed + "|tagger|0|Test Committer <committer@example.invalid>",
			signed + "|message|1|signed",
			signed + "|message|2|-----BEGIN SSH SIGNATURE-----",
			signed + "|message|3|U1NIU0lHAAAA",
			signed + "|message|4|-----END SSH SIGNATURE-----",
		}},
		{"a deleted tag", fmt.Sprintf("(delete) %s refs/tags/v-one %s\n", zero, one), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(strings.NewReader(tt.stdin))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			var got []string
			err = Walk(t.Context(), r.Repo, "origin", p, func(rd Reading) {
				if rd.Tag == "" {
					return
				}
				if rd.Commit != "" {
					t.Errorf("a reading of tag %s names commit %s", rd.Tag, rd.Commit)
				}
				got = append(got, fmt.Sprintf("%s|%s|%d|%s", rd.Tag, rd.Field, rd.Line, rd.Text))
			})
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tag readings:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

// TestTagReadingsCraftedTag shows that a tag object whose header names
// something other than an object id is refused: what a tag names goes
// on to git commands, and git itself writes ids only.
func TestTagReadingsCraftedTag(t *testing.T) {
	f := newFixture(t)
	raw := "object refs/heads/main\ntype commit\ntag v-crafted\ntagger T <t@example.invalid> 0 +0000\n\nmessage\n"
	out, err := f.repo.Run(t.Context(), []byte(raw), "hash-object", "-t", "tag", "-w", "--literally", "--stdin")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	crafted := strings.TrimSpace(string(out))
	err = tagReadings(t.Context(), f.repo.Repo, crafted, map[string]bool{}, func(Reading) {})
	if err == nil {
		t.Fatal("tagReadings succeeded, want an error")
	}
	if strings.Contains(err.Error(), "refs/heads/main") {
		t.Errorf("the error repeats the header: %v", err)
	}
}

// TestDuplicatedHeaders shows that a commit or a tag that names one of
// the headers the check reads twice is refused: git writes each once,
// and reading only one of the two would leave the other unread. The
// error names the header and repeats nothing of its value.
func TestDuplicatedHeaders(t *testing.T) {
	f := newFixture(t)
	tree := f.repo.Git("rev-parse", f.a+"^{tree}")
	who := " Zorvex Quimby <zq@example.invalid> 1767323045 +0000\n"
	commit := func(headers string) string {
		return "tree " + tree + "\nparent " + f.a + "\n" + headers + "\nmessage\n"
	}
	tagBody := func(headers string) string {
		return "object " + f.a + "\ntype commit\n" + headers + "\nmessage\n"
	}
	folded := func(raw string) string { return strings.ReplaceAll(strings.TrimSuffix(raw, "\n"), "\n", "\n ") + "\n" }
	plain := "author" + who + "committer" + who
	tests := []struct {
		name, kind, raw, header string
	}{
		{"two authors", "commit", commit("author" + who + "author" + who + "committer" + who), "author"},
		{"two committers", "commit", commit("author" + who + "committer" + who + "committer" + who), "committer"},
		{"two taggers", "tag", tagBody("tag v-x\ntagger" + who + "tagger" + who), "tagger"},
		{"two tag names", "tag", tagBody("tag v-x\ntag v-y\ntagger" + who), "tag"},
		{"two objects", "tag", "object " + f.a + "\nobject " + f.b + "\ntype commit\ntag v-x\ntagger" + who + "\nmessage\n", "object"},
		{"two taggers in a mergetag", "commit", commit(plain + "mergetag " + folded(tagBody("tag v-x\ntagger"+who+"tagger"+who))), "tagger"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := f.repo.Run(t.Context(), []byte(tt.raw), "hash-object", "-t", tt.kind, "-w", "--literally", "--stdin")
			if err != nil {
				t.Fatalf("fixture: %v", err)
			}
			sha := strings.TrimSpace(string(out))
			if tt.kind == "commit" {
				err = Readings(t.Context(), f.repo.Repo, sha, func(Reading) {})
			} else {
				err = tagReadings(t.Context(), f.repo.Repo, sha, map[string]bool{}, func(Reading) {})
			}
			if err == nil {
				t.Fatal("read without an error, want one")
			}
			if !strings.Contains(err.Error(), "the "+tt.header+" header appears twice") {
				t.Errorf("error = %v, want one that names the %s header", err, tt.header)
			}
			if low := strings.ToLower(err.Error()); strings.Contains(low, "quimby") || strings.Contains(low, "v-x") {
				t.Errorf("the error repeats a value: %v", err)
			}
		})
	}
}

// TestMergetagCraftedObject shows that a mergetag whose object is no
// object id is refused, as a crafted tag object is.
func TestMergetagCraftedObject(t *testing.T) {
	f := newFixture(t)
	tree := f.repo.Git("rev-parse", f.a+"^{tree}")
	raw := "tree " + tree + "\nparent " + f.a + "\n" +
		"author T <t@example.invalid> 1767323045 +0000\ncommitter T <t@example.invalid> 1767323045 +0000\n" +
		"mergetag object refs/heads/main\n type commit\n tag v-crafted\n tagger T <t@example.invalid> 0 +0000\n \n m\n" +
		"\nmessage\n"
	out, err := f.repo.Run(t.Context(), []byte(raw), "hash-object", "-t", "commit", "-w", "--literally", "--stdin")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	err = Readings(t.Context(), f.repo.Repo, strings.TrimSpace(string(out)), func(Reading) {})
	if err == nil {
		t.Fatal("Readings succeeded, want an error")
	}
	if strings.Contains(err.Error(), "refs/heads/main") || strings.Contains(err.Error(), "v-crafted") {
		t.Errorf("the error repeats the header: %v", err)
	}
}

// TestParseCommitLeavesRaw shows that reading a mergetag, whose lines
// are joined into one value, leaves the object it was read from as it
// was.
func TestParseCommitLeavesRaw(t *testing.T) {
	id := strings.Repeat("c", 40)
	raw := []byte("tree " + id + "\nparent " + id + "\nauthor T <t@example.invalid> 0 +0000\n" +
		"mergetag object " + id + "\n type commit\n tag v-a\n tagger T <t@example.invalid> 0 +0000\n \n notes\n" +
		"committer T <t@example.invalid> 0 +0000\n\nmessage\n")
	before := bytes.Clone(raw)
	c, err := parseCommit(raw)
	if err != nil {
		t.Fatalf("parseCommit: %v", err)
	}
	if !bytes.Equal(raw, before) {
		t.Errorf("parseCommit changed the object it read:\n%q\nwant\n%q", raw, before)
	}
	if want := "object " + id + "\ntype commit\ntag v-a\ntagger T <t@example.invalid> 0 +0000\n\nnotes"; len(c.mergetags) != 1 || string(c.mergetags[0]) != want {
		t.Errorf("mergetags = %q, want [%q]", c.mergetags, want)
	}
	if string(c.committer) != "T <t@example.invalid>" {
		t.Errorf("committer = %q", c.committer)
	}
}

// TestPatchPathMalformed shows that a quoted path header that does not
// unquote is an error, and that the error repeats nothing of it.
func TestPatchPathMalformed(t *testing.T) {
	for _, header := range []string{`"b/zorvex quimby`, `"b/zorvex\quimby"`, `"b/zorvex\400quimby"`} {
		t.Run(header, func(t *testing.T) {
			_, err := patchPath([]byte(header))
			if err == nil {
				t.Fatal("patchPath succeeded, want an error")
			}
			if strings.Contains(err.Error(), "zorvex") || strings.Contains(err.Error(), "quimby") {
				t.Errorf("the error repeats the header: %v", err)
			}
			patch := "diff --git a/x b/x\n+++ " + header + "\n@@ -0,0 +1 @@\n+zorvex\n"
			err = addedLines([]byte(patch), func(string, int, []byte) {})
			if err == nil || strings.Contains(err.Error(), "zorvex") {
				t.Errorf("addedLines error = %v, want one that repeats nothing", err)
			}
		})
	}
}

// TestPushTips covers the objects a push names: one per ref that is
// not deleted, once each, in the order of the input.
func TestPushTips(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	stdin := fmt.Sprintf("refs/heads/x %s refs/heads/x %s\n(delete) %s refs/heads/y %s\nrefs/heads/z %s refs/heads/z %s\nrefs/heads/w %s refs/heads/w %s\n",
		b, zero, zero, a, a, b, b, zero)
	p, err := Parse(strings.NewReader(stdin))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := p.Tips(), []string{b, a}; !reflect.DeepEqual(got, want) {
		t.Errorf("Tips = %v, want %v", got, want)
	}
	if got := (Push{}).Tips(); len(got) != 0 {
		t.Errorf("Tips of an empty push = %v, want none", got)
	}
}

func mustParseTag(t *testing.T, raw []byte) tag {
	t.Helper()
	got, err := parseTag(raw)
	if err != nil {
		t.Fatalf("parseTag: %v", err)
	}
	return got
}

// TestParseTag covers a tag object that lacks a header: an old tag has
// no tagger, and the object id is what the walk refuses to go without.
func TestParseTag(t *testing.T) {
	id := strings.Repeat("c", 40)
	got := mustParseTag(t, []byte("object "+id+"\ntype commit\ntag v-old\n\nthe message\n\nwith a blank line\n"))
	if got.object != id || string(got.name) != "v-old" || got.tagger != nil {
		t.Errorf("header = %q %q %q", got.object, got.name, got.tagger)
	}
	if want := "the message\n\nwith a blank line\n"; string(got.message) != want {
		t.Errorf("message = %q, want %q", got.message, want)
	}
	if got := mustParseTag(t, []byte("tag v-none\n\nmessage\n")); got.object != "" {
		t.Errorf("object = %q, want none", got.object)
	}
	// A tag with an empty message has no blank line after its header.
	got = mustParseTag(t, []byte("object "+id+"\ntype commit\ntag v-empty\ntagger T <t@example.invalid> 0 +0000\n"))
	if got.message != nil || string(got.tagger) != "T <t@example.invalid>" || string(got.name) != "v-empty" {
		t.Errorf("empty message: %q %q %q", got.message, got.tagger, got.name)
	}
	// A message without a final newline keeps its last line.
	got = mustParseTag(t, []byte("object "+id+"\ntag v-short\n\nno newline"))
	if string(got.message) != "no newline" {
		t.Errorf("message = %q, want %q", got.message, "no newline")
	}
}

// TestIdentity covers an identity header without an email: there is no
// ">" to cut at, so the whole value is read, date included.
func TestIdentity(t *testing.T) {
	tests := []struct{ name, value, want string }{
		{"name and email", "T <t@example.invalid> 0 +0000", "T <t@example.invalid>"},
		{"no email", "T 0 +0000", "T 0 +0000"},
		{"empty", "", ""},
		{"text after the email that is no date: all of it", "T <t@example.invalid> zorvex quimby", "T <t@example.invalid> zorvex quimby"},
		{"a date without a zone: all of it", "T <t@example.invalid> 0", "T <t@example.invalid> 0"},
		{"a negative zone", "T <t@example.invalid> 1767323045 -0300", "T <t@example.invalid>"},
		{"text before the date: all of it", "T <t@example.invalid> zorvex 0 +0000", "T <t@example.invalid> zorvex 0 +0000"},
		{"text after the zone: all of it", "T <t@example.invalid> 0 +0000 zorvex", "T <t@example.invalid> 0 +0000 zorvex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(identity([]byte(tt.value))); got != tt.want {
				t.Errorf("identity(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// readings returns what Walk reads of one commit, as "field|path|line|text".
func readings(t *testing.T, r *gittest.Repo, sha string) []string {
	t.Helper()
	var out []string
	err := Readings(t.Context(), r.Repo, sha, func(rd Reading) {
		if rd.Commit != sha {
			t.Errorf("reading of commit %s, want %s", rd.Commit, sha)
		}
		out = append(out, fmt.Sprintf("%s|%s|%d|%s", rd.Field, rd.Path, rd.Line, rd.Text))
	})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	return out
}

// TestReadings covers the four readings of a commit: the message, the
// two identities, the paths it adds or renames to, and the lines it
// adds.
func TestReadings(t *testing.T) {
	r := gittest.New(t)
	r.Write("kept.txt", "one\ntwo\nthree\n")
	r.Write("moved.txt", "a line long enough to be seen as the same file after a rename\nand a second line\n")
	root := r.Commit("root subject\n\nroot body")

	r.Write("kept.txt", "one\ntwo changed\nthree\nfour\n")
	r.Write("dir/new file.txt", "++ looks like a header\n-- so does this\nno newline at the end")
	r.Git("mv", "moved.txt", "renamed.txt")
	second := r.WithEnv("GIT_AUTHOR_NAME=Other Author", "GIT_AUTHOR_EMAIL=other@example.invalid").Commit("second")

	r.Git("rm", "--quiet", "dir/new file.txt")
	third := r.Commit("third")

	tests := []struct {
		name string
		sha  string
		want []string
	}{
		{
			name: "a root commit is compared with the empty tree",
			sha:  root,
			want: []string{
				"message||1|root subject",
				"message||2|",
				"message||3|root body",
				"author||0|Test Author <author@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
				"path|kept.txt|0|kept.txt",
				"path|moved.txt|0|moved.txt",
				"line|kept.txt|1|one",
				"line|kept.txt|2|two",
				"line|kept.txt|3|three",
				"line|moved.txt|1|a line long enough to be seen as the same file after a rename",
				"line|moved.txt|2|and a second line",
			},
		},
		{
			name: "added and renamed paths, added lines with their numbers",
			sha:  second,
			want: []string{
				"message||1|second",
				"author||0|Other Author <other@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
				"path|dir/new file.txt|0|dir/new file.txt",
				"path|renamed.txt|0|renamed.txt",
				"line|dir/new file.txt|1|++ looks like a header",
				"line|dir/new file.txt|2|-- so does this",
				"line|dir/new file.txt|3|no newline at the end",
				"line|kept.txt|2|two changed",
				"line|kept.txt|4|four",
			},
		},
		{
			name: "a commit that only removes adds no path and no line",
			sha:  third,
			want: []string{
				"message||1|third",
				"author||0|Test Author <author@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readings(t, r, tt.sha)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// TestReadingsMerge shows that a merge commit is compared with its
// first parent: what the second parent brings counts as added by the
// merge, and what the first parent already had does not.
func TestReadingsMerge(t *testing.T) {
	f := newFixture(t)
	got := readings(t, f.repo, f.m)
	want := []string{
		"message||1|m",
		"author||0|Test Author <author@example.invalid>",
		"committer||0|Test Committer <committer@example.invalid>",
		"path|d.txt|0|d.txt",
		"line|d.txt|1|d one",
		"line|d.txt|2|d two",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestReadingsMergetag covers the tags a merge commit embeds: merging a
// signed tag writes the whole tag object, signature included, into a
// mergetag header of the merge, folded one space deep. Each one is read
// like a pushed tag, and numbered in the order of the headers.
func TestReadingsMergetag(t *testing.T) {
	f := newFixture(t)
	tree := f.repo.Git("rev-parse", f.m+"^{tree}")
	raw := "tree " + tree + "\n" +
		"parent " + f.c + "\n" +
		"parent " + f.d + "\n" +
		"author Test Author <author@example.invalid> 1767323045 +0000\n" +
		"committer Test Committer <committer@example.invalid> 1767323045 +0000\n" +
		"mergetag object " + f.d + "\n" +
		" type commit\n" +
		" tag v-side\n" +
		" tagger Tag Maker <maker@example.invalid> 1767323045 +0000\n" +
		" \n" +
		" side notes\n" +
		" -----BEGIN SSH SIGNATURE-----\n" +
		" U1NIU0lHAAAA\n" +
		" -----END SSH SIGNATURE-----\n" +
		"mergetag object " + f.b + "\n" +
		" type commit\n" +
		" tag v-b\n" +
		" tagger Other Maker <other@example.invalid> 1767323045 +0000\n" +
		" \n" +
		" b notes\n" +
		"\n" +
		"merge the side\n"
	merge := f.repo.WriteObject("commit", raw)
	var got []string
	err := Readings(t.Context(), f.repo.Repo, merge, func(rd Reading) {
		if rd.Commit != merge || rd.Tag != "" {
			t.Errorf("reading names commit %q and tag %q, want commit %s", rd.Commit, rd.Tag, merge)
		}
		if rd.Mergetag > 0 {
			got = append(got, fmt.Sprintf("%d|%s|%d|%s", rd.Mergetag, rd.Field, rd.Line, rd.Text))
		}
	})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	want := []string{
		"1|tag name|0|v-side",
		"1|tagger|0|Tag Maker <maker@example.invalid>",
		"1|message|1|side notes",
		"1|message|2|-----BEGIN SSH SIGNATURE-----",
		"1|message|3|U1NIU0lHAAAA",
		"1|message|4|-----END SSH SIGNATURE-----",
		"2|tag name|0|v-b",
		"2|tagger|0|Other Maker <other@example.invalid>",
		"2|message|1|b notes",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergetag readings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestReadingsQuotedPath covers a path git quotes in a patch: one with a
// byte outside ASCII, a double quote or a backslash. A line reading
// carries the path itself, as a path reading does.
func TestReadingsQuotedPath(t *testing.T) {
	r := gittest.New(t)
	paths := []string{"caf\u00e9/f.txt", `say "hi".txt`, `back\slash.txt`, "tab\there.txt"}
	for _, p := range paths {
		r.Write(p, "x\n")
	}
	sha := r.Commit("quoted paths")
	var lines, added []string
	err := Readings(t.Context(), r.Repo, sha, func(rd Reading) {
		switch rd.Field {
		case FieldLine:
			lines = append(lines, rd.Path)
		case FieldPath:
			added = append(added, rd.Path)
		}
	})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	slices.Sort(lines)
	slices.Sort(added)
	want := slices.Sorted(slices.Values(paths))
	if !reflect.DeepEqual(lines, want) || !reflect.DeepEqual(added, want) {
		t.Errorf("line paths %q, added paths %q, want %q", lines, added, want)
	}

	// With core.quotePath off, git leaves bytes outside ASCII unescaped
	// inside the quotes, and a byte that is no UTF-8 would not survive
	// the unquoting. The path goes in through the index, so the file
	// system need not accept it.
	off := r.WithEnv("GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.quotePath", "GIT_CONFIG_VALUE_0=false")
	odd := "odd\xff\"name.txt"
	blob := off.WriteObject("blob", "x\n")
	off.Git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+odd)
	off.Git("commit", "--quiet", "--message", "an odd path")
	var oddLines []string
	err = Readings(t.Context(), off.Repo, off.Git("rev-parse", "HEAD"), func(rd Reading) {
		if rd.Field == FieldLine {
			oddLines = append(oddLines, rd.Path)
		}
	})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	if !reflect.DeepEqual(oddLines, []string{odd}) {
		t.Errorf("line paths %q, want %q", oddLines, []string{odd})
	}
}

func TestReadingsUnknownCommit(t *testing.T) {
	f := newFixture(t)
	err := Readings(t.Context(), f.repo.Repo, strings.Repeat("12", 20), func(Reading) {})
	if err == nil {
		t.Error("Readings of a commit the repository lacks succeeded")
	}
}

func TestIsObjectID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"a sha1 id", strings.Repeat("a1", 20), true},
		{"a sha256 id", strings.Repeat("a1", 32), true},
		{"all zeros", zero, true},
		{"39 digits", strings.Repeat("a", 39), false},
		{"41 digits", strings.Repeat("a", 41), false},
		{"63 digits", strings.Repeat("a", 63), false},
		{"upper case", strings.Repeat("A1", 20), false},
		{"not hex", strings.Repeat("g1", 20), false},
		{"an option", "--all", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isObjectID(tt.id); got != tt.want {
				t.Errorf("isObjectID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// TestPatchErrorsHoldNoContent shows that a patch the parser cannot
// read is reported by its hunk number: neither the path nor a line of
// the file reaches the output.
func TestPatchErrorsHoldNoContent(t *testing.T) {
	tests := []struct {
		name  string
		patch string
	}{
		{"a hunk cut short", "+++ b/zorvex-quimby.txt\n@@ -0,0 +1,2 @@\n+one\n"},
		{"a line that is no hunk line", "+++ b/zorvex-quimby.txt\n@@ -0,0 +1,2 @@\n+one\nzorvex quimby\n"},
		{"a header without ranges", "+++ b/zorvex-quimby.txt\n@@ zorvex quimby @@\n"},
		{"a range that is no number", "+++ b/zorvex-quimby.txt\n@@ -1 +zorvex,quimby @@\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addedLines([]byte(tt.patch), func(string, int, []byte) {})
			if err == nil {
				t.Fatal("addedLines succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "hunk 1") {
				t.Errorf("the error does not name the hunk: %v", err)
			}
			if strings.Contains(err.Error(), "zorvex") || strings.Contains(err.Error(), "quimby") {
				t.Errorf("the error repeats the patch: %v", err)
			}
		})
	}
}
