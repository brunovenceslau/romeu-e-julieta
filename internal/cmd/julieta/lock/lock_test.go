// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lock_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/lock"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// miseToml is the mise.toml of the repos of these tests: two tools, one
// locked in testdata/valid.lock and one not.
const miseToml = "[tools]\ngh = \"2.102.0\"\ngo = \"1.27.3\"\n"

// sandbox is a HOME with a manifest of the project shop, the repos api
// (primary, the working directory) and web, each with a mise.toml and
// the mise.lock of testdata.
type sandbox struct {
	api, web string
}

// newSandbox makes the sandbox and sets HOME and JULIETA_MANIFEST.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	home := t.TempDir()
	s := &sandbox{api: filepath.Join(t.TempDir(), "api"), web: filepath.Join(home, "src", "shop", "web")}
	mem := t.TempDir()
	for _, p := range []string{s.api, s.web, filepath.Join(mem, "api"), filepath.Join(mem, "web")} {
		require.NoError(t, os.MkdirAll(p, 0o700))
	}
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	m := manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: s.api, Memory: filepath.Join(mem, "api")},
			{Dir: "web", URL: "git://127.0.0.1:9418/web.git", Ref: "main", Path: s.web, Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", home)
	v, err := manifest.EncodeEnv(m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	t.Chdir(s.api)
	return s
}

// write writes a file of a repo.
func write(t *testing.T, repo, name string, data []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(repo, name), data, 0o600))
}

// validLock is testdata/valid.lock, read before a test changes the
// working directory.
var validLock = func() []byte {
	b, err := os.ReadFile(filepath.Join("testdata", "valid.lock"))
	if err != nil {
		panic(err)
	}
	return b
}()

// fixture returns testdata/valid.lock.
func fixture(t *testing.T) []byte {
	t.Helper()
	return validLock
}

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: time.Now, Version: cli.Version{Version: "v0.1.0-test", Protocol: manifest.Protocol}}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// fakeMise puts a mise that runs script first on PATH.
func fakeMise(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "mise"), []byte("#!/bin/sh\n"+script+"\n"), 0o700))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

// noMise makes a PATH that holds no mise, so a run of one fails.
func noMise(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	for _, p := range []string{"sh", "cat", "echo", "env", "sort"} {
		if full, err := procgroup.LookPath(p); err == nil {
			require.NoError(t, os.Symlink(full, filepath.Join(bin, p)))
		}
	}
	t.Setenv("PATH", bin)
}

// TestCheckFresh: a lock that covers every tool on both linux platforms
// passes, writes nothing and starts no mise (04 4.3, REQ-0406).
func TestCheckFresh(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte("[tools]\ngh = \"2.102.0\"\ngo = \"1.27.3\"\n"))
	write(t, s.api, "mise.lock", fixture(t))
	noMise(t)
	before, err := os.ReadDir(s.api)
	require.NoError(t, err)
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api ok\n", stdout)
	assert.Empty(t, stderr)
	after, err := os.ReadDir(s.api)
	require.NoError(t, err)
	assert.Len(t, after, len(before), "nothing was written")
}

// TestCheckFindings: a tool without an entry and a lockable entry
// without a linux platform each fail the check with exit 6,
// check-findings (RJ-328), and the details name each by its slug.
func TestCheckFindings(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte("[tools]\ngh = \"2.102.0\"\njq = \"1.7.1\"\n"))
	lock := strings.Replace(string(fixture(t)), "[tools.gh.\"platforms.linux-arm64\"]", "[tools.gh.\"platforms.windows-x64\"]", 1)
	write(t, s.api, "mise.lock", []byte(lock))
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stdout, "finding lock-tool-missing jq\n")
	assert.Contains(t, stdout, "finding lock-platform-missing gh linux-arm64\n")
	assert.True(t, strings.HasPrefix(stderr, "error RJ-328 check-findings: "), stderr)
	assert.Contains(t, stderr, "lock-tool-missing")
}

// TestCheckNoLock: a mise.toml with no mise.lock fails every tool.
func TestCheckNoLock(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	code, stdout, _ := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code)
	assert.Contains(t, stdout, "finding lock-tool-missing gh\n")
	assert.Contains(t, stdout, "finding lock-tool-missing go\n")
}

// dropFirstChecksum returns the lock without the first checksum line,
// so one platform of one entry lacks its checksum.
func dropFirstChecksum(lock string) string {
	var out []string
	dropped := false
	for l := range strings.SplitSeq(lock, "\n") {
		if !dropped && strings.HasPrefix(l, "checksum = ") {
			dropped = true
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// TestCheckNoChecksum: an entry without a checksum is a finding, exit 6,
// and with one finding only the exit is still 6 (REQ-0407 as decided on
// 2026-10-22); the exceptions file of the repo excuses its tool.
func TestCheckNoChecksum(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	write(t, s.api, "mise.lock", []byte(dropFirstChecksum(string(fixture(t)))))
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "lock api 1 findings\nfinding lock-no-checksum gh linux-arm64\n", stdout)
	assert.Contains(t, stderr, "error RJ-328 check-findings: ")

	require.NoError(t, os.MkdirAll(filepath.Join(s.api, ".julieta"), 0o755))
	write(t, s.api, ".julieta/lock-exceptions", []byte("gh the vendor publishes no checksum\n"))
	code, stdout, stderr = julieta(t, "lock", "--check")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api ok\n", stdout)
}

// TestCheckBadChecksum: a checksum that is not sha256: and 64 lowercase
// hexadecimal digits is a finding, and no exception excuses it.
func TestCheckBadChecksum(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	lock := strings.Replace(string(fixture(t)), "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386", "md5:zz", 1)
	write(t, s.api, "mise.lock", []byte(lock))
	require.NoError(t, os.MkdirAll(filepath.Join(s.api, ".julieta"), 0o755))
	write(t, s.api, ".julieta/lock-exceptions", []byte("gh the vendor publishes no checksum\n"))
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "lock api 1 findings\nfinding lock-bad-checksum gh linux-x64\n", stdout)
}

// TestCheckUnparsed: a lock or a mise.toml that does not parse is a
// finding, exit 6, not an error of another exit (REQ-0406).
func TestCheckUnparsed(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	write(t, s.api, "mise.lock", []byte("lockfile_version = 3\nlockfile_version = 3\n"))
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stdout, "finding lock-unparsed mise.lock: ")
	write(t, s.api, "mise.lock", fixture(t))
	write(t, s.api, "mise.toml", []byte("[tools\n"))
	code, stdout, _ = julieta(t, "lock", "--check")
	assert.Equal(t, 6, code)
	assert.Contains(t, stdout, "finding lock-unparsed mise.toml: ")
}

// TestCheckLinkedLock: a mise.lock that is a link cannot be read as a
// lock: lock-unparsed (RJ-325), exit 2, and the hint names the fix.
func TestCheckLinkedLock(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	write(t, s.api, "real.lock", fixture(t))
	require.NoError(t, os.Symlink("real.lock", filepath.Join(s.api, "mise.lock")))
	code, _, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 2, code, stderr)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-325 lock-unparsed: "), stderr)
	assert.Contains(t, stderr, "fix: julieta lock --repo api\n")
}

// TestCheckNothing: a repo with no mise.toml has nothing to check.
func TestCheckNothing(t *testing.T) {
	newSandbox(t)
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api no mise.toml\n", stdout)
}

// TestCheckRepoFlag: --repo names the repo, whatever the working
// directory is.
func TestCheckRepoFlag(t *testing.T) {
	s := newSandbox(t)
	write(t, s.web, "mise.toml", []byte(miseToml))
	code, stdout, _ := julieta(t, "lock", "--check", "--repo", "web")
	assert.Equal(t, 6, code)
	assert.Contains(t, stdout, "lock web 2 findings\n")
	code, _, stderr := julieta(t, "lock", "--check", "--repo", "nope")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "nope")
}

// TestLock runs mise lock with the platforms of 07 7.3 (REQ-0405): the
// two linux ones and the repo's own lockfile_platforms.
func TestLock(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte("[settings]\nlockfile_platforms = [\"macos-arm64\"]\n"+miseToml))
	rec := filepath.Join(t.TempDir(), "rec")
	fakeMise(t, `echo "$@" > '`+rec+`'; pwd >> '`+rec+`'; echo locked > mise.lock`)
	code, stdout, stderr := julieta(t, "lock")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api platforms linux-x64,linux-arm64,macos-arm64\n", stdout)
	b, err := os.ReadFile(rec)
	require.NoError(t, err)
	real, err := filepath.EvalSymlinks(s.api)
	require.NoError(t, err)
	assert.Equal(t, "lock --platform linux-x64,linux-arm64,macos-arm64\n"+real+"\n", string(b))
}

// TestLockNoMiseToml: with no mise.toml there is nothing to lock, and
// mise is not started.
func TestLockNoMiseToml(t *testing.T) {
	newSandbox(t)
	noMise(t)
	code, stdout, stderr := julieta(t, "lock")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api no mise.toml\n", stdout)
}

// TestLockFails: a mise that fails is exit 1.
func TestLockFails(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	fakeMise(t, `echo boom >&2; exit 1`)
	code, _, stderr := julieta(t, "lock")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "boom")
}

// TestLockRefusesALinkedLock: a mise.lock that is a link is refused
// before mise runs, since mise would write where it points (the audit of
// 2026-10-22).
func TestLockRefusesALinkedLock(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	target := filepath.Join(t.TempDir(), "elsewhere")
	require.NoError(t, os.Symlink(target, filepath.Join(s.api, "mise.lock")))
	rec := filepath.Join(t.TempDir(), "rec")
	fakeMise(t, `echo ran > '`+rec+`'; echo locked > mise.lock`)
	code, _, stderr := julieta(t, "lock")
	assert.Equal(t, 2, code, stderr)
	assert.NoFileExists(t, rec, "mise was not started")
	assert.NoFileExists(t, target)
}

// TestLockUnparsedMiseToml: a mise.toml that does not parse is
// lock-unparsed, exit 2, and mise is not started.
func TestLockUnparsedMiseToml(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte("[tools\n"))
	noMise(t)
	code, _, stderr := julieta(t, "lock")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "RJ-325")
}

// nested makes the primary repo the parent of the secondary: the repo of
// a working directory is the deepest that holds it.
func nested(t *testing.T) *sandbox {
	t.Helper()
	home := t.TempDir()
	parent := filepath.Join(home, "src", "shop")
	web := filepath.Join(parent, "web")
	mem := t.TempDir()
	for _, p := range []string{web, filepath.Join(mem, "api"), filepath.Join(mem, "web")} {
		require.NoError(t, os.MkdirAll(p, 0o700))
	}
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	m := manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: parent, Memory: filepath.Join(mem, "api")},
			{Dir: "web", URL: "git://127.0.0.1:9418/web.git", Ref: "main", Path: web, Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", home)
	v, err := manifest.EncodeEnv(m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	return &sandbox{api: parent, web: web}
}

// TestRepoOfTheWorkingDirectory: the repo is the deepest whose path
// holds the working directory, a link to it counts, a sibling whose name
// starts with the repo's does not, and --repo wins over the directory.
func TestRepoOfTheWorkingDirectory(t *testing.T) {
	s := nested(t)
	write(t, s.web, "mise.toml", []byte(miseToml))
	t.Chdir(s.web)
	_, stdout, _ := julieta(t, "lock", "--check")
	assert.Equal(t, "lock web 2 findings\nfinding lock-tool-missing gh\nfinding lock-tool-missing go\n", stdout, "the deepest repo holds the directory")

	require.NoError(t, os.MkdirAll(filepath.Join(s.web, "deep", "er"), 0o755))
	t.Chdir(filepath.Join(s.web, "deep", "er"))
	_, stdout, _ = julieta(t, "lock", "--check")
	assert.Contains(t, stdout, "lock web ")

	t.Chdir(s.api)
	_, stdout, _ = julieta(t, "lock", "--check")
	assert.Equal(t, "lock api no mise.toml\n", stdout, "the parent of web is not in web")

	require.NoError(t, os.Mkdir(filepath.Join(s.api, "other"), 0o755))
	t.Chdir(filepath.Join(s.api, "other"))
	_, stdout, _ = julieta(t, "lock", "--check")
	assert.Equal(t, "lock api no mise.toml\n", stdout, "a directory of the parent only")

	t.Chdir(filepath.Join(s.web, "deep"))
	_, stdout, _ = julieta(t, "lock", "--check", "--repo", "api")
	assert.Equal(t, "lock api no mise.toml\n", stdout, "--repo wins")

	sibling := s.web + "-old"
	require.NoError(t, os.MkdirAll(sibling, 0o755))
	t.Chdir(sibling)
	_, stdout, _ = julieta(t, "lock", "--check")
	assert.Equal(t, "lock api no mise.toml\n", stdout, "web-old is not in web, but in the parent")

	outside := t.TempDir()
	t.Chdir(outside)
	code, _, stderr := julieta(t, "lock", "--check")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "in no repo of the manifest")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(s.web, link))
	t.Chdir(link)
	_, stdout, _ = julieta(t, "lock", "--check")
	assert.Contains(t, stdout, "lock web ", "a link to the repo is in it")
}
