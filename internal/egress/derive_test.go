// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress_test

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// testCatalog is a small catalog: the meta and base sets of the real
// one, the Go family, a tool whose domain has no upload entry, and gh.
func testCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	c, err := catalog.Build([]catalog.Source{
		{Name: "base.yaml", Data: []byte(`schema: catalog/v1
meta: [mise-versions.jdx.dev, github.com, tuf-repo-cdn.sigstore.dev]
base: [github.com, api.github.com]
domains:
  mise-versions.jdx.dev: {upload: false}
  tuf-repo-cdn.sigstore.dev: {upload: false}
  github.com: {upload: true}
  api.github.com: {upload: true}
`)},
		{Name: "go.yaml", Data: []byte(`schema: catalog/v1
domains:
  dl.google.com: {upload: false}
  proxy.golang.org: {upload: false}
  storage.googleapis.com: {upload: true}
tools:
  core:go: [dl.google.com, proxy.golang.org, storage.googleapis.com]
  aqua:cli/cli: []
  core:node: [nodejs.org]
`)},
	})
	require.NoError(t, err)
	return c
}

// repoOf is a repo whose files are the map, by name.
func repoOf(dir string, files map[string]string) egress.Repo {
	return egress.Repo{Dir: dir, URL: "https://git.example/" + dir + ".git", Read: func(_ context.Context, name string) ([]byte, bool, error) {
		s, ok := files[name]
		return []byte(s), ok, nil
	}}
}

// lockOf writes a mise.lock of the pinned layout: each tool is
// "<key> <backend>" and holds the URL of linux-x64.
func lockOf(tools ...[3]string) string {
	var b strings.Builder
	b.WriteString("lockfile_version = 3\n")
	for _, tool := range tools {
		key, backend, url := tool[0], tool[1], tool[2]
		b.WriteString("\n[[tools.\"" + key + "\"]]\nversion = \"1.0.0\"\nbackend = \"" + backend + "\"\n")
		if url != "" {
			b.WriteString("\n[tools.\"" + key + "\".\"platforms.linux-x64\"]\nchecksum = \"sha256:" + strings.Repeat("a", 64) + "\"\nurl = \"" + url + "\"\n")
		}
	}
	return b.String()
}

const toml = "[tools]\ngo = \"1.27.3\"\n"

func gate(domain string, reason egress.Reason) egress.Gate {
	return egress.Gate{Domain: domain, Reason: reason}
}

// TestDeriveSteps is the table of the eight steps of 07 7.5.
func TestDeriveSteps(t *testing.T) {
	goLock := lockOf([3]string{"go", "core:go", "https://dl.google.com/go/go1.27.3.linux-amd64.tar.gz"})
	metaAuto := []string{"mise-versions.jdx.dev", "tuf-repo-cdn.sigstore.dev"}
	baseGated := []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload)}
	tests := []struct {
		name  string
		repos []egress.Repo
		spec  *spec.Egress
		kit   []string
		auto  []string
		gated []egress.Gate
	}{
		{
			// Step 1: no mise files, the repo contributes nothing. Step 4
			// and step 7: meta and base, github.com and api.github.com
			// being upload-capable.
			name:  "a repo with no mise files adds nothing",
			repos: []egress.Repo{repoOf("api", map[string]string{"README.md": "x"})},
			auto:  metaAuto,
			gated: baseGated,
		},
		{
			// Step 2, found: the domains of the catalog; step 7 moves the
			// upload-capable one.
			name:  "a tool of the catalog adds its domains",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": toml, "mise.lock": goLock})},
			auto:  append([]string{"dl.google.com", "proxy.golang.org"}, metaAuto...),
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("storage.googleapis.com", egress.ReasonUpload)},
		},
		{
			// Step 2, not found with a spec entry: gated.
			name: "a tool outside the catalog with an egress.tools entry is gated",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": "[tools]\nx = \"1\"\n",
				"mise.lock": lockOf([3]string{"x", "aqua:acme/tool", ""})})},
			spec:  &spec.Egress{Tools: map[string][]string{"aqua:acme/tool": {"dl.acme.dev", "dl.google.com"}}},
			auto:  metaAuto,
			gated: append([]egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("dl.acme.dev", egress.ReasonSpecTool), gate("dl.google.com", egress.ReasonSpecTool)}, gate("github.com", egress.ReasonUpload)),
		},
		{
			// An egress.tools entry of a tool the locks do not hold adds nothing.
			name:  "an egress.tools entry no lock uses adds nothing",
			repos: []egress.Repo{repoOf("api", nil)},
			spec:  &spec.Egress{Tools: map[string][]string{"aqua:acme/tool": {"dl.acme.dev"}}},
			auto:  metaAuto,
			gated: baseGated,
		},
		{
			// A catalog entry wins over a spec entry for the same tool.
			name: "a tool of the catalog ignores an egress.tools entry for it",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": "[tools]\ngh = \"1\"\n",
				"mise.lock": lockOf([3]string{"gh", "aqua:cli/cli", ""})})},
			spec:  &spec.Egress{Tools: map[string][]string{"aqua:cli/cli": {"dl.acme.dev"}}},
			auto:  metaAuto,
			gated: baseGated,
		},
		{
			// Step 3: a host the catalog did not produce is gated.
			name: "a lock host the catalog did not produce is gated",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": toml,
				"mise.lock": lockOf([3]string{"go", "core:go", "https://mirror.acme.dev/go1.27.3.tar.gz"})})},
			auto:  append([]string{"dl.google.com", "proxy.golang.org"}, metaAuto...),
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("mirror.acme.dev", egress.ReasonLockHost), gate("storage.googleapis.com", egress.ReasonUpload)},
		},
		{
			// Step 3 for an http: tool: the host of the lock is gated,
			// never auto, whatever the mise.toml says (the residual of
			// the package comment).
			name: "the lock host of an http tool is gated",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": "[tools]\n\"http:foo\" = \"1\"\n",
				"mise.lock": lockOf([3]string{"http:foo", "http:foo", "https://files.acme.dev/foo.tar.gz"})})},
			spec:  &spec.Egress{Tools: map[string][]string{"http:foo": {"files.acme.dev"}}},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("files.acme.dev", egress.ReasonSpecTool), gate("github.com", egress.ReasonUpload)},
		},
		{
			// Step 5: egress.extra is gated, whether or not the domain can
			// upload; a domain already auto stays auto.
			name:  "egress.extra is gated, and a domain that is auto stays so",
			repos: []egress.Repo{repoOf("api", nil)},
			spec:  &spec.Egress{Extra: []string{"registry.acme.dev", "storage.googleapis.com", "mise-versions.jdx.dev"}},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("registry.acme.dev", egress.ReasonExtra), gate("storage.googleapis.com", egress.ReasonExtra)},
		},
		{
			// Step 6 and 7: a kit domain with an entry and no upload is
			// auto; one that can upload, or has no entry, is gated.
			name:  "kit-declared domains are gated when they upload or have no entry",
			repos: []egress.Repo{repoOf("api", nil)},
			kit:   []string{"proxy.golang.org", "storage.googleapis.com", "kit.acme.dev"},
			auto:  append([]string{"proxy.golang.org"}, metaAuto...),
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("kit.acme.dev", egress.ReasonKit), gate("storage.googleapis.com", egress.ReasonKit)},
		},
		{
			// I10: a kit declaring x.example gates it, and says so.
			name:  "a kit that declares x.example gates it as kit-declared",
			repos: []egress.Repo{repoOf("api", nil)},
			kit:   []string{"x.example"},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("x.example", egress.ReasonKit)},
		},
		{
			// A domain the base set produced keeps upload when a kit
			// repeats it: the catalog said it first.
			name:  "a kit that repeats a base domain leaves the reason upload",
			repos: []egress.Repo{repoOf("api", nil)},
			kit:   []string{"github.com"},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload)},
		},
		{
			// A kit domain that is also a spec extra is kit-declared.
			name:  "a kit domain that egress.extra repeats is kit-declared",
			repos: []egress.Repo{repoOf("api", nil)},
			spec:  &spec.Egress{Extra: []string{"x.example"}},
			kit:   []string{"x.example"},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("x.example", egress.ReasonKit)},
		},
		{
			// Step 7 for a catalog domain without a domains entry.
			name: "a catalog domain with no entry is gated",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": "[tools]\nnode = \"1\"\n",
				"mise.lock": lockOf([3]string{"node", "core:node", ""})})},
			auto:  metaAuto,
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("nodejs.org", egress.ReasonUpload)},
		},
		{
			// A platform table that records no url (a version-only entry
			// with a checksum) adds no host and is no error.
			name: "a platform with no url adds no host",
			repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": toml,
				"mise.lock": "lockfile_version = 3\n\n[[tools.go]]\nversion = \"1.0.0\"\nbackend = \"core:go\"\n\n[tools.go.\"platforms.linux-x64\"]\nchecksum = \"sha256:" + strings.Repeat("a", 64) + "\"\n"})},
			auto:  append([]string{"dl.google.com", "proxy.golang.org"}, metaAuto...),
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("storage.googleapis.com", egress.ReasonUpload)},
		},
		{
			name: "a lock without a mise.toml is still read",
			repos: []egress.Repo{repoOf("api", map[string]string{
				"mise.lock": lockOf([3]string{"go", "core:go", "https://mirror.acme.dev/go.tar.gz"})})},
			auto:  append([]string{"dl.google.com", "proxy.golang.org"}, metaAuto...),
			gated: []egress.Gate{gate("api.github.com", egress.ReasonUpload), gate("github.com", egress.ReasonUpload), gate("mirror.acme.dev", egress.ReasonLockHost), gate("storage.googleapis.com", egress.ReasonUpload)},
		},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			got, err := egress.Derive(t.Context(), egress.Input{Repos: tt.repos, Spec: tt.spec, Kit: tt.kit, Catalog: testCatalog(t)})
			require.NoError(t, err)
			assert.Equal(t, sorted(tt.auto), got.Auto)
			assert.Equal(t, tt.gated, got.Gated)
		})
	}
}

// sorted returns the sorted copy of list.
func sorted(list []string) []string {
	out := append([]string(nil), list...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// TestDeriveUnionOfRepos: the sets of two repos are one, whatever the
// order of the repos, and so is the digest.
func TestDeriveUnionOfRepos(t *testing.T) {
	a := repoOf("api", map[string]string{"mise.toml": toml, "mise.lock": lockOf([3]string{"go", "core:go", "https://dl.google.com/go.tar.gz"})})
	b := repoOf("web", map[string]string{"mise.toml": "[tools]\nx = \"1\"\n", "mise.lock": lockOf([3]string{"x", "aqua:acme/tool", "https://dl.acme.dev/x.tar.gz"})})
	sp := &spec.Egress{Tools: map[string][]string{"aqua:acme/tool": {"dl.acme.dev"}}}
	one, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{a, b}, Spec: sp, Catalog: testCatalog(t)})
	require.NoError(t, err)
	two, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{b, a}, Spec: sp, Catalog: testCatalog(t)})
	require.NoError(t, err)
	assert.Equal(t, one, two)
	assert.Contains(t, one.Auto, "proxy.golang.org")
	assert.Contains(t, one.Gated, gate("dl.acme.dev", egress.ReasonSpecTool))
}

// TestDeriveEffectiveAndDigest: the effective set is the sorted union of
// auto and gated, and its digest is the egress digest of 03 3.12
// (07 7.5, step 8).
func TestDeriveEffectiveAndDigest(t *testing.T) {
	got, err := egress.Derive(t.Context(), egress.Input{
		Repos: []egress.Repo{repoOf("api", nil)}, Spec: &spec.Egress{Extra: []string{"registry.acme.dev"}}, Catalog: testCatalog(t)})
	require.NoError(t, err)
	want := []string{"api.github.com", "github.com", "mise-versions.jdx.dev", "registry.acme.dev", "tuf-repo-cdn.sigstore.dev"}
	assert.Equal(t, want, got.Effective())
	sum, err := got.Digest()
	require.NoError(t, err)
	wantSum, err := canon.Digest(canon.Egress, egress.EffectiveSet(want))
	require.NoError(t, err)
	assert.Equal(t, wantSum, sum)
	again, err := egress.Derive(t.Context(), egress.Input{
		Repos: []egress.Repo{repoOf("api", nil)}, Spec: &spec.Egress{Extra: []string{"registry.acme.dev", "registry.acme.dev"}}, Catalog: testCatalog(t)})
	require.NoError(t, err)
	againSum, err := again.Digest()
	require.NoError(t, err)
	assert.Equal(t, sum, againSum, "a domain written twice is one")
}

// rowOf returns the row of the error of the table err carries.
func rowOf(t *testing.T, err error) *cli.Error {
	t.Helper()
	require.Error(t, err)
	var e *cli.Error
	require.ErrorAs(t, err, &e)
	return e
}

// TestDeriveLockMissing: a mise.toml with no mise.lock is lock-missing
// (RJ-340), with the hint that locks the repo.
func TestDeriveLockMissing(t *testing.T) {
	got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": toml})}, Catalog: testCatalog(t)})
	e := rowOf(t, err)
	assert.Equal(t, "RJ-340", e.ID)
	assert.Equal(t, 2, e.Exit)
	assert.Equal(t, "julieta lock --repo api", e.Hint())
	assert.Contains(t, e.Message, "api")
	assert.Equal(t, egress.Result{}, got)
}

// TestDeriveUnknownTool: a backend:tool in no catalog entry and no
// egress.tools entry is catalog-unknown-tool (RJ-327), exit 2, naming
// each tool and the two fixes.
func TestDeriveUnknownTool(t *testing.T) {
	files := map[string]string{"mise.toml": "[tools]\nx = \"1\"\n", "mise.lock": lockOf(
		[3]string{"go", "core:go", ""},
		[3]string{"x", "aqua:acme/tool", ""},
		[3]string{"y", "aqua:acme/other", ""},
	)}
	got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", files)}, Catalog: testCatalog(t)})
	e := rowOf(t, err)
	assert.Equal(t, "RJ-327", e.ID)
	assert.Equal(t, 2, e.Exit)
	assert.Contains(t, e.Message, "aqua:acme/tool")
	assert.Contains(t, e.Message, "aqua:acme/other")
	assert.NotContains(t, e.Message, "core:go")
	assert.Contains(t, e.Message, "catalog")
	assert.Contains(t, e.Message, "egress.tools")
	assert.Equal(t, []cli.Detail{{Key: "aqua:acme/other", Value: "api"}, {Key: "aqua:acme/tool", Value: "api"}}, e.Details)
	assert.Equal(t, egress.Result{}, got, "no partial result")
}

// TestDeriveUnparsed: a lock that does not parse, or has an unknown
// top-level key, is lock-unparsed (RJ-325) naming the repo and the fix,
// with no partial result.
func TestDeriveUnparsed(t *testing.T) {
	tests := []struct{ name, lock string }{
		{"not toml", "lockfile_version = \n"},
		{"an unknown top-level key", "lockfile_version = 3\nsurprise = 1\n"},
		{"a duplicate key", "lockfile_version = 3\nlockfile_version = 3\n"},
		{"another layout", "lockfile_version = 2\n"},
		{"over the cap", "# " + strings.Repeat("x", egress.MaxFileSize) + "\n"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{
				repoOf("ok", nil), repoOf("api", map[string]string{"mise.toml": toml, "mise.lock": tt.lock})}, Catalog: testCatalog(t)})
			e := rowOf(t, err)
			assert.Equal(t, "RJ-325", e.ID)
			assert.Equal(t, 2, e.Exit)
			assert.Contains(t, e.Message, "api")
			assert.Contains(t, e.Message, "https://git.example/api.git")
			assert.Contains(t, e.Message, "relock with julieta lock")
			assert.Equal(t, "julieta lock --repo api", e.Hint())
			assert.Equal(t, egress.Result{}, got)
		})
	}
}

// TestDeriveLockHostInvalid: a host of the lock that fails the egress
// domain rule of 03 3.1 is lock-host-invalid (RJ-326) naming the entry.
func TestDeriveLockHostInvalid(t *testing.T) {
	tests := []struct{ name, url string }{
		{"an IP literal", "https://10.0.0.1/go.tar.gz"},
		{"an IPv6 literal", "https://[::1]/go.tar.gz"},
		{"a port", "https://dl.google.com:8443/go.tar.gz"},
		{"a capital", "https://DL.google.com/go.tar.gz"},
		{"a special-use name", "https://files.localhost/go.tar.gz"},
		{"a private suffix", "https://files.corp/go.tar.gz"},
		{"a single label", "https://intranet/go.tar.gz"},
		{"a scheme that is not http", "ftp://dl.google.com/go.tar.gz"},
		{"no host", "https:///go.tar.gz"},
		{"not a url", "://x"},
		{"a wildcard", "https://*.google.com/go.tar.gz"},
		{"user information", "https://user@dl.google.com/x"},
		{"a backslash", `https://dl.google.com\\@evil.example/go.tar.gz`},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			files := map[string]string{"mise.toml": toml, "mise.lock": lockOf([3]string{"go", "core:go", tt.url})}
			got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", files)}, Catalog: testCatalog(t)})
			e := rowOf(t, err)
			assert.Equal(t, "RJ-326", e.ID)
			assert.Equal(t, 2, e.Exit)
			assert.Contains(t, e.Hint(), "julieta lock --repo api")
			assert.Contains(t, e.Message, "api")
			assert.Contains(t, e.Message, `"go"`)
			assert.Contains(t, e.Message, "linux-x64")
			assert.Equal(t, egress.Result{}, got)
		})
	}
}

// TestDeriveRefusesADomainOfAnotherSource: a domain of egress.extra, an
// egress.tools entry or a kit that fails the rule is a defect of the
// caller, which decoded the spec and the kit with the same rule; it
// is refused, never allowed.
func TestDeriveRefusesADomainOfAnotherSource(t *testing.T) {
	repos := []egress.Repo{repoOf("api", nil)}
	for name, in := range map[string]egress.Input{
		"extra": {Repos: repos, Spec: &spec.Egress{Extra: []string{"*.acme.dev"}}},
		"kit":   {Repos: repos, Kit: []string{"10.0.0.1"}},
		"tools": {Repos: []egress.Repo{repoOf("api", map[string]string{"mise.toml": "[tools]\nx = \"1\"\n",
			"mise.lock": lockOf([3]string{"x", "aqua:acme/tool", ""})})}, Spec: &spec.Egress{Tools: map[string][]string{"aqua:acme/tool": {"10.0.0.1"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			in.Catalog = testCatalog(t)
			got, err := egress.Derive(t.Context(), in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "domain")
			assert.Equal(t, egress.Result{}, got)
		})
	}
}

// TestDeriveReadFailure: a reader that fails is an error of the caller,
// and one that refuses the file (over the cap, a link) is lock-unparsed.
func TestDeriveReadFailure(t *testing.T) {
	boom := egress.Repo{Dir: "api", URL: "u", Read: func(context.Context, string) ([]byte, bool, error) {
		return nil, false, assert.AnError
	}}
	_, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{boom}, Catalog: testCatalog(t)})
	require.ErrorIs(t, err, assert.AnError)
	refused := egress.Repo{Dir: "api", URL: "u", Read: func(context.Context, string) ([]byte, bool, error) {
		return nil, false, egress.ErrUnreadable
	}}
	_, err = egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{refused}, Catalog: testCatalog(t)})
	assert.Equal(t, "RJ-325", rowOf(t, err).ID)
}

// TestDeriveReasonPrecedence: a domain several sources gate keeps the
// first reason of upload, kit, extra, spec-tool and lock-host, and Auto
// and Gated share no domain.
func TestDeriveReasonPrecedence(t *testing.T) {
	files := map[string]string{"mise.toml": "[tools]\nx = \"1\"\ngo = \"1\"\n", "mise.lock": lockOf(
		[3]string{"x", "aqua:acme/tool", "https://both.acme.dev/x.tar.gz"},
		[3]string{"go", "core:go", "https://only.acme.dev/go.tar.gz"},
	)}
	sp := &spec.Egress{
		Extra: []string{"both.acme.dev", "dl.google.com", "storage.googleapis.com"},
		Tools: map[string][]string{"aqua:acme/tool": {"both.acme.dev", "tool.acme.dev", "only.acme.dev"}},
	}
	got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", files)}, Spec: sp, Catalog: testCatalog(t)})
	require.NoError(t, err)
	reasons := map[string]egress.Reason{}
	for _, g := range got.Gated {
		reasons[g.Domain] = g.Reason
		assert.NotContains(t, got.Auto, g.Domain, "disjoint")
	}
	assert.Equal(t, egress.ReasonExtra, reasons["both.acme.dev"], "extra over spec-tool and lock-host")
	assert.Equal(t, egress.ReasonSpecTool, reasons["only.acme.dev"], "spec-tool over lock-host")
	assert.Equal(t, egress.ReasonSpecTool, reasons["tool.acme.dev"])
	assert.Equal(t, egress.ReasonUpload, reasons["storage.googleapis.com"], "the catalog produced it and it can upload")
	assert.Contains(t, got.Auto, "dl.google.com", "extra repeats a domain the catalog allows")
}

// TestDeriveLockHostOfAKit: a lock host that a kit declares, with an
// entry and no upload, is allowed by the kit and stays auto.
func TestDeriveLockHostOfAKit(t *testing.T) {
	files := map[string]string{"mise.toml": toml, "mise.lock": lockOf([3]string{"go", "core:go", "https://proxy.golang.org/go.tar.gz"})}
	got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", files)}, Kit: []string{"proxy.golang.org"}, Catalog: testCatalog(t)})
	require.NoError(t, err)
	assert.Contains(t, got.Auto, "proxy.golang.org")
	for _, g := range got.Gated {
		assert.NotEqual(t, "proxy.golang.org", g.Domain)
	}
}

// TestDeriveEmptyIsNotNil: with nothing to allow or to gate, Auto and
// Gated are empty lists and not nil, so a caller that encodes them
// writes [] and not null.
func TestDeriveEmptyIsNotNil(t *testing.T) {
	got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", nil)}})
	require.NoError(t, err)
	assert.NotNil(t, got.Auto)
	assert.NotNil(t, got.Gated)
	assert.NotNil(t, got.Rules)
	assert.Empty(t, got.Auto)
	assert.Empty(t, got.Gated)
	assert.Empty(t, got.Rules)
}

// TestDeriveRules is the table of the rules romeu applies (07 7.5,
// REQ-0634, REQ-1316): every domain a source of romeu produced has a
// rule at its port, 443 for the catalog and the spec and the port of
// the scheme for a lock host, and a domain that only a kit declared has
// none, auto or gated, though it stays in the effective set.
func TestDeriveRules(t *testing.T) {
	r := func(domain string, port int) egress.Rule { return egress.Rule{Domain: domain, Port: port} }
	base := []egress.Rule{r("api.github.com", 443), r("github.com", 443), r("mise-versions.jdx.dev", 443), r("tuf-repo-cdn.sigstore.dev", 443)}
	with := func(extra ...egress.Rule) []egress.Rule {
		out := append(append([]egress.Rule(nil), base...), extra...)
		slices.SortFunc(out, func(a, b egress.Rule) int {
			return cmp.Or(strings.Compare(a.Domain, b.Domain), cmp.Compare(a.Port, b.Port))
		})
		return out
	}
	goLock := func(url string) map[string]string {
		return map[string]string{"mise.toml": toml, "mise.lock": lockOf([3]string{"go", "core:go", url})}
	}
	tests := []struct {
		name      string
		files     map[string]string
		spec      *spec.Egress
		kit       []string
		rules     []egress.Rule
		effective []string // domains that must stay in the effective set
	}{
		{
			name:      "a domain only a kit declared and the catalog allows has no rule",
			kit:       []string{"proxy.golang.org"},
			rules:     base,
			effective: []string{"proxy.golang.org"},
		},
		{
			name:      "a domain only a kit declared that is gated has no rule",
			kit:       []string{"kit.acme.dev", "storage.googleapis.com"},
			rules:     base,
			effective: []string{"kit.acme.dev", "storage.googleapis.com"},
		},
		{
			name:  "a kit domain the catalog also produced has the rule of the catalog",
			files: goLock(""),
			kit:   []string{"proxy.golang.org"},
			rules: with(r("dl.google.com", 443), r("proxy.golang.org", 443), r("storage.googleapis.com", 443)),
		},
		{
			name:  "a kit domain a lock also names has the rule of the lock host",
			files: goLock("https://mirror.acme.dev/go.tar.gz"),
			kit:   []string{"mirror.acme.dev"},
			rules: with(r("dl.google.com", 443), r("mirror.acme.dev", 443), r("proxy.golang.org", 443), r("storage.googleapis.com", 443)),
		},
		{
			name:  "a kit domain egress.extra repeats has the rule of the spec",
			spec:  &spec.Egress{Extra: []string{"x.example"}},
			kit:   []string{"x.example"},
			rules: with(r("x.example", 443)),
		},
		{
			name: "an egress.tools domain has a rule at 443",
			files: map[string]string{"mise.toml": "[tools]\nx = \"1\"\n",
				"mise.lock": lockOf([3]string{"x", "aqua:acme/tool", ""})},
			spec:  &spec.Egress{Tools: map[string][]string{"aqua:acme/tool": {"dl.acme.dev"}}},
			rules: with(r("dl.acme.dev", 443)),
		},
		{
			name:  "a lock host on http has a rule at 80",
			files: goLock("http://mirror.acme.dev/go.tar.gz"),
			rules: with(r("dl.google.com", 443), r("mirror.acme.dev", 80), r("proxy.golang.org", 443), r("storage.googleapis.com", 443)),
		},
		{
			name:  "a catalog domain a lock reaches on http has both rules",
			files: goLock("http://dl.google.com/go.tar.gz"),
			rules: with(r("dl.google.com", 80), r("dl.google.com", 443), r("proxy.golang.org", 443), r("storage.googleapis.com", 443)),
		},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			got, err := egress.Derive(t.Context(), egress.Input{Repos: []egress.Repo{repoOf("api", tt.files)}, Spec: tt.spec, Kit: tt.kit, Catalog: testCatalog(t)})
			require.NoError(t, err)
			assert.Equal(t, tt.rules, got.Rules)
			for _, dom := range tt.effective {
				assert.Contains(t, got.Effective(), dom)
			}
			for _, rule := range got.Rules {
				assert.Contains(t, got.Effective(), rule.Domain, "a rule is for a derived domain")
			}
		})
	}
}
