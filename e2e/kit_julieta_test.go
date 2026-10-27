// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/container"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The julieta kit (06 6.4), read from kits/julieta/ of the repository:
// its descriptor, and its install steps run once in the workload's base
// as a smoke test (10 10.1), composed with os-base
// (kit_os_base_test.go), as a project spec lists both. A kit build is
// what probe B2 measures; the steps here are the descriptor's own
// commands, run by docker exec in the order and as the user the
// descriptor names, with the environment the hook declares and nothing
// else of the kit's args.

// The paths the steps use, which the cases assert against. No case
// mounts a kit's directory: in a sandbox of a v2 agent with v3 mixins,
// sbx puts no file of a mixin's directory and no layer of its recipe
// into the sandbox (probe of 2026-10-27), so a kit's steps carry what
// they need. kitSums is the sums file the first step writes.
const (
	kitSums    = "/var/lib/julieta-kit-install/SHA256SUMS"
	kitWorkDir = "/var/lib/julieta-kit-install"
	bashEnvRel = "/etc/sandbox-persistent.sh"
	pathLine   = `export PATH="$HOME/.local/bin:$HOME/.local/share/mise/shims:$PATH"`
)

// kitDescriptor is the part of the descriptor these cases read, decoded
// with no unknown key.
type kitDescriptor struct {
	SchemaVersion string            `yaml:"schemaVersion"`
	Kind          string            `yaml:"kind"`
	DisplayName   string            `yaml:"displayName"`
	Description   string            `yaml:"description"`
	Args          map[string]kitArg `yaml:"args"`
	Capabilities  []struct {
		Type   string    `yaml:"type"`
		Config yaml.Node `yaml:"config"`
	} `yaml:"capabilities"`
}

// kitArg is one arg of a descriptor.
type kitArg struct {
	Default     string `yaml:"default"`
	Description string `yaml:"description"`
	Pattern     string `yaml:"pattern"`
	Env         string `yaml:"env"`
}

// kitHook is one install hook, with the args of the kit it belongs to,
// from which it takes the environment it declares.
type kitHook struct {
	Description string            `yaml:"description"`
	User        string            `yaml:"user"`
	Env         []string          `yaml:"env"`
	Command     []string          `yaml:"command"`
	args        map[string]kitArg `yaml:"-"`
}

// readKit returns the directory and descriptor source of the kit name,
// and its descriptor decoded.
func readKit(t *testing.T, name string) (dir string, d kitDescriptor, src []byte) {
	t.Helper()
	dir = filepath.Join(config(t).Root, "kits", name)
	src, err := os.ReadFile(filepath.Join(dir, name+".yaml"))
	require.NoError(t, err)
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(&d))
	return dir, d, src
}

// network returns the config of the network-policy@1 capability: a map
// from a phase to the hosts it allows.
func (d kitDescriptor) network(t *testing.T) map[string]struct {
	Allow []string `yaml:"allow"`
} {
	t.Helper()
	for _, c := range d.Capabilities {
		if c.Type == "com.docker.sandbox/network-policy@1" {
			var phases map[string]struct {
				Allow []string `yaml:"allow"`
			}
			require.NoError(t, c.Config.Decode(&phases))
			return phases
		}
	}
	require.FailNow(t, "the descriptor has no network-policy@1 capability")
	return nil
}

// hooks returns the install hooks of the lifecycle@1 capability, in
// order.
func (d kitDescriptor) hooks(t *testing.T) []kitHook {
	t.Helper()
	for _, c := range d.Capabilities {
		if c.Type == "com.docker.sandbox/lifecycle@1" {
			var cfg struct {
				Install []kitHook `yaml:"install"`
			}
			require.NoError(t, c.Config.Decode(&cfg))
			for i := range cfg.Install {
				cfg.Install[i].args = d.Args
			}
			return cfg.Install
		}
	}
	require.FailNow(t, "the descriptor has no lifecycle@1 capability")
	return nil
}

// The install hooks by position, which TestKitJulietaDescriptor holds to
// their descriptions.
const (
	hookDir = iota
	hookMise
	hookHerdr
	hookInstall
	hookBashEnv
	hookZshenv
	hookHerdrConfig
	hookCount
)

// TestKitJulietaDescriptor holds what the descriptor declares: a mixin of
// schema 3 on the capabilities sitting M0 recorded; a network policy of
// the shape the workload's own descriptor uses (the fixture), naming the
// install phase and the two domains of REQ-0592 and no other host; args
// with a pattern and an environment name, and no arg pasted into any
// script; and no arg tz, since TZ is os-base's now that A12 found
// packages the workload lacks (REQ-0593, 02 2.1).
func TestKitJulietaDescriptor(t *testing.T) {
	_, d, src := readKit(t, "julieta")
	assert.Equal(t, "3", d.SchemaVersion)
	assert.Equal(t, "mixin", d.Kind)
	first, _, _ := strings.Cut(string(src), "\n")
	assert.True(t, strings.HasPrefix(first, "# syntax=docker/sandbox-kit:3.0.0-m."), "line 1 is the frontend line: %q", first)

	var types []string
	for _, c := range d.Capabilities {
		types = append(types, c.Type)
	}
	assert.Equal(t, []string{"com.docker.sandbox/network-policy@1", "com.docker.sandbox/lifecycle@1"}, types)

	// The fixture is the workload's descriptor, read from the pinned
	// image: its policy maps a phase to {allow: [host:port]}. The kit's
	// keeps that shape; the phase install is the one the workload's
	// descriptor does not show.
	raw, err := os.ReadFile(filepath.Join(config(t).Root, "e2e", "testdata", "workload-descriptor.yaml"))
	require.NoError(t, err)
	var fixture struct {
		Source        string `yaml:"source"`
		NetworkPolicy struct {
			Config map[string]struct {
				Allow []string `yaml:"allow"`
			} `yaml:"config"`
		} `yaml:"networkPolicy"`
		ProvidesTZ bool `yaml:"providesTzdata"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &fixture))
	assert.Contains(t, fixture.Source, "sitting M0")
	require.Contains(t, fixture.NetworkPolicy.Config, "runtime", "the workload's policy names the phase runtime")
	require.NotEmpty(t, fixture.NetworkPolicy.Config["runtime"].Allow)
	assert.False(t, fixture.ProvidesTZ, "the workload's descriptor lists no tzdata")

	np := d.network(t)
	require.Len(t, np, 1, "one phase")
	require.Contains(t, np, "install", "the install phase only (REQ-0592)")
	assert.Equal(t, []string{"github.com:443", "objects.githubusercontent.com:443"}, np["install"].Allow)

	for _, name := range []string{"miseVersion", "herdrVersion"} {
		a := d.Args[name]
		assert.NotEmpty(t, a.Pattern, "%s has a pattern", name)
		assert.NotEmpty(t, a.Env, "%s reaches the steps as an environment variable", name)
	}
	assert.NotContains(t, d.Args, "tz", "TZ is os-base's (REQ-0593)")
	assert.NotContains(t, d.Args, "bashEnv", "the BASH_ENV file is the workload's, at a fixed path")

	hooks := d.hooks(t)
	require.Len(t, hooks, hookCount)
	for i, h := range hooks {
		script := h.Command[len(h.Command)-1]
		assert.Equal(t, []string{"bash", "-euc"}, h.Command[:2], "hook %d: bash with -e and -u", i)
		assert.NotContains(t, script, "${{", "hook %d pastes no kit arg into its script", i)
	}
	assert.Equal(t, []string{"JULIETA_MISE_VERSION"}, hooks[hookMise].Env)
	assert.Equal(t, []string{"JULIETA_HERDR_VERSION"}, hooks[hookHerdr].Env)
	assert.Contains(t, hooks[hookMise].Description, "mise")
	assert.Contains(t, hooks[hookHerdr].Description, "herdr")
	assert.Contains(t, hooks[hookInstall].Description, "removed")
}

// kitSandbox is a container of the workload that outlives its commands,
// so the install hooks run in it one after the other. d is the julieta
// kit's descriptor, and base os-base's.
type kitSandbox struct {
	t    *testing.T
	id   string
	d    kitDescriptor
	base kitDescriptor
}

// dockerOut runs one docker command of the case and returns its output,
// standard error included.
func dockerOut(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{"docker"}, args...),
		Env:    os.Environ(),
		Stdout: &out,
		Stderr: &out,
	})
	return out.String(), err
}

// startKit starts a container of the workload with extra docker run
// options, and no file of the julieta or os-base kit in it, and removes
// it at the end of the case.
func startKit(t *testing.T, extra ...string) kitSandbox {
	t.Helper()
	_, d, _ := readKit(t, "julieta")
	_, base, _ := readKit(t, "os-base")
	w := workload(t)
	args := []string{"run", "-d", "--rm", "--pull=never",
		"--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g"}
	args = append(args, extra...)
	out, err := dockerOut(t, append(args, "--entrypoint", "sleep", w.Pins.Workload.Ref, "3600")...)
	require.NoError(t, err, "%s", out)
	id := strings.TrimSpace(out)
	t.Cleanup(func() { _, _ = dockerOut(t, "rm", "-f", id) })
	return kitSandbox{t: t, id: id, d: d, base: base}
}

// exec runs argv in the container as user, with extra environment, and
// returns its output and exit status.
func (k kitSandbox) exec(user string, env []string, argv ...string) (string, int) {
	k.t.Helper()
	args := []string{"exec", "-u", user}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	out, err := dockerOut(k.t, append(append(args, k.id), argv...)...)
	if err == nil {
		return out, 0
	}
	exit, ok := errors.AsType[*procgroup.ExitError](err)
	require.True(k.t, ok, "docker exec: %v: %s", err, out)
	return out, exit.Code
}

// sh runs a script as user with bash, for the cases' own setup and reads.
func (k kitSandbox) sh(user, script string) (string, int) {
	k.t.Helper()
	return k.exec(user, nil, "bash", "-c", script)
}

// hookEnv returns the environment a hook gets from its kit's args: each
// name it declares that an arg carries as env, with the arg's default.
// An undeclared name is not passed, as with sbx.
func hookEnv(h kitHook) []string {
	var env []string
	for _, name := range h.Env {
		for _, a := range h.args {
			if a.Env == name {
				env = append(env, name+"="+a.Default)
			}
		}
	}
	return env
}

// transport lists the curl exit statuses of a download that never
// reached the host: resolve, connect, time out, TLS, send, receive.
// Q27 makes them infrastructure errors; a checksum mismatch is not.
var transport = []int{6, 7, 28, 35, 52, 56}

// run runs one hook as its user with its declared environment, and
// returns its output and status. A hook that ended on a transport error
// of its download ends the case as an infrastructure error.
func (k kitSandbox) run(h kitHook, env ...string) (string, int) {
	k.t.Helper()
	out, code := k.exec(h.User, append(hookEnv(h), env...), h.Command...)
	if slices.Contains(transport, code) {
		container.Fail(k.t, &container.InfraError{Precondition: "the download of an install step of the julieta kit", Err: fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out))})
	}
	return out, code
}

// runAll runs the hooks, each of which must pass.
func (k kitSandbox) runAll(hooks ...kitHook) {
	k.t.Helper()
	for _, h := range hooks {
		out, code := k.run(h)
		require.Zero(k.t, code, "%s: %s", h.Description, out)
	}
}

// withSums returns the hook with the sums file of its last command
// replaced by path.
func withSums(h kitHook, path string) kitHook {
	c := slices.Clone(h.Command)
	last := len(c) - 1
	c[last] = strings.ReplaceAll(c[last], kitSums, path)
	return kitHook{Description: h.Description, User: h.User, Env: h.Env, Command: c, args: h.args}
}

// TestKitJulietaSelfContained holds that the julieta kit's directory
// is its descriptor alone: sbx puts no other file of it into the
// sandbox, so a step that read one would fail there and pass here.
func TestKitJulietaSelfContained(t *testing.T) {
	dir, _, _ := readKit(t, "julieta")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"julieta.yaml"}, names)
}

// TestKitJulietaInstall runs the install steps once, in the order of the
// descriptor, in a container of the workload (network by the rule of
// 10 10.1 and Q27), followed by those of os-base, as the project spec of
// 03 lists the kits; then asserts what they leave: mise and herdr at the
// pinned versions (REQ-0589), the PATH lines in ~/.zshenv and in the
// BASH_ENV file, so a non-interactive bash and zsh find julieta and the
// shims (REQ-0590), the herdr config that herdr itself accepts
// (REQ-0591), the download directory gone, and that TZ from os-base's
// arg names a real zone.
func TestKitJulietaInstall(t *testing.T) {
	k := startKit(t)
	w := workload(t)
	hooks := k.d.hooks(t)

	// The BASH_ENV file is the workload's own, at the path the kit
	// appends to, and the hook leaves its owner as it was.
	env, code := k.exec("1000", nil, "printenv", "BASH_ENV")
	require.Zero(t, code, env)
	assert.Equal(t, bashEnvRel+"\n", env, "the workload's BASH_ENV is the file the kit appends to")
	ownerBefore, code := k.sh("0", "stat -c %U:%G "+bashEnvRel)
	require.Zero(t, code, ownerBefore)

	k.runAll(hooks...)
	k.runBase()

	out, code := k.exec("1000", nil, "mise", "--version")
	require.Zero(t, code, out)
	assert.Contains(t, out, w.Pins.Mise.Version)
	out, code = k.exec("1000", nil, "herdr", "--version")
	require.Zero(t, code, out)
	assert.Contains(t, out, w.Pins.Herdr.Version)
	_, code = k.sh("0", "test ! -e "+kitWorkDir)
	assert.Zero(t, code, "the download directory is removed")

	zshenv, code := k.sh("1000", `cat "$HOME/.zshenv"`)
	require.Zero(t, code, zshenv)
	assert.Equal(t, pathLine+"\n", zshenv, "~/.zshenv")
	bashEnv, code := k.sh("0", "cat "+bashEnvRel)
	require.Zero(t, code, bashEnv)
	assert.Contains(t, strings.Split(bashEnv, "\n"), pathLine, "the BASH_ENV file")
	ownerAfter, code := k.sh("0", "stat -c %U:%G "+bashEnvRel)
	require.Zero(t, code, ownerAfter)
	assert.Equal(t, ownerBefore, ownerAfter, "the owner of the BASH_ENV file")

	// A non-interactive bash with a bare PATH, as an agent's command
	// runs, reads BASH_ENV and so finds julieta and a shim.
	out, code = k.sh("1000", `mkdir -p "$HOME/.local/bin" "$HOME/.local/share/mise/shims"
printf '#!/bin/sh\n' | tee "$HOME/.local/bin/e2e-julieta" "$HOME/.local/share/mise/shims/e2e-shim" >/dev/null
chmod +x "$HOME/.local/bin/e2e-julieta" "$HOME/.local/share/mise/shims/e2e-shim"`)
	require.Zero(t, code, out)
	for _, name := range []string{"e2e-julieta", "e2e-shim"} {
		out, code = k.exec("1000", nil, "env", "-i", "HOME=/home/agent", "PATH=/usr/bin:/bin", "BASH_ENV="+bashEnvRel, "bash", "-c", "command -v "+name)
		assert.Zero(t, code, "a non-interactive bash does not find %s: %s", name, out)
		// zsh, which os-base brings, reads ~/.zshenv in every shell,
		// interactive or not.
		out, code = k.exec("1000", nil, "env", "-i", "HOME=/home/agent", "PATH=/usr/bin:/bin", "zsh", "-c", "whence -p "+name)
		assert.Zero(t, code, "a non-interactive zsh does not find %s: %s", name, out)
	}

	cfg, code := k.sh("1000", `cat "$HOME/.config/herdr/config.toml"`)
	require.Zero(t, code, cfg)
	assert.Equal(t, "[update]\nversion_check = false\nmanifest_check = false\n\n[keys]\nprefix = \"ctrl+b\"\n", cfg)
	out, code = k.exec("1000", nil, "herdr", "config", "check")
	assert.Zero(t, code, "herdr rejects the config: %s", out)
	assert.Contains(t, out, "config: ok")

	// The fault of that check: a key herdr does not know fails it, so the
	// pass above says something.
	out, code = k.sh("1000", `mkdir -p /tmp/h2/.config/herdr
printf '[update]\nbogus = false\n' > /tmp/h2/.config/herdr/config.toml
HOME=/tmp/h2 herdr config check`)
	assert.NotZero(t, code, "herdr config check accepted an unknown key: %s", out)

	// TZ: os-base's tzdata makes the arg tz name a real zone. Its default
	// is UTC by name, and a zone with summer time resolves to one of its
	// two abbreviations; without the database libc would print "Etc" and
	// "Europe" with offset 0.
	assertZones(t, k)
}

// TestKitJulietaKeepsWhatIsThere holds the appends: a ~/.zshenv, a
// BASH_ENV file and a herdr config that another kit wrote keep their
// lines, the PATH lines are appended once however often the steps run,
// and a herdr config that exists is left as it is.
func TestKitJulietaKeepsWhatIsThere(t *testing.T) {
	k := startKit(t)
	hooks := k.d.hooks(t)
	out, code := k.sh("1000", `mkdir -p "$HOME/.config/herdr"
printf '%s\n' 'export OTHER_KIT=1' > "$HOME/.zshenv"
printf '%s\n' '[update]' 'version_check = true' > "$HOME/.config/herdr/config.toml"`)
	require.Zero(t, code, out)
	out, code = k.sh("0", `printf '%s\n' 'export OTHER_KIT=1' >> `+bashEnvRel)
	require.Zero(t, code, out)

	for range 2 {
		k.runAll(hooks[hookBashEnv], hooks[hookZshenv], hooks[hookHerdrConfig])
	}
	zshenv, _ := k.sh("1000", `cat "$HOME/.zshenv"`)
	assert.Equal(t, "export OTHER_KIT=1\n"+pathLine+"\n", zshenv)
	bashEnv, _ := k.sh("0", "cat "+bashEnvRel)
	lines := strings.Split(bashEnv, "\n")
	assert.Contains(t, lines, "export OTHER_KIT=1")
	n := 0
	for _, l := range lines {
		if l == pathLine {
			n++
		}
	}
	assert.Equal(t, 1, n, "the PATH line is in the BASH_ENV file once")
	cfg, _ := k.sh("1000", `cat "$HOME/.config/herdr/config.toml"`)
	assert.Equal(t, "[update]\nversion_check = true\n", cfg, "an existing herdr config is not rewritten")
}

// TestKitJulietaSumsGuardTheInstall is the fault test of the checksum
// lines. A sums file of wrong hashes fails the mise step and the herdr
// step with FAILED; a sums file with no line for the downloaded name
// fails each with "no file was verified", so ignoring the missing files
// of the other architecture never lets an unchecked file pass; and the
// last step, which checks the files once more, installs nothing from a
// download that is not the pin.
func TestKitJulietaSumsGuardTheInstall(t *testing.T) {
	k := startKit(t)
	hooks := k.d.hooks(t)
	z := strings.Repeat("0", 64)
	out, code := k.sh("0", fmt.Sprintf(`printf '%%s  mise-x86_64\n%%s  mise-aarch64\n%%s  herdr-x86_64\n%%s  herdr-aarch64\n' %[1]s %[1]s %[1]s %[1]s > /tmp/wrong.sums
printf '%%s  unrelated\n' %[1]s > /tmp/none.sums`, z))
	require.Zero(t, code, out)
	k.runAll(hooks[hookDir])

	for _, h := range []kitHook{hooks[hookMise], hooks[hookHerdr]} {
		out, code := k.run(withSums(h, "/tmp/wrong.sums"))
		assert.NotZero(t, code, "%s: a download that is not the pin passed: %s", h.Description, out)
		assert.Contains(t, out, "FAILED", h.Description)
		out, code = k.run(withSums(h, "/tmp/none.sums"))
		assert.NotZero(t, code, "%s: a download with no line passed: %s", h.Description, out)
		assert.Contains(t, out, "no file was verified", h.Description)
	}

	// The two real downloads, then the last step against wrong sums.
	k.runAll(hooks[hookMise], hooks[hookHerdr])
	out, code = k.run(withSums(hooks[hookInstall], "/tmp/wrong.sums"))
	assert.NotZero(t, code, "the install step passed on wrong sums: %s", out)
	assert.Contains(t, out, "FAILED")
	for _, bin := range []string{"mise", "herdr"} {
		_, missing := k.sh("0", "test -e /usr/local/bin/"+bin)
		assert.NotZero(t, missing, "%s was installed though its sha256 did not match", bin)
	}
}

// TestKitJulietaHostileArg is the fault test of the args: a version that
// is not digits and dots, set as an arg value would be by a project
// definition, fails the step at its first line, with no command run from
// the value, no download and nothing written.
func TestKitJulietaHostileArg(t *testing.T) {
	k := startKit(t)
	hooks := k.d.hooks(t)
	k.runAll(hooks[hookDir])
	for _, tc := range []struct {
		hook int
		name string
	}{{hookMise, "JULIETA_MISE_VERSION"}, {hookHerdr, "JULIETA_HERDR_VERSION"}} {
		for _, bad := range []string{
			"1.0.0$(touch /tmp/pwned)",
			"1.0.0`touch /tmp/pwned`",
			"1.0.0;touch /tmp/pwned",
			"1.0.0 --output /etc/passwd",
			"../../etc/passwd",
			"1.0.0\ntouch /tmp/pwned",
			"",
		} {
			h := hooks[tc.hook]
			// The value replaces the arg's default: the env given last wins.
			out, code := k.run(h, tc.name+"="+bad)
			// Status 1 is the check of the value; a step that went on to
			// its download would end on curl's own status (22 for a 404).
			assert.Equal(t, 1, code, "%s=%q was not stopped by the check of the value: %s", tc.name, bad, out)
			left, lcode := k.sh("0", "ls -A "+kitWorkDir)
			require.Zero(t, lcode, left)
			// The sums file is the first step's; nothing else is there.
			assert.Equal(t, "SHA256SUMS", strings.TrimSpace(left), "%s=%q left a file in the download directory", tc.name, bad)
			_, absent := k.sh("0", "test ! -e /tmp/pwned")
			assert.Zero(t, absent, "%s=%q ran the command it holds", tc.name, bad)
		}
	}
}
