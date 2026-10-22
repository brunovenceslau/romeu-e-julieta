// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"math"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeOf returns the Go type of the format f.
func typeOf(f Format) reflect.Type {
	if f == FormatHostSettings {
		return reflect.TypeFor[HostSettings]()
	}
	return reflect.TypeFor[Project]()
}

// rulePaths returns every rule path of the type t, as walk names them,
// with the kind of the value at each.
func rulePaths(t reflect.Type) map[string]reflect.Kind {
	out := map[string]reflect.Kind{}
	var visit func(t reflect.Type, p string, first map[reflect.Type]string)
	visit = func(t reflect.Type, p string, first map[reflect.Type]string) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			if _, ok := first[t]; ok {
				return // the type's paths are those of its first place
			}
			first = withType(first, t, p)
		}
		out[p] = t.Kind()
		switch t.Kind() {
		case reflect.Struct:
			for f := range t.Fields() {
				visit(f.Type, join(p, jsonName(f)), first)
			}
		case reflect.Slice:
			visit(t.Elem(), p+"[]", first)
			visit(t.Elem(), p+"[0]", first)
		case reflect.Map:
			out[p+"{key}"] = reflect.String
			visit(t.Elem(), p+"{}", first)
		}
	}
	visit(t, "", map[reflect.Type]string{})
	return out
}

// TestRulesAreWellFormed holds the rule table to its own form: a unique
// slug, a field of the format's type (or projects/, the directory of
// the set rules), at least one constraint, each constraint on a value of
// the kind it reads, anchored patterns that compile, a message and a
// scope.
func TestRulesAreWellFormed(t *testing.T) {
	slugs := map[string]bool{}
	for _, r := range rules {
		t.Run(r.Slug, func(t *testing.T) {
			assert.False(t, slugs[r.Slug], "the slug is unique")
			slugs[r.Slug] = true
			assert.Regexp(t, `^[a-z0-9]+(-[a-z0-9]+)*$`, r.Slug)
			assert.Contains(t, []Format{FormatProject, FormatHostSettings}, r.Format)
			assert.Contains(t, []Scope{Both, RomeuOnly}, r.Scope)
			assert.Regexp(t, `^(REQ-[0-9]{4}|0[0-9] [0-9]+\.[0-9]+)$`, r.Source)
			assert.NotEmpty(t, r.Message)
			assert.NotContains(t, r.Message+r.Text, "`", "the rule table prints the cells in a table row")
			assert.NotContains(t, r.Message+r.Text, "|", "the rule table prints the cells in a table row")
			kinds := rulePaths(typeOf(r.Format))
			kind, ok := kinds[r.Field]
			if r.Field != "projects/" {
				require.True(t, ok, "the field %q is a path of the format's type", r.Field)
			}
			constraints := 0
			for _, p := range []string{r.Pattern, r.NotPattern} {
				if p != "" {
					constraints++
					_, err := regexp.Compile(p)
					require.NoError(t, err)
					assert.True(t, strings.HasPrefix(p, "^") && strings.HasSuffix(p, "$") || p == r.NotPattern, "a pattern is anchored")
				}
			}
			if r.Pattern != "" || r.NotPattern != "" || r.MaxBytes != 0 || len(r.Reserved) > 0 || len(r.Enum) > 0 || r.value != nil {
				assert.Equal(t, reflect.String, kind, "a string constraint is on a string")
			}
			for _, set := range []bool{r.MaxBytes != 0, len(r.Reserved) > 0, len(r.Enum) > 0, r.value != nil, r.Range,
				r.MinItems > 0, len(r.OneOf) > 0, r.inProject != nil, r.inSet != nil} {
				if set {
					constraints++
				}
			}
			assert.Positive(t, constraints, "the rule has a constraint")
			if r.Range {
				assert.Contains(t, []reflect.Kind{reflect.Float64, reflect.Int}, kind)
			}
			if r.MinItems > 0 {
				assert.Equal(t, reflect.Slice, kind)
			}
			if len(r.OneOf) > 0 {
				assert.Equal(t, reflect.Struct, kind)
			}
			if r.inSet == nil && r.Format == FormatProject {
				assert.Equal(t, Both, r.Scope, "a rule that reads one file is checked by both validators")
			}
		})
	}
}

// ruleBySlug returns the rule of Rules with the slug s.
func ruleBySlug(t *testing.T, s string) Rule {
	t.Helper()
	i := slices.IndexFunc(rules, func(r Rule) bool { return r.Slug == s })
	require.GreaterOrEqual(t, i, 0, "a rule %q", s)
	return rules[i]
}

// accepts reports whether the value rule r accepts the string v.
func accepts(r Rule, v string) bool { return checkOne(r, reflect.ValueOf(v)) }

// checkOne reports whether the rule r accepts the value v.
func checkOne(r Rule, v reflect.Value) bool {
	re, err := compiled()
	if err != nil {
		panic(err)
	}
	return check(r, v, re) == ""
}

// TestNameAndPathRules is the table of the name and path rules of 03
// 3.1 (I12): for each rule, values it accepts and values it refuses,
// each refusal one reason.
//
//romeu:invariant I12 test
func TestNameAndPathRules(t *testing.T) {
	cases := map[string]struct{ ok, bad []string }{
		"project-name": {
			ok:  []string{"shop", "my-config", "a1", "a" + strings.Repeat("b", 39)},
			bad: []string{"", "a", "1shop", "Shop", "shop-", "-shop", "sh_op", "sh.op", "a" + strings.Repeat("b", 40), "shop\n"},
		},
		"repo-dir": {
			ok:  []string{"shop-api", "a", "0", "a.b_c-d", "memory2", strings.Repeat("a", 63)},
			bad: []string{"", ".", "..", "a/b", "/a", "Shop", "-a", ".hidden", "memory", "MEMORY", "review", "ledger", ".romeu", "SbxEnv.yaml", ".git", strings.Repeat("a", 64)},
		},
		"repo-url": {
			ok: []string{"https://github.com/example/shop-api", "https://github.com/example/shop-api.git", "https://GitHub.com/Example/Shop", "https://ghe.example.com/o/r"},
			bad: []string{"http://github.com/a/b", "git@github.com:a/b", "https://gitlab.com/group/sub/repo", "https://github.com/a", "https://github.com:443/a/b",
				"https://user@github.com/a/b", "https://github.com/a/b?x=1", "https://github.com/a/b#f", "https://github.com/../b", "https://github.com/a/.git", "https://localhost/a/b", "https://1.2.3.4/a/b", "https://github.com/a/b/"},
		},
		"repo-ref": {
			ok:  []string{"main", "feature/x", "a@b", "é", "a/@", "HEAD/x", "v1.2", "release-1.0"},
			bad: []string{"", "-x", "HEAD", "@", "a@{b", "a..b", ".a", "a/.b", "a.lock", "a/b.lock", "a/", "/a", "a//b", "a.", "a b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", "a\\b", "a\x7fb", "a\tb", "\xff"},
		},
		"workload": {
			ok: []string{"docker.io/docker/sbx-kit-claude@sha256:" + strings.Repeat("a", 64), "ghcr.io/o/r/sub@sha256:" + strings.Repeat("0", 64),
				"registry.example.com:5000/a__b@sha256:" + strings.Repeat("f", 64)},
			bad: []string{"docker.io/docker/sbx-kit-claude:latest", "docker/sbx-kit-claude@sha256:" + strings.Repeat("a", 64),
				"docker.io/docker/sbx-kit-claude@sha256:" + strings.Repeat("A", 64), "docker.io/docker/sbx-kit-claude@sha256:" + strings.Repeat("a", 63),
				"docker.io/Docker/x@sha256:" + strings.Repeat("a", 64)},
		},
		"kit-product": {ok: []string{"julieta", "os-base", "0", strings.Repeat("a", 63)}, bad: []string{"", "-a", "A", "a_b", "a.b", strings.Repeat("a", 64)}},
		"kit-arg-key": {ok: []string{"tz", "signingKey", "a_b", "a" + strings.Repeat("b", 31)}, bad: []string{"", "Tz", "1a", "a-b", "a" + strings.Repeat("b", 32)}},
		"kit-arg-value": {
			ok:  []string{"", "Etc/UTC", "ssh-ed25519 AAAA", "é", strings.Repeat("a", 1024)},
			bad: []string{"a\nb", "a\rb", "a\tb", "\x00", "\u200e", "\xff", strings.Repeat("a", 1025)},
		},
		"secret-name": {ok: []string{"github", "npm_token", "a-b", "0"}, bad: []string{"", "GitHub", "_a", "a.b", "a@b", strings.Repeat("a", 64)}},
		"tab-label":   {ok: []string{"main", "a_b-c", strings.Repeat("a", 32)}, bad: []string{"", "Main", "-a", "a b", strings.Repeat("a", 33)}},
		"pane-id":     {ok: []string{"agent", "0", "a_b"}, bad: []string{"", "_a", "Agent", "a.b", strings.Repeat("a", 33)}},
		"pane-cwd": {
			ok:  []string{"app", "a/b", ".", "a..b", "..a"},
			bad: []string{"/abs", "..", "../a", "a/../b", "a/", "a//b", "./a", "a/.", "a\nb"},
		},
		"egress-extra": {
			ok:  []string{"api.stripe.com", "a.b", "xn--bcher-kva.example", "a-b.example", "0.example.com"},
			bad: []string{"", "localhost", "*.example.com", "example.com:443", "1.2.3.4", "[::1]", "::1", "Example.com", "a..b", ".a.b", "a.b.", "-a.b", "a-.b", "ab--cd.example", "a_b.example", "a.123", strings.Repeat("a", 64) + ".com", "é.example"},
		},
		"pane-env-key": {ok: []string{"PORT", "_X", "A1", "PATHX", "XHOME"}, bad: []string{"", "port", "1A", "A-B", "PATH", "HOME", "BASH_ENV", "ENV", "LD_PRELOAD", "LD_"}},
		"secret-key": {
			ok:  []string{"github@shop", "npm_token@my-config"},
			bad: []string{"github", "@shop", "github@", "github@Shop", "github@shop@x", "GitHub@shop", "github@shop-"},
		},
		"secret-argv0": {ok: []string{"/usr/bin/security", "/Applications/1Password.app/x"}, bad: []string{"security", "./x", "/a/../b", "/a/", "", "/a\nb"}},
		"secret-in-argv": {
			ok: []string{"read", "op://dev/npm/token", "ghp", "xghp_a", "a=xghp_b", strings.Repeat("a", 39), strings.Repeat("a", 41), "x" + strings.Repeat("a", 40),
				strings.Repeat("g", 40)},
			bad: []string{"ghp_x", "gho_x", "ghs_x", "ghu_x", "ghr_x", "github_pat_x", strings.Repeat("0", 40), "0123456789abcdef0123456789abcdef01234567",
				"--token=ghp_x", "Authorization: Bearer ghp_x", " ghp_x", "ghp_x ", "\tgho_x", "TOKEN=" + strings.Repeat("A", 40), strings.Repeat("F", 40), " " + strings.Repeat("a", 40) + " "},
		},
		"credential-helper": {
			ok:  []string{"osxkeychain", "manager-core", "/usr/local/bin/helper", "/opt/git-credential-x_1.2+b"},
			bad: []string{"", "Osx", "a b", "rel/path", "/a/../b", "/usr/bin/x y", "/bin/x;rm", "/bin/$(x)", "/bin/x|y", "/bin/`x`", "/bin/x&", "/bin/x'y"},
		},
		"agent":           {ok: []string{"sbx-kit-claude", "a", "a.b_c-1"}, bad: []string{"", "-a", "A", "a b", "a\nb", ".a", strings.Repeat("a", 64)}},
		"port":            {ok: []string{"8080", "8080:8080", "8080:80/tcp", "53:53/udp", "1:65535"}, bad: []string{"", "0", "65536", "8080:0", "a:1", "8080/sctp", "-1", "1:2:3", "8080:8080/tcp\n"}},
		"memory":          {ok: []string{"8g", "512m", "1024", "2G", "1k"}, bad: []string{"", "0", "08g", "8gb", "8 g", "-8g", "1.5g", "8t"}},
		"salvage-exclude": {ok: []string{"node_modules/", ".venv/", "target", "a/b/", "*.log"}, bad: []string{"", "/abs/", "../x", "a/../b", "a//b", "a\nb", "-x"}},
		"egress-tool":     {ok: []string{"core:go", "aqua:jqlang/jq", "http:internal-cli", "npm:@scope/pkg"}, bad: []string{"", "go", ":go", "core:", "Core:go", "core:go x", "-a:b", "core:go\n"}},
		"secret-service":  {ok: []string{"github", "npm", "a_b-c"}, bad: []string{"", "-x", "GitHub", "a b", "a\nb"}},
		"step-arg":        {ok: []string{"npm", "", "--flag=x", "a b", "é"}, bad: []string{"a\nb", "a\x00b", "\x1b[31m", "a\rb"}},
		"step-argv0":      {ok: []string{"npm", "/usr/bin/env"}, bad: []string{"", "-x"}},
		"pane-env-value":  {ok: []string{"", "3000", "a b", "é"}, bad: []string{"a\nb", "\x00", "\x1b"}},
		"git-host":        {ok: []string{"github.com", "ghe.example.com"}, bad: []string{"GitHub.com", "github.com:443", "localhost", "*.github.com"}},
		"workload-allowlist": {
			ok:  []string{"docker.io/docker/sbx-kit-claude", "ghcr.io/a/b/c"},
			bad: []string{"docker/sbx-kit-claude", "docker.io/docker/sbx-kit-claude@sha256:" + strings.Repeat("a", 64), "docker.io/Docker/x", "docker.io"},
		},
	}
	for slug, c := range cases {
		r := ruleBySlug(t, slug)
		for _, v := range c.ok {
			assert.True(t, accepts(r, v), "%s accepts %q", slug, v)
		}
		for _, v := range c.bad {
			assert.False(t, accepts(r, v), "%s refuses %q", slug, v)
		}
	}
	assert.False(t, accepts(ruleBySlug(t, "node-split"), "diagonal"))
	assert.True(t, accepts(ruleBySlug(t, "node-split"), "row"))
	for v, ok := range map[float64]bool{0.1: true, 0.5: true, 0.9: true, 0.09: false, 0.91: false, 0: false, 1: false, math.NaN(): false, math.Inf(1): false} {
		assert.Equal(t, ok, checkOne(ruleBySlug(t, "node-ratio"), reflect.ValueOf(v)), "node-ratio and %v", v)
	}
	for v, ok := range map[int]bool{1: true, 4: true, 256: true, 0: false, -1: false, 257: false} {
		assert.Equal(t, ok, checkOne(ruleBySlug(t, "cpus"), reflect.ValueOf(v)), "cpus and %v", v)
	}
}

// FuzzValueRules is the fuzz target of the value rules of Rules (10
// 10.1, Fuzz; I12): no rule panics on any string, and a value a rule of
// 03 3.1 accepts has the properties the rule promises. Its seeds are
// committed under testdata/fuzz/FuzzValueRules.
func FuzzValueRules(f *testing.F) {
	for _, s := range []string{"shop", "..", "a/../b", "https://github.com/a/b", "main", "a.lock", "xn--a.b", "PATH", "ghp_x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, r := range rules {
			if r.inProject != nil || r.inSet != nil || len(r.OneOf) > 0 || r.Range || r.MinItems > 0 {
				continue
			}
			if !accepts(r, s) {
				continue
			}
			switch r.Slug {
			case "repo-dir":
				if strings.ContainsAny(s, "/\\") || s == "." || s == ".." || strings.EqualFold(s, ".git") {
					t.Fatalf("repo-dir accepts %q", s)
				}
			case "pane-cwd":
				if path.IsAbs(s) || slices.Contains(strings.Split(s, "/"), "..") {
					t.Fatalf("pane-cwd accepts %q", s)
				}
			case "repo-ref", "config-ref":
				if strings.Contains(s, "..") || strings.HasPrefix(s, "-") || strings.ContainsAny(s, " ~^:?*[\\\x00\n") {
					t.Fatalf("%s accepts %q", r.Slug, s)
				}
			case "egress-extra", "egress-tool-domain", "git-host":
				if s != strings.ToLower(s) || strings.ContainsAny(s, ":*/[] ") {
					t.Fatalf("%s accepts %q", r.Slug, s)
				}
			case "repo-url", "config-url":
				if !strings.HasPrefix(s, "https://") || strings.Count(s, "/") != 4 || strings.ContainsAny(s, "@?# ") {
					t.Fatalf("%s accepts %q", r.Slug, s)
				}
			case "secret-in-argv":
				if strings.HasPrefix(s, "ghp_") || strings.HasPrefix(s, "github_pat_") {
					t.Fatalf("secret-in-argv accepts %q", s)
				}
			case "pane-env-key":
				if s == "PATH" || s == "HOME" || strings.HasPrefix(s, "LD_") {
					t.Fatalf("pane-env-key accepts %q", s)
				}
			}
		}
	})
}
