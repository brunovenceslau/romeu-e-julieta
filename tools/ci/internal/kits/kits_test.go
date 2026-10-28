// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package kits_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/kits"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// The pin material of the fixtures is this repository's own: the pin
// file, mise's signed checksum file with its signature and key, and
// ci.yml. A fixture changes one thing of it, in memory.
var realFiles = []string{
	pins.Path,
	"kits/mise-minisign.pub",
	"kits/mise/SHASUMS256.txt",
	"kits/mise/SHASUMS256.txt.minisig",
	".github/workflows/ci.yml",
}

// tree is a fixture tree: path to content.
type tree map[string]string

func (tr tree) files() []check.TreeFile {
	var files []check.TreeFile
	for p, c := range tr {
		files = append(files, check.TreeFile{Path: p, Mode: "100644", Content: []byte(c)})
	}
	return files
}

// frontendRef is the frontend of the real pin file.
func frontendRef(base tree) string {
	p, err := pins.Parse([]byte(base[pins.Path]))
	if err != nil || p.Frontend == nil {
		return ""
	}
	return p.Frontend.Ref
}

// goodKit is a descriptor from the facts of sitting M0 (source: sitting
// M0): the frontend line first, args with a default, and one install
// hook of the lifecycle@1 capability, whose step downloads, checks the
// download against a checksum file of the kit, and installs.
func goodKit(ref string, step string) string {
	return "# syntax=" + ref + "\n" +
		"# source: sitting M0\n" +
		"schemaVersion: \"3\"\n" +
		"kind: mixin\n" +
		"args:\n" +
		"  miseVersion:\n    default: \"2026.10.3\"\n" +
		"capabilities:\n" +
		"  - type: com.docker.sandbox/lifecycle@1\n" +
		"    config:\n" +
		"      install:\n" +
		"        - command: |\n" + indent(step, "            ") +
		"          user: \"0\"\n"
}

func indent(s, pad string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(s, "\n"), "\n") {
		b.WriteString(pad + line + "\n")
	}
	return b.String()
}

const goodStep = `curl -fsSL -o /tmp/herdr https://example.invalid/herdr
sha256sum -c /usr/share/sandbox/kit/probe/herdr.sha256
install -m 0755 /tmp/herdr /usr/local/bin/herdr
`

// base returns the real pin material and one good kit.
func base(t *testing.T) tree {
	t.Helper()
	root := citest.ModuleRoot(t)
	tr := tree{}
	for _, p := range realFiles {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		require.NoError(t, err)
		tr[p] = string(b)
	}
	require.NotEmpty(t, frontendRef(tr), "the real pin file pins a frontend")
	tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), goodStep)
	return tr
}

func messages(findings []check.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Where+": "+f.Msg)
	}
	return out
}

// TestKitsGoodTreePasses is the control: the real pin material and a
// good kit give no finding.
func TestKitsGoodTreePasses(t *testing.T) {
	assert.Empty(t, messages(kits.Of(base(t).files())))
}

// TestKitsRepositoryPasses runs the rules on the kits and the pin file
// of this repository.
func TestKitsRepositoryPasses(t *testing.T) {
	tr := tree{}
	root := citest.ModuleRoot(t)
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "kits"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		tr[filepath.ToSlash(rel)] = string(b)
		return err
	}))
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)
	tr[".github/workflows/ci.yml"] = string(ci)
	assert.Empty(t, messages(kits.Of(tr.files())))
}

// TestKitsRules fails a fixture for each rule of 10 10.2, and the
// fixtures of the step rules each change the step of the good kit.
func TestKitsRules(t *testing.T) {
	tests := []struct {
		name   string
		change func(tr tree)
		want   string // a part of the one finding expected
	}{
		{"a second frontend pin in another kit", func(tr tree) {
			ref := frontendRef(tr)
			tr["kits/other/other.yaml"] = goodKit(strings.Replace(ref, "@sha256:", "-other@sha256:", 1), goodStep)
		}, "a second frontend pin"},
		{"a second frontend line in one descriptor", func(tr tree) {
			tr["kits/probe/probe.yaml"] += "# syntax=" + frontendRef(tr) + "\n"
		}, "below line 1: a second frontend pin"},
		{"a second frontend line in a dockerfile", func(tr tree) {
			tr["kits/probe/probe.dockerfile"] = "FROM scratch\n# syntax=" + frontendRef(tr) + "\n"
		}, "below line 1: a second frontend pin"},
		{"a frontend that is not the pin", func(tr tree) {
			tr["kits/probe/probe.yaml"] = strings.Replace(tr["kits/probe/probe.yaml"], "@sha256:", "-other@sha256:", 1)
		}, "the frontend differs from the pin"},
		{"a descriptor without the frontend line", func(tr tree) {
			tr["kits/probe/probe.yaml"] = strings.TrimPrefix(tr["kits/probe/probe.yaml"], "# syntax="+frontendRef(tr)+"\n")
		}, "line 1 is not the frontend line"},
		{"a lone kit.yaml is a kit", func(tr tree) {
			tr["kits/lone/kit.yaml"] = "schemaVersion: \"3\"\n"
		}, "line 1 is not the frontend line"},
		{"an install step over five lines", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), goodStep+"true\ntrue\ntrue\n")
		}, "install step 1 has 6 lines, over the 5"},
		{"a dockerfile RUN step over five lines", func(tr tree) {
			tr["kits/probe/probe.dockerfile"] = "FROM scratch\nRUN a \\\n b \\\n c \\\n d \\\n e \\\n f\n"
		}, "the RUN step at line 2 has 6 lines"},
		{"a download without a checksum line", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL -o /tmp/x https://example.invalid/x\ninstall /tmp/x /usr/local/bin/x\n")
		}, "has no valid sha256sum -c line after it"},
		{"a checksum line before the download", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "sha256sum -c /kit/x.sha256\ncurl -fsSL -o /tmp/x https://example.invalid/x\n")
		}, "has no valid sha256sum -c line after it"},
		{"a wget download without a checksum line", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "wget -q https://example.invalid/x\n")
		}, "has no valid sha256sum -c line after it"},
		{"a download piped", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL https://example.invalid/x | tee /tmp/x\nsha256sum -c /kit/x.sha256\n")
		}, `holds "|"`},
		{"a download followed by a semicolon", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL -o /tmp/x https://example.invalid/x; true\nsha256sum -c /kit/x.sha256\n")
		}, `holds ";"`},
		{"a download followed by &&", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL -o /tmp/x https://example.invalid/x && true\nsha256sum -c /kit/x.sha256\n")
		}, `holds "&&"`},
		{"a download with a command substitution", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL -o /tmp/x \"https://example.invalid/$(uname -m)\"\nsha256sum -c /kit/x.sha256\n")
		}, `holds "$("`},
		{"a -c that reads a file the step downloaded", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSL -o /tmp/x https://example.invalid/x\ncurl -fsSL -o /tmp/x.sha256 https://example.invalid/x.sha256\nsha256sum -c /tmp/x.sha256\n")
		}, "the checksum line reads /tmp/x.sha256, a file the step downloaded"},
		{"a -c that reads the file a wget downloaded by its URL", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "wget -q https://example.invalid/SUMS\nshasum -a 256 -c SUMS\n")
		}, "the checksum line reads SUMS, a file the step downloaded"},
		{"a -c that reads a file curl -O downloaded", func(tr tree) {
			tr["kits/probe/probe.yaml"] = goodKit(frontendRef(tr), "curl -fsSLO https://example.invalid/SUMS\nsha256sum --check SUMS\n")
		}, "the checksum line reads SUMS, a file the step downloaded"},
		{"a mise version arg that is not the pin", func(tr tree) {
			tr["kits/probe/probe.yaml"] = strings.Replace(tr["kits/probe/probe.yaml"], "2026.10.3", "2026.10.2", 1)
		}, "the default of the arg miseVersion differs from the pin"},
		{"a herdr hash arg that is not the pin", func(tr tree) {
			tr["kits/probe/probe.yaml"] = strings.Replace(tr["kits/probe/probe.yaml"], "args:\n", "args:\n  herdrSha256Arm64:\n    default: \""+strings.Repeat("0", 64)+"\"\n", 1)
		}, "the default of the arg herdrSha256Arm64 differs from the pin"},
		{"a herdr arg with no pin", func(tr tree) {
			tr[pins.Path] = strings.Replace(tr[pins.Path], "herdr:\n  version: 0.9.5\n  sha256:\n    linux-amd64: 14d9e8bc8b4a9e0118d77c32266f76d461f09b40342eb23752960bd89f59929d\n    linux-arm64: 17d7536f2da50859523680ef279dffcac8e4a235f4e46fe8da6c2ec44141897c\n", "", 1)
			tr["kits/probe/probe.yaml"] = strings.Replace(tr["kits/probe/probe.yaml"], "args:\n", "args:\n  herdrVersion:\n    default: \"0.9.5\"\n", 1)
		}, "the arg herdrVersion has no pin"},
		{"a pin file whose mise differs from the mise step of ci.yml", func(tr tree) {
			tr[".github/workflows/ci.yml"] = strings.Replace(tr[".github/workflows/ci.yml"], "version: 2026.10.3", "version: 2026.10.2", 1)
		}, `the version input of the mise step is "2026.10.2"`},
		{"a ci.yml without a mise step", func(tr tree) {
			tr[".github/workflows/ci.yml"] = "jobs: {}\n"
		}, "no step uses jdx/mise-action"},
		{"a pin file that is not valid", func(tr tree) {
			tr[pins.Path] = "bogus: 1\n"
		}, "kits/pins.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base(t)
			tt.change(tr)
			got := messages(kits.Of(tr.files()))
			require.NotEmpty(t, got)
			var matched int
			for _, m := range got {
				if strings.Contains(m, tt.want) {
					matched++
				}
			}
			assert.Equal(t, 1, matched, "findings: %q", got)
		})
	}
}

// TestKitsMiseSignature covers 07 7.4: a good pin passes (the control
// above), and each way the signed checksum file and the pin can part
// fails.
func TestKitsMiseSignature(t *testing.T) {
	tests := []struct {
		name   string
		change func(tr tree)
		want   string
	}{
		{"a checksum file with one changed byte", func(tr tree) {
			tr["kits/mise/SHASUMS256.txt"] = "0" + tr["kits/mise/SHASUMS256.txt"][1:]
		}, "the signature does not verify"},
		{"a pinned hash absent from the file", func(tr tree) {
			tr[pins.Path] = strings.Replace(tr[pins.Path], "8d48bc510b7d844fad0bc7156c0855c2a1e7230c51e498a7f6b63b021f8b57c5", strings.Repeat("a", 64), 1)
		}, "the pinned hash of mise-v2026.10.3-linux-x64 is not the one the signed kits/mise/SHASUMS256.txt lists"},
		{"a pin change without the file", func(tr tree) {
			delete(tr, "kits/mise/SHASUMS256.txt")
			delete(tr, "kits/mise/SHASUMS256.txt.minisig")
		}, "kits/mise/SHASUMS256.txt: missing, so the pin of mise"},
		{"a pin change without the key", func(tr tree) {
			delete(tr, "kits/mise-minisign.pub")
		}, "kits/mise-minisign.pub: missing"},
		{"a pin of a version the file does not list", func(tr tree) {
			tr[pins.Path] = strings.Replace(tr[pins.Path], "version: 2026.10.3", "version: 2026.10.4", 1)
			tr[".github/workflows/ci.yml"] = strings.Replace(tr[".github/workflows/ci.yml"], "version: 2026.10.3", "version: 2026.10.4", 1)
		}, "lists no mise-v2026.10.4-linux-x64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := base(t)
			tt.change(tr)
			got := messages(kits.Of(tr.files()))
			var matched int
			for _, m := range got {
				if strings.Contains(m, tt.want) {
					matched++
				}
			}
			assert.GreaterOrEqual(t, matched, 1, "findings: %q", got)
		})
	}
}
