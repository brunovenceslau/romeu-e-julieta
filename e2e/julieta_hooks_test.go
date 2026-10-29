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

// hooksScript builds the sandbox of the hooks cases: setup points the
// global core.hooksPath at the dispatcher it wrote, with no line of the
// case setting it, and the primary repo has a tracked pre-commit and
// post-rewrite and a local post-commit, each logging to /tmp/hooklog.
const hooksScript = `
export JULIETA_MANIFEST=%s
step setup "$J" setup
step global git config --global core.hooksPath
git config --global user.name e2e
git config --global user.email e2e@example.invalid
git config --global commit.gpgsign false
cd ` + primaryPath + `
mkdir -p .githooks
cat >.githooks/pre-commit <<'EOF2'
#!/bin/sh
echo "tracked pre-commit $*" >>/tmp/hooklog
[ ! -e /tmp/stop ] || { echo "the tracked hook says no" >&2; exit 3; }
[ ! -e /tmp/term ] || kill -TERM $$
EOF2
cat >.githooks/post-rewrite <<'EOF2'
#!/bin/sh
echo "tracked post-rewrite $1 stdin=$(cat)" >>/tmp/hooklog
EOF2
cat >.git/hooks/post-rewrite <<'EOF2'
#!/bin/sh
echo "local post-rewrite $1 stdin=$(cat)" >>/tmp/hooklog
EOF2
cat >.git/hooks/post-commit <<'EOF2'
#!/bin/sh
echo "local post-commit" >>/tmp/hooklog
EOF2
chmod +x .githooks/* .git/hooks/post-rewrite .git/hooks/post-commit
`

// TestJulietaHooks runs the hook dispatcher in the workload (04 4.3;
// I21): a git commit runs the tracked pre-commit and the local
// post-commit through it, with the hook's arguments, and the stdin of
// post-rewrite reaches both the tracked and the local hook; the first non-zero
// status stops the chain and is git's failure, the stage line names the
// stage and its status, and the failure is in julieta state until the
// same hook passes; a stage a signal ended is "exited 128+15".
func TestJulietaHooks(t *testing.T) {
	steps := scenario(t, strings.Replace(hooksScript, "%s", manifestEnv(t, 1, originURL+"api.git"), 1)+`
step ok git commit -q --allow-empty -m first
step log cat /tmp/hooklog
: >/tmp/hooklog
step amend git commit -q --amend --allow-empty -m second
step rewrite cat /tmp/hooklog
: >/tmp/hooklog
touch /tmp/stop
step stopped git commit -q --allow-empty -m third
step stoppedlog cat /tmp/hooklog
step state sh -c 'cat "$HOME"/.local/state/julieta/hook-failures/*.json'
rm /tmp/stop
step recovered git commit -q --allow-empty -m third
step cleared sh -c 'ls -A "$HOME/.local/state/julieta/hook-failures" | wc -l'
touch /tmp/term
step signalled git commit -q --allow-empty -m fourth
step signalstate sh -c 'cat "$HOME"/.local/state/julieta/hook-failures/*.json'
`)
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)
	assert.Equal(t, sandboxHome+"/.local/share/julieta/hooks\n", step(t, steps, "global").stdout, "setup set the global key")

	r := step(t, steps, "ok")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "tracked pre-commit \nlocal post-commit\n", step(t, steps, "log").stdout,
		".githooks/pre-commit, then $GIT_DIR/hooks/post-commit, through the dispatcher")

	assert.Equal(t, 0, step(t, steps, "amend").code, step(t, steps, "amend").stderr)
	rewrite := step(t, steps, "rewrite").stdout
	assert.Contains(t, rewrite, "tracked post-rewrite amend stdin=")
	assert.Regexp(t, `tracked post-rewrite amend stdin=[0-9a-f]{40} [0-9a-f]{40}\n`, rewrite, "the tracked hook reads git's stdin")
	assert.Regexp(t, `local post-rewrite amend stdin=[0-9a-f]{40} [0-9a-f]{40}\n`, rewrite, "the local hook gets its own copy of the stdin")

	r = step(t, steps, "stopped")
	assert.NotEqual(t, 0, r.code, "git stops the commit on the hook's status")
	assert.Contains(t, r.stderr, "the tracked hook says no\n")
	assert.Contains(t, r.stderr, "julieta hooks: pre-commit stage tracked ("+primaryPath+"/.githooks/pre-commit) exited 3\n")
	assert.Equal(t, "tracked pre-commit \n", step(t, steps, "stoppedlog").stdout, "the chain stopped before the commit's own hooks")
	state := step(t, steps, "state").stdout
	assert.Contains(t, state, `"exitCode":3`)
	assert.Contains(t, state, `"hook":"pre-commit"`)
	assert.Contains(t, state, `"repo":"`+primaryPath+`"`)

	assert.Equal(t, 0, step(t, steps, "recovered").code, step(t, steps, "recovered").stderr)
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "cleared").stdout), "a later success of the same hook clears the record")

	r = step(t, steps, "signalled")
	assert.NotEqual(t, 0, r.code)
	assert.Contains(t, r.stderr, ") exited 128+15\n")
	assert.Contains(t, step(t, steps, "signalstate").stdout, `"exitCode":143`)
}

// systemScript adds to hooksScript a system hooks directory, as the
// sandbox kit sets one with the system core.hooksPath, whose pre-commit
// logs to /tmp/hooklog and fails while /tmp/sysstop exists, and record,
// which writes the record setup keeps of the system value. The workload
// runs as nobody, who cannot write /etc/gitconfig, so the case plants
// the record julieta state holds (the file system-hooks-path.json of
// internal/hooks) after setup has run; the read of a planted system
// config is a test of internal/gitsafe.
const systemScript = `
step norecord sh -c 'ls -A "$HOME/.local/state/julieta" | grep -c system-hooks-path || true'
mkdir -p /tmp/syshooks syshooks
cat >/tmp/syshooks/pre-commit <<'EOF2'
#!/bin/sh
echo "system pre-commit $*" >>/tmp/hooklog
[ ! -e /tmp/sysstop ] || { echo "the system hook says no" >&2; exit 4; }
EOF2
printf '#!/bin/sh\necho "relative pre-commit" >>/tmp/hooklog\n' >syshooks/pre-commit
printf '#!/bin/sh\nexit 0\n' >/tmp/syshooks/pre-push
printf '#!/bin/sh\ngrep -q refs/heads/main && { echo "the tracked pre-push saw main" >&2; exit 1; }\nexit 0\n' >.githooks/pre-push
mkdir -p /tmp/loop
cp "$HOME/.local/share/julieta/hooks/pre-commit" /tmp/loop/pre-commit
chmod +x /tmp/syshooks/pre-commit /tmp/syshooks/pre-push .githooks/pre-push syshooks/pre-commit
record() { printf '{"path":"%s"}\n' "$1" >"$HOME/.local/state/julieta/system-hooks-path.json"; }
`

// TestJulietaHooksSystemStage runs the system stage of the dispatcher
// in the workload (04 4.3, REQ-0408): setup records no system value
// where the system config sets none; with a system hooks path recorded,
// its pre-commit runs first, with the hook's arguments, and a failure
// there stops the chain, is git's failure and names the stage system; a
// recorded path that is relative, or that is julieta's own dispatcher
// dir, runs nothing in that stage and the chain goes on. A system
// pre-push that reads no stdin leaves the ref lines to the tracked one,
// which refuses the push, and a system pre-commit that is a copy of the
// dispatcher's script runs the chain once more without its system
// stage, then ends.
func TestJulietaHooksSystemStage(t *testing.T) {
	script := strings.Replace(hooksScript, "%s", manifestEnv(t, 1, originURL+"api.git"), 1) + systemScript + `
# setup moves no branch (REQ-0399): main is brought to origin's tip by
# hand, so the push at the end is a fast-forward.
git merge -q --ff-only origin/main
: >/tmp/hooklog
record /tmp/syshooks
step ok git commit -q --allow-empty -m first
step log cat /tmp/hooklog
: >/tmp/hooklog
touch /tmp/sysstop
step stopped git commit -q --allow-empty -m second
step stoppedlog cat /tmp/hooklog
step state sh -c 'cat "$HOME"/.local/state/julieta/hook-failures/*.json'
rm /tmp/sysstop
: >/tmp/hooklog
record syshooks
step relative git commit -q --allow-empty -m third
step relativelog cat /tmp/hooklog
: >/tmp/hooklog
record "$HOME/.local/share/julieta/hooks"
step own git commit -q --allow-empty -m fourth
step ownlog cat /tmp/hooklog
record /tmp/syshooks
step push git push -q /tmp/srv/api.git HEAD:refs/heads/main
: >/tmp/hooklog
record /tmp/loop
step loop git commit -q --allow-empty -m fifth
step looplog cat /tmp/hooklog
`
	steps := scenario(t, script)
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)
	assert.Equal(t, "0\n", step(t, steps, "norecord").stdout, "no system config, no record")

	r := step(t, steps, "ok")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "system pre-commit \ntracked pre-commit \nlocal post-commit\n", step(t, steps, "log").stdout,
		"the system hook first, then the repo's, through the dispatcher")

	r = step(t, steps, "stopped")
	assert.NotEqual(t, 0, r.code, "git stops the commit on the system hook's status")
	assert.Contains(t, r.stderr, "the system hook says no\n")
	assert.Contains(t, r.stderr, "julieta hooks: pre-commit stage system (/tmp/syshooks/pre-commit) exited 4\n")
	assert.Equal(t, "system pre-commit \n", step(t, steps, "stoppedlog").stdout, "the chain stopped at the system stage")
	state := step(t, steps, "state").stdout
	assert.Contains(t, state, `"stage":"system"`)
	assert.Contains(t, state, `"exitCode":4`)

	r = step(t, steps, "relative")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "tracked pre-commit \nlocal post-commit\n", step(t, steps, "relativelog").stdout, "a relative record runs nothing")

	r = step(t, steps, "own")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "tracked pre-commit \nlocal post-commit\n", step(t, steps, "ownlog").stdout,
		"julieta's own dispatcher dir is not a system stage, so the chain does not call itself")

	r = step(t, steps, "push")
	assert.NotEqual(t, 0, r.code, "the system pre-push read no stdin, and the tracked one still got the ref lines")
	assert.Contains(t, r.stderr, "the tracked pre-push saw main\n")
	assert.Contains(t, r.stderr, "stage tracked ("+primaryPath+"/.githooks/pre-push) exited 1\n")

	r = step(t, steps, "loop")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "tracked pre-commit \ntracked pre-commit \nlocal post-commit\n", step(t, steps, "looplog").stdout,
		"a system hook that runs the dispatcher again: the inner run skips the system stage, so the chain ends")
}
