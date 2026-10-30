// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// shopSpec is a valid spec of the project shop, two repos, the primary
// on its branch main and the secondary on origin's default branch.
const shopSpec = `schema: project/v1
name: shop
repos:
  - url: https://github.com/example/shop-api
    primary: true
    ref: main
  - url: https://github.com/example/shop-web
workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5
agent: sbx-kit-claude
egress:
  extra: [api.stripe.com]
`

// notCheckedLine is the line that names the rules romeu checks alone.
const notCheckedLine = "not checked, since they read host settings: config-project (REQ-0179), repo-url-host (REQ-0180), workload-repository (REQ-0190)\n"

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: time.Now, Version: cli.Version{Version: "v0.1.0-test", Protocol: manifest.Protocol}}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// errorID returns the id of the error report of --json on stderr, or of
// the text report's "error <id>" line.
func errorID(t *testing.T, stderr string) string {
	t.Helper()
	var e struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(stderr)), &e) == nil {
		return e.ID
	}
	for line := range strings.Lines(stderr) {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "error" {
			return strings.TrimSuffix(f[1], ":")
		}
	}
	return ""
}

// configRepo makes a config repo's projects/ directory in a new working
// directory with the files given, by name, and returns nothing: the
// tests name the files by their paths from it.
func configRepo(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "projects"), 0o700))
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projects", name), []byte(body), 0o600))
	}
	t.Chdir(dir)
}

// TestSpecValidatePasses: a valid spec passes with exit 0, and the
// output names the rules of 03 3.1 only romeu checks (REQ-0422,
// REQ-0174).
func TestSpecValidatePasses(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})

	code, stdout, stderr := julieta(t, "spec", "validate", "projects/shop.yaml")

	require.Equal(t, 0, code, stderr)
	assert.Equal(t, notCheckedLine+"spec validate 1 files 0 findings\n", stdout)
	assert.Empty(t, stderr)
}

// TestSpecValidateFindingsPerFile: a file that does not decode, one
// whose schema is broken and one that breaks a rule of 03 3.1 are each
// a finding, exit 6, check-findings (RJ-328), not exit 2 (REQ-0435).
func TestSpecValidateFindingsPerFile(t *testing.T) {
	for name, tc := range map[string]struct{ body, rule string }{
		"decode":  {"schema: project/v1\nname: [shop\n", "decode"},
		"schema":  {shopSpec + "colour: blue\n", "decode"},
		"rule":    {strings.Replace(shopSpec, "name: shop", "name: other", 1), "project-name-stem"},
		"pattern": {strings.Replace(shopSpec, "agent: sbx-kit-claude", "agent: -bad", 1), "agent"},
	} {
		t.Run(name, func(t *testing.T) {
			configRepo(t, map[string]string{"shop.yaml": tc.body})

			code, stdout, stderr := julieta(t, "spec", "validate", "projects/shop.yaml")

			assert.Equal(t, 6, code, stderr)
			assert.Equal(t, "RJ-328", errorID(t, stderr), stderr)
			assert.Contains(t, stdout, "finding "+tc.rule+" projects/shop.yaml: ")
			assert.Contains(t, stdout, notCheckedLine)
		})
	}
}

// TestSpecValidateFindingsAcrossFiles: two specs that each pass alone
// share a repo URL, which is a finding of the two together (REQ-0177).
func TestSpecValidateFindingsAcrossFiles(t *testing.T) {
	blog := strings.NewReplacer("name: shop", "name: blog", "shop-web", "blog-site").Replace(shopSpec)
	configRepo(t, map[string]string{"shop.yaml": shopSpec, "blog.yaml": blog})

	code, _, stderr := julieta(t, "spec", "validate", "projects/blog.yaml")
	require.Equal(t, 0, code, stderr)
	code, stdout, stderr := julieta(t, "spec", "validate", "projects/blog.yaml", "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "RJ-328", errorID(t, stderr))
	assert.Contains(t, stdout, "finding repo-url-unique projects/shop.yaml: repos[0].url: ")
	assert.Contains(t, stdout, "spec validate 2 files 1 findings\n")
}

// TestSpecValidateStrayFile: every entry of projects/ that is not
// *.yaml is a finding naming its path, exit 6, though the glob of the
// config repo's CI does not name it (REQ-0423, REQ-0178): a file of
// another extension, a dotfile and a subdirectory alike.
func TestSpecValidateStrayFile(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec, "notes.txt": "x", "blog.yml": "x", ".gitkeep": ""})
	require.NoError(t, os.Mkdir(filepath.Join("projects", "old"), 0o700))

	code, stdout, stderr := julieta(t, "spec", "validate", "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "RJ-328", errorID(t, stderr))
	assert.Contains(t, stdout, "finding projects-yaml-only projects/blog.yml: the file: ")
	assert.Contains(t, stdout, "finding projects-yaml-only projects/notes.txt: the file: ")
	assert.Contains(t, stdout, "finding projects-yaml-only projects/.gitkeep: the file: ")
	assert.Contains(t, stdout, "finding projects-yaml-only projects/old: the file: ")
	assert.Contains(t, stdout, "spec validate 5 files (4 from projects/) 4 findings\n")
}

// TestSpecValidateUnreadableIsUsage: a file that cannot be read, a
// directory given as a file, and a bad flag are exit 2, the only cases
// of exit 2 (REQ-0422).
func TestSpecValidateUnreadableIsUsage(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})
	for name, args := range map[string][]string{
		"missing":   {"projects/gone.yaml"},
		"directory": {"projects"},
		"flag":      {"--bogus", "projects/shop.yaml"},
		"no file":   {},
	} {
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := julieta(t, append([]string{"spec", "validate"}, args...)...)

			assert.Equal(t, 2, code, stderr)
			assert.Equal(t, "RJ-349", errorID(t, stderr), stderr)
			assert.Empty(t, stdout)
		})
	}
}

// TestSpecValidateManifestSize: a spec whose manifest would be over 32
// KiB encoded is the finding manifest-size (RJ-313, 03 3.6), by the size
// function of internal/manifest; one under it is not.
func TestSpecValidateManifestSize(t *testing.T) {
	var b strings.Builder
	b.WriteString(shopSpec + "salvage:\n  excludeIgnored:\n")
	for i := range 3000 {
		fmt.Fprintf(&b, "    - d%05d/\n", i)
	}
	configRepo(t, map[string]string{"shop.yaml": b.String()})

	code, stdout, stderr := julieta(t, "spec", "validate", "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "RJ-328", errorID(t, stderr))
	assert.Contains(t, stdout, "finding manifest-size projects/shop.yaml: the manifest of the project is at least ")
	assert.Contains(t, stdout, "over the cap of 32768 bytes; julieta measures a lower bound, and romeu sync measures it again\n")
}

// TestSpecValidateManifestSizeMeasuresTheManifest: the manifest the
// size is measured on carries the members romeu fills, so its size
// grows with the spec, and the stand-ins have the forms the decoder of
// internal/manifest accepts.
func TestSpecValidateManifestSizeMeasuresTheManifest(t *testing.T) {
	p := decodeShop(t, shopSpec)
	cat := embedded(t)
	m, err := sizedManifest(p, baseEgress(cat, p))
	require.NoError(t, err)
	env, err := manifest.EncodeEnv(m)
	require.NoError(t, err)
	back, err := manifest.DecodeEnv(env)
	require.NoError(t, err, "the stand-ins have the forms of manifest.v1")
	assert.Equal(t, "shop", back.Project)
	assert.Contains(t, back.Egress, "api.stripe.com")
	assert.Contains(t, back.Egress, "github.com", "the base set")
	require.Len(t, back.Repos, 2)
	assert.Equal(t, "/home/agent/src/shop/shop-web", back.Repos[1].Path)
}

// TestSpecValidateJSON: --json prints one document of the format
// julieta-spec.v1, which carries what the text output says: the files
// judged, the findings, the rules not checked and the warnings; the
// error report is on stderr and the exit code is the same.
func TestSpecValidateJSON(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec, "notes.txt": "x"})

	code, stdout, stderr := julieta(t, "spec", "validate", "--json", "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "RJ-328", errorID(t, stderr))
	var rep Report
	require.NoError(t, json.Unmarshal([]byte(stdout), &rep, json.RejectUnknownMembers(true)), stdout)
	assert.Equal(t, Report{
		Files:      []string{"projects/shop.yaml", "projects/notes.txt"},
		Findings: []Finding{{File: "projects/notes.txt", Message: "is a file under projects/ that is not *.yaml", Path: "the file",
			Rule: "projects-yaml-only"}},
		NotChecked: []string{"config-project", "repo-url-host", "workload-repository"},
		Schema:     "julieta-spec/v1",
		Warnings:   []setup.Warning{},
	}, rep)
	assert.Equal(t, 1, strings.Count(stdout, "\n"), "one document")
}

// TestSpecValidateJSONWarnings: with --json, a warning is a member of the
// document, and stderr holds nothing for it.
func TestSpecValidateJSONWarnings(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec + "  tools:\n    \"aqua:acme/tool\": [dl.acme.example]\n"})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-web": acmeLock}}
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), JSON: true}

	require.NoError(t, run(t.Context(), env, []string{"projects/shop.yaml"}, o.fetch))

	assert.Empty(t, stderr.String())
	var rep Report
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &rep))
	require.Len(t, rep.Warnings, 1)
	assert.Equal(t, "lock-host-gated", rep.Warnings[0].Slug)
	assert.Empty(t, rep.Findings)
}

// TestSpecValidateJSONEscapes: the text of a finding and of a warning
// is escaped in the --json document too (I29), so a control byte the
// config repo's content carries never reaches a terminal raw.
func TestSpecValidateJSONEscapes(t *testing.T) {
	var stdout bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&bytes.Buffer{}), JSON: true}
	rep := newReport(nil, []finding{{file: "projects/x\x1b[31m.yaml", path: "name", rule: "decode", message: "bad\x1b]0;t\x07"}},
		[]setup.Warning{{Slug: "lock-host-gated", Message: "host\x1b[2J"}})

	require.NoError(t, writeJSON(env, rep))

	var back Report
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &back))
	for _, s := range []string{back.Findings[0].File, back.Findings[0].Message, back.Warnings[0].Message} {
		assert.NotContains(t, s, "\x1b", "the decoded text holds no escape byte")
		assert.NotContains(t, s, "\x07")
	}
	assert.Equal(t, termsafe.Escape("projects/x\x1b[31m.yaml"), back.Findings[0].File)
}
