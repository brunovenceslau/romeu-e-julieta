// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headsJSON is snapshot/heads.json (08 8.4, REQ-0689), every field,
// read strictly.
type headsJSON struct {
	CreatedAt time.Time         `json:"createdAt"`
	Disabled  bool              `json:"disabled"`
	Refs      map[string]string `json:"refs"`
	Schema    string            `json:"schema"`
}

// snapshotScript sets the identity of the commits after setup and gives
// the primary a local post-commit that logs, so a case sees the repo's
// own hook run after julieta's snapshot. heads prints the primary's
// heads.json; verify checks its bundle in the primary.
const snapshotScript = `
export JULIETA_MANIFEST=%s
step setup "$J" setup
git config --global user.name e2e
git config --global user.email e2e@example.invalid
git config --global commit.gpgsign false
cd ` + primaryPath + `
printf '#!/bin/sh\necho "local post-commit" >>/tmp/hooklog\n' >.git/hooks/post-commit
chmod +x .git/hooks/post-commit
snap=/tmp/memory/api/snapshot
`

// snapshotScenario returns the scenario of a snapshot case: snapshotScript, then
// body.
func snapshotScenario(t *testing.T, body string) map[string]stepResult {
	t.Helper()
	return scenario(t, strings.Replace(snapshotScript, "%s", manifestEnv(t, 1, originURL+"api.git"), 1)+body)
}

// heads decodes the heads.json a step printed.
func heads(t *testing.T, r stepResult) headsJSON {
	t.Helper()
	require.Equal(t, 0, r.code, r.stderr)
	var h headsJSON
	decodeStrict(t, r.stdout, &h)
	assert.Equal(t, "snapshot/v1", h.Schema)
	return h
}

// TestJulietaSnapshot runs the snapshot in the workload (08 8.4; 10
// 10.1, snapshot): after setup, a commit, an amend and a merge each run
// it through the dispatcher (post-commit, post-rewrite, post-merge,
// REQ-0686), so snapshot/heads.json follows HEAD and its bundle passes
// git bundle verify; the amended commit is in the bundle as a
// reflog-only commit; the repo's own post-commit still runs; julieta
// snapshot --all writes the secondary's pair too and prints each path
// with its repo.
func TestJulietaSnapshot(t *testing.T) {
	steps := snapshotScenario(t, `
step none sh -c 'ls -A /tmp/memory/api | grep -c snapshot || true'
git commit -q --allow-empty -m first
first=$(git rev-parse HEAD)
step commit cat $snap/heads.json
step head1 git rev-parse HEAD
step verify1 git bundle verify -q $snap/heads.bundle
git commit -q --amend --allow-empty -m "first amended"
step amend cat $snap/heads.json
step head2 git rev-parse HEAD
step old echo "$first"
step lists git bundle list-heads $snap/heads.bundle
git checkout -q -b topic
git commit -q --allow-empty -m topic
git checkout -q main
git merge -q --no-ff -m merge topic
step merge cat $snap/heads.json
step head3 git rev-parse HEAD
step verify3 git bundle verify -q $snap/heads.bundle
step log cat /tmp/hooklog
step failures sh -c 'ls -A "$HOME/.local/state/julieta/hook-failures" 2>/dev/null | wc -l'
step all "$J" snapshot --all
step web cat /tmp/memory/web/snapshot/heads.json
step check "$J" memory check
`)
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "none").stdout), "no snapshot before the first commit")

	h := heads(t, step(t, steps, "commit"))
	head1 := strings.TrimSpace(step(t, steps, "head1").stdout)
	assert.Equal(t, head1, h.Refs["refs/heads/main"], "post-commit took the snapshot")
	assert.False(t, h.Disabled)
	assert.Equal(t, 0, step(t, steps, "verify1").code, step(t, steps, "verify1").stderr)

	h = heads(t, step(t, steps, "amend"))
	assert.Equal(t, strings.TrimSpace(step(t, steps, "head2").stdout), h.Refs["refs/heads/main"], "post-rewrite took the snapshot")
	old := strings.TrimSpace(step(t, steps, "old").stdout)
	assert.Equal(t, old, h.Refs["refs/salvage-reflog/main/1"], "the amended commit, as main@{1}")
	assert.Contains(t, step(t, steps, "lists").stdout, old+" refs/salvage-reflog/main/1\n", "and in the bundle")

	h = heads(t, step(t, steps, "merge"))
	assert.Equal(t, strings.TrimSpace(step(t, steps, "head3").stdout), h.Refs["refs/heads/main"], "post-merge took the snapshot")
	assert.Contains(t, h.Refs, "refs/heads/topic")
	assert.Equal(t, 0, step(t, steps, "verify3").code, step(t, steps, "verify3").stderr)

	assert.Equal(t, 3, strings.Count(step(t, steps, "log").stdout, "local post-commit\n"), "the repo's own post-commit ran after each commit")
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "failures").stdout), "no hook failure")

	r := step(t, steps, "all")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Contains(t, r.stdout, "wrote /tmp/memory/api/snapshot/heads.json createdAt ")
	assert.Contains(t, r.stdout, "wrote /tmp/memory/web/snapshot/heads.bundle repo web\n")
	assert.NotEmpty(t, heads(t, step(t, steps, "web")).Refs["refs/heads/main"])
	assert.Equal(t, 0, step(t, steps, "check").code, "the snapshot leaves the memory dir in its layout: %s", step(t, steps, "check").stdout)
}

// TestJulietaSnapshotWriteFailure (08 8.4, REQ-0691; 04 4.3, REQ-0409):
// a snapshot that cannot be written, here a snapshot dir julieta may
// not write, does not fail the commit: the previous bundle stays, no
// .tmp- file remains, the failure is recorded as a hook failure with
// the repo, and the repo's own post-commit still runs; the next commit
// that can write clears the record.
func TestJulietaSnapshotWriteFailure(t *testing.T) {
	steps := snapshotScenario(t, `
step uid id -u
git commit -q --allow-empty -m first
before=$(sha256sum $snap/heads.bundle)
chmod 500 $snap
: >/tmp/hooklog
step commit git commit -q --allow-empty -m second
chmod 700 $snap
step same sh -c '[ "$(sha256sum $1/heads.bundle)" = "$2" ] && echo same' sh $snap "$before"
step files ls -A $snap
step state sh -c 'cat "$HOME"/.local/state/julieta/hook-failures/*.json'
step log cat /tmp/hooklog
step again git commit -q --allow-empty -m third
step cleared sh -c 'ls -A "$HOME/.local/state/julieta/hook-failures" | wc -l'
`)
	require.NotEqual(t, "0", strings.TrimSpace(step(t, steps, "uid").stdout), "root writes a read-only directory, so the case runs as another user")
	r := step(t, steps, "commit")
	assert.Equal(t, 0, r.code, "the commit stands: %s", r.stderr)
	assert.Contains(t, r.stderr, "julieta hooks: post-commit stage julieta (snapshot) exited 1\n")
	assert.Equal(t, "same\n", step(t, steps, "same").stdout, "the previous bundle stays")
	assert.Equal(t, "heads.bundle\nheads.json\n", step(t, steps, "files").stdout, "no .tmp- file remains")
	state := step(t, steps, "state").stdout
	assert.Contains(t, state, `"hook":"post-commit"`)
	assert.Contains(t, state, `"repo":"`+primaryPath+`"`)
	assert.Contains(t, state, `"stage":"julieta"`)
	assert.Equal(t, "local post-commit\n", step(t, steps, "log").stdout, "the repo's own post-commit still ran")
	assert.Equal(t, 0, step(t, steps, "again").code)
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "cleared").stdout))
}

// TestJulietaSnapshotDisabled (08 8.4, REQ-0692): a repo whose own
// config sets core.hooksPath bypasses the dispatcher, so a commit takes
// no snapshot, and julieta snapshot writes disabled: true.
func TestJulietaSnapshotDisabled(t *testing.T) {
	steps := snapshotScenario(t, `
git config core.hooksPath .husky
git commit -q --allow-empty -m first
step none sh -c 'ls -A /tmp/memory/api | grep -c snapshot || true'
step snap "$J" snapshot
step heads cat $snap/heads.json
`)
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "none").stdout), "the dispatcher did not run")
	require.Equal(t, 0, step(t, steps, "snap").code, step(t, steps, "snap").stderr)
	assert.True(t, heads(t, step(t, steps, "heads")).Disabled)
}
