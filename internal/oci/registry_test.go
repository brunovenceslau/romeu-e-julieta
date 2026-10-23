// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

const fixturePath = "../../kits/git-ssh-sign/git-ssh-sign.yaml"

// publish adds a two-platform image carrying the sitting fixture, and
// returns the digest of its index and the digest of the amd64 manifest.
func publish(t *testing.T, f *fakeRegistry) (index, amd string) {
	t.Helper()
	ann := annotationOf(t, fixturePath)
	amd = f.add(mediaOCIManifest, manifestJSON(t, ann))
	arm := f.add(mediaOCIManifest, manifestJSON(t, ann+" "))
	attest := f.add(mediaOCIManifest, []byte(`{"schemaVersion":2}`))
	index = f.add(mediaOCIIndex, indexJSON(t,
		entry(amd, "linux", "amd64"),
		entry(arm, "linux", "arm64"),
		map[string]any{"mediaType": mediaOCIManifest, "digest": attest, "size": 10}))
	return index, amd
}

func TestFetchWorkloadReadsTheDescriptorOfAnIndex(t *testing.T) {
	f := newFakeRegistry(t)
	index, amd := publish(t, f)

	got, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
	file, err := os.ReadFile(fixturePath)
	require.NoError(t, err)
	want, err := ParseDescriptor(file)
	require.NoError(t, err)
	want.Capabilities.Kind = "workload"
	assert.Equal(t, want, got.Kit, "the annotation and the file are one grammar")
	assert.Equal(t, amd, got.ManifestDigest)
	assert.Equal(t, index, got.Workload.Digest)
}

func TestFetchWorkloadReadsASingleManifest(t *testing.T) {
	f := newFakeRegistry(t)
	m := f.add(mediaOCIManifest, manifestJSON(t, annotationOf(t, fixturePath)))

	got, err := f.client().FetchWorkload(t.Context(), f.ref(m), amd64)

	require.NoError(t, err)
	assert.Equal(t, m, got.ManifestDigest)
	assert.Len(t, got.Kit.Capabilities.SSHAgent, 1)
}

func TestFetchWorkloadReadsDockerMediaTypes(t *testing.T) {
	f := newFakeRegistry(t)
	ann := annotationOf(t, fixturePath)
	body := strings.Replace(string(manifestJSON(t, ann)), mediaOCIManifest, mediaDockerManifest, 1)
	m := f.add(mediaDockerManifest, []byte(body))
	e := entry(m, "linux", "amd64")
	e["mediaType"] = mediaDockerManifest
	idx := strings.Replace(string(indexJSON(t, e)), mediaOCIIndex, mediaDockerList, 1)
	index := f.add(mediaDockerList, []byte(idx))

	got, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
	assert.Equal(t, m, got.ManifestDigest)
}

func TestFetchWorkloadAsksForAnAnonymousToken(t *testing.T) {
	f := newFakeRegistry(t)
	f.needToken = true
	index, _ := publish(t, f)

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
	for _, r := range f.requests() {
		method, auth, _ := strings.Cut(r, "\t")
		if strings.HasPrefix(method, "GET /token") {
			assert.Empty(t, auth, "the token request carries no credential")
		}
		if strings.Contains(method, "/manifests/") && auth != "" {
			assert.Equal(t, "Bearer tok-1", auth)
		}
	}
	assert.Contains(t, strings.Join(f.requests(), "\n"), "GET /token\t")
}

func TestFetchWorkloadReadsATokenOfTheAccessTokenForm(t *testing.T) {
	f := newFakeRegistry(t)
	f.needToken = true
	f.tokenBody = `{"access_token":"tok-1"}`
	index, _ := publish(t, f)

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
}

// shapeOf returns the row of err, failing unless it is upstream-shape
// (RJ-310) naming the registry.
func shapeOf(t *testing.T, err error, f *fakeRegistry) *cli.Error {
	t.Helper()
	ce, ok := errors.AsType[*cli.Error](err)
	require.True(t, ok, "an error of the table: %v", err)
	require.Equal(t, "RJ-310", ce.ID, "%v", err)
	require.Equal(t, "upstream-shape", ce.Slug)
	require.Equal(t, 2, ce.Exit)
	assert.Contains(t, ce.Error(), strings.TrimPrefix(f.srv.URL, "https://"), "the registry is named")
	assert.Equal(t, []cli.Detail{{Key: "upstream", Value: strings.TrimPrefix(f.srv.URL, "https://")}}, ce.Details)
	return ce
}

func TestFetchWorkloadUpstreamShape(t *testing.T) {
	ann := annotationOf(t, fixturePath)
	tests := []struct {
		name  string
		serve func(t *testing.T, f *fakeRegistry) string // returns the digest to ask for
		want  string
	}{
		{"a manifest without schemaVersion", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"mediaType":"`+mediaOCIManifest+`","annotations":{"`+DescriptorAnnotation+`":"{}"}}`))
		}, "schemaVersion"},
		{"a manifest with schemaVersion 1", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":1}`))
		}, "schemaVersion"},
		{"a manifest with the wrong kind of schemaVersion", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":"2"}`))
		}, "typed members"},
		{"a manifest without the annotation", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIManifest+`","annotations":{"other":"x"}}`))
		}, DescriptorAnnotation},
		{"a manifest without annotations", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIManifest+`"}`))
		}, DescriptorAnnotation},
		{"an annotation that is not a string", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIManifest+`","annotations":{"`+DescriptorAnnotation+`":{"a":1}}}`))
		}, "typed members"},
		{"a manifest without mediaType", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"annotations":{"`+DescriptorAnnotation+`":"{}"}}`))
		}, "no mediaType"},
		{"an index without mediaType", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"manifests":[]}`))
		}, "no mediaType"},
		{"an index that carries a descriptor itself", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIIndex+`","manifests":[],"annotations":{"`+DescriptorAnnotation+`":"{}"}}`))
		}, "index carries"},
		{"a manifest that is not JSON", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`<html>`))
		}, "does not decode"},
		{"a manifest with a duplicate member name", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIManifest+`","schemaVersion":2,"annotations":{"`+DescriptorAnnotation+`":"{}"}}`))
		}, "does not decode"},
		{"a manifest of a type that is not read", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add("application/vnd.docker.distribution.manifest.v1+json", []byte(`{"schemaVersion":1}`))
		}, "does not read"},
		{"a manifest whose mediaType differs from its Content-Type", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"mediaType":"`+mediaDockerManifest+`"}`))
		}, "served as"},
		{"a response without a Content-Type", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add("", manifestJSON(t, ann))
		}, "Content-Type"},
		{"an index without manifests", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIIndex+`"}`))
		}, "no manifests"},
		{"an index without schemaVersion", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"manifests":[]}`))
		}, "schemaVersion"},
		{"an index entry without a digest", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIIndex+`","manifests":[{"mediaType":"`+mediaOCIManifest+`"}]}`))
		}, "no mediaType or no digest"},
		{"an index entry with a platform without architecture", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"mediaType":"`+mediaOCIIndex+`","manifests":[{"mediaType":"`+mediaOCIManifest+`","digest":"sha256:`+strings.Repeat("a", 64)+`","platform":{"os":"linux"}}]}`))
		}, "without os or architecture"},
		{"an index with no manifest for the platform", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			arm := f.add(mediaOCIManifest, manifestJSON(t, ann))
			return f.add(mediaOCIIndex, indexJSON(t, entry(arm, "linux", "arm64")))
		}, "no manifest for"},
		{"an index entry of a digest of another algorithm", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			e := entry("sha512:"+strings.Repeat("a", 128), "linux", "amd64")
			return f.add(mediaOCIIndex, indexJSON(t, e))
		}, "sha256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry(t)
			digest := tt.serve(t, f)

			_, err := f.client().FetchWorkload(t.Context(), f.ref(digest), amd64)

			ce := shapeOf(t, err, f)
			assert.Contains(t, ce.Error(), tt.want)
		})
	}
}

func TestFetchWorkloadUpstreamShapeOfTheToken(t *testing.T) {
	for name, body := range map[string]string{
		"a token response with neither member": `{"expires_in":5}`,
		"a token response that is not JSON":    `nope`,
		"a token that is not a string":         `{"token":5}`,
		"a token with a line break":            `{"token":"a\r\nX-Evil: 1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeRegistry(t)
			f.needToken = true
			f.tokenBody = body
			index, _ := publish(t, f)

			_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

			_ = shapeOf(t, err, f)
		})
	}
	t.Run("a challenge that is not Bearer", func(t *testing.T) {
		f := newFakeRegistry(t)
		f.override = func(w http.ResponseWriter, _ *http.Request) bool {
			w.Header().Set("Www-Authenticate", `Negotiate`)
			w.WriteHeader(http.StatusUnauthorized)
			return true
		}

		_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		_ = shapeOf(t, err, f)
	})
}

func TestFetchWorkloadRefusesADescriptorTheGrammarRefuses(t *testing.T) {
	f := newFakeRegistry(t)
	ann := `{"schemaVersion":"3","kind":"workload","capabilities":[{"type":"com.docker.sandbox/teleport@1"}]}`
	m := f.add(mediaOCIManifest, manifestJSON(t, ann))

	_, err := f.client().FetchWorkload(t.Context(), f.ref(m), amd64)

	require.ErrorIs(t, err, ErrInvalid)
	assert.Contains(t, err.Error(), strings.TrimPrefix(f.srv.URL, "https://"))
	_, isRow := errors.AsType[*cli.Error](err)
	assert.False(t, isRow, "the grammar's refusal has no row of the table of its own")
}

func TestFetchWorkloadReadsAWorkloadOnly(t *testing.T) {
	digest := "sha256:" + strings.Repeat("c", 64)
	tests := []struct {
		name string
		ann  string
		want string
	}{
		{"a mixin", `{"schemaVersion":"3","kind":"mixin"}`, "not a workload"},
		{"a set", `{"schemaVersion":"3","kind":"set","kits":[{"ref":"example.com/a/b","digest":"` + digest + `"}]}`, "not a workload"},
		{"a workload with an inline recipe", `{"schemaVersion":"3","kind":"workload","build":"FROM scratch"}`, "recipe"},
		{"a workload with a companion recipe", `{"schemaVersion":"3","kind":"workload","dockerfile":"x"}`, "recipe"},
		{"a workload with kits", `{"schemaVersion":"3","kind":"workload","kits":[{"ref":"example.com/a/b"}]}`, "kits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry(t)
			m := f.add(mediaOCIManifest, manifestJSON(t, tt.ann))

			_, err := f.client().FetchWorkload(t.Context(), f.ref(m), amd64)

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), tt.want)
			assert.Contains(t, err.Error(), strings.TrimPrefix(f.srv.URL, "https://"))
		})
	}
}

func TestFetchWorkloadRegistryUnreachable(t *testing.T) {
	tests := []struct {
		name  string
		serve func(w http.ResponseWriter, r *http.Request)
		want  string
	}{
		{"a 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }, "500"},
		{"a 404", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, "404"},
		{"a 401 that asks for a password", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Www-Authenticate", `Basic realm="x"`)
			w.WriteHeader(401)
		}, "anonymous"},
		{"a 401 again after the token", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				_, _ = w.Write([]byte(`{"token":"t"}`))
				return
			}
			w.Header().Set("Www-Authenticate", `Bearer realm="https://`+r.Host+`/token"`)
			w.WriteHeader(401)
		}, "anonymous"},
		{"a token service that fails", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/token" {
				w.WriteHeader(503)
				return
			}
			w.Header().Set("Www-Authenticate", `Bearer realm="https://`+r.Host+`/token"`)
			w.WriteHeader(401)
		}, "token service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeRegistry(t)
			f.override = func(w http.ResponseWriter, r *http.Request) bool { tt.serve(w, r); return true }

			_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

			ce, ok := errors.AsType[*cli.Error](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, "RJ-322", ce.ID)
			assert.Equal(t, 1, ce.Exit)
			assert.Contains(t, ce.Error(), tt.want)
			assert.Contains(t, ce.Error(), strings.TrimPrefix(f.srv.URL, "https://"))
		})
	}
	t.Run("a registry that is down", func(t *testing.T) {
		f := newFakeRegistry(t)
		ref := f.ref("sha256:" + strings.Repeat("a", 64))
		c := f.client()
		f.srv.Close()

		_, err := c.FetchWorkload(t.Context(), ref, amd64)

		ce, ok := errors.AsType[*cli.Error](err)
		require.True(t, ok, "%v", err)
		assert.Equal(t, "RJ-322", ce.ID)
	})
	t.Run("a context that is done", func(t *testing.T) {
		f := newFakeRegistry(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := f.client().FetchWorkload(ctx, f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		ce, ok := errors.AsType[*cli.Error](err)
		require.True(t, ok, "%v", err)
		assert.Equal(t, "RJ-322", ce.ID)
	})
}

func TestFetchWorkloadRefusesWhatIsNotAWorkloadReference(t *testing.T) {
	for _, ref := range []string{
		"http://example.com/org/kit@sha256:" + strings.Repeat("a", 64),
		"example.com/org/kit:latest",
		"example.com/org/kit@sha256:abc",
		"",
	} {
		t.Run(ref, func(t *testing.T) {
			_, err := NewClient().FetchWorkload(t.Context(), ref, amd64)

			require.Error(t, err)
		})
	}
}

func TestCheckRealms(t *testing.T) {
	tests := []struct {
		name       string
		challenges []string
		wantErr    error
		wantShape  bool
	}{
		{"a realm on the registry's site", []string{`Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`}, nil, false},
		{"a bare realm", []string{`bearer realm=registry, service=x`}, nil, true},
		{"a bare realm the library cuts at its colon", []string{`bearer realm=https://auth.docker.io/t, service=x`}, nil, true},
		{"a realm in the second challenge", []string{`Negotiate`, `Bearer realm="https://evil.example/t"`}, ErrForeignRealm, false},
		{"a realm in the second header", []string{`Basic realm="https://auth.docker.io/t"`, `Bearer realm="https://169.254.169.254/t"`}, ErrForeignRealm, false},
		{"a realm over http", []string{`Bearer realm="http://auth.docker.io/t"`}, ErrInsecure, false},
		{"a realm with a user name", []string{`Bearer realm="https://u:p@auth.docker.io/t"`}, ErrInsecure, false},
		{"a realm that is not a URL", []string{`Bearer realm="::"`}, nil, true},
		{"a challenge with no realm", []string{`Bearer service="x"`}, nil, true},
		{"a Basic realm that is not a URL", []string{`Basic realm="the registry"`}, nil, false},
		{"a Basic and then a Bearer challenge in one value", []string{`Basic realm="x", Bearer realm="https://evil.example/t",service="s"`}, nil, true},
		{"a scheme alone", []string{`Basic`}, nil, false},
		{"a value without a scheme", []string{`="x"`, `Basic`}, nil, true},
		{"a scheme the product does not read", []string{`Negotiate`}, nil, true},
		{"no challenge", nil, nil, true},
		{"another port of the registry's own address", []string{`Bearer realm="https://127.0.0.1:2/t"`}, ErrPrivateAddress, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := "registry-1.docker.io"
			if errors.Is(tt.wantErr, ErrPrivateAddress) {
				host = "127.0.0.1:1"
			}
			g := newGuard(nil, host)

			err := g.checkRealms(tt.challenges)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantShape:
				_, ok := errors.AsType[*shapeError](err)
				assert.True(t, ok, "%v", err)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestFetchWorkloadPicksOneManifestPerPlatform(t *testing.T) {
	ann := annotationOf(t, fixturePath)
	other := strings.Replace(ann, `"displayName":"git-ssh-sign"`, `"displayName":"other"`, 1)
	require.NotEqual(t, ann, other)
	variant := func(digest, arch, v string) map[string]any {
		e := entry(digest, "linux", arch)
		if v == "" {
			delete(e["platform"].(map[string]any), "variant")
		} else {
			e["platform"].(map[string]any)["variant"] = v
		}
		return e
	}
	t.Run("two entries for one platform are refused", func(t *testing.T) {
		f := newFakeRegistry(t)
		a := f.add(mediaOCIManifest, manifestJSON(t, ann))
		b := f.add(mediaOCIManifest, manifestJSON(t, other))
		index := f.add(mediaOCIIndex, indexJSON(t, variant(a, "amd64", ""), variant(b, "amd64", "")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

		ce := shapeOf(t, err, f)
		assert.Contains(t, ce.Error(), "more than one manifest")
	})
	t.Run("two variants and a host with none are refused", func(t *testing.T) {
		f := newFakeRegistry(t)
		a := f.add(mediaOCIManifest, manifestJSON(t, ann))
		b := f.add(mediaOCIManifest, manifestJSON(t, other))
		index := f.add(mediaOCIIndex, indexJSON(t, variant(a, "arm64", "v8"), variant(b, "arm64", "v7")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), Platform{OS: "linux", Architecture: "arm64"})

		ce := shapeOf(t, err, f)
		assert.Contains(t, ce.Error(), "more than one manifest")
	})
	t.Run("a host with a variant takes its own", func(t *testing.T) {
		f := newFakeRegistry(t)
		a := f.add(mediaOCIManifest, manifestJSON(t, ann))
		b := f.add(mediaOCIManifest, manifestJSON(t, other))
		index := f.add(mediaOCIIndex, indexJSON(t, variant(a, "arm64", "v8"), variant(b, "arm64", "v7")))

		got, err := f.client().FetchWorkload(t.Context(), f.ref(index), Platform{OS: "linux", Architecture: "arm64", Variant: "v7"})

		require.NoError(t, err)
		assert.Equal(t, b, got.ManifestDigest)
	})
	t.Run("a host with a variant does not take an entry without one", func(t *testing.T) {
		f := newFakeRegistry(t)
		a := f.add(mediaOCIManifest, manifestJSON(t, ann))
		index := f.add(mediaOCIIndex, indexJSON(t, variant(a, "arm64", "")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), Platform{OS: "linux", Architecture: "arm64", Variant: "v8"})

		ce := shapeOf(t, err, f)
		assert.Contains(t, ce.Error(), "no manifest for")
	})
	t.Run("one entry per platform is read", func(t *testing.T) {
		f := newFakeRegistry(t)
		a := f.add(mediaOCIManifest, manifestJSON(t, ann))
		b := f.add(mediaOCIManifest, manifestJSON(t, other))
		index := f.add(mediaOCIIndex, indexJSON(t, variant(a, "amd64", ""), variant(b, "arm64", "v8")))

		got, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

		require.NoError(t, err)
		assert.Equal(t, a, got.ManifestDigest)
	})
}

func TestEveryCapabilityTypeIsOfTheTypeForm(t *testing.T) {
	for _, typ := range capabilityTypes {
		assert.Regexp(t, typePattern, typ)
		// And the dispatch knows it: a type with no config is read, one with
		// a required config is refused for the config, never as unmodeled.
		_, err := ParseDescriptor(capability(typ, ""))
		if err != nil {
			assert.NotContains(t, err.Error(), "does not model", typ)
		}
	}
	assert.Len(t, capabilityTypes, 19)
}

func TestPlatformString(t *testing.T) {
	assert.Equal(t, "linux/amd64", amd64.String())
	assert.Equal(t, "linux/arm64/v8", Platform{OS: "linux", Architecture: "arm64", Variant: "v8"}.String())
}
