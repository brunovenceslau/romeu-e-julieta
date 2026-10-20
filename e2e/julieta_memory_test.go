// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryJSON is the --json document of the julieta memory commands,
// every field, read strictly.
type memoryJSON struct {
	Entries []struct {
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
		ID        string    `json:"id"`
		Path      string    `json:"path"`
		Status    string    `json:"status"`
		Tags      []string  `json:"tags"`
		Title     string    `json:"title"`
		UpdatedAt time.Time `json:"updatedAt"`
	} `json:"entries"`
	Repo   string `json:"repo"`
	Schema string `json:"schema"`
	Wrote  bool   `json:"wrote"`
}

// ulidForm is a lowercase ULID (03 3.12).
var ulidForm = regexp.MustCompile(`^[0-9a-hjkmnp-tv-z]{26}$`)

// TestJulietaMemory runs the memory commands in the workload (04 4.3;
// 10 10.1, memory: stamping, allowlist): after setup, add stamps the id
// and both times and prints the id, the path and the repo; a supplied
// id is the error usage; the same content again writes nothing; the
// entry is 0600 in a dir 0700 and the store lock is outside the memory
// dir; two julieta processes writing one store lose no entry; list
// --query filters and edit --status done closes; memory check passes,
// and fails on a symlink, a FIFO, a dotfile and a .git directory,
// never following them, and on a memory dir that is not mounted.
func TestJulietaMemory(t *testing.T) {
	steps := scenario(t, `
export JULIETA_MANIFEST=`+manifestEnv(t, 1, originURL+"api.git")+`
step setup "$J" setup
cd `+primaryPath+`
step add "$J" memory add --title "Flaky test on arm64" --body "On the arm64 runner." --tag ci --json
step stamp "$J" memory add --title x --id 01j9z3k8m6t2q4w5e7r9y1v3k5 --json
step dup "$J" memory add --title "Flaky test on arm64" --body "On the arm64 runner." --tag ci --json
step modes sh -c 'stat -c "%a %n" /tmp/memory/api/entries /tmp/memory/api/entries/*; ls -A /tmp/memory/api; stat -c "%a" "$HOME/.local/state/julieta/locks/api.lock"'
step adds sh -c 'i=0; failed=0
while [ "$i" -lt 20 ]; do
	"$1" memory add --title "a$i" >/dev/null & a=$!
	"$1" memory add --title "b$i" >/dev/null & b=$!
	wait "$a" || failed=$((failed+1))
	wait "$b" || failed=$((failed+1))
	i=$((i+1))
done
exit "$failed"' sh "$J"
step count sh -c '"$1" memory list | wc -l' sh "$J"
step query "$J" memory list --query ARM64 --json
id=$("$J" memory list --query arm64 --json | sed 's/.*"id":"\([0-9a-z]*\)".*/\1/')
step done "$J" memory edit "$id" --status done --json
step open sh -c '"$1" memory list --status open | wc -l' sh "$J"
step rm "$J" memory rm "$id"
step check "$J" memory check
ln -s /etc /tmp/memory/api/handoff
mkfifo /tmp/memory/api/entries/fifo
touch /tmp/memory/api/entries/.hidden
mkdir -p /tmp/memory/web/snapshot/.git
step findings "$J" memory check
rm -rf /tmp/memory/web
step unmounted "$J" memory check
step setup-unusable "$J" setup --json
`)
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)

	r := step(t, steps, "add")
	require.Equal(t, 0, r.code, r.stderr)
	var rep memoryJSON
	decodeStrict(t, r.stdout, &rep)
	assert.Equal(t, "julieta-memory/v1", rep.Schema)
	assert.Equal(t, "api", rep.Repo)
	assert.True(t, rep.Wrote)
	require.Len(t, rep.Entries, 1)
	e := rep.Entries[0]
	assert.Regexp(t, ulidForm, e.ID)
	assert.Equal(t, "/tmp/memory/api/entries/"+e.ID+".md", e.Path)
	assert.Equal(t, e.CreatedAt, e.UpdatedAt)
	assert.WithinDuration(t, time.Now(), e.CreatedAt, time.Hour)
	assert.Equal(t, []string{"ci"}, e.Tags)

	r = step(t, steps, "stamp")
	assert.Equal(t, 2, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stderr, "error RJ-349 usage: unknown flag: --id\n"), r.stderr)

	r = step(t, steps, "dup")
	require.Equal(t, 0, r.code, r.stderr)
	var dup memoryJSON
	decodeStrict(t, r.stdout, &dup)
	assert.False(t, dup.Wrote)
	assert.Equal(t, rep.Entries, dup.Entries)

	r = step(t, steps, "modes")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "700 /tmp/memory/api/entries\n600 "+e.Path+"\nentries\n600\n", r.stdout,
		"one entry file, 0600 in a dir 0700, and the store lock outside the memory dir")

	r = step(t, steps, "adds")
	require.Equal(t, 0, r.code, "the number of the 40 adds, two at a time, that failed; their stderr:\n%s", r.stderr)
	assert.Empty(t, r.stderr, "the 40 adds, two at a time")

	r = step(t, steps, "count")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "41", strings.TrimSpace(r.stdout), "two julieta processes at a time lose no entry")

	r = step(t, steps, "query")
	require.Equal(t, 0, r.code, r.stderr)
	var q memoryJSON
	decodeStrict(t, r.stdout, &q)
	assert.Equal(t, rep.Entries, q.Entries)

	r = step(t, steps, "done")
	require.Equal(t, 0, r.code, r.stderr)
	var done memoryJSON
	decodeStrict(t, r.stdout, &done)
	require.Len(t, done.Entries, 1)
	assert.Equal(t, "done", done.Entries[0].Status)
	assert.Equal(t, e.CreatedAt, done.Entries[0].CreatedAt)
	assert.Equal(t, "40", strings.TrimSpace(step(t, steps, "open").stdout))
	assert.Equal(t, 2, step(t, steps, "rm").code, "there is no memory rm")

	r = step(t, steps, "check")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "memory api /tmp/memory/api ok\nmemory web /tmp/memory/web ok\n", r.stdout)

	r = step(t, steps, "findings")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stderr, "error RJ-328 check-findings: "), r.stderr)
	assert.Equal(t, "memory api /tmp/memory/api 3 findings\n"+
		"finding dotfile /tmp/memory/api/entries/.hidden\n"+
		"finding fifo /tmp/memory/api/entries/fifo\n"+
		"finding symlink /tmp/memory/api/handoff\n"+
		"memory web /tmp/memory/web 1 findings\n"+
		"finding git /tmp/memory/web/snapshot/.git\n", r.stdout)

	r = step(t, steps, "unmounted")
	assert.Equal(t, 1, r.code, r.stderr)
	assert.Contains(t, r.stdout, "memory web /tmp/memory/web error: memstore: the memory dir is not mounted")

	r = step(t, steps, "setup-unusable")
	assert.Equal(t, 1, r.code, r.stderr)
	var sr setupJSON
	decodeStrict(t, r.stdout, &sr)
	require.Len(t, sr.Unusable, 2, sr.Unusable)
	assert.True(t, strings.HasPrefix(sr.Unusable[0], "memory check of api: "), sr.Unusable[0])
	assert.True(t, strings.HasPrefix(sr.Unusable[1], "memory check of web: "), sr.Unusable[1])
}
