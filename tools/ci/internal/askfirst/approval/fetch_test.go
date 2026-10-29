// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// jobClone is the ask-first job's checkout in a fixture: a clone of a remote
// that holds main at base, and the head of a pull request only under
// refs/pull/1/head, which a clone does not fetch, as GitHub serves the
// head of a pull request from a fork. The remote serves any commit it
// holds by id, as GitHub does for a public repository.
func jobClone(t *testing.T) (job *gittest.Repo, base, head string) {
	t.Helper()
	work, base := prRepo(t)
	remote := gittest.NewBare(t)
	remote.Git("config", "uploadpack.allowAnySHA1InWant", "true")
	work.Git("push", "--quiet", remote.Dir, "main")
	work.Git("checkout", "--quiet", "-b", "topic")
	work.Write("docs/adr/0002-new.md", "new\n")
	head = work.Commit("a record")
	work.Git("push", "--quiet", remote.Dir, "topic:refs/pull/1/head")
	job = gittest.New(t)
	job.Git("remote", "add", fetchRemote, remote.Dir)
	job.Git("fetch", "--quiet", fetchRemote, "main")
	job.Git("checkout", "--quiet", "-B", "main", "FETCH_HEAD")
	require.NoError(t, os.Remove(filepath.Join(job.Dir, ".git", "FETCH_HEAD")))
	return job, base, head
}

// TestPRFetchesTheHead holds REQ-1152 and REQ-0816: the ask-first job fetches
// head.sha when the clone lacks it, and only then; a run without the
// fetch, which is the local tools/ci ask-first, fails on a missing head and
// reads no network.
func TestPRFetchesTheHead(t *testing.T) {
	body := alert("> - [x] Approval: decisions - add a record")
	t.Run("a missing head is fetched", func(t *testing.T) {
		job, base, head := jobClone(t)
		_, err := job.Run(t.Context(), nil, "cat-file", "-e", head+"^{commit}")
		require.Error(t, err, "the clone lacks the head")
		findings, err := Judge(t.Context(), job.Repo, PR{Body: body, BaseSHA: base, HeadSHA: head}, true)
		require.NoError(t, err)
		assert.Empty(t, findings)
		_, err = os.Stat(filepath.Join(job.Dir, ".git", "FETCH_HEAD"))
		require.ErrorIs(t, err, os.ErrNotExist, "the fetch writes no FETCH_HEAD")
		refs := job.Git("for-each-ref", "--format=%(refname)")
		assert.NotContains(t, refs, "pull", "the fetch writes no ref")
	})
	t.Run("a head the clone holds is not fetched", func(t *testing.T) {
		job, base, head := jobClone(t)
		job.Git("fetch", "--quiet", fetchRemote, "refs/pull/1/head")
		// The remote is gone, so a network read fails the run.
		job.Git("remote", "set-url", fetchRemote, filepath.Join(t.TempDir(), "gone"))
		findings, err := Judge(t.Context(), job.Repo, PR{Body: body, BaseSHA: base, HeadSHA: head}, true)
		require.NoError(t, err)
		assert.Empty(t, findings)
	})
	t.Run("without the fetch a missing head is an error", func(t *testing.T) {
		job, base, head := jobClone(t)
		_, err := Judge(t.Context(), job.Repo, PR{Body: body, BaseSHA: base, HeadSHA: head}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "names no commit of this clone")
	})
	t.Run("a head the remote lacks is an error", func(t *testing.T) {
		job, base, _ := jobClone(t)
		_, err := Judge(t.Context(), job.Repo, PR{Body: body, BaseSHA: base, HeadSHA: strings.Repeat("e", 40)}, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fetching head.sha")
	})
}

// gitCalls records the argument list of every git command pr runs, by
// way of runGit, for the duration of the test.
func gitCalls(t *testing.T) func() [][]string {
	t.Helper()
	var mu sync.Mutex
	var calls [][]string
	old := runGit
	runGit = func(ctx context.Context, r git.Repo, args []string) ([]byte, error) {
		mu.Lock()
		calls = append(calls, slices.Clone(args))
		mu.Unlock()
		return old(ctx, r, args)
	}
	t.Cleanup(func() { runGit = old })
	return func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(calls)
	}
}

// requiredFlags are the flags 12 12.4 (REQ-1152) asks of each git
// command pr runs on head objects, beside -c core.hooksPath=/dev/null,
// which every one of them passes. The diff that lists the changed paths
// passes no --ignore-submodules (diffPaths says why), and pr runs no log
// and no show; were it to, they would pass the flags of the diff and
// --ignore-submodules=all.
var requiredFlags = map[string][]string{
	"fetch": {"--no-recurse-submodules", "--no-tags", "--no-write-fetch-head", "--no-auto-maintenance",
		"-c credential.helper=", "-c core.askPass=", "-c http.extraHeader=", "-c http.https://github.com/.extraheader=", "-c http.followRedirects=false"},
	"diff":      {"--no-ext-diff", "--no-textconv"},
	"log":       {"--no-ext-diff", "--no-textconv", "--ignore-submodules=all"},
	"show":      {"--no-ext-diff", "--no-textconv", "--ignore-submodules=all"},
	"rev-parse": nil, "rev-list": nil, "merge-base": nil, "ls-tree": nil, "cat-file": nil,
}

// subcommand returns the git subcommand of args, past the -c settings,
// and the settings, each as "-c <setting>".
func subcommand(args []string) (string, []string) {
	var settings []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" && i+1 < len(args) {
			settings = append(settings, "-c "+args[i+1])
			i++
			continue
		}
		return args[i], settings
	}
	return "", settings
}

// TestPRHostileRepository runs pr against a hostile fixture repository
// (REQ-1153): the clone's configuration names a textconv driver and an
// external diff driver for every path, runs hooks from a directory of
// its own and recurses into submodules, and the head adds a submodule and
// a .gitattributes that sends every path to the driver. pr fetches the
// head and judges it, and then no driver and no hook has run, every git
// command it ran passes -c core.hooksPath=/dev/null and the flags of
// requiredFlags, and none is outside that table, so removing a flag from
// a call, or a new kind of call, turns the test red. A control run of
// git without the flags shows the drivers and the hooks do run.
func TestPRHostileRepository(t *testing.T) {
	job, base, _ := jobClone(t)
	marks := t.TempDir()
	hooks := t.TempDir()
	script := func(name string) string {
		p := filepath.Join(t.TempDir(), name)
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\ntouch '"+filepath.Join(marks, name)+"'\ncat >/dev/null 2>&1 || true\n"), 0o755))
		return p
	}
	for _, h := range []string{"reference-transaction", "post-checkout", "post-merge", "pre-auto-gc", "post-index-change", "pre-commit"} {
		require.NoError(t, os.WriteFile(filepath.Join(hooks, h), []byte("#!/bin/sh\ntouch '"+filepath.Join(marks, "hook-"+h)+"'\n"), 0o755))
	}
	for key, value := range map[string]string{
		"diff.evil.textconv":      script("textconv"),
		"diff.external":           script("external"),
		"core.hooksPath":          hooks,
		"submodule.recurse":       "true",
		"fetch.recurseSubmodules": "true",
	} {
		job.Git("config", key, value)
	}
	job.Write(".gitattributes", "* diff=evil\n")

	// The head: a .gitattributes for the driver, a changed file, and a
	// submodule whose commit no remote holds.
	work := gittest.New(t)
	work.Git("fetch", "--quiet", job.Dir, "main")
	work.Git("checkout", "--quiet", "-b", "topic", "FETCH_HEAD")
	work.Write(".gitattributes", "* diff=evil\n")
	work.Write(".gitmodules", "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n")
	work.Write("docs/adr/0001-old.md", "changed\n")
	work.Git("add", "--all")
	work.Git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("1", 40)+",sub")
	work.Git("commit", "--quiet", "--message", "hostile")
	head := work.Git("rev-parse", "HEAD")
	remote := job.Git("remote", "get-url", fetchRemote)
	work.Git("push", "--quiet", remote, "HEAD:refs/pull/2/head")

	calls := gitCalls(t)
	_, err := Judge(t.Context(), job.Repo, PR{Body: "", BaseSHA: base, HeadSHA: head}, true)
	require.NoError(t, err)
	entries, err := os.ReadDir(marks)
	require.NoError(t, err)
	assert.Empty(t, entries, "a driver or a hook ran")

	got := calls()
	require.NotEmpty(t, got)
	kinds := map[string]bool{}
	for _, args := range got {
		sub, settings := subcommand(args)
		kinds[sub] = true
		assert.Equal(t, []string{"-c", "core.hooksPath=/dev/null"}, args[:2], "%q", args)
		want, known := requiredFlags[sub]
		if !assert.True(t, known, "a git command pr runs and no flag rule covers: %q", args) {
			continue
		}
		have := append(slices.Clone(args), settings...)
		for _, f := range want {
			assert.Contains(t, have, f, "%q lacks %s", args, f)
		}
		assert.NotEqual(t, "checkout", sub)
		assert.NotEqual(t, "archive", sub)
	}
	for _, sub := range []string{"fetch", "diff", "rev-parse", "rev-list", "merge-base", "ls-tree", "cat-file"} {
		assert.True(t, kinds[sub], "the fixture runs %s, so its flags are checked", sub)
	}

	// The control: without the flags, the same repository runs the
	// drivers and the hooks.
	job.Git("diff", base, head, "--", "docs/adr/0001-old.md")
	job.Git("diff", "--no-ext-diff", base, head, "--", "docs/adr/0001-old.md")
	job.Git("commit", "--quiet", "--allow-empty", "--message", "control")
	for _, m := range []string{"external", "textconv", "hook-pre-commit"} {
		_, err := os.Stat(filepath.Join(marks, m))
		assert.NoError(t, err, "the control runs %s", m)
	}
}

// TestPRQuotesTheAuthorsText is the fixture "pr escapes a workflow
// command in a PR body" of REQ-1153: a line of the author's text, with a
// leading space, a leading U+00A0, a "##[" or a line break, prints behind
// the fixed prefix "> " on one line, so the Actions runner, which trims
// the blanks at the start of a line and looks for "::", and for "##["
// anywhere, reads no command. Removing the prefix or the "##[" escape
// turns it red.
func TestPRQuotesTheAuthorsText(t *testing.T) {
	texts := []string{
		" ::error::a leading space",
		" ::warning::a leading no-break space",
		"##[error]the older form",
		"> - [ ] Approval: decisions - x ##[error]inside",
		"a line\n::error::after a line feed",
		"a line\r::error::after a carriage return",
		" ::error::after a line separator",
	}
	for _, text := range texts {
		t.Run(text, func(t *testing.T) {
			out := Finding{Where: bodyWhere, Msg: "malformed approval line, quoted below", Quote: text}.String()
			lines := strings.Split(out, "\n")
			require.Len(t, lines, 2, "%q", out)
			assert.True(t, strings.HasPrefix(lines[1], quotePrefix), "the fixed prefix: %q", lines[1])
			for _, l := range lines {
				assert.False(t, strings.HasPrefix(strings.TrimLeftFunc(l, unicode.IsSpace), "::"), "a command: %q", l)
				assert.NotContains(t, l, "##[", "the older form: %q", l)
			}
		})
	}
	t.Run("through a body", func(t *testing.T) {
		r := review{body: alert("> - [x] Approval: decisions - ok ##[error]x«"), base: mustList(t, prList), changed: []string{"docs/adr/x.md"}}
		var b strings.Builder
		Print(&b, r.findings())
		assert.Contains(t, b.String(), "\n> > - [x] Approval: decisions - ok ##\\[error]x\\u00ab\n")
	})
}
