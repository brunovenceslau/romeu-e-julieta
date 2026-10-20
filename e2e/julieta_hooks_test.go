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

// hooksScript builds the sandbox of the hooks cases: after setup, git
// points at the dispatcher julieta wrote and the primary repo has a
// tracked pre-commit and post-rewrite and a local post-commit, each
// logging to /tmp/hooklog.
//
// The line that sets the global core.hooksPath stands in for the half
// of setup that needs a verb of gitsafe (hooks_path.go of the setup
// package): until it lands, the case sets it the way that step will.
const hooksScript = `
export JULIETA_MANIFEST=%s
step setup "$J" setup
git config --global user.name e2e
git config --global user.email e2e@example.invalid
git config --global commit.gpgsign false
git config --global core.hooksPath "$HOME/.local/share/julieta/hooks"
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
// post-rewrite reaches the tracked hook alone; the first non-zero
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

	r := step(t, steps, "ok")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "tracked pre-commit \nlocal post-commit\n", step(t, steps, "log").stdout,
		".githooks/pre-commit, then $GIT_DIR/hooks/post-commit, through the dispatcher")

	assert.Equal(t, 0, step(t, steps, "amend").code, step(t, steps, "amend").stderr)
	rewrite := step(t, steps, "rewrite").stdout
	assert.Contains(t, rewrite, "tracked post-rewrite amend stdin=")
	assert.Regexp(t, `tracked post-rewrite amend stdin=[0-9a-f]{40} [0-9a-f]{40}\n`, rewrite, "the tracked hook reads git's stdin")
	assert.Contains(t, rewrite, "local post-rewrite amend stdin=\n", "the local hook gets none: the tracked one took it")

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
