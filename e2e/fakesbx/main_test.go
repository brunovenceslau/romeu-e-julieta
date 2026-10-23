// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// rig is a fake sbx configured for one test: a session copied from
// testdata, a state file, and a path to start it by.
type rig struct {
	t      *testing.T
	sbx    string // the path the fake is started by
	state  string
	cfg    config
	hanged int
}

// newRig configures the fake with the session testdata/<name>.jsonl.
func newRig(t *testing.T, name string) *rig {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", name+".jsonl"))
	require.NoError(t, err)
	session := filepath.Join(dir, name+".jsonl")
	require.NoError(t, os.WriteFile(session, data, 0o600))
	r := &rig{t: t, sbx: filepath.Join(dir, "sbx"), state: filepath.Join(dir, "state.json")}
	r.cfg = config{Session: session, State: r.state}
	r.write()
	return r
}

// write stores the configuration beside the fake's path.
func (r *rig) write() {
	r.t.Helper()
	data, err := jsonv2.Marshal(r.cfg)
	require.NoError(r.t, err)
	require.NoError(r.t, os.WriteFile(r.sbx+".json", data, 0o600))
}

// call runs the fake with the argv and returns its exit status and
// output.
func (r *rig) call(argv ...string) (code int, stdout, stderr string) {
	r.t.Helper()
	var out, errOut bytes.Buffer
	code = run(append([]string{r.sbx}, argv...), nil, strings.NewReader(""), &out, &errOut, func() { r.hanged++ })
	return code, out.String(), errOut.String()
}

// readState returns the state file.
func (r *rig) readState() state {
	r.t.Helper()
	data, err := os.ReadFile(r.state)
	require.NoError(r.t, err)
	var st state
	require.NoError(r.t, jsonv2.Unmarshal(data, &st))
	return st
}

// TestExit99OnApprovalAndValueForms is the acceptance of T029 and
// REQ-0485 and REQ-0509: the fake exits 99 on --auto-approve and on a
// value form of sbx secret set, whatever the recording holds, and reads
// no session or state before it does.
func TestExit99OnApprovalAndValueForms(t *testing.T) {
	r := newRig(t, "guard")
	tests := []struct {
		name string
		argv []string
	}{
		{"auto-approve", []string{"env", "run", "-d", "--auto-approve", "/abs/shop-env"}},
		{"auto-approve elsewhere", []string{"stop", "--auto-approve", "shop"}},
		{"-y", []string{"env", "run", "-y", "/abs/shop-env"}},
		{"force on run", []string{"env", "run", "-d", "--force", "/abs/shop-env"}},
		{"-f in a cluster on run", []string{"env", "run", "-df", "/abs/shop-env"}},
		{"secret by -t", []string{"secret", "set", "github", "-t", "abc"}},
		{"secret by --token", []string{"secret", "set", "github", "--token", "abc"}},
		{"secret by --token=", []string{"secret", "set", "github", "--token=abc"}},
		{"secret from stdin", []string{"secret", "set", "github"}},
		{"secret by oauth", []string{"secret", "set", "openai", "--oauth"}},
		{"secret with an empty command", []string{"secret", "set", "github", "--command", ""}},
		{"secret with a command and a token", []string{"secret", "set", "github", "--command", "/bin/true", "-t", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, errOut := r.call(tt.argv...)
			assert.Equal(t, 99, code)
			assert.Contains(t, errOut, "refused")
		})
	}
	st := r.readState()
	require.Len(t, st.Failures, len(tests), "each refusal is in the state file, written before the exit")
	assert.Contains(t, st.Failures[0].Reason, "I4")
	assert.Equal(t, tests[0].argv, st.Failures[0].Argv)
	assert.Len(t, st.Calls, len(tests))
	code, _, _ := r.call("secret", "set", "github", "--sandbox", "shop", "--command", "/usr/bin/true")
	assert.Equal(t, 0, code, "the form romeu calls is replayed")
}

// TestExit99WithoutAConfiguration refuses an approval even when the
// fake has no configuration to record it in.
func TestExit99WithoutAConfiguration(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{filepath.Join(t.TempDir(), "sbx"), "env", "run", "-y", "/abs/d"}, nil, nil, &out, &errOut, func() {})
	assert.Equal(t, 99, code)
}

// TestExit99OnTheEnvironment holds 04 4.1 from the fake's side: the
// child environment is PATH, HOME, LANG, TERM and the contributions the
// configuration declares, and none of its values is a secret.
func TestExit99OnTheEnvironment(t *testing.T) {
	r := newRig(t, "replay")
	r.cfg.AllowEnv = []string{"SSH_AUTH_SOCK"}
	r.cfg.Sentinels = []string{"SENTINEL-1f3a"}
	r.write()
	call := func(environ ...string) (int, string) {
		var out, errOut bytes.Buffer
		code := run([]string{r.sbx, "version"}, environ, nil, &out, &errOut, func() {})
		return code, errOut.String()
	}
	code, _ := call("PATH=/usr/bin:/bin", "HOME=/h", "LANG=C", "TERM=xterm", "SSH_AUTH_SOCK=/run/agent.sock")
	assert.Equal(t, 0, code, "the base and a declared contribution")

	for _, tt := range []struct{ name, kv, reason string }{
		{"a name outside the list", "FOO=1", "FOO"},
		{"a name that is the host's", "SSH_CONNECTION=1", "SSH_CONNECTION"},
		{"a sentinel in a declared variable", "SSH_AUTH_SOCK=/x/SENTINEL-1f3a", "holds a secret"},
		{"a sentinel in HOME", "HOME=SENTINEL-1f3a", "holds a secret"},
		{"a GitHub token", "LANG=ghp_abcdefghijklmnop", "holds a secret"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, errOut := call("PATH=/usr/bin:/bin", tt.kv)
			assert.Equal(t, 99, code)
			assert.Contains(t, errOut, tt.reason)
		})
	}
	st := r.readState()
	require.Len(t, st.Failures, 5, "each refusal is recorded before the exit")
	assert.Equal(t, []string{"version"}, st.Failures[0].Argv)
	assert.Contains(t, st.Failures[0].Reason, "FOO")
}

// TestExit99LeavesTheCommandOfAnExec holds that the words after "--" are
// the forwarded command's, never read as flags of sbx.
func TestExit99LeavesTheCommandOfAnExec(t *testing.T) {
	assert.Empty(t, refusal([]string{"env", "exec", "/abs/d", "--", "tool", "--auto-approve", "-y", "--force"}))
	assert.Empty(t, refusal([]string{"env", "exec", "-it", "--env", "-y=1", "/abs/d", "--", "sh"}))
	assert.Empty(t, refusal([]string{"env", "rm", "--force", "/abs/d"}), "--force on rm is not an approval of a plan")
}

// TestReplayIsStateful holds REQ-0928: ls answers differently after
// env run and after env rm, and the answers of one window come in the
// order recorded, the last one standing.
func TestReplayIsStateful(t *testing.T) {
	r := newRig(t, "replay")
	r.cfg.Replace = []replacement{{From: "/abs/root", To: "/tmp/real"}}
	r.write()
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:9418/shop"
	r.write()

	code, out, _ := r.call("version")
	assert.Equal(t, 0, code)
	assert.Equal(t, "sbx version v0.49.2 0123456789abcdef0123456789abcdef01234567\n", out)

	_, out, _ = r.call("ls", "--json")
	assert.Equal(t, "[]\n", out, "before the create")
	_, out, _ = r.call("ls", "--json")
	assert.Equal(t, "[]\n", out, "a read is answered again in its window")

	code, _, errOut := r.call("env", "run", "-d", "--clone", dir)
	assert.Equal(t, 0, code)
	assert.Equal(t, "created at /tmp/real\n", errOut, "the replacement applies to stderr")

	_, out, _ = r.call("ls", "--json")
	assert.JSONEq(t, `[{"name":"shop","status":"running"}]`, out)
	_, out, _ = r.call("ls", "--json")
	assert.JSONEq(t, `[{"name":"shop","status":"running","second":true}]`, out, "the second answer of the window")
	_, out, _ = r.call("ls", "--json")
	assert.JSONEq(t, `[{"name":"shop","status":"running","second":true}]`, out, "the last answer stands")

	code, _, errOut = r.call("stop", "shop")
	assert.Equal(t, 1, code, "a recorded failure is replayed with its status")
	assert.Equal(t, "stop failed\n", errOut)
	_, out, _ = r.call("ls", "--json")
	assert.JSONEq(t, `[{"name":"shop","status":"running"}]`, out, "a failed change moved the window on")

	r.call("stop", "shop")
	_, out, _ = r.call("ls", "--json")
	assert.JSONEq(t, `[{"name":"shop","status":"stopped"}]`, out)
	code, _, _ = r.call("policy", "allow", "network", "--sandbox", "shop", "github.com")
	assert.Equal(t, 0, code)
}

// setUpEnv writes an env dir with a primary clone, as romeu renders it.
func setUpEnv(t *testing.T, dir, name, primary string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, primary, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, primary, ".git", "config"), []byte("[core]\n\tbare = false\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sbxenv.yaml"), []byte("schemaVersion: \"1\"\nname: "+name+"\nagent: sbx-kit-claude\nworkspace:\n  path: ./"+primary+"\n  clone: true\n"), 0o644))
}

// TestUnrecordedShapeFails holds REQ-0927: an argv shape absent from the
// session fails, with the argv, in the output and in the state.
func TestUnrecordedShapeFails(t *testing.T) {
	r := newRig(t, "replay")
	code, _, errOut := r.call("env", "plan", "/abs/shop-env")
	assert.Equal(t, 97, code)
	assert.Contains(t, errOut, `["env" "plan" "/abs/shop-env"]`)
	st := r.readState()
	require.Len(t, st.Failures, 1)
	assert.Equal(t, []string{"env", "plan", "/abs/shop-env"}, st.Failures[0].Argv)
	assert.Equal(t, [][]string{{"env", "plan", "/abs/shop-env"}}, st.Calls)
	// A value that is not of the placeholder's type is another shape.
	code, _, _ = r.call("stop", "NOT A NAME")
	assert.Equal(t, 97, code)
}

// TestOutOfOrderFails holds REQ-0928: a call out of the recorded order
// fails.
func TestOutOfOrderFails(t *testing.T) {
	r := newRig(t, "replay")
	code, _, errOut := r.call("stop", "shop")
	assert.Equal(t, 97, code, "stop is recorded after the create")
	assert.Contains(t, errOut, "out of the recorded order")

	code, _, errOut = r.call("policy", "allow", "network", "--sandbox", "shop", "github.com")
	assert.Equal(t, 97, code)
	assert.Contains(t, errOut, "out of the recorded order")

	// A read recorded only after the create is refused before it.
	r2 := newRig(t, "replay")
	code, _, _ = r2.call("env", "exec", t.TempDir(), "--", "/x/julieta", "version")
	assert.Equal(t, 97, code, "env exec is recorded after the create")
	st := r2.readState()
	assert.Equal(t, 0, st.Mutations, "a refused call moves nothing on")
}

// TestMoreChangesThanRecorded fails the change the session did not
// record.
func TestMoreChangesThanRecorded(t *testing.T) {
	r := newRig(t, "replay")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:1/shop"
	r.write()
	require.Equal(t, 0, func() int { c, _, _ := r.call("env", "run", "-d", "--clone", dir); return c }())
	code, _, errOut := r.call("env", "run", "-d", "--clone", dir)
	assert.Equal(t, 97, code)
	assert.Contains(t, errOut, "out of the recorded order")
}

// TestEnvExecMatchesBeforeTheDoubleDash holds REQ-0927 for env exec: the
// argv before "--" is matched, and the command after it is not.
func TestEnvExecMatchesBeforeTheDoubleDash(t *testing.T) {
	r := newRig(t, "replay")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:1/shop"
	r.write()
	r.call("env", "run", "-d", "--clone", dir)

	_, out, _ := r.call("env", "exec", dir, "--", "/abs/julieta", "version", "--json")
	assert.JSONEq(t, `{"recorded":true}`, out)
	_, out, _ = r.call("env", "exec", dir, "--", "/anything", "at", "all")
	assert.JSONEq(t, `{"recorded":true}`, out)
	_, out, _ = r.call("env", "exec", dir, "--env", "JULIETA_MANIFEST=abc", "--", "/abs/julieta", "setup")
	assert.JSONEq(t, `{"recorded":"env"}`, out)

	code, _, _ := r.call("env", "exec", dir)
	assert.Equal(t, 97, code, "no command")
	code, _, _ = r.call("env", "exec", dir, "--")
	assert.Equal(t, 97, code, "an empty command")
	code, _, _ = r.call("env", "exec", "-it", dir, "--", "/bin/bash", "-l")
	assert.Equal(t, 97, code, "-it is another shape, absent from this session")
	code, _, _ = r.call("env", "exec", "relative/dir", "--", "x")
	assert.Equal(t, 97, code, "a relative dir is not a <dir>")
}

// TestEnvExecForwards runs the command after "--" instead of replaying
// it when the configuration names a command to forward to.
func TestEnvExecForwards(t *testing.T) {
	r := newRig(t, "replay")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:1/shop"
	r.cfg.Forward = []string{"/bin/sh", "-c", `echo "manifest=$JULIETA_MANIFEST words=$*"; exit 7`, "forwarder"}
	r.write()
	r.call("env", "run", "-d", "--clone", dir)

	var out, errOut bytes.Buffer
	code := run([]string{r.sbx, "env", "exec", dir, "--env", "JULIETA_MANIFEST=abc", "--", "/abs/julieta", "setup", "--json"},
		nil, strings.NewReader(""), &out, &errOut, func() {})
	assert.Equal(t, 7, code, "the status of the forwarded command")
	assert.Equal(t, "manifest=abc words=/abs/julieta setup --json\n", out.String())
}

// TestEnvExecForwardFailsWithoutACommand and a forward command that cannot run
// are the fake's own failures.
func TestEnvExecForwardFailures(t *testing.T) {
	// macOS has /usr/bin/true and no /bin/true, so the program is found
	// the way the host finds it.
	yes, err := procgroup.LookPath("true")
	require.NoError(t, err)
	var out, errOut bytes.Buffer
	assert.Equal(t, 98, forward(t.Context(), config{Forward: []string{yes}}, []string{"env", "exec", "/d", "--"}, nil, nil, &out, &errOut))
	assert.Equal(t, 98, forward(t.Context(), config{Forward: []string{yes}}, []string{"env", "exec", "/d"}, nil, nil, &out, &errOut))
	assert.Equal(t, 98, forward(t.Context(), config{Forward: []string{"/nonexistent/forwarder"}}, []string{"env", "exec", "/d", "--", "x"}, nil, nil, &out, &errOut))
	assert.Equal(t, 0, forward(t.Context(), config{Forward: []string{yes}}, []string{"env", "exec", "/d", "--", "x"}, nil, nil, &out, &errOut))
}

// TestCreateWritesTheRemoteStanza holds REQ-0929: on create the fake
// writes the recorded remote.sandbox-<name> stanza into the config of
// the primary clone.
func TestCreateWritesTheRemoteStanza(t *testing.T) {
	r := newRig(t, "replay")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:9418/shop"
	r.write()
	code, _, _ := r.call("env", "run", "-d", "--clone", dir)
	require.Equal(t, 0, code)
	config, err := os.ReadFile(filepath.Join(dir, "shop-api", ".git", "config"))
	require.NoError(t, err)
	assert.Equal(t, "[core]\n\tbare = false\n[remote \"sandbox-shop\"]\n\turl = git://127.0.0.1:9418/shop\n\tfetch = +refs/heads/*:refs/remotes/sandbox-shop/*\n", string(config))
}

// TestCreateEffectFailures are the fake's own failures: no clone to
// write into, no gitUrl for the stanza, no env file.
func TestCreateEffectFailures(t *testing.T) {
	t.Run("no primary clone", func(t *testing.T) {
		r := newRig(t, "replay")
		dir := t.TempDir()
		setUpEnv(t, dir, "shop", "shop-api")
		require.NoError(t, os.RemoveAll(filepath.Join(dir, "shop-api")))
		r.cfg.GitURL = "git://x/shop"
		r.write()
		code, _, errOut := r.call("env", "run", "-d", "--clone", dir)
		assert.Equal(t, 98, code)
		assert.Contains(t, errOut, "the primary clone")
	})
	t.Run("no gitUrl", func(t *testing.T) {
		r := newRig(t, "replay")
		dir := t.TempDir()
		setUpEnv(t, dir, "shop", "shop-api")
		code, _, errOut := r.call("env", "run", "-d", "--clone", dir)
		assert.Equal(t, 98, code)
		assert.Contains(t, errOut, "gitUrl")
	})
	t.Run("no env file", func(t *testing.T) {
		r := newRig(t, "replay")
		r.cfg.GitURL = "git://x/shop"
		r.write()
		code, _, _ := r.call("env", "run", "-d", "--clone", t.TempDir())
		assert.Equal(t, 98, code)
	})
}

// TestReadEnvFile names the sandbox and finds the workspace as sbx does.
func TestReadEnvFile(t *testing.T) {
	tests := []struct {
		name, yaml, wantName, wantWorkspace string
	}{
		{"a name and a string workspace", "name: shop\nagent: a\nworkspace: ./shop-api\n", "shop", "/d/shop-api"},
		{"no name", "agent: claude\nworkspace:\n  path: shop-api\n  clone: true\n", "claude-shop-api", "/d/shop-api"},
		{"an absolute workspace", "name: x\nworkspace: /abs/ws\n", "x", "/abs/ws"},
		{"the project dir variable", "name: x\nworkspace: ${{ env.projectDir }}/src\n", "x", "/d/src"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "sbxenv.yaml"), []byte(tt.yaml), 0o644))
			name, ws, err := readEnvFile(dir)
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, strings.Replace(tt.wantWorkspace, "/d/", dir+"/", 1), ws)
		})
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sbxenv.yaml"), []byte("name: x\n"), 0o644))
	_, _, err := readEnvFile(dir)
	require.ErrorContains(t, err, "no primary workspace")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sbxenv.yaml"), []byte(":\n\t- ["), 0o644))
	_, _, err = readEnvFile(dir)
	assert.Error(t, err)
}

// TestHang records the call and then never answers: the timeout of its
// class ends it.
func TestHang(t *testing.T) {
	r := newRig(t, "replay")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://x/shop"
	r.write()
	r.call("env", "run", "-d", "--clone", dir)
	r.call("stop", "shop")
	r.call("stop", "shop")
	r.call("policy", "allow", "network", "--sandbox", "shop", "github.com")
	code, _, _ := r.call("env", "rm", "--force", dir)
	assert.Equal(t, 98, code, "the stub returns, which the binary never does")
	assert.Equal(t, 1, r.hanged)
	assert.Equal(t, 5, r.readState().Mutations, "the hanging call was counted before it hung")
}

// TestStateSurvivesBetweenCalls: each call is a process, so the progress
// is read back from the file.
func TestStateSurvivesBetweenCalls(t *testing.T) {
	r := newRig(t, "replay")
	r.call("ls", "--json")
	r.call("ls", "--json")
	st := r.readState()
	assert.Equal(t, [][]string{{"ls", "--json"}, {"ls", "--json"}}, st.Calls)
	assert.Equal(t, map[string]int{"0:1": 2}, st.Reads)
}

// TestConcurrentCallsAreSerialized: calls that start at once take the
// lock one by one, and none is lost.
func TestConcurrentCallsAreSerialized(t *testing.T) {
	r := newRig(t, "replay")
	const n = 12
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			var out, errOut bytes.Buffer
			assert.Equal(t, 0, run([]string{r.sbx, "ls", "--json"}, nil, strings.NewReader(""), &out, &errOut, func() {}))
		})
	}
	wg.Wait()
	st := r.readState()
	assert.Len(t, st.Calls, n)
	assert.Equal(t, n, st.Reads["0:1"])
}

// TestBrokenSetups are the failures of the fake itself, exit 98.
func TestBrokenSetups(t *testing.T) {
	t.Run("no configuration", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := run([]string{filepath.Join(t.TempDir(), "sbx"), "version"}, nil, nil, &out, &errOut, func() {})
		assert.Equal(t, 98, code)
		assert.Contains(t, errOut.String(), configEnv)
	})
	t.Run("no session", func(t *testing.T) {
		r := newRig(t, "replay")
		r.cfg.Session = filepath.Join(t.TempDir(), "missing.jsonl")
		r.write()
		code, _, _ := r.call("version")
		assert.Equal(t, 98, code)
	})
	t.Run("a state file that is not JSON", func(t *testing.T) {
		r := newRig(t, "replay")
		require.NoError(t, os.WriteFile(r.state, []byte("{"), 0o600))
		code, _, errOut := r.call("version")
		assert.Equal(t, 98, code)
		assert.Contains(t, errOut, "state.json")
	})
	t.Run("a state path that cannot be locked", func(t *testing.T) {
		r := newRig(t, "replay")
		r.cfg.State = filepath.Join(t.TempDir(), "no", "such", "dir", "state.json")
		r.write()
		code, _, _ := r.call("version")
		assert.Equal(t, 98, code)
	})
	t.Run("a state that cannot be written", func(t *testing.T) {
		r := newRig(t, "replay")
		dir := t.TempDir()
		r.cfg.State = filepath.Join(dir, "state.json")
		r.write()
		require.NoError(t, os.Mkdir(r.cfg.State, 0o755)) // a directory where the file goes
		code, _, _ := r.call("version")
		assert.Equal(t, 98, code)
	})
}

// TestConfigFromTheEnvironment reads the file the variable names before
// the file beside the fake.
func TestConfigFromTheEnvironment(t *testing.T) {
	r := newRig(t, "replay")
	elsewhere := filepath.Join(t.TempDir(), "cfg.json")
	data, err := jsonv2.Marshal(r.cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(elsewhere, data, 0o600))
	require.NoError(t, os.Remove(r.sbx+".json"))
	var out, errOut bytes.Buffer
	code := run([]string{r.sbx, "version"}, []string{"LANG=C", configEnv + "=" + elsewhere}, nil, &out, &errOut, func() {})
	assert.Equal(t, 0, code)
	assert.Equal(t, "sbx version v0.49.2 0123456789abcdef0123456789abcdef01234567\n", out.String())
}

// TestLoadConfigRefuses refuses a configuration with a relative path or
// an empty replacement.
func TestLoadConfigRefuses(t *testing.T) {
	tests := []struct {
		name, body string
	}{
		{"a relative session", `{"session":"s.jsonl","state":"/abs/state.json"}`},
		{"a relative state", `{"session":"/abs/s.jsonl","state":"state.json"}`},
		{"an empty from", `{"session":"/abs/s.jsonl","state":"/abs/st.json","replace":[{"from":"","to":"x"}]}`},
		{"an empty forward word", `{"session":"/abs/s.jsonl","state":"/abs/st.json","forward":["a",""]}`},
		{"not JSON", `{`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sbx")
			require.NoError(t, os.WriteFile(path+".json", []byte(tt.body), 0o600))
			_, err := loadConfig(path, nil)
			assert.Error(t, err)
		})
	}
}

// TestAllTheRecordedChangesAreMade fails a change after the last one the
// session recorded, with the number of changes it has: the branch that
// is not the out-of-order one.
func TestAllTheRecordedChangesAreMade(t *testing.T) {
	r := newRig(t, "onechange")
	code, _, _ := r.call("stop", "shop")
	require.Equal(t, 0, code)
	code, _, errOut := r.call("stop", "shop")
	assert.Equal(t, 97, code)
	assert.Contains(t, errOut, "has recorded 1 changes, all made")
	st := r.readState()
	require.Len(t, st.Failures, 1)
	assert.Equal(t, 1, st.Mutations, "a refused change moves nothing on")
	code, out, _ := r.call("ls", "--json")
	assert.Equal(t, 0, code, "a read is still answered")
	assert.Equal(t, "[]\n", out)
}

// TestAFailedCreateWritesNoStanza holds that the effects of an entry are
// made for an exit of 0 and not for another: the answer is the failure,
// and the config of the clone is as it was.
func TestAFailedCreateWritesNoStanza(t *testing.T) {
	r := newRig(t, "failedcreate")
	dir := t.TempDir()
	setUpEnv(t, dir, "shop", "shop-api")
	r.cfg.GitURL = "git://127.0.0.1:9418/shop"
	r.write()
	before, err := os.ReadFile(filepath.Join(dir, "shop-api", ".git", "config"))
	require.NoError(t, err)
	code, _, errOut := r.call("env", "run", "-d", "--clone", dir)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "did not start")
	after, err := os.ReadFile(filepath.Join(dir, "shop-api", ".git", "config"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}
