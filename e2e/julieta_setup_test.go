// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/container"
)

// The sandbox the julieta cases build in the container, which has no
// network but its loopback: two bare origins, api and web, served by
// git daemon on 127.0.0.1 (the one git:// host gitsafe accepts), the
// primary api cloned at a path of its own and one commit behind
// origin's main, and the secondary web missing below $HOME.
const (
	sandboxHome  = "/tmp/home"
	primaryPath  = "/tmp/work/api"
	secondaryDir = sandboxHome + "/src/shop/web"
	originURL    = "git://127.0.0.1:9418/"
)

// emptyRunDigest is the digest of kind run of the empty run, {} (03
// 3.12), a literal since e2e may not import internal/canon (REQ-0971).
// TestEmptyRunDigest of internal/cmd/julieta/setup holds the same
// value; when canon changes it, "go test ./internal/cmd/julieta/setup
// -run '^TestEmptyRunDigest$'" fails and prints the new one.
const emptyRunDigest = "d33e4b621d0923ab86c327af8e7819ddba70d80740f12b580bf056748a01ff25"

// prologue builds the sandbox and starts the daemon. $1 is julieta.
const prologue = `set -eu
J=$1
shift
export HOME=` + sandboxHome + `
mkdir -p "$HOME" /tmp/srv /tmp/work
g() { git -c init.defaultBranch=main -c user.name=e2e -c user.email=e2e@example.invalid -c commit.gpgsign=false "$@"; }
for r in api web; do
	g init -q --bare /tmp/srv/$r.git
	g clone -q /tmp/srv/$r.git /tmp/seed-$r 2>/dev/null
	g -C /tmp/seed-$r commit -q --allow-empty -m one
	g -C /tmp/seed-$r push -q origin main
done
g clone -q /tmp/srv/api.git ` + primaryPath + `
g -C /tmp/seed-api commit -q --allow-empty -m two
g -C /tmp/seed-api push -q origin main
git daemon --listen=127.0.0.1 --port=9418 --export-all --base-path=/tmp/srv --reuseaddr --detach
i=0
until git ls-remote ` + originURL + `api.git >/dev/null 2>&1; do
	i=$((i+1))
	[ "$i" -lt 100 ] || { echo "git daemon did not answer" >&2; exit 97; }
	sleep 0.1
done
# step <name> <argv...> runs one command and prints its exit status,
# its stdout and its stderr between marker lines.
step() {
	name=$1
	shift
	set +e
	"$@" >/tmp/out 2>/tmp/err
	code=$?
	set -e
	echo "@@@ $name exit $code"
	cat /tmp/out
	echo "@@@ $name stderr"
	cat /tmp/err
}
`

// stepResult is what one step of a scenario printed.
type stepResult struct {
	code           int
	stdout, stderr string
}

// scenario runs the prologue, then script, in a container of the
// workload with the bin directory mounted, and returns each step's
// result by name. A container that fails, such as a prologue that
// cannot build the sandbox, fails the test with its output.
func scenario(t *testing.T, script string) map[string]stepResult {
	t.Helper()
	w := workload(t)
	out, err := w.Run(t.Context(), []container.Mount{container.Mount(w.Bin)}, "sh", "-c", prologue+script, "sh", w.Julieta)
	require.NoError(t, err, "%s", out)
	steps := map[string]stepResult{}
	var name string
	var res stepResult
	var cur *strings.Builder
	end := func() {
		if name != "" {
			res.stderr = cur.String()
			steps[name] = res
		}
	}
	for line := range strings.Lines(string(out)) {
		rest, ok := strings.CutPrefix(line, "@@@ ")
		if !ok {
			require.NotNil(t, cur, "output before the first step: %q", line)
			cur.WriteString(line)
			continue
		}
		switch f := strings.Fields(rest); {
		case len(f) == 3 && f[1] == "exit":
			end()
			code, err := strconv.Atoi(f[2])
			require.NoError(t, err, "marker %q", line)
			name, res, cur = f[0], stepResult{code: code}, new(strings.Builder)
		case len(f) == 2 && f[0] == name && f[1] == "stderr":
			res.stdout, cur = cur.String(), new(strings.Builder)
		default:
			require.FailNow(t, "a marker line out of place", "%q", line)
		}
	}
	end()
	return steps
}

// step returns the result of the step name, which must have run.
func step(t *testing.T, steps map[string]stepResult, name string) stepResult {
	t.Helper()
	r, ok := steps[name]
	require.True(t, ok, "the step %s did not run: %v", name, steps)
	return r
}

// manifestEnv returns the env value of the manifest of the sandbox, of
// the protocol given, with the primary's origin at apiURL: canonical
// JSON (sorted keys, no spacing), base64url without padding (03 3.6).
// runDigest is emptyRunDigest.
func manifestEnv(t *testing.T, protocol int, apiURL string) string {
	t.Helper()
	repo := func(dir, path, url string, primary bool) map[string]any {
		return map[string]any{"base": []string{}, "dir": dir, "memory": "/tmp/memory/" + dir, "path": path,
			"primary": primary, "ref": "main", "url": url}
	}
	doc := map[string]any{
		"agent":      "sbx-kit-claude",
		"egress":     []string{"dl.google.com", "github.com"},
		"generation": "01j9zc2q8x7w4t6y3r5e1v9k0p",
		"project":    "shop",
		"protocol":   protocol,
		"repos":      []any{repo("api", primaryPath, apiURL, true), repo("web", secondaryDir, originURL+"web.git", false)},
		"run":        map[string]any{},
		"runDigest":  emptyRunDigest,
		"salvage":    map[string]any{"capBytes": 0, "excludeIgnored": []string{}},
		"schema":     "manifest/v1",
		"specCommit": strings.Repeat("ab", 20),
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(doc))
	return base64.RawURLEncoding.EncodeToString(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
}

// errorID returns the id of the --json error report a step printed on
// stderr.
func errorID(t *testing.T, r stepResult) string {
	t.Helper()
	var e struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(r.stderr), &e), "stderr %q", r.stderr)
	return e.ID
}

// setupJSON is the --json document of julieta setup, every field, read
// strictly.
type setupJSON struct {
	Errors []string `json:"errors"`
	Link   struct {
		Changed bool   `json:"changed"`
		Path    string `json:"path"`
		Target  string `json:"target"`
	} `json:"link"`
	Repos []struct {
		Cloned        bool   `json:"cloned"`
		DefaultBranch string `json:"defaultBranch"`
		Dir           string `json:"dir"`
		Error         string `json:"error"`
		FastForward   string `json:"fastForward"`
		Fetched       bool   `json:"fetched"`
		From          string `json:"from"`
		To            string `json:"to"`
		Path          string `json:"path"`
	} `json:"repos"`
	Schema   string        `json:"schema"`
	Unusable []string      `json:"unusable"`
	Version  string        `json:"version"`
	Warnings []warningJSON `json:"warnings"`
}

// warningJSON is one warning of julieta's --json documents.
type warningJSON struct {
	Message string `json:"message"`
	Slug    string `json:"slug"`
}

// decodeStrict decodes data into v, refusing an unknown member and
// trailing data.
func decodeStrict(t *testing.T, data string, v any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(v), "%q", data)
	assert.False(t, dec.More(), "one document: %q", data)
}

// TestJulietaSetup runs julieta setup in the workload (04 4.3): a
// manifest of protocol N is accepted, and one of protocol 0 and one of
// N+1 are refused with manifest-protocol (RJ-342); the accepted run
// links $HOME/.local/bin/julieta to the binary, clones the missing
// secondary and fast-forwards the primary's clean default branch,
// warning default-branch-moved; a second run changes nothing; a fetch
// that fails is the warning fetch-failed and exit 0.
func TestJulietaSetup(t *testing.T) {
	const n = 1
	steps := scenario(t, `
step p0 env JULIETA_MANIFEST=`+manifestEnv(t, 0, originURL+"api.git")+` "$J" setup --json
step p2 env JULIETA_MANIFEST=`+manifestEnv(t, n+1, originURL+"api.git")+` "$J" setup --json
old=$(git -C `+primaryPath+` rev-parse HEAD)
step p1 env JULIETA_MANIFEST=`+manifestEnv(t, n, originURL+"api.git")+` "$J" setup --json
step heads sh -c 'echo "$1"; git -C `+primaryPath+` rev-parse HEAD; git -C /tmp/srv/api.git rev-parse main; readlink "$HOME/.local/bin/julieta"; git -C `+secondaryDir+` rev-parse HEAD' sh "$old"
step again env JULIETA_MANIFEST=`+manifestEnv(t, n, originURL+"api.git")+` "$J" setup --json
step fetch env JULIETA_MANIFEST=`+manifestEnv(t, n, originURL+"gone.git")+` "$J" setup --json
`)
	for _, p := range []string{"p0", "p2"} {
		r := step(t, steps, p)
		assert.Equal(t, 2, r.code, "protocol %s: %s", p, r.stderr)
		assert.Empty(t, r.stdout, p)
		assert.Equal(t, "RJ-342", errorID(t, r), p)
	}

	heads := strings.Fields(step(t, steps, "heads").stdout)
	require.Len(t, heads, 5)
	old, head, tip, link := heads[0], heads[1], heads[2], heads[3]
	w := workload(t)
	r := step(t, steps, "p1")
	require.Equal(t, 0, r.code, r.stderr)
	var rep setupJSON
	decodeStrict(t, r.stdout, &rep)
	assert.Equal(t, "julieta-setup/v1", rep.Schema)
	assert.True(t, rep.Link.Changed)
	assert.Equal(t, sandboxHome+"/.local/bin/julieta", rep.Link.Path)
	assert.Equal(t, w.Julieta, rep.Link.Target)
	assert.Equal(t, w.Julieta, link, "the link points at the running binary")
	assert.Empty(t, rep.Unusable)
	require.Len(t, rep.Repos, 2)
	assert.Equal(t, "moved", rep.Repos[0].FastForward)
	assert.Equal(t, old, rep.Repos[0].From)
	assert.Equal(t, tip, rep.Repos[0].To)
	assert.Equal(t, tip, head, "the primary's main is at origin's")
	assert.NotEqual(t, old, tip)
	assert.True(t, rep.Repos[1].Cloned, "the missing secondary is cloned")
	assert.Equal(t, []warningJSON{{Slug: "default-branch-moved", Message: "api " + old + ".." + tip}}, rep.Warnings)
	assert.Equal(t, "warning default-branch-moved: api "+old+".."+tip+"\n", r.stderr)

	r = step(t, steps, "again")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Empty(t, r.stderr)
	rep = setupJSON{}
	decodeStrict(t, r.stdout, &rep)
	assert.False(t, rep.Link.Changed)
	for _, repo := range rep.Repos {
		assert.Equal(t, "current", repo.FastForward, repo.Dir)
		assert.False(t, repo.Cloned, repo.Dir)
	}

	r = step(t, steps, "fetch")
	require.Equal(t, 0, r.code, "a failed fetch is a warning: %s", r.stderr)
	rep = setupJSON{}
	decodeStrict(t, r.stdout, &rep)
	require.Len(t, rep.Warnings, 1)
	assert.Equal(t, "fetch-failed", rep.Warnings[0].Slug)
	assert.True(t, strings.HasPrefix(rep.Warnings[0].Message, "api: "), rep.Warnings[0].Message)
	assert.Contains(t, r.stderr, "warning fetch-failed: api: ")
}
