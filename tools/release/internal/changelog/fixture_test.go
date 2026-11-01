// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pinned"
)

// moduleRoot is the root of the repository, four directories above
// this package's: the cliff.toml and CHANGELOG.md header the fixtures
// copy are its own, so the golden reads the template that ships.
func moduleRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	return abs
}

// cliffPath resolves the git-cliff that mise.toml pins, as the command
// does.
func cliffPath(t *testing.T) string {
	t.Helper()
	path, err := pinned.Which(t.Context(), moduleRoot(t), "git-cliff")
	require.NoError(t, err)
	return path
}

// repo is a fixture repository of merge commits, with a bare origin.
type repo struct {
	t    *testing.T
	dir  string
	bare string
	// clock is the date of the next commit, one minute after the last,
	// so that the order of the commits is the order of the script.
	clock time.Time
}

// newRepo makes a repository on main under a new directory, with the
// files the command reads at their first state, committed, and a bare
// origin that holds main.
func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir(), bare: t.TempDir(), clock: time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)}
	r.git("init", "-q", "-b", "main")
	root := moduleRoot(t)
	for _, name := range []string{Config, File} {
		data, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		r.write(map[string]string{name: string(data)})
	}
	r.write(map[string]string{
		"VERSION":                              "1.0.0\n",
		errorsPage:                             errorsPageWith("| `RJ-101` | `locked` | 1 | `wait` | [internal/state](../../internal/state/errors.go) | none |", "| `RJ-102` | `drift` | 2 | `decide` | [internal/cli](../../internal/cli/errors.go) | none |"),
		versionsFile:                           versionsWith(1, 1),
		kitsEmbed:                              "package kits\n\nimport \"embed\"\n\n//go:embed julieta os-base git-ssh-sign/git-ssh-sign.yaml\nvar FS embed.FS\n",
		"kits/julieta/julieta.yaml":            "name: julieta\n",
		"kits/os-base/os-base.yaml":            "name: os-base\n",
		"kits/git-ssh-sign/git-ssh-sign.yaml":  "name: git-ssh-sign\n",
		"kits/git-ssh-sign/testdata/case.yaml": "a fixture, not embedded\n",
	})
	r.commit("chore: the fixture files")
	r.git("init", "-q", "--bare", r.bare)
	r.git("remote", "add", "origin", r.bare)
	r.git("push", "-q", "origin", "main")
	return r
}

// errorsPageWith is the generated error table with the given rows.
func errorsPageWith(rows ...string) string {
	return "# Errors\n\n| Id | Slug | Exit | Fix | Declared in | Recovered in |\n|---|---|---|---|---|---|\n" + strings.Join(rows, "\n") + "\n"
}

// versionsWith is internal/spec/versions.go with the two versions.
func versionsWith(project, hostSettings int) string {
	return "package spec\n\nconst (\n\tProjectVersion = " + strconv.Itoa(project) + "\n\tHostSettingsVersion = " + strconv.Itoa(hostSettings) + "\n)\n"
}

// env is the environment of the fixture's git: no configuration of the
// user or the system, a fixed identity, and the clock's date.
func (r *repo) env() []string {
	date := r.clock.Format(time.RFC3339)
	return append(toolEnv(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
}

// git runs git in the repository and fails the test on an error.
func (r *repo) git(args ...string) string {
	r.t.Helper()
	argv := append([]string{"git", "-c", "core.hooksPath=/dev/null", "-c", "tag.gpgSign=false", "-c", "commit.gpgSign=false"}, args...)
	out, err := procgroup.CombinedOutput(context.WithoutCancel(r.t.Context()), procgroup.Command{
		Class: procgroup.ToolGit, Argv: argv, Dir: r.dir, Env: r.env(),
	})
	require.NoError(r.t, err, "git %s: %s", strings.Join(args, " "), out)
	return string(out)
}

// write writes the files, name to content, under the repository.
func (r *repo) write(files map[string]string) {
	r.t.Helper()
	for name, content := range files {
		path := filepath.Join(r.dir, filepath.FromSlash(name))
		require.NoError(r.t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(r.t, os.WriteFile(path, []byte(content), 0o644))
	}
}

// commit stages everything and commits it with message, a minute after
// the last commit.
func (r *repo) commit(message string) {
	r.t.Helper()
	r.clock = r.clock.Add(time.Minute)
	r.git("add", "-A")
	r.git("commit", "-q", "--no-verify", "--allow-empty", "-m", message)
}

// pr merges a pull request into main as GitHub does: a branch with one
// commit that writes files, merged with --no-ff under the subject, "
// (#n)" appended, and the body.
func (r *repo) pr(n int, subject, body string, files map[string]string) {
	r.t.Helper()
	branch := "pr-" + strconv.Itoa(n)
	r.git("checkout", "-q", "-b", branch)
	r.write(files)
	r.commit("the change of " + branch)
	r.git("checkout", "-q", "main")
	r.merge(subject+" (#"+strconv.Itoa(n)+")", body, branch)
}

// merge merges branch into the current branch with --no-ff under the
// subject and body.
func (r *repo) merge(subject, body, branch string) {
	r.t.Helper()
	r.clock = r.clock.Add(time.Minute)
	message := subject
	if body != "" {
		message += "\n\n" + body
	}
	r.git("merge", "-q", "--no-ff", "--no-edit", "-m", message, branch)
}

// tag makes a lightweight tag at HEAD.
func (r *repo) tag(name string) {
	r.t.Helper()
	r.git("tag", name)
}

// read returns a file of the repository.
func (r *repo) read(name string) string {
	r.t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, name))
	require.NoError(r.t, err)
	return string(data)
}

// scripted is the fixture history the golden reads: two pull requests
// before the tag v1.0.0-alpha.1, then the changes the Upgrading part
// lists, a breaking subject, the contracts approval line, the release
// pull request, an in-branch merge of each kind and an entry with no
// Why section, and one with a "Why not" heading, two Why sections and an
// HTML comment.
func scripted(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	r.pr(1, "feat(spec): the project spec reader", "## Why\n\nA project is read from its spec.\n\n## What changed\n\n- the reader", map[string]string{"a.txt": "a\n"})
	r.pr(2, "fix: the lock is released on a signal", "## Why\n\nA signal left the lock behind. Closes #7.\n", map[string]string{"b.txt": "b\n"})
	r.tag("v1.0.0-alpha.1")
	r.pr(3, "feat(sync)!: gate 2 refuses an unlisted host", "## Why\n\nAn unlisted host passed.\n", map[string]string{
		errorsPage: errorsPageWith(
			"| `RJ-101` | `locked` | 1 | `wait` | [internal/state](../../internal/state/errors.go) | none |",
			"| `RJ-102` | `drift` | 3 | `decide` | [internal/cli](../../internal/cli/errors.go) | none |",
			"| `RJ-103` | `unlisted-host` | 1 | `decide` | [internal/gate](../../internal/gate/errors.go) | none |"),
	})
	r.pr(4, "docs(spec): the host settings gain a field", "## Why\n\nThe field was missing.\n\n## Lessons\n\nnone\n\n> [!IMPORTANT]\n> - [x] Approval: contracts - host-settings/v2\n", map[string]string{
		versionsFile:                           versionsWith(2, 1),
		"kits/julieta/julieta.yaml":            "name: julieta\nversion: 2\n",
		"kits/git-ssh-sign/testdata/case.yaml": "changed, still not embedded\n",
	})
	r.pr(5, "chore(release): v1.0.0-alpha.1", "## Why\n\nThe release pull request, skipped.\n", map[string]string{"c.txt": "c\n"})
	// A branch that merged main twice before its own pull request: once
	// under the merge type, once under git's default subject.
	r.git("checkout", "-q", "-b", "pr-6")
	r.write(map[string]string{"d.txt": "d\n"})
	r.commit("the change of pr-6")
	r.git("checkout", "-q", "main")
	r.write(map[string]string{"e.txt": "e\n"})
	r.commit("chore: a commit on main outside a pull request")
	r.git("checkout", "-q", "pr-6")
	r.merge("merge: main into pr-6 for the e file", "", "main")
	r.git("checkout", "-q", "main")
	r.write(map[string]string{"f.txt": "f\n"})
	r.commit("chore: another commit on main")
	r.git("checkout", "-q", "pr-6")
	r.merge("Merge remote-tracking branch 'origin/main' into pr-6", "", "main")
	r.git("checkout", "-q", "main")
	r.merge("feat(doctor): the gh-floor row (#6)", "## Why\n\nThe floor is checked. See #12 and #6.\n", "pr-6")
	r.pr(7, "refactor: one place for the host paths", "Something that is not a Why section.\n", map[string]string{"g.txt": "g\n"})
	// Two merges under GitHub's default subject, as the first pull
	// requests of the repository were merged: the title in the body is
	// conventional in one, which is an entry, and plain in the other.
	r.git("checkout", "-q", "-b", "pr-11")
	r.write(map[string]string{"j.txt": "j\n"})
	r.commit("the change of pr-11")
	r.git("checkout", "-q", "main")
	r.merge("Merge pull request #11 from owner/feat/t011-hooks", "feat(hooks): the hook dispatcher of the sandbox", "pr-11")
	r.git("checkout", "-q", "-b", "pr-12")
	r.write(map[string]string{"k.txt": "k\n"})
	r.commit("the change of pr-12")
	r.git("checkout", "-q", "main")
	r.merge("Merge pull request #12 from owner/docs/prologue", "Add the prologue", "pr-12")
	r.pr(10, "docs(adr): the alternative not taken", "<!-- the template's comment -->\n\n## Why not\n\nThe alternative.\n\n## Why\n\nThe reason. <!-- an aside -->\n\n## Why\n\nA second Why, not read.\n", map[string]string{"i.txt": "i\n"})
	return r
}
