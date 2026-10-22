// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// fakeDoc is a document a fake registry serves.
type fakeDoc struct {
	ctype string
	body  []byte
}

// fakeRegistry is a registry on a local TLS server: it serves the
// documents added to it, by digest, under /v2/org/kit/manifests/, and a
// token service under /token. It records the requests it saw.
type fakeRegistry struct {
	t   *testing.T
	srv *httptest.Server

	mu   sync.Mutex
	docs map[string]fakeDoc
	// needToken makes the manifests answer 401 with a Bearer challenge
	// until a request carries the token.
	needToken bool
	// override, when set, may answer a request itself and return true.
	override func(w http.ResponseWriter, r *http.Request) bool
	// tokenBody is what /token answers.
	tokenBody string
	// seen are the requests, as "METHOD path" with the Authorization
	// header after a tab.
	seen []string
}

// newFakeRegistry starts a fake registry.
func newFakeRegistry(t *testing.T) *fakeRegistry {
	t.Helper()
	f := &fakeRegistry{t: t, docs: map[string]fakeDoc{}, tokenBody: `{"token":"tok-1","expires_in":300}`}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// serve answers one request.
func (f *fakeRegistry) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.seen = append(f.seen, r.Method+" "+r.URL.Path+"\t"+r.Header.Get("Authorization"))
	override, needToken, tokenBody := f.override, f.needToken, f.tokenBody
	f.mu.Unlock()
	if override != nil && override(w, r) {
		return
	}
	if r.URL.Path == "/token" {
		_, _ = w.Write([]byte(tokenBody))
		return
	}
	digest, ok := strings.CutPrefix(r.URL.Path, "/v2/org/kit/manifests/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	if needToken && r.Header.Get("Authorization") != "Bearer tok-1" {
		w.Header().Set("Www-Authenticate", `Bearer realm="`+f.srv.URL+`/token",service="fake",scope="repository:org/kit:pull"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	doc, ok := f.docs[digest]
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", doc.ctype)
	w.Header().Set("Docker-Content-Digest", digest)
	_, _ = w.Write(doc.body)
}

// add serves body as ctype under its own digest and returns the digest.
func (f *fakeRegistry) add(ctype string, body []byte) string {
	d := sha256Digest(body)
	f.mu.Lock()
	f.docs[d] = fakeDoc{ctype: ctype, body: body}
	f.mu.Unlock()
	return d
}

// addAs serves body as ctype under the digest given, which it need not
// match: a tampered document.
func (f *fakeRegistry) addAs(digest, ctype string, body []byte) {
	f.mu.Lock()
	f.docs[digest] = fakeDoc{ctype: ctype, body: body}
	f.mu.Unlock()
}

// ref returns the workload reference of digest on this registry.
func (f *fakeRegistry) ref(digest string) string {
	return strings.TrimPrefix(f.srv.URL, "https://") + "/org/kit@" + digest
}

// client returns a Client that trusts this registry's certificate.
func (f *fakeRegistry) client() *Client {
	f.t.Helper()
	c := NewClient()
	trust(f.t, c, f.srv)
	return c
}

// requests returns what the registry saw.
func (f *fakeRegistry) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

// trust makes c accept the certificate of srv.
func trust(t *testing.T, c *Client, srv *httptest.Server) {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	c.http.Transport.(*http.Transport).TLSClientConfig.RootCAs = pool
}

// sha256Digest returns the OCI digest of data.
func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// annotationOf returns the descriptor file at path as the compact JSON
// an image carries in its annotation.
func annotationOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var tree map[string]any
	require.NoError(t, yaml.Unmarshal(data, &tree))
	// As an image carries it: a workload, with the phase the kit schema
	// requires of an ssh-agent capability.
	tree["kind"] = "workload"
	for _, c := range tree["capabilities"].([]any) {
		item := c.(map[string]any)
		if item["type"] == typeSSHAgent {
			item["config"].(map[string]any)["phase"] = []any{"install", "runtime"}
		}
	}
	out, err := jsonv2.Marshal(tree, jsonv2.Deterministic(true))
	require.NoError(t, err)
	return string(out)
}

// manifestJSON returns a manifest carrying annotation, with members
// this release does not read.
func manifestJSON(t *testing.T, annotation string) []byte {
	t.Helper()
	out, err := jsonv2.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     mediaOCIManifest,
		"config":        map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": "sha256:" + strings.Repeat("0", 64), "size": 2},
		"layers":        []any{},
		"annotations":   map[string]any{DescriptorAnnotation: annotation, "org.example/other": "x"},
		"futureMember":  map[string]any{"a": 1},
	}, jsonv2.Deterministic(true))
	require.NoError(t, err)
	return out
}

// indexJSON returns an index of the entries given, with members this
// release does not read.
func indexJSON(t *testing.T, entries ...map[string]any) []byte {
	t.Helper()
	list := make([]any, len(entries))
	for i, e := range entries {
		list[i] = e
	}
	out, err := jsonv2.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     mediaOCIIndex,
		"manifests":     list,
		"futureMember":  true,
	}, jsonv2.Deterministic(true))
	require.NoError(t, err)
	return out
}

// entry returns an index entry.
func entry(digest, os, arch string) map[string]any {
	return map[string]any{
		"mediaType": mediaOCIManifest, "digest": digest, "size": 100,
		"platform": map[string]any{"os": os, "architecture": arch, "variant": "v8"},
	}
}

var amd64 = Platform{OS: "linux", Architecture: "amd64"}
