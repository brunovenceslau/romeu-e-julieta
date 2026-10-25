// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The julieta-claude kit (06 6.4), read from kits/julieta-claude/: its
// descriptor, and its install steps run in the workload's base as the
// smoke test of 10 10.1, as kit_julieta_test.go runs those of the
// julieta kit. Then the pinned Claude Code itself, offline and with no
// login, fires the hooks the kit wired; julieta setup, status and the
// SessionStart print read what the kit installed; and the PreToolUse
// guard refuses each pattern of 06 6.4 (REQ-0745).
//
// The PreToolUse document is e2e/testdata/agent/2.1.296/
// pre-tool-use-bash.json, recorded on 2026-10-24 from Claude Code
// 2.1.296 of the pinned workload, run against a stub of the API that
// answered with one Bash call; the cases replace its command.

// The paths the kit's steps read and write. claudeKitContent is where
// the overlay of julieta-claude.dockerfile puts the kit's files; sbx
// stages only kit.yaml and kit.dockerfile at kitStageRoot (B2,
// 2026-10-25), so the cases mount the kit's directory there instead.
const (
	claudeKitContent = "/usr/share/julieta-claude"
	claudeDropIn     = "/etc/claude-code/managed-settings.d/50-julieta-claude.json"
	claudeMemory     = "/var/lib/julieta-claude/agent-memory"
	claudeSkills     = "/home/agent/.claude/skills"
	agentHome        = "/home/agent"
	docsDir          = "/e2e-docs"
)

// The install hooks of julieta-claude by position.
const (
	claudeHookSettings = iota
	claudeHookMemory
	claudeHookSkillsDir
	claudeHookSkills
	claudeHookCount
)

// TestKitJulietaClaudeDescriptor holds what the descriptor declares: a
// mixin of schema 3 with the lifecycle capability only (no download, so
// no network), the arg julietaBin with a pattern and an environment
// name, and steps that paste no arg into script text.
func TestKitJulietaClaudeDescriptor(t *testing.T) {
	_, d, src := readKit(t, "julieta-claude")
	assert.Equal(t, "3", d.SchemaVersion)
	assert.Equal(t, "mixin", d.Kind)
	assert.Equal(t, "julieta-claude", d.DisplayName)
	first, _, _ := strings.Cut(string(src), "\n")
	assert.True(t, strings.HasPrefix(first, "# syntax=docker/sandbox-kit:3.0.0-m."), first)
	require.Len(t, d.Capabilities, 1)
	assert.Equal(t, "com.docker.sandbox/lifecycle@1", d.Capabilities[0].Type)
	require.Equal(t, []string{"julietaBin"}, keysOf(d.Args))
	assert.Equal(t, "JULIETA_BIN", d.Args["julietaBin"].Env)
	assert.NotEmpty(t, d.Args["julietaBin"].Pattern)
	hooks := d.hooks(t)
	require.Len(t, hooks, claudeHookCount)
	for i, h := range hooks {
		assert.Equal(t, []string{"bash", "-euc"}, h.Command[:2], "hook %d", i)
		assert.NotContains(t, h.Command[len(h.Command)-1], "${{", "hook %d", i)
		want := "0"
		if i == claudeHookSkillsDir {
			want = "agent" // ~/.claude is never made by root
		}
		assert.Equal(t, want, h.User, "hook %d", i)
	}
	assert.Equal(t, []string{"JULIETA_BIN"}, hooks[claudeHookSettings].Env)
}

// TestKitJulietaClaudeRecipe holds the overlay to what the steps read:
// the recipe copies the settings template and the skills, and nothing
// else, to claudeKitContent, and no step reads a file of the kit at
// kitStageRoot, where sbx stages no file of it but the descriptor and
// the recipe.
func TestKitJulietaClaudeRecipe(t *testing.T) {
	dir, _, src := readKit(t, "julieta-claude")
	recipe, err := os.ReadFile(filepath.Join(dir, "julieta-claude.dockerfile"))
	require.NoError(t, err)
	var steps []string
	for line := range strings.SplitSeq(string(recipe), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			steps = append(steps, line)
		}
	}
	assert.Equal(t, []string{
		"FROM scratch",
		"COPY managed-settings.json " + claudeKitContent + "/managed-settings.json",
		"COPY skills/ " + claudeKitContent + "/skills/",
	}, steps)
	assert.NotContains(t, string(src), kitStageRoot+"julieta-claude")
}

// keysOf returns the keys of m.
func keysOf[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// startClaudeKit starts a container of the workload with no network,
// the kit's directory where sbx stages it, the documents docs at
// docsDir, and, with bin, the bin directory of julieta at its own
// path, which the kit's arg names: without it, the .romeu/bin mount is
// absent.
func startClaudeKit(t *testing.T, bin bool, docs map[string]string) kitSandbox {
	t.Helper()
	dir, d, _ := readKit(t, "julieta-claude")
	w := workload(t)
	docDir := t.TempDir()
	require.NoError(t, os.Chmod(docDir, 0o755))
	for name, body := range docs {
		require.NoError(t, os.WriteFile(filepath.Join(docDir, name), []byte(body), 0o644))
	}
	mounts := [][2]string{{dir, claudeKitContent}, {docDir, docsDir}}
	if bin {
		mounts = append(mounts, [2]string{w.Bin, w.Bin})
	}
	args := []string{"run", "-d", "--rm", "--pull=never", "--network=none",
		"--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g"}
	for _, m := range mounts {
		require.False(t, strings.ContainsAny(m[0], ",="), "a bind mount cannot name %s", m[0])
		args = append(args, "--mount", "type=bind,source="+m[0]+",target="+m[1]+",readonly")
	}
	out, err := dockerOut(t, append(args, "--entrypoint", "sleep", w.Pins.Workload.Ref, "3600")...)
	require.NoError(t, err, "%s", out)
	id := strings.TrimSpace(out)
	t.Cleanup(func() { _, _ = dockerOut(t, "rm", "-f", id) })
	return kitSandbox{t: t, id: id, d: d}
}

// install runs every install step of the kit with julietaBin set to
// path, each of which must pass.
func (k kitSandbox) install(path string) {
	k.t.Helper()
	for _, h := range k.d.hooks(k.t) {
		out, code := k.run(h, "JULIETA_BIN="+path)
		require.Zero(k.t, code, "%s: %s", h.Description, out)
	}
}

// claudeSettings is the part of the drop-in the cases read.
type claudeSettings struct {
	AutoMemoryEnabled   *bool  `json:"autoMemoryEnabled"`
	AutoMemoryDirectory string `json:"autoMemoryDirectory"`
	Hooks               map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

// settings reads the drop-in the kit installed.
func (k kitSandbox) settings() claudeSettings {
	k.t.Helper()
	out, code := k.sh("0", "cat "+claudeDropIn)
	require.Zero(k.t, code, out)
	var s claudeSettings
	require.NoError(k.t, json.Unmarshal([]byte(out), &s), out)
	return s
}

// hookCommand returns the one command of the event and matcher.
func (s claudeSettings) hookCommand(t *testing.T, event, matcher string) string {
	t.Helper()
	var found []string
	for _, e := range s.Hooks[event] {
		if e.Matcher == matcher {
			for _, h := range e.Hooks {
				found = append(found, h.Command)
			}
		}
	}
	require.Len(t, found, 1, "%s %s", event, matcher)
	return found[0]
}

// hookRun runs a hook command as the agent, the shell reading the
// document doc of docsDir on stdin, and returns its stdout, its stderr
// and its status.
func (k kitSandbox) hookRun(command, doc string) (stdout, stderr string, code int) {
	k.t.Helper()
	out, _ := k.exec("agent", []string{"HOME=" + agentHome}, "sh", "-c",
		`sh -c "$1" <"$2" >/tmp/hook-out 2>/tmp/hook-err; c=$?; cat /tmp/hook-out; echo "@@@ $c"; cat /tmp/hook-err`,
		"sh", command, docsDir+"/"+doc)
	stdout, rest, ok := strings.Cut(out, "@@@ ")
	require.True(k.t, ok, "%q", out)
	status, stderr, _ := strings.Cut(rest, "\n")
	code, err := strconv.Atoi(status)
	require.NoError(k.t, err, "%q", out)
	return stdout, stderr, code
}

// julietaRun runs julieta with args as the agent, HOME its own and the
// manifest m set when not empty, and returns stdout, stderr and status.
func (k kitSandbox) julietaRun(m string, args ...string) (stdout, stderr string, code int) {
	k.t.Helper()
	env := []string{"HOME=" + agentHome}
	if m != "" {
		env = append(env, "JULIETA_MANIFEST="+m)
	}
	out, _ := k.exec("agent", env, append([]string{"sh", "-c",
		`"$0" "$@" >/tmp/j-out 2>/tmp/j-err; c=$?; cat /tmp/j-out; echo "@@@ $c"; cat /tmp/j-err`,
		workload(k.t).Julieta}, args...)...)
	stdout, rest, ok := strings.Cut(out, "@@@ ")
	require.True(k.t, ok, "%q", out)
	status, stderr, _ := strings.Cut(rest, "\n")
	code, err := strconv.Atoi(status)
	require.NoError(k.t, err, "%q", out)
	return stdout, stderr, code
}

// guardDocs returns the recorded PreToolUse document with its command
// replaced, one per case, and one document that does not decode.
func guardDocs(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(config(t).Root, "e2e", "testdata", "agent", "2.1.296", "pre-tool-use-bash.json"))
	require.NoError(t, err)
	docs := map[string]string{"recorded.json": string(raw), "undecodable.json": `{"tool_name":"Bash","tool_input":`}
	for name, command := range map[string]string{
		"merge.json":    "gh pr merge 1 --squash",
		"tag.json":      "git push origin v1.2.0",
		"rulesets.json": "gh api -X POST repos/o/r/rulesets --input r.json",
		"settings.json": "gh api -X PATCH repos/o/r -f allow_auto_merge=true",
		"allowed.json":  "gh pr view 1",
		// widened on 2026-10-25 (REQ-0598)
		"release-create.json":    "gh release create v1.2.0 --notes x",
		"release-delete.json":    "gh release delete v1.2.0 --yes",
		"api-merge.json":         "gh api -X PUT repos/o/r/pulls/5/merge",
		"graphql-merge.json":     `gh api graphql -f query='mutation { mergePullRequest(input: {pullRequestId: "x"}) { clientMutationId } }'`,
		"graphql-automerge.json": `gh api graphql -f query='mutation { enablePullRequestAutoMerge(input: {pullRequestId: "x"}) { clientMutationId } }'`,
		"api-tag-ref.json":       "gh api -X POST repos/o/r/git/refs -f ref=refs/tags/v1.2.0 -f sha=abc",
		"api-release.json":       "gh api -X POST repos/o/r/releases -f tag_name=v1.2.0",
		// a refused command after a command substitution in quotes
		"after-subst.json": `echo "$(date)"; gh pr merge 1`,
		// a heredoc body that only names one is no command
		"heredoc.json":    "cat <<'EOF'\ngh pr merge 1\nEOF",
		"unparsable.json": "echo 'unclosed",
	} {
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc))
		doc["tool_input"].(map[string]any)["command"] = command
		b, err := json.Marshal(doc)
		require.NoError(t, err)
		docs[name] = string(b)
	}
	return docs
}

// The refusals of 06 6.4, by document: each case asserts that the text
// says the action is the maintainer's and names the route.
var refusalDocs = []string{"merge.json", "tag.json", "rulesets.json", "settings.json",
	"release-create.json", "release-delete.json", "api-merge.json", "graphql-merge.json", "graphql-automerge.json",
	"api-tag-ref.json", "api-release.json", "after-subst.json"}

// assertRefusal holds a refusal's text to 06 6.4 (REQ-0598).
func assertRefusal(t *testing.T, stderr, doc string) {
	t.Helper()
	assert.Contains(t, stderr, "the maintainer's action, not the agent's", doc)
	assert.Contains(t, stderr, "in an issue, or write it into the handoff", doc)
}

// TestKitJulietaClaudeInstall runs the install steps in the workload and
// asserts what they leave (Acceptance of T079): the two skills of
// skills/, whole and byte for byte, read-only for the agent; the
// agent's own memory dir empty and not writable, and the disabling
// setting; the hook config, SessionStart for startup, resume, clear and
// compact and SessionEnd once, each command invoking julieta by the
// path of the arg; and a second run of the steps leaves the same.
func TestKitJulietaClaudeInstall(t *testing.T) {
	w := workload(t)
	k := startClaudeKit(t, true, nil)
	k.install(w.Julieta)

	want := map[string]string{}
	skillsRoot := filepath.Join(config(t).Root, "skills")
	require.NoError(t, filepath.WalkDir(skillsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(skillsRoot, p)
		want[rel] = hex.EncodeToString(sum[:])
		return err
	}))
	sums := func() map[string]string {
		out, code := k.sh("0", "cd "+claudeSkills+" && find julieta handoff -type f -exec sha256sum {} +")
		require.Zero(t, code, out)
		got := map[string]string{}
		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			sum, name, ok := strings.Cut(line, "  ")
			require.True(t, ok, line)
			got[name] = sum
		}
		return got
	}
	assert.Equal(t, want, sums(), "the skills of skills/, each whole directory (REQ-0595)")
	_, code := k.sh("agent", "touch "+claudeSkills+"/julieta/SKILL.md")
	assert.NotZero(t, code, "the agent cannot change an installed skill")

	_, code = k.sh("agent", "touch "+claudeMemory+"/MEMORY.md")
	assert.NotZero(t, code, "the agent's memory dir is not writable")
	_, code = k.sh("agent", "rmdir "+claudeMemory)
	assert.NotZero(t, code, "the agent cannot remove its memory dir")
	out, code := k.sh("0", "ls -A "+claudeMemory)
	require.Zero(t, code, out)
	assert.Empty(t, strings.TrimSpace(out))

	s := k.settings()
	require.NotNil(t, s.AutoMemoryEnabled)
	assert.False(t, *s.AutoMemoryEnabled, "the disabling setting (Q8)")
	assert.Equal(t, claudeMemory, s.AutoMemoryDirectory)
	var starts []string
	for _, e := range s.Hooks["SessionStart"] {
		starts = append(starts, e.Matcher)
	}
	assert.Equal(t, []string{"startup", "resume", "clear", "compact"}, starts)
	require.Len(t, s.Hooks["SessionEnd"], 1, "SessionEnd once")
	assert.Empty(t, s.Hooks["SessionEnd"][0].Matcher)
	for _, c := range []string{
		s.hookCommand(t, "SessionStart", "startup"), s.hookCommand(t, "SessionEnd", ""), s.hookCommand(t, "PreToolUse", "Bash"),
	} {
		assert.True(t, strings.HasPrefix(c, w.Julieta+" "), "julieta by its absolute path: %s", c)
	}

	// The owners and modes: the managed settings are root's and not the
	// agent's to write; ~/.claude and ~/.claude/skills are the agent's;
	// each skill directory is root's.
	modes, code := k.sh("0", "stat -c '%n %U %a' /etc/claude-code /etc/claude-code/managed-settings.d "+claudeDropIn+
		" "+agentHome+"/.claude "+claudeSkills+" "+claudeSkills+"/julieta "+claudeSkills+"/handoff")
	require.Zero(t, code, modes)
	assert.Equal(t, "/etc/claude-code root 755\n/etc/claude-code/managed-settings.d root 755\n"+claudeDropIn+" root 644\n"+
		agentHome+"/.claude agent 755\n"+claudeSkills+" agent 755\n"+claudeSkills+"/julieta root 755\n"+claudeSkills+"/handoff root 755\n", modes)
	_, code = k.sh("agent", "touch /etc/claude-code/managed-settings.d/60-agent.json")
	assert.NotZero(t, code, "the agent cannot add a managed drop-in")

	k.install(w.Julieta)
	assert.Equal(t, want, sums(), "a second run")
	assert.Equal(t, s, k.settings(), "a second run")
}

// claudeManifest returns the manifest of the setup scenario with the
// agent handle of the claude profile and the agent's HOME.
func claudeManifest(t *testing.T) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(manifestEnv(t, 1, originURL+"api.git"))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	doc["agent"] = "sbx-kit-claude"
	// A secondary repo is cloned below $HOME, the agent's here.
	for _, r := range doc["repos"].([]any) {
		repo := r.(map[string]any)
		repo["path"] = strings.Replace(repo["path"].(string), sandboxHome+"/", agentHome+"/", 1)
	}
	b, err := json.Marshal(doc, json.Deterministic(true))
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

// slugLines returns the lines of text that print a warning of slug.
func slugLines(text, slug string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "warning "+slug+": ") {
			out = append(out, line)
		}
	}
	return out
}

// TestKitJulietaClaudeWithJulieta reads what the kit installed with
// julieta and the pinned agent: julieta setup warns agent-path-missing
// once per missing required path, and the SessionStart print shows it
// (08 8.2 item 1); with the kit's files as installed, no skills-stale
// and no agent-hooks-missing, so the kit wires julieta's exact
// commands; a changed skill is skills-stale, which SessionStart prints
// (06 6.3 step 6); a hook removed from the settings is
// agent-hooks-missing in julieta status; and claude -p, offline and
// with no login, fires the SessionStart and SessionEnd hooks of the
// kit, which julieta records.
func TestKitJulietaClaudeWithJulieta(t *testing.T) {
	w := workload(t)
	k := startClaudeKit(t, true, nil)
	k.install(w.Julieta)
	script := strings.Replace(prologue, "export HOME="+sandboxHome, "export HOME="+agentHome, 1) + "true\n"
	out, code := k.exec("agent", nil, "sh", "-c", script, "sh", w.Julieta)
	require.Zero(t, code, out)
	m := claudeManifest(t)

	_, stderr, code := k.julietaRun(m, "setup")
	require.Zero(t, code, stderr)
	missing := slugLines(stderr, "agent-path-missing")
	require.Len(t, missing, 2, stderr)
	assert.Contains(t, missing[0], agentHome+"/.claude/projects, ")
	assert.Contains(t, missing[1], agentHome+"/.claude/todos, ")
	assert.Empty(t, slugLines(stderr, "skills-stale"), stderr)
	assert.Empty(t, slugLines(stderr, "agent-hooks-missing"), stderr)
	assert.Empty(t, slugLines(stderr, "agent-memory-writable"), stderr)
	show, _, code := k.julietaRun("", "handoff", "show", "--hook")
	require.Zero(t, code)
	assert.Len(t, slugLines(show, "agent-path-missing"), 2, "SessionStart shows the warning: %s", show)

	out, code = k.sh("agent", "mkdir -p "+agentHome+"/.claude/projects "+agentHome+"/.claude/todos")
	require.Zero(t, code, out)
	out, code = k.sh("0", "echo changed >>"+claudeSkills+"/julieta/SKILL.md")
	require.Zero(t, code, out)
	_, stderr, code = k.julietaRun(m, "setup")
	require.Zero(t, code, stderr)
	assert.Empty(t, slugLines(stderr, "agent-path-missing"), stderr)
	assert.Len(t, slugLines(stderr, "skills-stale"), 1, stderr)
	show, _, code = k.julietaRun("", "handoff", "show", "--hook")
	require.Zero(t, code)
	assert.Len(t, slugLines(show, "skills-stale"), 1, "SessionStart prints skills-stale: %s", show)

	k.install(w.Julieta)
	_, stderr, code = k.julietaRun(m, "setup")
	require.Zero(t, code, stderr)
	assert.Empty(t, slugLines(stderr, "skills-stale"), stderr)
	_, stderr, code = k.julietaRun("", "status")
	require.Zero(t, code, stderr)
	assert.Empty(t, slugLines(stderr, "agent-hooks-missing"), stderr)

	out, code = k.sh("0", `sed -i 's/"SessionEnd"/"SessionEndOff"/' `+claudeDropIn)
	require.Zero(t, code, out)
	_, stderr, code = k.julietaRun("", "status")
	require.Zero(t, code, stderr)
	hooks := slugLines(stderr, "agent-hooks-missing")
	require.Len(t, hooks, 1, stderr)
	assert.Contains(t, hooks[0], "lack julieta's SessionEnd;")
	k.install(w.Julieta)

	// The record of the session hooks the calls above made is removed,
	// so what remains is the agent's.
	out, _ = k.exec("agent", []string{"HOME=" + agentHome}, "sh", "-c",
		"rm -f "+agentHome+"/.local/state/julieta/hooks.json && cd /tmp/work/api && timeout 90 "+agentHome+"/.local/bin/claude -p hi; cat "+agentHome+"/.local/state/julieta/hooks.json")
	var rec struct {
		SessionStartAt string `json:"sessionStartAt"`
		SessionEndAt   string `json:"sessionEndAt"`
	}
	_, last, _ := strings.Cut(out, "{")
	require.NoError(t, json.Unmarshal([]byte("{"+last), &rec), out)
	assert.NotEqual(t, "0001-01-01T00:00:00Z", rec.SessionStartAt, "the agent fired the kit's SessionStart: %s", out)
	assert.NotEqual(t, "0001-01-01T00:00:00Z", rec.SessionEndAt, "the agent fired the kit's SessionEnd: %s", out)
	assert.NotEmpty(t, rec.SessionEndAt)
}

// TestKitJulietaClaudeGuard runs julieta hooks guard on the recorded
// document, one case per pattern of 06 6.4 and one for a document it
// cannot decode, each exit 2 (REQ-0410, REQ-0745), and an allowed
// command exit 0; then the same documents through the PreToolUse
// command the kit installed.
func TestKitJulietaClaudeGuard(t *testing.T) {
	w := workload(t)
	k := startClaudeKit(t, true, guardDocs(t))
	k.install(w.Julieta)
	pre := k.settings().hookCommand(t, "PreToolUse", "Bash")

	for _, doc := range append([]string{"recorded.json"}, refusalDocs...) {
		out, code := k.exec("agent", nil, "sh", "-c", `"$0" hooks guard <"$1"`, w.Julieta, docsDir+"/"+doc)
		assert.Equal(t, 2, code, "%s: %s", doc, out)
		assertRefusal(t, out, doc)

		stdout, stderr, code := k.hookRun(pre, doc)
		assert.Equal(t, 2, code, "%s through the kit's hook: %s", doc, stderr)
		assert.Empty(t, stdout, doc)
		assertRefusal(t, stderr, doc)
	}
	out, code := k.exec("agent", nil, "sh", "-c", `"$0" hooks guard <"$1"`, w.Julieta, docsDir+"/undecodable.json")
	assert.Equal(t, 2, code, out)
	assert.Contains(t, out, "cannot be decoded")
	assert.Contains(t, out, "fails closed")
	_, stderr, code := k.hookRun(pre, "undecodable.json")
	assert.Equal(t, 2, code, stderr)

	out, code = k.exec("agent", nil, "sh", "-c", `"$0" hooks guard <"$1"`, w.Julieta, docsDir+"/unparsable.json")
	assert.Equal(t, 2, code, out)
	assert.Contains(t, out, "cannot be parsed")

	for _, doc := range []string{"allowed.json", "heredoc.json"} {
		stdout, stderr, code := k.hookRun(pre, doc)
		assert.Zero(t, code, "%s: %s", doc, stderr)
		assert.Empty(t, stdout+stderr, doc)
	}
}

// TestKitJulietaClaudeMountAbsent holds the wrappers to REQ-0673 when
// julieta cannot run, as with the .romeu/bin mount absent after an sbx
// change: each session hook prints one stdout line naming the failure
// and julieta status and exits 0, and the PreToolUse guard exits 2
// (REQ-0599).
func TestKitJulietaClaudeMountAbsent(t *testing.T) {
	w := workload(t)
	k := startClaudeKit(t, false, guardDocs(t))
	k.install(w.Julieta)
	_, code := k.sh("0", "test ! -e "+w.Julieta)
	require.Zero(t, code, "the bin mount is absent")
	s := k.settings()

	t.Run("PreToolUse with the .romeu/bin mount absent refuses gh pr merge", func(t *testing.T) {
		stdout, stderr, code := k.hookRun(s.hookCommand(t, "PreToolUse", "Bash"), "merge.json")
		assert.Equal(t, 2, code, stderr)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "julieta hooks guard failed: "+w.Julieta+" could not run, so the tool call is refused")
	})
	for _, c := range []struct{ event, matcher, what string }{
		{"SessionStart", "startup", "julieta handoff show --hook"},
		{"SessionStart", "resume", "julieta handoff show --hook"},
		{"SessionStart", "clear", "julieta handoff show --hook"},
		{"SessionStart", "compact", "julieta handoff show --hook"},
		{"SessionEnd", "", "julieta handoff write --facts"},
	} {
		stdout, stderr, code := k.hookRun(s.hookCommand(t, c.event, c.matcher), "recorded.json")
		assert.Zero(t, code, "%s %s: %s", c.event, c.matcher, stderr)
		assert.Equal(t, c.what+" failed: "+w.Julieta+" could not run, the .romeu/bin mount may be absent; run julieta status\n", stdout, c.event)
	}
}

// TestKitJulietaClaudeHostileArg is the fault test of the arg: a
// julietaBin that is empty, relative or holds a character sed or sh
// treat as special fails the settings step at its check, with nothing
// written and no command run from the value.
func TestKitJulietaClaudeHostileArg(t *testing.T) {
	k := startClaudeKit(t, false, nil)
	h := k.d.hooks(t)[claudeHookSettings]
	for _, bad := range []string{"", "relative/julieta", "/a b", "/a;touch /tmp/pwned", "/a#b", "/a&b", "/a$(touch /tmp/pwned)", "/a\n/b"} {
		out, code := k.run(h, "JULIETA_BIN="+bad)
		assert.Equal(t, 1, code, "%q: %s", bad, out)
		_, absent := k.sh("0", "test ! -e "+claudeDropIn+" && test ! -e /tmp/pwned")
		assert.Zero(t, absent, "%q wrote the drop-in or ran its command", bad)
	}
}

// TestKitJulietaClaudeOwners holds the steps to the owners they set
// whatever was there: a ~/.claude that is missing is made by the agent,
// so Claude Code can still write its projects and todos below it, and
// an /etc/claude-code the agent could write before is root's after.
func TestKitJulietaClaudeOwners(t *testing.T) {
	w := workload(t)
	k := startClaudeKit(t, true, nil)
	out, code := k.sh("0", "rm -rf "+agentHome+"/.claude && install -d -o agent -m 0777 /etc/claude-code /etc/claude-code/managed-settings.d")
	require.Zero(t, code, out)
	k.install(w.Julieta)
	modes, code := k.sh("0", "stat -c '%n %U %a' /etc/claude-code /etc/claude-code/managed-settings.d "+agentHome+"/.claude "+claudeSkills)
	require.Zero(t, code, modes)
	assert.Equal(t, "/etc/claude-code root 755\n/etc/claude-code/managed-settings.d root 755\n"+agentHome+"/.claude agent 755\n"+claudeSkills+" agent 755\n", modes)
	out, code = k.sh("agent", "mkdir -p "+agentHome+"/.claude/projects "+agentHome+"/.claude/todos")
	assert.Zero(t, code, "the agent still writes its state below ~/.claude: %s", out)
}
