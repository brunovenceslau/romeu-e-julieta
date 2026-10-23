// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// flipCase is one row of the generated flip table (flip_gen_test.go):
// a top-level field of spec.Project and whether a change to it flips
// the widening digest and the recreate digest.
type flipCase struct {
	field    string
	widening bool
	recreate bool
}

// inputs are the three inputs of Build.
type inputs struct {
	p  spec.Project
	hs spec.HostSettings
	r  Resolved
}

// baseInputs returns a project with every field set, its host settings
// and its resolved inputs.
func baseInputs() inputs {
	return inputs{
		p: spec.Project{
			Schema:   "project/v1",
			Name:     "shop",
			Repos:    []spec.Repo{{URL: "https://github.com/example/shop-api", Dir: "shop-api", Primary: true, Ref: "main"}},
			Workload: "docker.io/example/dev@sha256:" + hex64('a'),
			Agent:    "claude",
			Kits: []spec.Kit{
				{Product: "julieta"},
				{Personal: "tools", Args: map[string]string{"level": "1"}},
			},
			SandboxOptions: &spec.SandboxOptions{CPUs: 4, Memory: "8g"},
			Secrets:        []string{"github"},
			Egress:         &spec.Egress{Extra: []string{"x.example"}, Tools: map[string][]string{"aqua:foo/bar": {"foo.example"}}},
			Ports:          []string{"8080:8080/tcp"},
			Salvage:        &spec.Salvage{ExcludeIgnored: []string{"node_modules"}},
			Run:            &spec.Run{Panes: []spec.Pane{{ID: "main", Repo: "shop-api"}}},
		},
		hs: spec.HostSettings{Secrets: map[string]spec.Secret{
			"github@shop": {Service: "github.com", Argv: []string{"/usr/bin/env", "-i", "/opt/homebrew/bin/gh", "auth", "token"}},
		}},
		r: Resolved{
			WorkloadCapabilities: oci.Capabilities{Kind: "workload", Network: &oci.Network{Runtime: oci.Rules{Allow: []oci.NetEntry{{Hosts: []string{"registry.example"}}}}}},
			Kits: []ResolvedKit{
				{Source: SourceProduct, ID: "julieta", Files: []File{{Path: "spec.yaml", Mode: ModeRegular, Data: []byte("kind: kit\n")}}},
				{Source: SourcePersonal, ID: "tools", Files: []File{{Path: "bin/run", Mode: ModeExecutable, Data: []byte("#!/bin/sh\n")}}, Capabilities: oci.Capabilities{Kind: "mixin", Lifecycle: &oci.Lifecycle{Interactive: []string{"bash"}}}},
			},
			EgressGated: []GatedDomain{{Domain: "github.com", Reason: ReasonUpload}},
			Render:      Render{"kits": []any{"julieta", "tools"}, "sandboxOptions": map[string]any{"cpus": 4, "memory": "8g"}},
		},
	}
}

// digests returns the widening and recreate digests of in.
func digests(t *testing.T, in inputs) (widening, recreate string) {
	t.Helper()
	s, err := Build(in.p, in.hs, in.r)
	require.NoError(t, err)
	w, err := Digest(s)
	require.NoError(t, err)
	r, err := RecreateDigest(in.p, s)
	require.NoError(t, err)
	return w.String(), r.String()
}

// fieldChanges change one top-level field of spec.Project each, as an
// edit of the spec does, together with what that edit changes in the
// render. A field of the generated table with no entry here fails the
// flip test, so a new field is never passed by omission.
var fieldChanges = map[string]func(*inputs){
	"name": func(in *inputs) {
		in.p.Name = "shop2"
		in.hs.Secrets["github@shop2"] = in.hs.Secrets["github@shop"]
	},
	"repos":    func(in *inputs) { in.p.Repos[0].URL = "https://github.com/example/other" },
	"workload": func(in *inputs) { in.p.Workload = "docker.io/example/dev@sha256:" + hex64('b') },
	"agent":    func(in *inputs) { in.p.Agent = "codex" },
	"kits":     func(in *inputs) { in.p.Kits[1].Args = map[string]string{"level": "2"} },
	"sandboxOptions": func(in *inputs) {
		in.p.SandboxOptions.CPUs = 8
		in.r.Render["sandboxOptions"] = map[string]any{"cpus": 8, "memory": "8g"}
	},
	"secrets": func(in *inputs) {
		in.p.Secrets = append(in.p.Secrets, "npm")
		in.hs.Secrets["npm@shop"] = spec.Secret{Service: "npmjs.org", Argv: []string{"/usr/bin/false"}}
	},
	"egress":  func(in *inputs) { in.p.Egress.Extra = append(in.p.Egress.Extra, "y.example") },
	"ports":   func(in *inputs) { in.p.Ports = append(in.p.Ports, "9090:9090/tcp") },
	"salvage": func(in *inputs) { in.p.Salvage.ExcludeIgnored = []string{".cache"} },
	"run":     func(in *inputs) { in.p.Run.Panes[0].ID = "other" },
}

//romeu:invariant I7 test
func TestFlipTable(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, flipTable)
	for _, tt := range flipTable {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()
			change, ok := fieldChanges[tt.field]
			require.True(t, ok, "the field %s has no change in fieldChanges", tt.field)
			baseW, baseR := digests(t, baseInputs())
			in := baseInputs()
			change(&in)
			w, r := digests(t, in)
			assert.Equal(t, tt.widening, w != baseW, "the widening digest flips on %s: want %t", tt.field, tt.widening)
			assert.Equal(t, tt.recreate, r != baseR, "the recreate digest flips on %s: want %t", tt.field, tt.recreate)
		})
	}
}

// TestFlipTableOfTheResolvedMembers holds the members of the widening
// set that I7 names below a field: what romeu resolved for the spec,
// which no spec edit shows.
//
//romeu:invariant I7 test
func TestFlipTableOfTheResolvedMembers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		change   func(*inputs)
		recreate bool
	}{
		{"a repo's ref", func(in *inputs) { in.p.Repos[0].Ref = "develop" }, true},
		{"a secret's argv", func(in *inputs) {
			in.hs.Secrets["github@shop"] = spec.Secret{Service: "github.com", Argv: []string{"/usr/bin/env", "-i", "/usr/local/bin/gh", "auth", "token"}}
		}, true},
		{"a secret's service", func(in *inputs) {
			in.hs.Secrets["github@shop"] = spec.Secret{Service: "ghe.example", Argv: in.hs.Secrets["github@shop"].Argv}
		}, true},
		{"the workload repository", func(in *inputs) { in.p.Workload = "docker.io/other/dev@sha256:" + hex64('a') }, true},
		{"the workload capabilities", func(in *inputs) {
			in.r.WorkloadCapabilities = oci.Capabilities{Kind: "workload", Network: &oci.Network{Runtime: oci.Rules{Allow: []oci.NetEntry{{Hosts: []string{"other.example"}}}}}}
		}, true},
		{"a kit's content", func(in *inputs) { in.r.Kits[1].Files[0].Data = []byte("#!/bin/sh\nexit 1\n") }, true},
		{"a kit file's mode", func(in *inputs) { in.r.Kits[1].Files[0].Mode = ModeRegular }, true},
		{"a kit's capabilities", func(in *inputs) {
			in.r.Kits[1].Capabilities = oci.Capabilities{Kind: "mixin", Lifecycle: &oci.Lifecycle{Interactive: []string{"zsh"}}}
		}, true},
		{"the order of the kits", func(in *inputs) {
			in.p.Kits[0], in.p.Kits[1] = in.p.Kits[1], in.p.Kits[0]
			in.r.Kits[0], in.r.Kits[1] = in.r.Kits[1], in.r.Kits[0]
		}, true},
		{"a derived gated domain", func(in *inputs) {
			in.r.EgressGated = append(in.r.EgressGated, GatedDomain{Domain: "kit.example", Reason: ReasonKitDeclared})
		}, false},
		{"a spec tool override", func(in *inputs) { in.p.Egress.Tools["aqua:foo/bar"] = []string{"bar.example"} }, false},
		{"the gated part of the render", func(in *inputs) { in.r.Render["kits"] = []any{"julieta"} }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseW, baseR := digests(t, baseInputs())
			in := baseInputs()
			tt.change(&in)
			w, r := digests(t, in)
			assert.NotEqual(t, baseW, w, "the widening digest flips")
			assert.Equal(t, tt.recreate, r != baseR, "the recreate digest flips: want %t", tt.recreate)
		})
	}
}

func TestRenderDigestStripsOnlyTheUngatedFields(t *testing.T) {
	t.Parallel()
	base := Render{"kits": []any{"a"}, "sandboxOptions": map[string]any{"cpus": 4}, "run": "x"}
	stripped := Render{"kits": []any{"a"}}
	a, err := RenderDigest(base)
	require.NoError(t, err)
	b, err := RenderDigest(stripped)
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Contains(t, base, "sandboxOptions", "the input is not changed")

	_, err = RenderDigest(nil)
	assert.Error(t, err)
}

func TestBuildRefusesWhatItCannotAccountFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*inputs)
	}{
		{"a secret with no host-settings entry", func(in *inputs) { delete(in.hs.Secrets, "github@shop") }},
		{"a secret bound to another project", func(in *inputs) {
			in.hs.Secrets = map[string]spec.Secret{"github@other": in.hs.Secrets["github@shop"]}
		}},
		{"a secret with an empty argv", func(in *inputs) { in.hs.Secrets["github@shop"] = spec.Secret{Service: "github.com"} }},
		{"a workload without a digest", func(in *inputs) { in.p.Workload = "docker.io/example/dev" }},
		{"a workload pinned by a tag", func(in *inputs) { in.p.Workload = "docker.io/example/dev@latest" }},
		{"a workload with a short digest", func(in *inputs) { in.p.Workload = "docker.io/example/dev@sha256:abc" }},
		{"resolved kits in another order", func(in *inputs) { in.r.Kits[0], in.r.Kits[1] = in.r.Kits[1], in.r.Kits[0] }},
		{"a resolved kit of another source", func(in *inputs) { in.r.Kits[0].Source = SourcePersonal }},
		{"a capability integer a float64 cannot hold", func(in *inputs) {
			// The structure of a credential file is the one member that
			// holds any JSON value.
			in.r.WorkloadCapabilities = oci.Capabilities{Kind: "workload", Credentials: []oci.Credential{{
				Service: "svc", Phases: []string{"runtime"},
				OAuth: &oci.OAuth{CredentialFile: &oci.CredentialFile{Path: "/a.json", Structure: map[string]any{"uid": int64(1)<<60 + 1}}},
			}}}
		}},
		{"a kit capability integer a float64 cannot hold", func(in *inputs) {
			in.r.Kits[1].Capabilities = oci.Capabilities{Kind: "mixin", Credentials: []oci.Credential{{
				Service: "svc", Phases: []string{"runtime"},
				OAuth: &oci.OAuth{CredentialFile: &oci.CredentialFile{Path: "/a.json", Structure: map[string]any{"n": map[string]any{"m": uint64(1)<<63 + 1}}}},
			}}}
		}},

		{"a kit naming both sources", func(in *inputs) { in.p.Kits[0].Personal = "x" }},
		{"a kit naming no source", func(in *inputs) { in.p.Kits[0] = spec.Kit{} }},
		{"fewer resolved kits than kits", func(in *inputs) { in.r.Kits = in.r.Kits[:1] }},
		{"a kit file of another mode", func(in *inputs) { in.r.Kits[0].Files[0].Mode = "120000" }},
		{"a repo without a dir", func(in *inputs) { in.p.Repos[0].Dir = "" }},
		{"no render", func(in *inputs) { in.r.Render = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := baseInputs()
			tt.change(&in)
			_, err := Build(in.p, in.hs, in.r)
			assert.Error(t, err)
		})
	}
}

func TestBuildAddsTheSpecDomainsWithTheirReasons(t *testing.T) {
	t.Parallel()
	in := baseInputs()
	in.p.Egress.Extra = append(in.p.Egress.Extra, "x.example")
	s, err := Build(in.p, in.hs, in.r)
	require.NoError(t, err)
	assert.Equal(t, []GatedDomain{
		{Domain: "foo.example", Reason: ReasonSpecToolOverride},
		{Domain: "github.com", Reason: ReasonUpload},
		{Domain: "x.example", Reason: ReasonSpecExtra},
	}, s.EgressGated)
}

func TestRecreateDigestRefusesTheSetOfAnotherProject(t *testing.T) {
	t.Parallel()
	in := baseInputs()
	s, err := Build(in.p, in.hs, in.r)
	require.NoError(t, err)
	other := in.p
	other.Name = "other"
	_, err = RecreateDigest(other, s)
	assert.Error(t, err)
}

// TestEveryGatedLeafReachesTheSet changes, one at a time, every leaf
// below a gated field of spec.Project (a string, a bool or an int, at
// any depth, in each element of a list and each value of a map) and
// requires that the widening digest flips or that Build refuses the
// change: a new member of spec.Repo, spec.Kit or spec.Egress that Build
// does not read fails here (I7).
//
//romeu:invariant I7 test
func TestEveryGatedLeafReachesTheSet(t *testing.T) {
	t.Parallel()
	base, _ := digests(t, baseInputs())
	for _, field := range wideningFields {
		count := leafCount(t, field)
		require.Positive(t, count, "the fixture sets no leaf of %s", field)
		for i := range count {
			in := baseInputs()
			path := mutateLeaf(t, &in.p, field, i)
			t.Run(path, func(t *testing.T) {
				s, err := Build(in.p, in.hs, in.r)
				if err != nil {
					return // Build reads the leaf: it refuses the change.
				}
				got, err := Digest(s)
				require.NoError(t, err)
				assert.NotEqual(t, base, got.String(), "a change of %s leaves the widening digest as it was", path)
			})
		}
	}
}

// leafCount returns how many leaves the base fixture holds below the
// top-level field of spec.Project.
func leafCount(t *testing.T, field string) int {
	t.Helper()
	in := baseInputs()
	n := 0
	walkLeaves(fieldValue(t, &in.p, field), field, func(string, reflect.Value) { n++ })
	return n
}

// mutateLeaf changes the leaf number i below field in p and returns its
// path.
func mutateLeaf(t *testing.T, p *spec.Project, field string, i int) string {
	t.Helper()
	n, path := 0, ""
	walkLeaves(fieldValue(t, p, field), field, func(at string, v reflect.Value) {
		if n == i {
			path = at
			switch v.Kind() {
			case reflect.String:
				if v.Len() == 0 {
					v.SetString("x")
				} else {
					b := []byte(v.String())
					b[0]++
					v.SetString(string(b))
				}
			case reflect.Bool:
				v.SetBool(!v.Bool())
			case reflect.Int:
				v.SetInt(v.Int() + 1)
			default:
				t.Fatalf("%s: a leaf of the kind %s", at, v.Kind())
			}
		}
		n++
	})
	return path
}

// fieldValue returns the field of p whose JSON name is name, settable.
func fieldValue(t *testing.T, p *spec.Project, name string) reflect.Value {
	t.Helper()
	v := reflect.ValueOf(p).Elem()
	for i := range v.NumField() {
		tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
		if tag == name {
			return v.Field(i)
		}
	}
	t.Fatalf("spec.Project has no field %s", name)
	return reflect.Value{}
}

// walkLeaves calls visit with each settable leaf below v, in a fixed
// order; a map's values are copied out, changed and stored back, since
// a map value is not settable.
func walkLeaves(v reflect.Value, at string, visit func(string, reflect.Value)) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			walkLeaves(v.Elem(), at, visit)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			walkLeaves(v.Field(i), at+"."+tag, visit)
		}
	case reflect.Slice:
		for i := range v.Len() {
			walkLeaves(v.Index(i), fmt.Sprintf("%s[%d]", at, i), visit)
		}
	case reflect.Map:
		for _, k := range slices.SortedFunc(slices.Values(v.MapKeys()), func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) }) {
			elem := reflect.New(v.Type().Elem()).Elem()
			elem.Set(v.MapIndex(k))
			if elem.Kind() == reflect.Slice {
				elem.Set(reflect.AppendSlice(reflect.MakeSlice(elem.Type(), 0, elem.Len()), elem))
			}
			walkLeaves(elem, at+"{"+k.String()+"}", func(path string, leaf reflect.Value) {
				visit(path, leaf)
				v.SetMapIndex(k, elem)
			})
		}
	default:
		visit(at, v)
	}
}
