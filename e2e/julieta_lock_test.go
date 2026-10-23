// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJulietaLock runs julieta lock in the workload at the pinned mise
// (04 4.3, REQ-0405): mise lock gets the two linux platforms and the
// platform of the repo's lockfile_platforms, the lock it writes holds
// all three, with the checksum the mise.toml names, a second lock leaves
// it as it was, and lock --check passes on it (REQ-0407).
func TestJulietaLock(t *testing.T) {
	withPlatforms := "[settings]\nlockfile_platforms = [\"macos-arm64\"]\n\n" + helloToml
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+withPlatforms+`'

step lock "$J" lock
step platforms grep -o 'platforms[.][a-z0-9-]*' mise.lock
step sum1 sha256sum mise.lock
step again "$J" lock
step sum2 sha256sum mise.lock
step check "$J" lock --check
step checksums grep -c 'checksum = "sha256:' mise.lock
`)
	r := step(t, steps, "lock")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "lock api platforms linux-x64,linux-arm64,macos-arm64\n", r.stdout)
	assert.Equal(t, []string{"platforms.linux-arm64", "platforms.linux-x64", "platforms.macos-arm64"}, strings.Fields(step(t, steps, "platforms").stdout))
	require.Equal(t, 0, step(t, steps, "again").code, step(t, steps, "again").stderr)
	assert.Equal(t, step(t, steps, "sum1").stdout, step(t, steps, "sum2").stdout, "a second lock changes nothing")

	r = step(t, steps, "check")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "lock api ok\n", r.stdout)
	assert.Empty(t, r.stderr)
	assert.Equal(t, "3", strings.TrimSpace(step(t, steps, "checksums").stdout), "mise records the checksum the mise.toml names on each platform")
}

// TestJulietaLockCheckNoChecksum: the http backend records a URL and no
// checksum when mise.toml names none, and lock --check fails on it, exit
// 6, lock-no-checksum; the exceptions file excuses the tool, and a
// checksum of another form is a finding that no exception excuses
// (REQ-0407 as decided on 2026-10-22).
func TestJulietaLockCheckNoChecksum(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+bareToml+`'
step lock "$J" lock
step nosum "$J" lock --check
mkdir .julieta
printf 'http:hello the test server publishes no checksum\n' > .julieta/lock-exceptions
step excused "$J" lock --check
sed -i 's/^url = /checksum = "md5:zz"\nurl = /' mise.lock
step bad "$J" lock --check
`)
	require.Equal(t, 0, step(t, steps, "lock").code, step(t, steps, "lock").stderr)
	r := step(t, steps, "nosum")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Contains(t, r.stdout, "finding lock-no-checksum http:hello linux-arm64\n")
	assert.Contains(t, r.stdout, "finding lock-no-checksum http:hello linux-x64\n")
	r = step(t, steps, "excused")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "lock api ok\n", r.stdout)
	r = step(t, steps, "bad")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Contains(t, r.stdout, "finding lock-bad-checksum http:hello linux-x64\n")
}

// TestJulietaLockCheckFindings: lock --check exits 6, check-findings
// (RJ-328), for a tool of mise.toml with no entry in the lock and for a
// lockable entry that lacks a linux platform, reads and writes nothing
// else and starts no mise: it runs with a PATH that holds none, and the
// lock is as it was after it (REQ-0406).
func TestJulietaLockCheckFindings(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+helloToml+`'
put mise.lock '`+helloLock+`'
printf '"http:other" = { version = "2.0", url = "http://127.0.0.1:18080/other.tar.gz" }\n' >> mise.toml
sum() { sha256sum mise.lock mise.toml; }
step before sum
step missing env PATH=/usr/bin:/bin "$J" lock --check
sed -i 's/platforms.linux-arm64/platforms.macos-arm64/' mise.lock
step platform env PATH=/usr/bin:/bin "$J" lock --check
step after sum
`)
	r := step(t, steps, "missing")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Contains(t, r.stdout, "finding lock-tool-missing http:other\n")
	assert.Contains(t, r.stderr, "error RJ-328 check-findings: ")

	r = step(t, steps, "platform")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Contains(t, r.stdout, "finding lock-platform-missing http:hello linux-arm64\n")
	assert.Contains(t, r.stdout, "finding lock-tool-missing http:other\n")
}

// TestJulietaLockPreCommit: with the dispatcher setup installs, a commit
// that stages a mise.toml whose tools the lock does not hold fails in
// pre-commit, exit 6 with the id of the findings, and one that stages
// nothing of mise does not run the check (04 4.3, REQ-0408).
func TestJulietaLockPreCommit(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
step setup env -u JULIETA_MANIFEST JULIETA_MANIFEST="$JULIETA_MANIFEST" "$J" setup --json
put mise.toml '`+helloToml+`'
git add mise.toml
step stale g commit -q -m stale
git reset -q
put mise.lock '`+helloLock+`'
git add mise.toml mise.lock
step fresh g commit -q -m fresh
echo other > other.txt
git add other.txt
printf '[tools]\n"http:hello" = { version = "1.0", url = "http://127.0.0.1:18080/hello-1.0.tar.gz", checksum = "sha256:'$SUM'" }\nnewtool = "1"\n' > mise.toml
step unrelated g commit -q -m unrelated
step commita g commit -a -q -m everything
echo z > z.txt
git add z.txt
step commitonly g commit -q -m only z.txt
`)
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)
	r := step(t, steps, "stale")
	assert.NotEqual(t, 0, r.code, "the commit is refused")
	assert.Contains(t, r.stderr, "finding lock-tool-missing http:hello")
	assert.Contains(t, r.stderr, "julieta hooks: pre-commit stage julieta (lock-check) exited 6")
	assert.Equal(t, 0, step(t, steps, "fresh").code, step(t, steps, "fresh").stderr)
	assert.Equal(t, 0, step(t, steps, "unrelated").code, "mise.toml is changed but not staged: "+step(t, steps, "unrelated").stderr)
	r = step(t, steps, "commita")
	assert.NotEqual(t, 0, r.code, "git commit -a records the stale mise.toml of the work tree, which the hook judges in the temporary index")
	assert.Contains(t, r.stderr, "finding lock-tool-missing newtool")
	assert.Equal(t, 0, step(t, steps, "commitonly").code, "git commit <path> records that path only, and the dirty mise.toml is not in it: "+step(t, steps, "commitonly").stderr)
}
