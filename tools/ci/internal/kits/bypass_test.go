// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package kits_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/kits"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// hookKit is a descriptor (source: sitting M0) whose lifecycle@1 entry
// holds hooks, written as the YAML of the lines under the capability's
// config: key.
func hookKit(ref, config string) string {
	return "# syntax=" + ref + "\nschemaVersion: \"3\"\nkind: mixin\n" +
		"capabilities:\n  - type: com.docker.sandbox/lifecycle@1\n    config:\n" + indent(config, "      ")
}

// expect counts the findings that hold want and fails unless it is n.
func expect(t *testing.T, got []string, want string, n int) {
	t.Helper()
	var matched int
	for _, m := range got {
		if strings.Contains(m, want) {
			matched++
		}
	}
	assert.Equal(t, n, matched, "findings holding %q: %q", want, got)
}

// TestKitsBypasses holds one fixture to each spelling that once got a
// download past the install-step rules.
func TestKitsBypasses(t *testing.T) {
	const get = "curl -fsSL -o /tmp/x https://example.invalid/x"
	const sum = "sha256sum -c /usr/share/sandbox/kit/probe/x.sha256"
	step := func(s string) func(tr tree) {
		return func(tr tree) { tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), s) }
	}
	hook := func(config string) func(tr tree) {
		return func(tr tree) { tr["kits/probe/probe.yaml"] = hookKit(frontendRef(tr), config) }
	}
	dockerfile := func(name, content string) func(tr tree) {
		return func(tr tree) { tr["kits/probe/"+name] = content }
	}
	pinned := "docker/sandbox-kit:3.0.0-m.8@sha256:e6a397771e865625047cf84256a914c7974b3e749d858259b5bc9c653ddb6f0d"
	tests := []struct {
		name   string
		change func(tr tree)
		want   string
	}{
		// Chaining and substitution, on any line.
		{"cd then curl", step("cd /tmp && " + get + "\n" + sum + "\n"), `holds "&&"`},
		{"set -e then curl", step("set -e; " + get + "\n" + sum + "\n"), `holds ";"`},
		{"bash on a process substitution", step("bash <(curl -fsSL https://example.invalid/x)\n"), `holds "<("`},
		{"an output process substitution", step("tee >(sha256sum) < /dev/null\n"), `holds ">("`},
		{"a backtick", step("echo `" + get + "`\n"), "holds \"`\""},
		{"a checksum line ended by || true", step(get + "\n" + sum + " || true\n"), `holds "|"`},
		{"cat into sha256sum -c -", step(get + "\ncat sums | sha256sum -c -\n"), `holds "|"`},
		{"a heredoc", step(get + "\ncat <<EOF\nEOF\n" + sum + "\n"), `holds "<<"`},
		{"a download in the background", step(get + " &\n" + sum + "\n"), `holds "&"`},
		// Where the download is.
		{"curl by a path", step("/usr/bin/curl -fsSL -o /tmp/x https://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		{"curl after sudo", step("sudo curl -fsSL -o /tmp/x https://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		{"curl after env", step("env A=b curl -fsSL -o /tmp/x https://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		{"curl after an assignment", step("A=b curl -fsSL -o /tmp/x https://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		{"curl after a tab", step("curl\t-fsSL\t-o\t/tmp/x\thttps://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		{"wget by a path", step("/usr/bin/wget -q https://example.invalid/x\ntrue\n"), "has no valid sha256sum -c line"},
		// The output file, in a cluster.
		{"curl -fsSLo", step("curl -fsSLo /tmp/x https://example.invalid/x\nsha256sum -c /tmp/x\n"), "reads /tmp/x, a file the step downloaded"},
		{"wget -qO", step("wget -qO /tmp/x https://example.invalid/x\nsha256sum -c /tmp/x\n"), "reads /tmp/x, a file the step downloaded"},
		{"curl with a redirect", step("curl -fsSL https://example.invalid/x > /tmp/x\nsha256sum -c /tmp/x\n"), "reads /tmp/x, a file the step downloaded"},
		// The checksum line.
		{"a checksum line on the standard input", step(get + "\nsha256sum -c -\n"), "reads its standard input"},
		{"a checksum line with no operand", step(get + "\nsha256sum -c\n"), "reads its standard input"},
		{"a checksum line with a variable operand", step(get + "\nsha256sum -c \"$SUMS\"\n"), "reads its standard input or a name it computes"},
		{"an echoed checksum line", step(get + "\necho sha256sum -c /usr/share/sandbox/kit/probe/x.sha256\n"), "has no valid sha256sum -c line"},
		{"a checksum command that does not check", step(get + "\nsha256sum /tmp/x\n"), "has no valid sha256sum -c line"},
		{"a downloaded file used before its checksum line", step(get + "\nchmod +x /tmp/x\n" + sum + "\n"), "uses /tmp/x before the checksum line"},
		// Shell wrappers and other shapes of a command.
		{"sh -ec", hook("install:\n  - command: [sh, -ec, \"" + get + "\"]\n"), "has no valid sha256sum -c line"},
		{"bash -lc with arguments after the script", hook("install:\n  - command: [bash, -lc, \"" + get + "\", name, extra]\n"), "has no valid sha256sum -c line"},
		{"a bare argv", hook("install:\n  - command: [curl, -o, /tmp/x, \"https://example.invalid/x\"]\n"), "has no valid sha256sum -c line"},
		{"a map command", hook("install:\n  - command: {run: x}\n"), "cannot read it"},
		{"a command list holding a map", hook("install:\n  - command: [sh, {a: b}]\n"), "an item that is not a string"},
		{"a startup hook", hook("startup:\n  - command: " + get + "\n"), "startup step 1: the download on command 1 has no valid"},
		// The dockerfile.
		{"RUN with two flags", dockerfile("probe.dockerfile", "FROM scratch\nRUN --mount=type=cache,target=/c --network=none curl -o /tmp/x https://example.invalid/x\n"), "the RUN step at line 2: the download on command 1 has no valid"},
		{"a RUN in exec form", dockerfile("probe.dockerfile", "FROM scratch\nRUN [\"sh\", \"-c\", \"curl -o /tmp/x https://example.invalid/x\"]\n"), "has no valid sha256sum -c line"},
		{"an ONBUILD RUN", dockerfile("probe.dockerfile", "FROM scratch\nONBUILD RUN curl -o /tmp/x https://example.invalid/x\n"), "the RUN step at line 2"},
		{"a heredoc RUN", dockerfile("probe.dockerfile", "FROM scratch\nRUN <<EOF\ncurl -o /tmp/x u\nEOF\n"), "a heredoc, whose script"},
		{"a comment inside a continuation", dockerfile("probe.dockerfile", "FROM scratch\nRUN a \\\n# note\n b \\\n c \\\n d \\\n e \\\n f\n"), "the RUN step at line 2 has 6 lines"},
		{"an ADD of a URL", dockerfile("probe.dockerfile", "FROM scratch\nADD https://example.invalid/x /x\n"), "ADD downloads a URL with no --checksum"},
		{"a file named Dockerfile", dockerfile("Dockerfile", "FROM scratch\nRUN curl -o /tmp/x https://example.invalid/x\n"), "the RUN step at line 2"},
		{"a file named Containerfile", dockerfile("Containerfile", "FROM scratch\nRUN curl -o /tmp/x https://example.invalid/x\n"), "the RUN step at line 2"},
		{"the build: block", func(tr tree) {
			tr["kits/probe/probe.yaml"] = "# syntax=" + frontendRef(tr) + "\nschemaVersion: \"3\"\nkind: mixin\nbuild: |\n  FROM scratch\n  RUN curl -o /tmp/x https://example.invalid/x\n"
		}, "(build:): the RUN step at line 2"},
		{"another syntax directive on a dockerfile", dockerfile("probe.dockerfile", "# syntax=docker/dockerfile:1\nFROM scratch\n"), "not the sandbox-kit frontend"},
		{"another syntax directive without a space", dockerfile("probe.dockerfile", "#syntax=docker/dockerfile:1\nFROM scratch\n"), "not the sandbox-kit frontend"},
		{"a dockerfile on another sandbox-kit frontend", dockerfile("probe.dockerfile", "# syntax="+strings.Replace(pinned, "m.8", "m.9", 1)+"\nFROM scratch\n"), "the frontend differs from the pin"},
		// Strict decoding.
		{"an unknown top-level key", func(tr tree) { tr["kits/probe/probe.yaml"] += "bogusKey: 1\n" }, `"bogusKey" is not a key of a v3 descriptor`},
		{"an unknown hook key", hook("install:\n  - command: true\n    shell: bash\n"), `"shell" is not a key of a lifecycle@1 hook`},
		{"an unknown lifecycle key", hook("install: []\nafter: []\n"), `"after" is not a key of a lifecycle@1 config`},
		{"a second YAML document", func(tr tree) { tr["kits/probe/probe.yaml"] += "---\nkind: mixin\n" }, "more than one YAML document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base(t)
			tt.change(tr)
			expect(t, messages(kits.Of(tr.files())), tt.want, 1)
		})
	}
}

// TestKitsAcceptsWhatItShould checks the spellings that must stay clean.
func TestKitsAcceptsWhatItShould(t *testing.T) {
	tests := []struct {
		name string
		kit  func(ref string) string
	}{
		{"a wrapped script with arguments after it", func(ref string) string {
			return hookKit(ref, "install:\n  - command: [sh, -c, \"curl -fsSLo /tmp/x https://example.invalid/x\\nsha256sum -c /kit/x.sha256\\ninstall /tmp/x /usr/local/bin/x\", name]\n")
		}},
		{"a package install that names curl", func(ref string) string {
			return hookKit(ref, "install:\n  - command: apt-get install -y curl\n")
		}},
		{"a hook with the other documented keys", func(ref string) string {
			return hookKit(ref, "install:\n  - command: true\n    user: \"0\"\n    env: [A]\n    description: x\nstartup:\n  - command: true\n    background: true\n")
		}},
		{"wget with its output flag and a shasum -a 256 line", func(ref string) string {
			return goodKit(ref, "wget -qO /tmp/x https://example.invalid/x\nshasum -a 256 -c /kit/x.sha256\ninstall /tmp/x /usr/local/bin/x\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base(t)
			tr["kits/probe/probe.yaml"] = tt.kit(frontendRef(tr))
			assert.Empty(t, messages(kits.Of(tr.files())))
		})
	}
	t.Run("a dockerfile on the pinned frontend, and an ADD with a checksum", func(t *testing.T) {
		tr := base(t)
		tr["kits/probe/probe.dockerfile"] = "# syntax=" + frontendRef(tr) + "\nFROM scratch\nRUN echo ok\nADD --checksum=sha256:" + strings.Repeat("a", 64) + " https://example.invalid/x /x\n"
		assert.Empty(t, messages(kits.Of(tr.files())))
	})
	t.Run("a directory with no descriptor is not a kit", func(t *testing.T) {
		assert.Empty(t, messages(kits.Of(tree{"kits/mise/SHASUMS256.txt": "x\n"}.files())))
		assert.Empty(t, messages(kits.Of(nil)))
	})
	t.Run("files below a kit's subdirectory are not descriptors", func(t *testing.T) {
		tr := base(t)
		tr["kits/probe/files/data.yaml"] = "not: a descriptor\n"
		assert.Empty(t, messages(kits.Of(tr.files())))
	})
}

// TestKitsFailsClosedWithoutPins checks that a kit with no pin file, or
// with a pin file that pins no frontend, is a finding, and that a tree
// with no kit needs neither.
func TestKitsFailsClosedWithoutPins(t *testing.T) {
	ref := "docker/sandbox-kit:3.0.0-m.8@sha256:" + strings.Repeat("a", 64)
	kit := goodKit(ref, "true\n")
	t.Run("no pin file", func(t *testing.T) {
		expect(t, messages(kits.Of(tree{"kits/probe/probe.yaml": kit}.files())), "kits/pins.yaml: missing, so no kit has a frontend pin", 1)
	})
	t.Run("no frontend in the pin file", func(t *testing.T) {
		pin := "herdr:\n  version: 0.9.5\n  sha256:\n    linux-amd64: " + strings.Repeat("a", 64) + "\n    linux-arm64: " + strings.Repeat("b", 64) + "\n"
		expect(t, messages(kits.Of(tree{"kits/probe/probe.yaml": kit, pins.Path: pin}.files())), "pins no frontend", 1)
	})
	t.Run("no kit", func(t *testing.T) {
		assert.Empty(t, messages(kits.Of(tree{"kits/mise/SHASUMS256.txt": "x\n"}.files())))
	})
}

// TestKitsScaffoldPasses runs tools/new kit for real, in a fixture
// repository with this repository's pin file, and holds what it wrote to
// the rules.
func TestKitsScaffoldPasses(t *testing.T) {
	root := citest.ModuleRoot(t)
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	var env []string
	for _, key := range goenv.PassThrough {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	env = slices.Concat(env, goenv.Pin, goenv.Build)
	bin := filepath.Join(t.TempDir(), "new")
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{goBin, "build", "-o", bin, "./tools/new"},
		Dir:   root,
		Env:   env,
	})
	require.NoError(t, err, "%s", out)

	tr := base(t)
	repo := gittest.New(t)
	repo.Write(pins.Path, tr[pins.Path])
	out, err = procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{bin, "kit", "scaffolded"},
		Dir:   repo.Dir,
		Env:   append([]string{"PATH=" + os.Getenv("PATH")}, repo.Env...),
	})
	require.NoError(t, err, "%s", out)
	written, err := os.ReadFile(filepath.Join(repo.Dir, "kits", "scaffolded", "scaffolded.yaml"))
	require.NoError(t, err)
	tr["kits/scaffolded/scaffolded.yaml"] = string(written)
	assert.Empty(t, messages(kits.Of(tr.files())))
}
