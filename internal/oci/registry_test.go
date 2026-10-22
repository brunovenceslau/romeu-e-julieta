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
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"annotations":{"other":"x"}}`))
		}, DescriptorAnnotation},
		{"a manifest without annotations", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2}`))
		}, DescriptorAnnotation},
		{"an annotation that is not a string", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"annotations":{"`+DescriptorAnnotation+`":{"a":1}}}`))
		}, "typed members"},
		{"a manifest that is not JSON", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`<html>`))
		}, "does not decode"},
		{"a manifest with a duplicate member name", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIManifest, []byte(`{"schemaVersion":2,"schemaVersion":2,"annotations":{"`+DescriptorAnnotation+`":"{}"}}`))
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
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2}`))
		}, "no manifests"},
		{"an index without schemaVersion", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"manifests":[]}`))
		}, "schemaVersion"},
		{"an index entry without a digest", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"manifests":[{"mediaType":"`+mediaOCIManifest+`"}]}`))
		}, "no mediaType or no digest"},
		{"an index entry with a platform without architecture", func(t *testing.T, f *fakeRegistry) string {
			t.Helper()
			return f.add(mediaOCIIndex, []byte(`{"schemaVersion":2,"manifests":[{"mediaType":"`+mediaOCIManifest+`","digest":"sha256:`+strings.Repeat("a", 64)+`","platform":{"os":"linux"}}]}`))
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
		}, "user name"},
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

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name                           string
		host, repo, wantHost, wantRepo string
	}{
		{"docker hub", "docker.io", "docker/sbx-kit-claude", "registry-1.docker.io", "docker/sbx-kit-claude"},
		{"docker hub official image", "docker.io", "debian", "registry-1.docker.io", "library/debian"},
		{"index.docker.io", "index.docker.io", "a/b", "registry-1.docker.io", "a/b"},
		{"another registry", "ghcr.io", "a/b", "ghcr.io", "a/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, repo := endpoint(tt.host, tt.repo)

			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestParseChallenge(t *testing.T) {
	scheme, p := parseChallenge(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:docker/sbx-kit-claude:pull"`)
	assert.Equal(t, "Bearer", scheme)
	assert.Equal(t, map[string]string{"realm": "https://auth.docker.io/token", "service": "registry.docker.io", "scope": "repository:docker/sbx-kit-claude:pull"}, p)

	scheme, p = parseChallenge(`bearer realm=https://a.example/t, service=x`)
	assert.Equal(t, "bearer", scheme)
	assert.Equal(t, map[string]string{"realm": "https://a.example/t", "service": "x"}, p)

	for _, bad := range []string{`Bearer realm`, `Bearer realm="unterminated`, ``} {
		scheme, _ := parseChallenge(bad)
		if bad == `` {
			assert.Empty(t, scheme)
			continue
		}
		assert.Empty(t, scheme, bad)
	}
}
