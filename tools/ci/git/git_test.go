// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

func TestRun(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	sha := r.Commit("a")

	tests := []struct {
		name    string
		stdin   string
		args    []string
		want    string
		wantErr string
	}{
		{"standard output is returned", "", []string{"rev-parse", "HEAD"}, sha + "\n", ""},
		{"standard input is passed", sha + "\n", []string{"cat-file", "--batch-check"}, sha + " commit ", ""},
		{"a failing command carries what git said", "", []string{"rev-parse", "--verify", "nonesuch"}, "", "git rev-parse:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := r.Run(t.Context(), []byte(tt.stdin), tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "fatal") {
					t.Fatalf("error = %v, want one that holds %q and git's message", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(out), tt.want) {
				t.Errorf("output = %q, want prefix %q", out, tt.want)
			}
		})
	}
}

// TestRunIgnoresReplaceRefs shows that a replace ref does not change
// what Run reads: a push sends the objects themselves, so the checks
// read those and not a local stand-in.
func TestRunIgnoresReplaceRefs(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	base := r.Commit("base")
	r.Write("a.txt", "the real line\n")
	real := r.Commit("real")
	r.Git("checkout", "--quiet", "--detach", base)
	r.Write("a.txt", "a stand-in line\n")
	standIn := r.Commit("stand-in")
	r.Git("replace", real, standIn)
	out, err := r.Run(t.Context(), nil, "cat-file", "-p", real+":a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "the real line\n" {
		t.Errorf("read %q through a replace ref, want the real object", out)
	}
}

// TestRunPrintableError shows that what git says on failure is kept,
// with no control character of a path it repeats. Git 2.53 already
// replaces those in its own fatal messages; Run quotes what still
// holds one, for a git or a message that does not.
func TestRunPrintableError(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	r.Commit("a")
	_, err := r.Run(t.Context(), nil, "cat-file", "-p", "HEAD:x\x1b[2Jy")
	if err == nil {
		t.Fatal("Run succeeded, want an error")
	}
	if strings.Contains(err.Error(), "\x1b") || !strings.Contains(err.Error(), "fatal") {
		t.Errorf("error = %q, want git's message without the escape", err)
	}
}

func TestPrintable(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"plain ASCII", "docs/a b.md", "docs/a b.md"},
		{"plain outside ASCII", "docs/caf\u00e9.md", "docs/caf\u00e9.md"},
		{"an escape", "a\x1bb", `"a\x1bb"`},
		{"a newline", "a\nb", `"a\nb"`},
		{"a tab", "a\tb", `"a\tb"`},
		{"DEL", "a\x7fb", `"a\x7fb"`},
		{"a zero-width space", "a\u200bb", `"a\u200bb"`},
		{"not UTF-8", "a\xffb", `"a\xffb"`},
		{"quoted keeps only ASCII", "caf\u00e9\n", `"caf\u00e9\n"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := git.Printable(tt.in); got != tt.want {
				t.Errorf("Printable(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRunStopsWithItsContext(t *testing.T) {
	r := gittest.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.Run(ctx, nil, "rev-parse", "--git-dir"); err == nil {
		t.Error("Run succeeded under a cancelled context")
	}
}
