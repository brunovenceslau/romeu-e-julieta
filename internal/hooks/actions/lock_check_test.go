// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// lockText is a mise.lock of the layout of the pinned mise with gh
// locked on both linux platforms.
const lockText = `lockfile_version = 3

[[tools.gh]]
version = "2.102.0"
backend = "aqua:cli/cli"
specifiers = ["2.102.0"]

[tools.gh."platforms.linux-arm64"]
checksum = "sha256:7862c86c72f43df3a2d93ddde6f473285b4e2af61b494849846827e513ef6484"
url = "https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_arm64.tar.gz"

[tools.gh."platforms.linux-x64"]
checksum = "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386"
url = "https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_amd64.tar.gz"
`

// lockRepo makes a repository with one commit and returns it.
func lockRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	commit(t, dir, "one")
	return dir
}

// runAction runs the action of pre-commit in dir and returns its
// error and what it wrote to stderr.
func runAction(t *testing.T, dir string, env ...string) (string, error) {
	t.Helper()
	var stderr bytes.Buffer
	err := lockCheckAction.Run(t.Context(), Input{Hook: "pre-commit", Dir: dir, Env: append([]string{"HOME=" + t.TempDir()}, env...), Stdout: &bytes.Buffer{}, Stderr: &stderr})
	return stderr.String(), err
}

// stage writes the files into dir and stages them.
func stage(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
		git(t, dir, "add", name)
	}
}

// TestLockCheckIsTheActionOfPreCommit (04 4.3, REQ-0408): pre-commit has
// the action lock-check, which stops the chain on a failure, and no
// other hook has it.
func TestLockCheckIsTheActionOfPreCommit(t *testing.T) {
	got := For("pre-commit")
	require.Len(t, got, 1)
	assert.Equal(t, "lock-check", got[0].Name)
	assert.False(t, got[0].Continues)
	for _, hook := range []string{"post-commit", "pre-push", "commit-msg"} {
		for _, a := range For(hook) {
			assert.NotEqual(t, "lock-check", a.Name, hook)
		}
	}
}

// TestLockCheckRunsWhenAMiseFileIsStaged: a staged mise.toml whose
// tools the lock does not hold fails with check-findings (RJ-328, exit
// 6), and the findings are printed.
func TestLockCheckRunsWhenAMiseFileIsStaged(t *testing.T) {
	for _, staged := range []map[string]string{
		{"mise.toml": "[tools]\ngh = \"2.102.0\"\njq = \"1.7\"\n"},
		{"mise.toml": "[tools]\ngh = \"2.102.0\"\njq = \"1.7\"\n", "mise.lock": lockText},
	} {
		dir := lockRepo(t)
		stage(t, dir, staged)
		stderr, err := runAction(t, dir)
		require.Error(t, err)
		assert.Equal(t, "RJ-328", cli.Of(err).ID)
		assert.Equal(t, 6, cli.ExitCode(err))
		assert.Contains(t, stderr, "finding lock-tool-missing jq\n")
	}
}

// TestLockCheckPasses: a fresh lock passes, and so does a commit that
// stages no mise file, though the lock on disk is stale: the check
// reads the repo when a mise file is staged (REQ-0619).
func TestLockCheckPasses(t *testing.T) {
	dir := lockRepo(t)
	stage(t, dir, map[string]string{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "mise.lock": lockText})
	stderr, err := runAction(t, dir)
	require.NoError(t, err)
	assert.Empty(t, stderr)

	git(t, dir, "commit", "-q", "-m", "mise")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\njq = \"1.7\"\n"), 0o600))
	stage(t, dir, map[string]string{"other": "x"})
	stderr, err = runAction(t, dir)
	require.NoError(t, err, "mise.toml is changed but not staged")
	assert.Empty(t, stderr)
}

// TestLockCheckNoChecksumIsAFinding: an entry without a checksum fails
// the hook, and the exceptions file, staged with it, excuses its tool
// (REQ-0407 as decided on 2026-10-22).
func TestLockCheckNoChecksumIsAFinding(t *testing.T) {
	noSum := strings.ReplaceAll(lockText, "checksum = \"sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386\"\n", "")
	dir := lockRepo(t)
	stage(t, dir, map[string]string{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "mise.lock": noSum})
	stderr, err := runAction(t, dir)
	require.Error(t, err)
	assert.Equal(t, 6, cli.ExitCode(err))
	assert.Contains(t, stderr, "finding lock-no-checksum gh linux-x64\n")

	stage(t, dir, map[string]string{mise.ExceptionsFile: "gh the vendor publishes no checksum\n"})
	stderr, err = runAction(t, dir)
	require.NoError(t, err)
	assert.Empty(t, stderr)
}

// TestLockCheckNoMiseFile: a dir with no repository passes and starts no
// git.
func TestLockCheckNoMiseFile(t *testing.T) {
	stderr, err := runAction(t, t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, stderr)
}

// TestLockCheckDeletedMise: removing mise.toml leaves nothing to check.
func TestLockCheckDeletedMise(t *testing.T) {
	dir := lockRepo(t)
	stage(t, dir, map[string]string{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "mise.lock": lockText})
	git(t, dir, "commit", "-q", "-m", "mise")
	git(t, dir, "rm", "-q", "mise.toml", "mise.lock")
	_, err := runAction(t, dir)
	require.NoError(t, err)
}

// TestLockCheckBrokenLock: a lock that does not parse is a finding, exit
// 6, not an error of the hook (REQ-0406); so is a mise.toml.
func TestLockCheckBrokenLock(t *testing.T) {
	for _, files := range []map[string]string{
		{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "mise.lock": "lockfile_version = 3\nlockfile_version = 3\n"},
		{"mise.toml": "[tools\n", "mise.lock": lockText},
	} {
		dir := lockRepo(t)
		stage(t, dir, files)
		stderr, err := runAction(t, dir)
		require.Error(t, err)
		assert.Equal(t, "RJ-328", cli.Of(err).ID)
		assert.Equal(t, 6, cli.ExitCode(err))
		assert.Contains(t, stderr, "finding lock-unparsed ")
	}
}

// tempIndexOf copies the index of dir to a file of the git dir, as git
// commit does for a commit that stages for itself, runs setup with
// GIT_INDEX_FILE pointing at it and returns the variable for the hook.
func tempIndexOf(t *testing.T, dir string, setup func(env ...string)) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
	require.NoError(t, err)
	idx := filepath.Join(dir, ".git", "next-index-123.lock")
	require.NoError(t, os.WriteFile(idx, b, 0o600))
	setup("GIT_INDEX_FILE=" + idx)
	return "GIT_INDEX_FILE=" + idx
}

// gitIn runs plain git in dir with the extra variables.
func gitIn(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	_, err := gitErrEnv(t, dir, env, args...)
	require.NoError(t, err)
}

// committed makes a repo whose HEAD holds a fresh mise.toml and mise.lock.
func committed(t *testing.T) string {
	t.Helper()
	dir := lockRepo(t)
	stage(t, dir, map[string]string{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "mise.lock": lockText})
	git(t, dir, "commit", "-q", "-m", "mise")
	return dir
}

// TestLockCheckCommitA: git commit -a stages every tracked file in a
// temporary index it hands the hook; a stale mise.toml that only the work
// tree holds is judged there, and the repository's own index, which
// stages nothing, would pass it.
func TestLockCheckCommitA(t *testing.T) {
	dir := committed(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\njq = \"1\"\n"), 0o600))
	stderr, err := runAction(t, dir)
	require.NoError(t, err, "nothing is staged in the repository's own index: %s", stderr)

	idx := tempIndexOf(t, dir, func(env ...string) { gitIn(t, dir, env, "add", "-u") })
	stderr, err = runAction(t, dir, idx)
	require.Error(t, err)
	assert.Equal(t, 6, cli.ExitCode(err))
	assert.Contains(t, stderr, "finding lock-tool-missing jq\n")
}

// TestLockCheckCommitOnly: git commit <path> builds its temporary index
// from HEAD and the path; a stale mise.toml that is only in the work tree
// is not in that commit, so the commit passes.
func TestLockCheckCommitOnly(t *testing.T) {
	dir := committed(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\njq = \"1\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o600))
	idx := filepath.Join(dir, ".git", "next-index-7.lock")
	env := []string{"GIT_INDEX_FILE=" + idx}
	gitIn(t, dir, env, "read-tree", "HEAD")
	gitIn(t, dir, env, "add", "other.txt")
	stderr, err := runAction(t, dir, env...)
	require.NoError(t, err, stderr)

	gitIn(t, dir, env, "add", "mise.toml")
	_, err = runAction(t, dir, env...)
	require.Error(t, err, "naming mise.toml in the commit makes it part of it")
}

// TestLockCheckDirtyTreeDoesNotBlockAGoodCommit: a good mise.toml and
// lock staged, a stale change of the work tree left unstaged, and the
// commit passes: the index is what is judged.
func TestLockCheckDirtyTreeDoesNotBlockAGoodCommit(t *testing.T) {
	dir := committed(t)
	stage(t, dir, map[string]string{"mise.toml": "[tools]\ngh = \"2.102.0\"\n", "other.txt": "x"})
	stage(t, dir, map[string]string{"mise.lock": lockText + "\n# comment\n"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\njq = \"1\"\n"), 0o600))
	stderr, err := runAction(t, dir)
	require.NoError(t, err, stderr)
}

// TestLockCheckRefusesAnIndexOutsideTheGitDir: a GIT_INDEX_FILE that is
// not a file below the git dir is an error, so a variable of the hook's
// caller cannot make the action read another file.
func TestLockCheckRefusesAnIndexOutsideTheGitDir(t *testing.T) {
	dir := committed(t)
	other := filepath.Join(t.TempDir(), "index")
	require.NoError(t, os.WriteFile(other, nil, 0o600))
	for _, idx := range []string{other, filepath.Join(dir, ".git"), filepath.Join(dir, ".git", "..", "x"), filepath.Join(dir, "mise.toml"), ".git/../mise.toml"} {
		_, err := runAction(t, dir, "GIT_INDEX_FILE="+idx)
		require.Error(t, err, idx)
		assert.NotEqual(t, 6, cli.ExitCode(err), idx)
	}
	stderr, err := runAction(t, dir, "GIT_INDEX_FILE=.git/index")
	require.NoError(t, err, "a relative path below the git dir is read from the hook's directory: %s", stderr)
	_, err = runAction(t, dir, "GIT_DIR="+filepath.Join(dir, ".git"), "GIT_INDEX_FILE="+filepath.Join(dir, ".git", "index"))
	require.NoError(t, err)
}

// TestLockCheckRefusesWhatTheIndexHoldsBadly: a mise.toml that is a link
// in the index is an error of the hook that names it.
func TestLockCheckRefusesWhatTheIndexHoldsBadly(t *testing.T) {
	dir := lockRepo(t)
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(dir, "mise.toml")))
	git(t, dir, "add", "mise.toml")
	_, err := runAction(t, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mise.toml")
	assert.NotEqual(t, 6, cli.ExitCode(err))
}

// TestLockCheckStagesOnlyTheLock: staging mise.lock alone asks for the
// check, which reads the mise.toml of HEAD with it.
func TestLockCheckStagesOnlyTheLock(t *testing.T) {
	dir := committed(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\njq = \"1\"\n"), 0o600))
	git(t, dir, "add", "mise.toml")
	git(t, dir, "commit", "-q", "-m", "stale toml, lock behind") // the commit is made without the hook here
	stage(t, dir, map[string]string{"mise.lock": lockText + "\n"})
	stderr, err := runAction(t, dir)
	require.Error(t, err, "only mise.lock is staged, and it is behind the mise.toml of HEAD")
	assert.Contains(t, stderr, "finding lock-tool-missing jq\n")
}

// gitErrEnv is gitErr with extra variables after the fixed environment.
func gitErrEnv(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir, Stdout: &stdout, Stderr: &stderr,
		Env: append([]string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}, env...)})
	if err != nil {
		return stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), nil
}
