// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package install_test

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
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/install"
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

// recorder returns the file the fake mise appends one line to on each run: the
// working directory and the argv, so a test counts the runs.
func recorder(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "rec")
}

// runs returns the lines of the recorder, one for each run of mise.
func runs(t *testing.T, rec string) []string {
	t.Helper()
	b, err := os.ReadFile(rec)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// withLock gives the repo a mise.toml and the mise.lock of testdata.
func withLock(t *testing.T, repo string) {
	t.Helper()
	write(t, repo, "mise.toml", []byte(miseToml))
	write(t, repo, "mise.lock", fixture(t))
}

// TestInstallRunsLocked runs mise install in each repo that has a
// mise.toml, locked, with the environment of 04 4.3 (REQ-0403,
// REQ-0426), and prints one line for each repo.
func TestInstallRunsLocked(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	withLock(t, s.web)
	rec := recorder(t)
	fakeMise(t, `echo "$(pwd) $@ locked=$MISE_LOCKED trusted=$MISE_TRUSTED_CONFIG_PATHS" >> '`+rec+`'`)
	code, stdout, stderr := julieta(t, "install")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "install api installed\ninstall web installed\n", stdout)
	got := runs(t, rec)
	require.Len(t, got, 2)
	realAPI, err := filepath.EvalSymlinks(s.api)
	require.NoError(t, err)
	assert.Equal(t, realAPI+" install locked=1 trusted="+s.api+":"+s.web, got[0])
}

// TestInstallSkips records the lock-set digest after a success and does
// nothing when it is unchanged; --force overrides, and a changed lock
// installs again (REQ-0618, REQ-0403).
func TestInstallSkips(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)

	code, stdout, stderr := julieta(t, "install")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", stdout)
	assert.Len(t, runs(t, rec), 1)

	code, stdout, stderr = julieta(t, "install")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "install api skipped, the lock-set is unchanged\ninstall web no mise.toml\n", stdout)
	assert.Len(t, runs(t, rec), 1, "the skip started no mise")

	code, stdout, _ = julieta(t, "install", "--force")
	assert.Equal(t, 0, code)
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", stdout)
	assert.Len(t, runs(t, rec), 2)

	write(t, s.api, "mise.lock", append(fixture(t), []byte("\n# changed\n")...))
	code, _, _ = julieta(t, "install")
	assert.Equal(t, 0, code)
	assert.Len(t, runs(t, rec), 3, "a changed lock installs again")
}

// TestInstallFailureRecordsNothing: a failed install records no digest,
// so the next one runs.
func TestInstallFailureRecordsNothing(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'; echo boom >&2; exit 1`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "boom")
	julieta(t, "install")
	assert.Len(t, runs(t, rec), 2)
}

// TestInstallRepoFlag installs the repo named alone.
func TestInstallRepoFlag(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	withLock(t, s.web)
	rec := recorder(t)
	fakeMise(t, `echo "$(pwd)" >> '`+rec+`'`)
	code, stdout, stderr := julieta(t, "install", "--repo", "web")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "install web installed\n", stdout)
	assert.Len(t, runs(t, rec), 1)
	code, _, stderr = julieta(t, "install", "--repo", "nope")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "nope")
}

// TestInstallLockMissing: a mise.toml with no mise.lock is lock-missing
// (RJ-340), exit 2, and the hint is julieta lock --repo <dir>; no repo
// is installed (REQ-0438).
func TestInstallLockMissing(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	write(t, s.web, "mise.toml", []byte(miseToml))
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 2, code)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-340 lock-missing: "), stderr)
	assert.Contains(t, stderr, "fix: julieta lock --repo web\n")
	assert.Empty(t, runs(t, rec), "nothing was installed")
}

// TestInstallVersionOnly warns version-only-tool once for each
// version-only tool of a lock, and still installs (REQ-0617).
func TestInstallVersionOnly(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml+"\"go:golang.org/x/tools/gopls\" = \"0.23.0\"\n"))
	write(t, s.api, "mise.lock", fixture(t))
	fakeMise(t, `:`)
	code, stdout, stderr := julieta(t, "install")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", stdout)
	assert.Equal(t, 1, strings.Count(stderr, "warning version-only-tool: "), stderr)
	assert.Contains(t, stderr, "go:golang.org/x/tools/gopls")
}

// TestInstallBlocked: a download the egress policy blocked is
// install-blocked (RJ-341), exit 1, whose details name the tool, the
// host, the sbx command that approves it once, that the approval is
// lost at recreate, the durable fix and whether the host is in the
// manifest's egress set (REQ-0439, REQ-0637).
func TestInstallBlocked(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	fakeMise(t, `echo "mise ERROR Failed to install gh@2.102.0: HTTP status client error (403 Forbidden) for url (https://github.com/cli/cli/x.tgz)" >&2; exit 1`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-341 install-blocked: "), stderr)
	assert.Contains(t, stderr, "tool: gh\n")
	assert.Contains(t, stderr, "host: github.com\n")
	assert.Contains(t, stderr, "sbx policy allow network --sandbox shop github.com")
	assert.Contains(t, stderr, "lost when the sandbox is recreated")
	assert.Contains(t, stderr, "gate 2")
	assert.Contains(t, stderr, "is not in the egress set", "the sandbox of this test carries an empty set")
}

// TestInstallTimeout: a mise that hangs is killed with its group and
// install exits 1 with subprocess-timeout (RJ-323), naming the tool
// (REQ-0300). The limit is set through the seam of the timeout table.
func TestInstallTimeout(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	fakeMise(t, `echo "mise   gh@2.102.0"; sleep 30`)
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Now: time.Now}
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 300*time.Millisecond)
	code := cli.Main(ctx, []string{"install"}, env)
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(stderr.String(), "error RJ-323 subprocess-timeout: "), stderr.String())
	assert.Contains(t, stderr.String(), "tool: gh@2.102.0\n")
}

// TestInstallRefusesOnTheFindingsOfLockCheck: install runs the check
// itself, refuses on the same findings (check-findings, RJ-328, exit 6)
// and installs nothing, in any repo (decided on 2026-10-22); the
// exceptions file of the repo excuses a missing checksum.
func TestInstallRefusesOnTheFindingsOfLockCheck(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	withLock(t, s.web)
	var dropped bool
	var out []string
	for l := range strings.SplitSeq(string(fixture(t)), "\n") {
		if !dropped && strings.HasPrefix(l, "checksum = ") {
			dropped = true
			continue
		}
		out = append(out, l)
	}
	write(t, s.web, "mise.lock", []byte(strings.Join(out, "\n")))
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 6, code, stderr)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-328 check-findings: "), stderr)
	assert.Contains(t, stderr, "lock-no-checksum")
	assert.Contains(t, stderr, "web gh linux-arm64: lock-no-checksum")
	assert.Empty(t, runs(t, rec), "nothing was installed, not even in the repo that is fine")

	require.NoError(t, os.MkdirAll(filepath.Join(s.web, ".julieta"), 0o755))
	write(t, s.web, ".julieta/lock-exceptions", []byte("gh the vendor publishes no checksum\n"))
	code, _, stderr = julieta(t, "install")
	assert.Equal(t, 0, code, stderr)
	assert.Len(t, runs(t, rec), 2)
}

// TestInstallUnparsed: a lock or a mise.toml that does not parse is
// lock-unparsed (RJ-325), exit 2, with the hint of the repo.
func TestInstallUnparsed(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	write(t, s.api, "mise.lock", []byte("lockfile_version = 3\nlockfile_version = 3\n"))
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 2, code, stderr)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-325 lock-unparsed: "), stderr)
	assert.Contains(t, stderr, "fix: julieta lock --repo api\n")
	write(t, s.api, "mise.lock", fixture(t))
	write(t, s.api, "mise.toml", []byte("[tools\n"))
	code, _, stderr = julieta(t, "install")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "RJ-325")
	assert.Empty(t, runs(t, rec))
}

// TestInstallFailureIsMiseFailed: a mise that fails is mise-failed
// (RJ-351), exit 1, with the argv and the status in the details.
func TestInstallFailureIsMiseFailed(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	fakeMise(t, `echo boom >&2; exit 4`)
	code, _, stderr := julieta(t, "install")
	assert.Equal(t, 1, code)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-351 mise-failed: "), stderr)
	assert.Contains(t, stderr, "argv: mise install\n")
	assert.Contains(t, stderr, "status: exited 4\n")
	assert.Contains(t, stderr, "stderr: boom\n")
}

// TestInstallOneRepoRecordsNothing: a run over one repo neither skips
// nor records the digest, so the next full run still installs, and then
// skips.
func TestInstallOneRepoRecordsNothing(t *testing.T) {
	s := newSandbox(t)
	withLock(t, s.api)
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)
	for range 2 {
		code, _, stderr := julieta(t, "install", "--repo", "api")
		require.Equal(t, 0, code, stderr)
	}
	assert.Len(t, runs(t, rec), 2, "a run over one repo is never skipped")
	_, stdout, _ := julieta(t, "install")
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", stdout, "nothing was recorded by the runs over one repo")
	_, stdout, _ = julieta(t, "install")
	assert.Equal(t, "install api skipped, the lock-set is unchanged\ninstall web no mise.toml\n", stdout)
}

// TestInstallRefusesARepoPathWithAColon: a repo path that
// MISE_TRUSTED_CONFIG_PATHS would read as two paths reaches no mise.
func TestInstallRefusesARepoPathWithAColon(t *testing.T) {
	s := newSandbox(t)
	colon := filepath.Join(filepath.Dir(s.api), "a:b")
	require.NoError(t, os.Rename(s.api, colon))
	m := manifestWithPrimary(t, colon)
	t.Setenv(setup.ManifestEnv, m)
	t.Chdir(colon)
	withLock(t, colon)
	rec := recorder(t)
	fakeMise(t, `echo run >> '`+rec+`'`)
	code, _, stderr := julieta(t, "install")
	assert.NotEqual(t, 0, code, stderr)
	assert.Empty(t, runs(t, rec))
}

// manifestWithPrimary returns the env value of a manifest whose primary
// repo is at path, and whose secondary is below HOME.
func manifestWithPrimary(t *testing.T, path string) string {
	t.Helper()
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	m := manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: path, Memory: t.TempDir()},
		},
	}
	v, err := manifest.EncodeEnv(m)
	require.NoError(t, err)
	return v
}
