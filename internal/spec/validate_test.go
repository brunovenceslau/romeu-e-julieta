// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// rulesOf returns the slugs of the findings of err, sorted.
func rulesOf(t *testing.T, err error) []string {
	t.Helper()
	var inv *spec.InvalidError
	require.ErrorAs(t, err, &inv)
	var out []string
	for _, f := range inv.Findings {
		out = append(out, f.Rule)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestValidateRefusesAProject refuses one break of each rule across the
// values of one spec, and names the rule.
func TestValidateRefusesAProject(t *testing.T) {
	cases := []struct {
		name, old, repl, rule string
	}{
		{"no primary", "    primary: true\n", "", "repos-primary"},
		{"two primaries", "  - url: https://github.com/example/shop-web\n", "  - url: https://github.com/example/shop-web\n    primary: true\n", "repos-primary"},
		{"a dir twice, in another case", "  - url: https://github.com/example/shop-web\n", "  - url: https://github.com/example/shop-web\n    dir: Shop-API\n", "repo-dir-unique"},
		{"a reserved dir", "    dir: shop-api\n", "    dir: Memory\n", "repo-dir"},
		{"a default dir that breaks the rule", "  - url: https://github.com/example/shop-web\n", "  - url: https://github.com/example/Shop_Web\n", "repo-dir"},
		{"a kit with both sources", "  - personal: my-home\n", "  - personal: my-home\n    product: julieta\n", "kit-source"},
		{"a kit with no source", "  - personal: my-home\n", "  - args: {a: b}\n", "kit-source"},
		{"a bad ref", "    ref: main\n", "    ref: a..b\n", "repo-ref"},
		{"a tag of a workload", "workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5", "workload: docker.io/docker/sbx-kit-claude:latest", "workload"},
		{"a wildcard domain", "  extra: [api.stripe.com]", "  extra: [\"*.stripe.com\"]", "egress-extra"},
		{"a secret name", "secrets: [github]", "secrets: [GitHub]", "secret-name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := spec.DecodeProject(valid(t, c.old, c.repl))
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.Contains(t, rulesOf(t, err), c.rule)
		})
	}
}

// file returns a file of projects/ that holds the example spec named
// name, with its repos' URLs renamed after it so that two specs share
// none.
func file(t *testing.T, name string) spec.File {
	t.Helper()
	s := string(valid(t, "name: shop\n", "name: "+name+"\n"))
	s = strings.ReplaceAll(s, "example/shop-", "example/"+name+"-")
	s = strings.ReplaceAll(s, "dir: shop-api", "dir: "+name+"-api")
	s = strings.ReplaceAll(s, "repo: shop-api", "repo: "+name+"-api")
	s = strings.ReplaceAll(s, "repo: shop-web", "repo: "+name+"-web")
	return spec.File{Path: "projects/" + name + ".yaml", Data: []byte(s)}
}

// host returns host settings whose config repo is the primary of the
// spec cfg.
func host(t *testing.T, cfg string) *spec.HostSettings {
	t.Helper()
	h, err := spec.DecodeHostSettings(fixture(t, "valid-host-settings.yaml"))
	require.NoError(t, err)
	h.Config.URL = "https://github.com/example/" + cfg + "-api"
	return &h
}

// TestLoadSetReadsAConfigRepo reads two specs, one of them the config
// project, with and without host settings.
func TestLoadSetReadsAConfigRepo(t *testing.T) {
	files := []spec.File{file(t, "my-config"), file(t, "shop")}
	specs, err := spec.LoadSet(files, nil)
	require.NoError(t, err)
	assert.Len(t, specs, 2)
	_, err = spec.LoadSet(files, host(t, "my-config"))
	require.NoError(t, err)
}

// TestLoadSetRefusesASet refuses each rule across the files of
// projects/, with the error of 04 4.4 where the rule has one, and
// leaves the host rules to romeu (REQ-0174).
func TestLoadSetRefusesASet(t *testing.T) {
	shop := file(t, "shop")
	stem := shop
	stem.Path = "projects/other.yaml"
	upper := file(t, "upper")
	upper.Data = []byte(strings.Replace(string(upper.Data), "https://github.com/example/upper-api", "https://GitHub.com/Example/Shop-API.git", 1))
	cases := []struct {
		name  string
		files []spec.File
		host  *spec.HostSettings
		rule  string
		id    string
	}{
		{"a stray file", []spec.File{shop, {Path: "projects/README.md"}}, nil, "projects-yaml-only", "RJ-345"},
		{"a .yml file", []spec.File{shop, {Path: "projects/x.yml", Data: shop.Data}}, nil, "projects-yaml-only", "RJ-345"},
		{"a name that is not the stem", []spec.File{stem}, nil, "project-name-stem", ""},
		{"a name twice", []spec.File{shop, {Path: "projects/sub/shop.yaml", Data: shop.Data}}, nil, "project-name-unique", ""},
		{"a URL twice, in another case and with .git", []spec.File{shop, upper}, nil, "repo-url-unique", ""},
		{"no config project", []spec.File{shop}, host(t, "my-config"), "config-project", "RJ-346"},
		{"an unknown host", []spec.File{shop}, withHosts(host(t, "shop"), "ghe.example.com"), "repo-url-host", ""},
		{"a workload not on the allowlist", []spec.File{shop}, withWorkloads(host(t, "shop"), "ghcr.io/o/r"), "workload-repository", ""},
		{"a file that does not decode", []spec.File{shop, {Path: "projects/bad.yaml", Data: []byte("schema: project/v1\nname: [\n")}}, nil, "decode", ""},
		{"a newer file", []spec.File{shop, {Path: "projects/new.yaml", Data: []byte("schema: project/v9\n")}}, nil, "format-newer", "RJ-311"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := spec.LoadSet(c.files, c.host)
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.Contains(t, rulesOf(t, err), c.rule)
			if c.id != "" {
				assert.Equal(t, c.id, idOf(err))
			}
		})
	}
	_, err := spec.LoadSet([]spec.File{shop}, nil)
	require.NoError(t, err, "julieta spec validate does not check the host rules")
}

// withHosts returns h with gitHosts holding only names.
func withHosts(h *spec.HostSettings, names ...string) *spec.HostSettings {
	h.GitHosts = nil
	for _, n := range names {
		h.GitHosts = append(h.GitHosts, spec.GitHost{Host: n})
	}
	return h
}

// withWorkloads returns h with workloadRepositories set to repos.
func withWorkloads(h *spec.HostSettings, repos ...string) *spec.HostSettings {
	h.WorkloadRepositories = repos
	return h
}

// TestHostOnly names the three rules of 03 3.1 that read host settings,
// the ones julieta spec validate reports it did not check (REQ-0174).
func TestHostOnly(t *testing.T) {
	var slugs []string
	for _, r := range spec.HostOnly() {
		slugs = append(slugs, r.Slug)
	}
	assert.Equal(t, []string{"config-project", "repo-url-host", "workload-repository"}, slugs)
}

// TestFindingsNeverHoldTheValue holds every message of a refusal of host
// settings to the path and the rule, so a secret value never reaches a
// terminal or a log (I25).
func TestFindingsNeverHoldTheValue(t *testing.T) {
	data := strings.Replace(string(fixture(t, "valid-host-settings.yaml")), "-s, romeu/shop/github, -w]", "-s, ghs_SENTINELVALUE, -w]", 1)
	_, err := spec.DecodeHostSettings([]byte(data))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SENTINEL")
	for e := err; e != nil; e = errors.Unwrap(e) {
		assert.NotContains(t, e.Error(), "SENTINEL")
	}
}

// findingLines returns the lines of the findings of err.
func findingLines(t *testing.T, err error) []string {
	t.Helper()
	var inv *spec.InvalidError
	require.ErrorAs(t, err, &inv)
	var out []string
	for _, f := range inv.Findings {
		out = append(out, f.String())
	}
	return out
}

// TestDefaultsAndHosts pins what the rules read beside the values: a
// dir left out is the URL's base name without .git, a host is compared
// lowercased, two specs whose primary is config.url are refused, and a
// hand-built Project is validated with the defaults DecodeProject fills
// and left as it is.
func TestDefaultsAndHosts(t *testing.T) {
	p, err := spec.DecodeProject(valid(t, "  - url: https://github.com/example/shop-web\n", "  - url: https://github.com/example/shop-web.git\n"))
	require.NoError(t, err)
	assert.Equal(t, "shop-web", p.Repos[1].Dir)

	upper := file(t, "shop")
	upper.Data = []byte(strings.Replace(string(upper.Data), "https://github.com/example/shop-api", "https://GitHub.COM/example/shop-api", 1))
	_, err = spec.LoadSet([]spec.File{upper}, host(t, "shop"))
	require.NoError(t, err, "a host in another case is the gitHosts entry")

	twin := file(t, "twin")
	twin.Data = []byte(strings.ReplaceAll(string(twin.Data), "example/twin-api", "example/shop-api"))
	_, err = spec.LoadSet([]spec.File{file(t, "shop"), twin}, host(t, "shop"))
	require.Error(t, err)
	assert.Contains(t, rulesOf(t, err), "config-project")

	hand := spec.Project{Schema: "project/v1", Name: "shop", Agent: "a",
		Workload: "docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5",
		Repos:    []spec.Repo{{URL: "https://github.com/example/shop-api", Primary: true}}}
	require.NoError(t, hand.Validate())
	assert.Empty(t, hand.Repos[0].Dir, "Validate leaves the Project as it is")
	hand.Repos[0].URL = "https://github.com/example/memory"
	assert.Contains(t, rulesOf(t, hand.Validate()), "repo-dir", "the default dir is checked")
}

// TestAccessorsReturnCopies changes what Rules and Ungated return and
// shows the validators unchanged.
func TestAccessorsReturnCopies(t *testing.T) {
	rs := spec.Rules()
	for i := range rs {
		rs[i].Pattern, rs[i].Reserved = "", nil
		if rs[i].Row != nil {
			rs[i].Row.ID = "RJ-000"
		}
	}
	u := spec.Ungated()
	u[0] = "changed"
	_, err := spec.DecodeProject(valid(t, "    dir: shop-api\n", "    dir: Memory\n"))
	assert.Contains(t, rulesOf(t, err), "repo-dir")
	assert.NotEqual(t, "changed", spec.Ungated()[0])
	assert.Equal(t, "RJ-314", spec.SecretInArgv.ID)
	_, err = spec.Classify()
	require.NoError(t, err)
}

// TestDecodeErrorsHoldNoValue shows that a key or a value of a refused
// file reaches no error text: an unknown key with an escape and a bell,
// a duplicate key that is a token, and a token where a list belongs
// (I25, I29).
func TestDecodeErrorsHoldNoValue(t *testing.T) {
	const token = "ghp_SENTINELVALUE"
	cases := map[string][]byte{
		"an unknown key":   valid(t, "ports:", "\"x\\e[31m\\a"+token+"\": 1\nports:"),
		"a duplicate key":  valid(t, "ports:", token+": 1\n"+token+": 2\nports:"),
		"a wrong kind":     valid(t, "secrets: [github]", "secrets: "+token),
		"a nested unknown": valid(t, "    primary: true\n", "    primary: true\n    "+token+": 1\n"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := spec.DecodeProject(data)
			require.ErrorIs(t, err, spec.ErrInvalid)
			assert.NotContains(t, err.Error(), "SENTINEL")
			assert.NotContains(t, err.Error(), "\x1b")
			assert.NotContains(t, err.Error(), "\a")
		})
	}
	h := strings.Replace(string(fixture(t, "valid-host-settings.yaml")), "  github@shop:\n", "  github@shop:\n    "+token+": x\n", 1)
	_, err := spec.DecodeHostSettings([]byte(h))
	require.ErrorIs(t, err, spec.ErrInvalid)
	assert.NotContains(t, err.Error(), "SENTINEL")
}

// TestDecodeCapsTheNodes refuses a file of more nodes than the cap,
// though it is under the size cap.
func TestDecodeCapsTheNodes(t *testing.T) {
	data := valid(t, "ports: [\"8080:8080/tcp\"]", "ports: ["+strings.Repeat("1,", 120000)+"1]")
	require.Less(t, len(data), spec.MaxFileSize)
	_, err := spec.DecodeProject(data)
	require.ErrorIs(t, err, spec.ErrInvalid)
	assert.ErrorContains(t, err, "nodes")
}
