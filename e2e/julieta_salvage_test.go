// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"encoding/base64"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The salvage the cases ask for: the id romeu would pass, and the dir
// of the primary's salvage.
const (
	salvageGeneration = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	salvageRunID      = "01jb000000000000000000000a"
	salvageDir        = "/tmp/memory/api/salvage/" + salvageGeneration + "/" + salvageRunID
	salvagePrefix     = "refs/salvage/" + salvageGeneration + "/" + salvageRunID + "/"
)

// salvageManifestJSON is salvage.v1 (03 3.11), every field, read
// strictly.
type salvageManifestJSON struct {
	AgentStop *struct {
		Killed  int `json:"killed"`
		Matched int `json:"matched"`
	} `json:"agentStop"`
	Bundles []struct {
		File   string `json:"file"`
		Nested string `json:"nested"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	} `json:"bundles"`
	Complete bool `json:"complete"`
	Files    []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	} `json:"files"`
	Generation string `json:"generation"`
	Refs       []struct {
		Nested string `json:"nested"`
		Ref    string `json:"ref"`
		SHA    string `json:"sha"`
	} `json:"refs"`
	Repo      string `json:"repo"`
	SalvageID string `json:"salvageId"`
	Schema    string `json:"schema"`
	Skipped   []struct {
		Detail string `json:"detail"`
		Item   string `json:"item"`
		Reason string `json:"reason"`
	} `json:"skipped"`
	Worktrees []struct {
		Branch   string `json:"branch"`
		Capture  string `json:"capture"`
		Detached bool   `json:"detached"`
		Dirty    bool   `json:"dirty"`
		Head     string `json:"head"`
		Location string `json:"location"`
		Nested   string `json:"nested"`
		Path     string `json:"path"`
	} `json:"worktrees"`
}

// reasons returns the skipped items of m as "reason item".
func (m salvageManifestJSON) reasons() []string {
	out := []string{}
	for _, s := range m.Skipped {
		out = append(out, s.Reason+" "+s.Item)
	}
	return out
}

// salvageReportJSON is julieta-salvage.v1, every field, read strictly;
// the fingerprint is decoded as any JSON object, to compare with what
// --fingerprint prints.
type salvageReportJSON struct {
	Fingerprint map[string]any `json:"fingerprint"`
	Manifests   []struct {
		Complete bool   `json:"complete"`
		Path     string `json:"path"`
		Repo     string `json:"repo"`
		Skipped  int    `json:"skipped"`
	} `json:"manifests"`
	SalvageID     string `json:"salvageId"`
	Schema        string `json:"schema"`
	SecretBearing bool   `json:"secretBearing"`
}

// salvageOptions are the members of the manifest a case sets.
type salvageOptions struct {
	agent    string
	capBytes int64
	exclude  []string
}

// salvageManifestEnv returns the env value of the manifest of the
// sandbox with a salvageRun, as romeu passes it to the sandbox half:
// manifestEnv's, with the agent, the cap and the exclusions of o.
func salvageManifestEnv(t *testing.T, o salvageOptions) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(manifestEnv(t, 1, originURL+"api.git"))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	if o.agent != "" {
		doc["agent"] = o.agent
	}
	if o.exclude == nil {
		o.exclude = []string{}
	}
	doc["salvage"] = map[string]any{"capBytes": o.capBytes, "excludeIgnored": o.exclude}
	doc["salvageRun"] = map[string]any{"id": salvageRunID}
	b, err := json.Marshal(doc, json.Deterministic(true))
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// salvageScript sets up the sandbox for a salvage case: setup clones
// the secondary, git gets an identity, and every commit of julieta's
// own would be signed by a gpg that fails, so a signature attempt is a
// failure (I18). g, of the prologue, commits without signing.
const salvageScript = `
export JULIETA_MANIFEST=%s
step setup "$J" setup
git config --global user.name e2e
git config --global user.email e2e@example.invalid
git config --global commit.gpgsign true
git config --global gpg.program /bin/false
cd ` + primaryPath + `
`

// salvageScenario returns the scenario of a salvage case: salvageScript
// with the manifest of o, then body.
func salvageScenario(t *testing.T, o salvageOptions, body string) map[string]stepResult {
	t.Helper()
	return scenario(t, strings.Replace(salvageScript, "%s", salvageManifestEnv(t, o), 1)+body)
}

// salvageManifest decodes the manifest a step printed.
func salvageManifest(t *testing.T, r stepResult) salvageManifestJSON {
	t.Helper()
	require.Equal(t, 0, r.code, r.stderr)
	var m salvageManifestJSON
	decodeStrict(t, r.stdout, &m)
	assert.Equal(t, "salvage/v1", m.Schema)
	assert.Equal(t, salvageRunID, m.SalvageID)
	return m
}

// TestJulietaSalvage runs the sandbox half in the workload (08 8.5; 05
// 5.2, I17 end-to-end cases, the ones the sandbox half reaches; 10
// 10.1): the bundle holds a hidden branch, a reflog-only commit, a
// detached HEAD commit, the capture of the dirt in a linked worktree,
// a tag and a stash, and leaves out a salvage ref of an earlier
// generation; a nested repository gets its own capture, stash and
// bundle; an untracked file over the cap is over-cap and out of the
// capture (untracked files count against the salvage cap); the capture
// commits are julieta-salvage's and unsigned although commit.gpgsign
// is set (I18), and the worktree and its index are unchanged; the dirs
// are 0700 and the files 0600, and julieta memory check passes;
// --fingerprint writes nothing and prints the object the --json
// document carries.
func TestJulietaSalvage(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{capBytes: 100}, `
g checkout -q -b hidden
g commit -q --allow-empty -m hidden
g checkout -q main
g commit -q --allow-empty -m before-amend
g commit -q --amend --allow-empty -m amended
g tag v1
g checkout -q --detach
g commit -q --allow-empty -m detached
detached=$(git rev-parse HEAD)
g checkout -q main
g update-ref refs/salvage/01j9zb0000000000000000000a/old HEAD
g worktree add -q /tmp/wt hidden
echo linked >/tmp/wt/linked
echo stash >stashed
g add stashed
g stash -q
echo small >small
head -c 200 /dev/zero >large
mkdir -p vendor/lib
g -C vendor/lib init -q
g -C vendor/lib commit -q --allow-empty -m nested
echo n >vendor/lib/n
g -C vendor/lib add n
g -C vendor/lib stash -q
echo nested-dirt >vendor/lib/dirt
step detached echo "$detached"
before=$(git status --porcelain=v2 --untracked-files=all; sha256sum .git/index)
step fingerprint "$J" salvage --fingerprint
step nothing sh -c 'ls -A /tmp/memory/api | grep -c salvage || true'
step salvage "$J" salvage --json
step same sh -c '[ "$(git status --porcelain=v2 --untracked-files=all; sha256sum .git/index)" = "$1" ] && echo same' sh "$before"
step manifest cat ` + salvageDir + `/manifest.json
step heads git bundle list-heads ` + salvageDir + `/repo.bundle
step verify git bundle verify -q ` + salvageDir + `/repo.bundle
step author git log -1 --format='%an %ae' ` + salvagePrefix + `worktree-0
step unsigned sh -c 'git cat-file commit ` + salvagePrefix + `worktree-0 | grep -c gpgsig || true'
step linked git show ` + salvagePrefix + `worktree-1:linked
step captured git ls-tree --name-only ` + salvagePrefix + `worktree-0
step nested git -C vendor/lib show ` + salvagePrefix + `worktree-0:dirt
step nestedstash git -C vendor/lib rev-parse ` + salvagePrefix + `stash-0
step modes find /tmp/memory/api/salvage -printf '%m %y %P\n'
step check "$J" memory check
`)
	r := step(t, steps, "fingerprint")
	require.Equal(t, 0, r.code, r.stderr)
	var fp map[string]any
	decodeStrict(t, r.stdout, &fp)
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "nothing").stdout), "--fingerprint writes nothing")

	r = step(t, steps, "salvage")
	require.Equal(t, 0, r.code, r.stderr)
	var rep salvageReportJSON
	decodeStrict(t, r.stdout, &rep)
	assert.Equal(t, "julieta-salvage/v1", rep.Schema)
	assert.Equal(t, fp, rep.Fingerprint, "the --json document carries the object --fingerprint prints")
	assert.Equal(t, "same\n", step(t, steps, "same").stdout, "the worktree and the index are unchanged")

	m := salvageManifest(t, step(t, steps, "manifest"))
	assert.Equal(t, "api", m.Repo)
	assert.Nil(t, m.AgentStop, "no --stop-agents")
	assert.Equal(t, []string{"over-cap worktree-0/large"}, m.reasons(), "untracked files count against the salvage cap")
	assert.False(t, m.Complete)

	heads := step(t, steps, "heads").stdout
	for _, ref := range []string{"refs/heads/main", "refs/heads/hidden", "refs/tags/v1", "refs/salvage-reflog/main/1",
		salvagePrefix + "worktree-0", salvagePrefix + "worktree-1", salvagePrefix + "stash-0"} {
		assert.Contains(t, heads, " "+ref+"\n", "the bundle holds %s", ref)
	}
	assert.Contains(t, heads, strings.TrimSpace(step(t, steps, "detached").stdout)+" ", "the detached HEAD's commit")
	assert.NotContains(t, heads, "refs/salvage/01j9zb", "no salvage ref of an earlier generation")
	assert.Equal(t, 0, step(t, steps, "verify").code, step(t, steps, "verify").stderr)

	assert.Equal(t, "julieta-salvage julieta-salvage@localhost\n", step(t, steps, "author").stdout)
	assert.Equal(t, "0\n", step(t, steps, "unsigned").stdout, "no signature with commit.gpgsign=true (I18)")
	assert.Equal(t, "linked\n", step(t, steps, "linked").stdout, "the dirt in the linked worktree")
	captured := step(t, steps, "captured").stdout
	assert.Contains(t, captured, "small\n")
	assert.NotContains(t, captured, "large\n", "over the cap")
	assert.NotContains(t, captured, "stashed\n", "the stash is its own ref")
	assert.Equal(t, "nested-dirt\n", step(t, steps, "nested").stdout, "a nested repository gets the dirt capture")
	assert.Equal(t, 0, step(t, steps, "nestedstash").code, "and the stash capture: %s", step(t, steps, "nestedstash").stderr)
	var nestedBundle bool
	for _, b := range m.Bundles {
		nestedBundle = nestedBundle || b.Nested == "worktree-0/vendor/lib"
	}
	assert.True(t, nestedBundle, "a nested repository gets its own bundle: %+v", m.Bundles)

	for line := range strings.Lines(step(t, steps, "modes").stdout) {
		f := strings.Fields(line)
		require.Len(t, f, 3, line)
		if f[1] == "d" {
			assert.Equal(t, "700", f[0], "dir %s", f[2])
		} else {
			assert.Equal(t, "600", f[0], "file %s", f[2])
		}
	}
	assert.Equal(t, 0, step(t, steps, "check").code, "the salvage keeps the layout: %s", step(t, steps, "check").stdout)
}

// TestJulietaSalvageSkips: a rebase in progress skips its worktree as
// in-progress-operation; ignored files go into ignored.tar.gz, salvage.excludeIgnored
// excluded-by-spec; a salvage dir without manifest.json, which an
// interrupted salvage leaves, is kept by the next salvage.
func TestJulietaSalvageSkips(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{capBytes: 1 << 20, exclude: []string{"cache/"}}, `
echo '*.log' >.gitignore
echo 'cache/' >>.gitignore
g add .gitignore
g commit -q -m ignore
echo log >a.log
mkdir cache
echo cached >cache/x
g worktree add -q /tmp/rebasing -b r1
cd /tmp/rebasing
echo one >f
g add f
g commit -q -m one
g checkout -q -b r2 HEAD~1
echo two >f
g add f
g commit -q -m two
set +e
g rebase -q r1 >/dev/null 2>&1
set -e
cd ` + primaryPath + `
mkdir -p /tmp/memory/api/salvage/` + salvageGeneration + `/01jaz00000000000000000000a
step salvage "$J" salvage
step manifest cat ` + salvageDir + `/manifest.json
step archive tar -tzf ` + salvageDir + `/ignored.tar.gz
step kept ls -A /tmp/memory/api/salvage/` + salvageGeneration + `
`)
	r := step(t, steps, "salvage")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Contains(t, r.stdout, "Salvage payloads are secret-bearing")
	m := salvageManifest(t, step(t, steps, "manifest"))
	assert.Contains(t, m.reasons(), "in-progress-operation worktree-1")
	assert.Contains(t, m.reasons(), "excluded-by-spec worktree-0/cache")
	assert.Equal(t, "worktree-0/a.log\n", step(t, steps, "archive").stdout)
	assert.Equal(t, "01jaz00000000000000000000a\n"+salvageRunID+"\n", step(t, steps, "kept").stdout, "the dir without a manifest is kept")
}

// TestJulietaSalvageDiskFull: a full disk while step 2 writes the
// capture's objects is the skipped item disk-full (REQ-0703): the
// primary's working tree is on a tmpfs that a filler file has filled.
func TestJulietaSalvageDiskFull(t *testing.T) {
	steps := scenario(t, `
export JULIETA_MANIFEST=`+diskFullManifest(t)+`
g clone -q /tmp/srv/api.git /dev/shm/api
cd /dev/shm/api
echo dirt >dirt
dd if=/dev/zero of=/dev/shm/filler bs=64k 2>/dev/null || true
step salvage "$J" salvage
rm -f /dev/shm/filler
step manifest cat `+salvageDir+`/manifest.json
`)
	r := step(t, steps, "salvage")
	require.Equal(t, 0, r.code, r.stderr)
	m := salvageManifest(t, step(t, steps, "manifest"))
	assert.Equal(t, []string{"disk-full worktree-0"}, m.reasons())
	assert.False(t, m.Complete)
}

// diskFullManifest returns the manifest of TestJulietaSalvageDiskFull:
// one repo, the primary, at /dev/shm/api.
func diskFullManifest(t *testing.T) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(salvageManifestEnv(t, salvageOptions{capBytes: 1 << 20}))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	repos := doc["repos"].([]any)
	api := repos[0].(map[string]any)
	api["path"] = "/dev/shm/api"
	doc["repos"] = []any{api}
	b, err := json.Marshal(doc, json.Deterministic(true))
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// agentScript makes the agent's required state paths below $HOME, with
// a memory file, a todo and a transcript, and the julieta state record
// of the hooks with a SessionStart at $start and a SessionEnd at $end.
const agentScript = `
mkdir -p "$HOME/.claude/projects/p/memory" "$HOME/.claude/todos" "$HOME/.local/state/julieta"
echo memory >"$HOME/.claude/projects/p/memory/MEMORY.md"
echo '{}' >"$HOME/.claude/projects/p/s.jsonl"
echo '[]' >"$HOME/.claude/todos/t.json"
hooks() { printf '{"sessionEndAt":"%s","sessionStartAt":"%s"}' "$2" "$1" >"$HOME/.local/state/julieta/hooks.json"; }
`

// TestJulietaSalvageStopAgents: a fixture process with the agent's
// name that ignores SIGTERM is killed after the wait, and the
// manifest's agentStop records it (10 10.1); the agent's memory files
// and todos go into agent/claude.tar.gz, the transcript only with
// --include-transcripts and transcript-excluded otherwise.
func TestJulietaSalvageStopAgents(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{agent: "claude", capBytes: 1 << 20}, agentScript+`
printf '#!/bin/sh\ntrap "" TERM\nwhile :; do sleep 1; done\n' >/tmp/claude
chmod 700 /tmp/claude
/tmp/claude &
sleep 0.5
step comm sh -c 'cat /proc/'$!'/comm'
start=$(date +%s)
step salvage "$J" salvage --stop-agents
step waited sh -c 'echo $(( $(date +%s) - $1 ))' sh "$start"
step gone sh -c 'kill -0 '$!' 2>/dev/null && echo alive || echo gone'
step manifest cat `+salvageDir+`/manifest.json
step agent tar -tzf `+salvageDir+`/agent/claude.tar.gz
`)
	assert.Equal(t, "claude\n", step(t, steps, "comm").stdout)
	r := step(t, steps, "salvage")
	require.Equal(t, 0, r.code, r.stderr)
	m := salvageManifest(t, step(t, steps, "manifest"))
	require.NotNil(t, m.AgentStop)
	assert.Equal(t, 1, m.AgentStop.Matched)
	assert.Equal(t, 1, m.AgentStop.Killed, "it ignored SIGTERM")
	assert.GreaterOrEqual(t, strings.TrimSpace(step(t, steps, "waited").stdout), "10", "SIGKILL after the 10 s wait")
	assert.Contains(t, step(t, steps, "gone").stdout, "gone")
	assert.Equal(t, []string{"transcript-excluded .claude/projects/p/s.jsonl"}, m.reasons())
	assert.True(t, m.Complete)
	assert.Equal(t, ".claude/projects/p/memory/MEMORY.md\n.claude/todos/t.json\n", step(t, steps, "agent").stdout)
}

// TestJulietaSalvageAgentNotMatched: the two agent-not-matched cases
// (REQ-0746): no agent process and a SessionStart without a later
// SessionEnd skips agent-not-matched; with a later SessionEnd, zero is
// recorded and nothing is skipped. A missing required profile path is
// agent-path-missing, a missing optional one no item; with
// --include-transcripts the transcript is kept.
func TestJulietaSalvageAgentNotMatched(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{agent: "claude", capBytes: 1 << 20}, agentScript+`
hooks 2026-10-29T10:00:00Z 2026-10-29T09:00:00Z
step open "$J" salvage --stop-agents --include-transcripts
step openmanifest cat `+salvageDir+`/manifest.json
step agent tar -tzf `+salvageDir+`/agent/claude.tar.gz
`)
	r := step(t, steps, "open")
	require.Equal(t, 0, r.code, r.stderr)
	m := salvageManifest(t, step(t, steps, "openmanifest"))
	require.NotNil(t, m.AgentStop)
	assert.Equal(t, 0, m.AgentStop.Matched)
	assert.Equal(t, []string{"agent-not-matched claude"}, m.reasons())
	assert.Contains(t, step(t, steps, "agent").stdout, ".claude/projects/p/s.jsonl\n", "kept with --include-transcripts")

	steps = salvageScenario(t, salvageOptions{agent: "claude", capBytes: 1 << 20}, agentScript+`
rm -r "$HOME/.claude/todos"
hooks 2026-10-29T09:00:00Z 2026-10-29T10:00:00Z
step ended "$J" salvage --stop-agents
step endedmanifest cat `+salvageDir+`/manifest.json
`)
	r = step(t, steps, "ended")
	require.Equal(t, 0, r.code, r.stderr)
	m = salvageManifest(t, step(t, steps, "endedmanifest"))
	require.NotNil(t, m.AgentStop)
	assert.Equal(t, 0, m.AgentStop.Matched)
	assert.Equal(t, []string{"agent-path-missing .claude/todos", "transcript-excluded .claude/projects/p/s.jsonl"}, m.reasons(),
		"no agent-not-matched; the optional .claude/plans is no item")
}

// TestJulietaSalvageWaitsOnTheStoreLocks: after --stop-agents, salvage
// waits on the store lock a stamp holds; while it waits more than 10 s
// it prints the lock's path in its --json progress on stderr; the
// stamp written while it waits is in the dir and in no manifest.
func TestJulietaSalvageWaitsOnTheStoreLocks(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{capBytes: 1 << 20}, `
lock=$HOME/.local/state/julieta/locks/_shared.lock
mkdir -p "$(dirname "$lock")" /tmp/memory/_shared/stamps
flock "$lock" sh -c 'sleep 12; echo stamp >/tmp/memory/_shared/stamps/01jb0000000000000000000000.json' &
sleep 0.5
step salvage "$J" salvage --stop-agents --json
step stamp cat /tmp/memory/_shared/stamps/01jb0000000000000000000000.json
step manifest cat `+salvageDir+`/manifest.json
`)
	r := step(t, steps, "salvage")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, `{"path":"/tmp/home/.local/state/julieta/locks/_shared.lock","progress":"lock-wait"}`+"\n", r.stderr)
	assert.Equal(t, "stamp\n", step(t, steps, "stamp").stdout, "the stamp finished first")
	m := salvageManifest(t, step(t, steps, "manifest"))
	for _, f := range m.Files {
		assert.NotContains(t, f.Path, "stamp")
	}
	assert.Empty(t, m.reasons())
}

// TestJulietaSalvageFingerprintMoves: the fingerprint of a rerun differs
// after an uncommitted edit, after a dirty file is edited again, and
// after an ignored file outside salvage.excludeIgnored changes size or
// modification time (05 5.2, I17 end-to-end cases 7, 8 and 10, the
// sandbox side); it is the same when nothing changed, or when only an
// excluded path did.
func TestJulietaSalvageFingerprintMoves(t *testing.T) {
	steps := salvageScenario(t, salvageOptions{capBytes: 1 << 20, exclude: []string{"cache/"}}, `
echo '*.log' >.gitignore
echo 'cache/' >>.gitignore
g add .gitignore
g commit -q -m ignore
echo log >a.log
step f0 "$J" salvage --fingerprint
step f1 "$J" salvage --fingerprint
mkdir -p cache
echo c >cache/x
step f2 "$J" salvage --fingerprint
echo edit >>.gitignore
step f3 "$J" salvage --fingerprint
echo again >>.gitignore
step f4 "$J" salvage --fingerprint
touch -d 2020-01-01 a.log
step f5 "$J" salvage --fingerprint
echo longer >a.log
step f6 "$J" salvage --fingerprint
`)
	var prints []string
	for _, name := range []string{"f0", "f1", "f2", "f3", "f4", "f5", "f6"} {
		r := step(t, steps, name)
		require.Equal(t, 0, r.code, r.stderr)
		prints = append(prints, r.stdout)
	}
	assert.Equal(t, prints[0], prints[1], "nothing changed")
	assert.Equal(t, prints[1], prints[2], "only an excluded path changed")
	assert.NotEqual(t, prints[2], prints[3], "an uncommitted edit")
	assert.NotEqual(t, prints[3], prints[4], "a dirty file edited again")
	assert.NotEqual(t, prints[4], prints[5], "an ignored file's modification time")
	assert.NotEqual(t, prints[5], prints[6], "an ignored file's size")
}
