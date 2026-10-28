// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/container"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The git-ssh-sign kit (06 6.4, REQ-0594), read from kits/git-ssh-sign/:
// its descriptor, and its two install steps run in the workload's base
// with a real ssh-agent, so a commit is signed and verified by git
// itself (10 10.1). The steps are run by docker exec, as the user and
// with the environment the descriptor names. The sandbox's own forwarding
// of the host socket is sbx's and is not run here.

// signDescriptor is the part of the descriptor these cases read.
type signDescriptor struct {
	SchemaVersion string `yaml:"schemaVersion"`
	Kind          string `yaml:"kind"`
	DisplayName   string `yaml:"displayName"`
	Description   string `yaml:"description"`
	Args          map[string]struct {
		Required    bool   `yaml:"required"`
		Description string `yaml:"description"`
		Pattern     string `yaml:"pattern"`
		Env         string `yaml:"env"`
	} `yaml:"args"`
	Capabilities []struct {
		Type   string    `yaml:"type"`
		Config yaml.Node `yaml:"config"`
	} `yaml:"capabilities"`
}

// signHook is one install hook.
type signHook struct {
	Description string   `yaml:"description"`
	User        string   `yaml:"user"`
	Env         []string `yaml:"env"`
	Command     []string `yaml:"command"`
}

// signReadKit returns the source of the descriptor and its decoding,
// with no unknown key.
func signReadKit(t *testing.T) (src []byte, d signDescriptor) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(config(t).Root, "kits", "git-ssh-sign", "git-ssh-sign.yaml"))
	require.NoError(t, err)
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(&d))
	return src, d
}

// signStrict decodes a config node into out with no unknown key. A
// yaml.Node cannot reject unknown keys itself, so the node is encoded
// again and read by a decoder that does.
func signStrict(t *testing.T, n *yaml.Node, out any) {
	t.Helper()
	b, err := yaml.Marshal(n)
	require.NoError(t, err)
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(out))
}

// capability returns the config node of the capability of type typ.
func (d signDescriptor) capability(t *testing.T, typ string) yaml.Node {
	t.Helper()
	for _, c := range d.Capabilities {
		if c.Type == typ {
			return c.Config
		}
	}
	require.FailNow(t, "the descriptor lacks the capability", typ)
	return yaml.Node{}
}

// hooks returns the install hooks of lifecycle@1, in order. The config
// holds install and nothing else.
func (d signDescriptor) hooks(t *testing.T) []signHook {
	t.Helper()
	cfg := d.capability(t, "com.docker.sandbox/lifecycle@1")
	var l struct {
		Install []signHook `yaml:"install"`
	}
	signStrict(t, &cfg, &l)
	return l.Install
}

// script returns the script of a hook, the last word of its command.
func (h signHook) script(t *testing.T) string {
	t.Helper()
	require.GreaterOrEqual(t, len(h.Command), 3, "%s: bash, -euc and a script", h.Description)
	return h.Command[len(h.Command)-1]
}

// The install hooks by position.
const (
	signHookConfig = iota
	signHookKey
	signHookCount
)

// signGoodKeys are values of the arg signingKey the pattern admits: the
// first two fields of a .pub file, of each type the kit supports.
var signGoodKeys = []string{
	"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFXYtdiQBGzmp5/HtUpj3N6SvOzEdxuGdJaukGuRYk+U",
	"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7+/=",
	"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTY==",
}

// signHostileKeys are values the pattern, and the step, refuse.
var signHostileKeys = []string{
	"ssh-ed25519 AAAA$(touch /tmp/pwned)",
	"ssh-ed25519 AAAA`touch /tmp/pwned`",
	"ssh-ed25519 AAAA;touch /tmp/pwned",
	"ssh-ed25519 AAAA comment",
	"ssh-ed25519 AAAA\n* namespaces=\"file\" ssh-ed25519 BBBB",
	"ssh-ed25519 AAAA\n",
	"ssh-ed25519 AAAA===",
	" ssh-ed25519 AAAA",
	"ssh-ed25519\tAAAA",
	"sk-ssh-ed25519@openssh.com AAAA",
	"-----BEGIN OPENSSH PRIVATE KEY-----",
	"/home/agent/.ssh/id_ed25519",
	"ssh-ed25519",
	"",
}

// signFact is the fixture of the facts about the ssh-agent capability.
type signFact struct {
	Source     string `yaml:"source"`
	SBX        string `yaml:"sbx"`
	Frontend   string `yaml:"frontend"`
	Capability struct {
		Type   string `yaml:"type"`
		Config struct {
			Phase        string   `yaml:"phase"`
			Unrestricted bool     `yaml:"unrestricted"`
			Sign         []string `yaml:"sign"`
		} `yaml:"config"`
	} `yaml:"capability"`
	Date     string `yaml:"date"`
	Observed struct {
		SignNamespaceGit  string `yaml:"signNamespaceGit"`
		SignNamespaceFile string `yaml:"signNamespaceFile"`
	} `yaml:"observed"`
}

// TestKitGitSSHSignDescriptor holds what the descriptor declares: a
// mixin of schema 3 on the pinned frontend line; the capabilities of
// sitting M0 and no other (so no network, mount, volume or credential
// grant), the ssh-agent one as the fixture records it (unrestricted
// false, signing in the namespace git only) in the runtime phase, which
// the fixture of sitting M0 does not name; the arg signingKey required,
// with a pattern and an environment name; and steps that paste no kit arg
// into their script (REQ-0594).
func TestKitGitSSHSignDescriptor(t *testing.T) {
	src, d := signReadKit(t)
	assert.Equal(t, "3", d.SchemaVersion)
	assert.Equal(t, "mixin", d.Kind)
	assert.Equal(t, "git-ssh-sign", d.DisplayName)

	pinSrc, err := os.ReadFile(filepath.Join(config(t).Root, "kits", "pins.yaml"))
	require.NoError(t, err)
	var pinned struct {
		Frontend struct {
			Ref string `yaml:"ref"`
		} `yaml:"frontend"`
	}
	require.NoError(t, yaml.Unmarshal(pinSrc, &pinned))
	require.NotEmpty(t, pinned.Frontend.Ref)
	first, _, _ := strings.Cut(string(src), "\n")
	assert.Equal(t, "# syntax="+pinned.Frontend.Ref, first, "line 1 is the frontend line of the pin file")

	raw, err := os.ReadFile(filepath.Join(config(t).Root, "kits", "git-ssh-sign", "testdata", "sitting-m0.yaml"))
	require.NoError(t, err)
	var fact signFact
	require.NoError(t, yaml.Unmarshal(raw, &fact))
	assert.Equal(t, "sitting M0", fact.Source, "the evidence is sitting M0's, not sitting M2's")
	assert.NotEmpty(t, fact.SBX)
	assert.NotEmpty(t, fact.Date, "the fixture dates its latest measurement")
	assert.Equal(t, pinned.Frontend.Ref, fact.Frontend, "the fixture names the pinned frontend, digest included")
	assert.Equal(t, "allowed", fact.Observed.SignNamespaceGit)
	assert.Equal(t, "refused", fact.Observed.SignNamespaceFile)

	var types []string
	for _, c := range d.Capabilities {
		types = append(types, c.Type)
	}
	assert.Equal(t, []string{fact.Capability.Type, "com.docker.sandbox/lifecycle@1"}, types)

	agent := d.capability(t, fact.Capability.Type)
	var got struct {
		Phase        string   `yaml:"phase"`
		Unrestricted bool     `yaml:"unrestricted"`
		Sign         []string `yaml:"sign"`
	}
	signStrict(t, &agent, &got)
	assert.Equal(t, "runtime", got.Phase, "the agent is reachable in the runtime phase only; the schema requires a phase")
	assert.False(t, got.Unrestricted, "the agent is not unrestricted")
	assert.Equal(t, []string{"git"}, got.Sign, "signing in the namespace git only")
	assert.Equal(t, fact.Capability.Config.Sign, got.Sign)
	assert.Equal(t, fact.Capability.Config.Unrestricted, got.Unrestricted)
	assert.Equal(t, fact.Capability.Config.Phase, got.Phase, "the fixture names the phase the kit declares")

	require.Contains(t, d.Args, "signingKey")
	a := d.Args["signingKey"]
	assert.True(t, a.Required, "signingKey has no default")
	assert.NotEmpty(t, a.Pattern)
	assert.Equal(t, "GIT_SSH_SIGN_KEY", a.Env)
	assert.Len(t, d.Args, 1)

	hooks := d.hooks(t)
	require.Len(t, hooks, signHookCount)
	for i, h := range hooks {
		assert.Equal(t, []string{"bash", "-euc"}, h.Command[:2], "hook %d: bash with -e and -u", i)
		assert.NotContains(t, h.script(t), "${{", "hook %d pastes no kit arg into its script", i)
		assert.NotContains(t, h.script(t), "kit.args", "hook %d", i)
		assert.Equal(t, "agent", h.User, "hook %d", i)
	}
	assert.Empty(t, hooks[signHookConfig].Env)
	assert.Equal(t, []string{"GIT_SSH_SIGN_KEY"}, hooks[signHookKey].Env)

	// The step checks the value with the arg's own pattern, in the C
	// locale so a range such as a-z means ASCII only.
	script := hooks[signHookKey].script(t)
	assert.Contains(t, script, "R='"+a.Pattern+"'", "the step's regex is the arg's pattern")
	assert.Contains(t, script, "export LC_ALL=C")
}

// TestKitGitSSHSignPattern holds the arg's pattern, as Go's regexp reads
// it, to the same good and hostile values the step is run with in
// TestKitGitSSHSignArgShape, so the arg and the step cannot drift apart.
func TestKitGitSSHSignPattern(t *testing.T) {
	_, d := signReadKit(t)
	re, err := regexp.Compile(d.Args["signingKey"].Pattern)
	require.NoError(t, err)
	for _, k := range signGoodKeys {
		assert.True(t, re.MatchString(k), "%q", k)
	}
	for _, k := range signHostileKeys {
		assert.False(t, re.MatchString(k), "%q", k)
	}
}

// signBox is a container of the workload that outlives its commands.
type signBox struct {
	t  *testing.T
	id string
	d  signDescriptor
}

// signDocker runs one docker command and returns its output, standard
// error included.
func signDocker(ctx context.Context, t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{"docker"}, args...),
		Env:    os.Environ(),
		Stdout: &out,
		Stderr: &out,
	})
	return out.String(), err
}

// startSign starts a container of the workload, removed at the end of
// the case and, failing that, by its own sleep. Like the harness, an
// absent daemon or image is an infrastructure error.
func startSign(t *testing.T) signBox {
	t.Helper()
	c := config(t)
	if err := c.CheckDaemon(t.Context()); err != nil {
		container.Fail(t, err)
	}
	if err := c.PullImage(t.Context(), c.Pins.Workload.Ref); err != nil {
		container.Fail(t, err)
	}
	_, d := signReadKit(t)
	out, err := signDocker(t.Context(), t, "run", "-d", "--rm", "--pull=never", "--network=none",
		"--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g",
		"--entrypoint", "sleep", c.Pins.Workload.Ref, "600")
	require.NoError(t, err, "%s", out)
	id := strings.TrimSpace(out)
	t.Cleanup(func() {
		// The context of the test is cancelled before the cleanups run.
		rm, err := signDocker(context.WithoutCancel(t.Context()), t, "rm", "-f", id)
		assert.NoError(t, err, "removing the container %s: %s", id, rm)
	})
	return signBox{t: t, id: id, d: d}
}

// exec runs argv as user with extra environment and returns its output
// and exit status.
func (b signBox) exec(user string, env []string, argv ...string) (string, int) {
	b.t.Helper()
	args := []string{"exec", "-u", user}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	out, err := signDocker(b.t.Context(), b.t, append(append(args, b.id), argv...)...)
	if err == nil {
		return out, 0
	}
	exit, ok := errors.AsType[*procgroup.ExitError](err)
	require.True(b.t, ok, "docker exec: %v: %s", err, out)
	return out, exit.Code
}

// sh runs a script as the agent with bash, for the cases' own setup and
// reads.
func (b signBox) sh(script string, env ...string) (string, int) {
	b.t.Helper()
	return b.exec("agent", env, "bash", "-c", script)
}

// runHook runs one hook as its user, with the environment it declares
// taken from env.
func (b signBox) runHook(h signHook, env ...string) (string, int) {
	b.t.Helper()
	var declared []string
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		for _, want := range h.Env {
			if name == want {
				declared = append(declared, e)
			}
		}
	}
	return b.exec(h.User, declared, h.Command...)
}

// keygen makes an ed25519 key pair at path in the container and returns
// the first two fields of its public key, the form of the arg.
func (b signBox) keygen(path string) string {
	b.t.Helper()
	out, code := b.sh("ssh-keygen -q -t ed25519 -N '' -C 'comment' -f " + path + " && cut -d' ' -f1,2 " + path + ".pub")
	require.Zero(b.t, code, out)
	return strings.TrimSpace(out)
}

// agent starts an ssh-agent on a socket in the container and returns
// the socket path.
func (b signBox) agent() string {
	b.t.Helper()
	out, code := b.sh("ssh-agent -a /tmp/sign-agent.sock >/dev/null && echo /tmp/sign-agent.sock")
	require.Zero(b.t, code, out)
	return strings.TrimSpace(out)
}

// add adds the private key at path to the agent on sock.
func (b signBox) add(sock, path string) {
	b.t.Helper()
	out, code := b.sh("ssh-add "+path+" 2>&1", "SSH_AUTH_SOCK="+sock)
	require.Zero(b.t, code, out)
}

// gitCommit makes an empty commit in a new repository at /tmp/repo as
// the agent, with the socket, and returns its output and status.
func (b signBox) gitCommit(sock string, gitArgs ...string) (string, int) {
	b.t.Helper()
	// Each argument is single-quoted; the keys hold no quote.
	var args string
	for _, a := range gitArgs {
		args += " '" + a + "'"
	}
	return b.sh("rm -rf /tmp/repo && git init -q /tmp/repo && cd /tmp/repo && git -c user.name=T -c user.email=t@example.invalid"+args+" commit -q --allow-empty -m m",
		"SSH_AUTH_SOCK="+sock)
}

// TestKitGitSSHSignSignsAndVerifies runs the two steps with a real key,
// then commits through a real ssh-agent holding only that key: the git
// config the steps wrote is what the acceptance names (gpg.format=ssh,
// commit.gpgsign=true, user.signingkey, the allowed-signers file), the
// commit is signed, and git itself verifies it against the file the
// step wrote. The steps run twice, and leave one line.
func TestKitGitSSHSignSignsAndVerifies(t *testing.T) {
	b := startSign(t)
	hooks := b.d.hooks(t)
	key := b.keygen("/tmp/sign-key")

	for range 2 {
		out, code := b.runHook(hooks[signHookConfig])
		require.Zero(t, code, out)
		out, code = b.runHook(hooks[signHookKey], "GIT_SSH_SIGN_KEY="+key)
		require.Zero(t, code, out)
	}

	for k, want := range map[string]string{
		"gpg.format":                 "ssh",
		"commit.gpgsign":             "true",
		"user.signingkey":            "key::" + key,
		"gpg.ssh.allowedsignersfile": "/home/agent/.config/git/allowed_signers",
	} {
		out, code := b.sh("git config --global --get " + k)
		require.Zero(t, code, "%s: %s", k, out)
		assert.Equal(t, want+"\n", out, k)
	}
	signers, code := b.sh(`cat "$HOME/.config/git/allowed_signers"`)
	require.Zero(t, code, signers)
	assert.Equal(t, `* namespaces="git" `+key+"\n", signers, "one line, limited to the namespace git")

	sock := b.agent()
	b.add(sock, "/tmp/sign-key")
	out, code := b.gitCommit(sock)
	require.Zero(t, code, "the commit was not signed: %s", out)
	out, code = b.sh("cd /tmp/repo && git cat-file commit HEAD")
	require.Zero(t, code, out)
	assert.Contains(t, out, "gpgsig -----BEGIN SSH SIGNATURE-----")
	out, code = b.sh("cd /tmp/repo && git verify-commit HEAD 2>&1")
	assert.Zero(t, code, "git does not verify the commit: %s", out)
	assert.Contains(t, out, "Good \"git\" signature")

	// The namespace of the allowed-signers line is limited to git: a
	// signature in the namespace file is refused, and the same file
	// verifies a signature in the namespace git, the positive control.
	verify := func(ns string) (string, int) {
		return b.sh(`cd /tmp && echo data > sign-data-` + ns + ` && ssh-keygen -q -Y sign -f /tmp/sign-key -n ` + ns + ` sign-data-` + ns + ` >/dev/null 2>&1 || exit 99
ssh-keygen -Y verify -f "$HOME/.config/git/allowed_signers" -I anyone -n ` + ns + ` -s sign-data-` + ns + `.sig < sign-data-` + ns + ` 2>&1`)
	}
	out, code = verify("git")
	assert.Zero(t, code, "a signature in the namespace git did not verify: %s", out)
	out, code = verify("file")
	assert.NotZero(t, code, "a signature in the namespace file verified: %s", out)
	assert.NotEqual(t, 99, code, "the signature in the namespace file was not made: %s", out)
}

// TestKitGitSSHSignOtherKeyIsNotTrusted is the fault test of the
// allowed-signers file: a commit signed with a key the step did not list
// does not verify, and an agent that lacks the listed key signs nothing,
// so the commit fails and no commit exists rather than going out unsigned.
func TestKitGitSSHSignOtherKeyIsNotTrusted(t *testing.T) {
	b := startSign(t)
	hooks := b.d.hooks(t)
	key := b.keygen("/tmp/sign-key")
	other := b.keygen("/tmp/sign-other")
	require.NotEqual(t, key, other)
	for _, h := range []signHook{hooks[signHookConfig], hooks[signHookKey]} {
		out, code := b.runHook(h, "GIT_SSH_SIGN_KEY="+key)
		require.Zero(t, code, out)
	}
	sock := b.agent()

	// The agent holds the other key only: the commit cannot be signed
	// with the key the config names, and fails with no commit made.
	b.add(sock, "/tmp/sign-other")
	out, code := b.gitCommit(sock)
	assert.NotZero(t, code, "a commit went out with an agent that lacks the key: %s", out)
	assert.Contains(t, out, "Couldn't find key in agent")
	_, head := b.sh("cd /tmp/repo && git rev-parse --verify HEAD")
	assert.NotZero(t, head, "a commit exists though the agent lacks the key")

	// Signed by the other key anyway, the commit is not verified.
	out, code = b.gitCommit(sock, "-c", "user.signingkey=key::"+other)
	require.Zero(t, code, out)
	out, code = b.sh("cd /tmp/repo && git verify-commit HEAD 2>&1")
	assert.NotZero(t, code, "a commit of a key that is not listed verified: %s", out)
}

// TestKitGitSSHSignConfigStepAloneFailsClosed pins what the first step
// leaves when the second one has not run: gpg.format=ssh and
// commit.gpgsign=true with no user.signingkey. On the workload's git, an
// agent that holds two keys is not asked to choose; the commit fails,
// names the missing signing key, and no commit exists. So the order of
// the steps keeps a failed second step from letting a commit go unsigned.
func TestKitGitSSHSignConfigStepAloneFailsClosed(t *testing.T) {
	b := startSign(t)
	hooks := b.d.hooks(t)
	b.keygen("/tmp/sign-key")
	b.keygen("/tmp/sign-other")
	out, code := b.runHook(hooks[signHookConfig])
	require.Zero(t, code, out)
	sock := b.agent()
	b.add(sock, "/tmp/sign-key")
	b.add(sock, "/tmp/sign-other")
	listed, code := b.sh("ssh-add -L | wc -l", "SSH_AUTH_SOCK="+sock)
	require.Zero(t, code, listed)
	require.Equal(t, "2", strings.TrimSpace(listed), "the agent holds two keys")

	out, code = b.gitCommit(sock)
	assert.NotZero(t, code, "a commit went out with no user.signingkey: %s", out)
	assert.Contains(t, out, "user.signingkey")
	_, head := b.sh("cd /tmp/repo && git rev-parse --verify HEAD")
	assert.NotZero(t, head, "a commit exists though no signing key is configured")
}

// TestKitGitSSHSignArgShape is the fault test of the arg: a key that is
// not a type and a base64 body, set as an arg value would be by a
// project definition, fails the step at its first line with no command
// run from the value, and writes neither the allowed-signers file nor
// user.signingkey. The values the pattern admits pass the same step, and
// each leaves one allowed-signers line.
func TestKitGitSSHSignArgShape(t *testing.T) {
	b := startSign(t)
	hooks := b.d.hooks(t)
	h := hooks[signHookKey]
	out, code := b.runHook(hooks[signHookConfig])
	require.Zero(t, code, out)
	for _, bad := range signHostileKeys {
		out, code := b.runHook(h, "GIT_SSH_SIGN_KEY="+bad)
		assert.Equal(t, 1, code, "%q was not stopped by the check of the value: %s", bad, out)
		_, missing := b.sh(`test ! -e "$HOME/.config/git/allowed_signers" && test ! -e /tmp/pwned`)
		assert.Zero(t, missing, "%q wrote a file or ran the command it holds", bad)
		out, code = b.sh("git config --global --get user.signingkey")
		assert.Equal(t, 1, code, "%q set user.signingkey: %s", bad, out)
	}
	for _, good := range signGoodKeys {
		out, code := b.runHook(h, "GIT_SSH_SIGN_KEY="+good)
		assert.Zero(t, code, "%q was refused: %s", good, out)
		signers, _ := b.sh(`cat "$HOME/.config/git/allowed_signers"`)
		assert.Equal(t, `* namespaces="git" `+good+"\n", signers)
	}
}
