// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The oci table of I28 (REQ-0515): each row is refused, through
// FetchWorkload, which gives the refusal the row upstream-shape and the
// registry's name, and through the sentinel the guard raises.

func TestGuardDigestMismatchAtEachHop(t *testing.T) {
	ann := annotationOf(t, fixturePath)
	t.Run("the index", func(t *testing.T) {
		f := newFakeRegistry(t)
		amd := f.add(mediaOCIManifest, manifestJSON(t, ann))
		good := indexJSON(t, entry(amd, "linux", "amd64"))
		asked := sha256Digest(good)
		// The registry serves other bytes under the digest asked for.
		f.addAs(asked, mediaOCIIndex, indexJSON(t, entry(amd, "linux", "amd64"), entry(amd, "linux", "arm64")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(asked), amd64)

		require.ErrorIs(t, err, ErrDigestMismatch)
		_ = shapeOf(t, err, f)
	})
	t.Run("the platform manifest", func(t *testing.T) {
		f := newFakeRegistry(t)
		listed := sha256Digest(manifestJSON(t, ann))
		f.addAs(listed, mediaOCIManifest, manifestJSON(t, ann+" "))
		index := f.add(mediaOCIIndex, indexJSON(t, entry(listed, "linux", "amd64")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

		require.ErrorIs(t, err, ErrDigestMismatch)
		_ = shapeOf(t, err, f)
	})
	t.Run("the annotation", func(t *testing.T) {
		// The annotation is a member of the manifest, so the digest of
		// the manifest covers it: an annotation changed in transit is
		// a manifest whose digest is not the one the index lists.
		f := newFakeRegistry(t)
		listed := sha256Digest(manifestJSON(t, ann))
		evil := `{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/privileged@1"}]}`
		f.addAs(listed, mediaOCIManifest, manifestJSON(t, evil))
		index := f.add(mediaOCIIndex, indexJSON(t, entry(listed, "linux", "amd64")))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

		require.ErrorIs(t, err, ErrDigestMismatch)
		_ = shapeOf(t, err, f)
	})
	t.Run("a single manifest", func(t *testing.T) {
		f := newFakeRegistry(t)
		asked := sha256Digest(manifestJSON(t, ann))
		f.addAs(asked, mediaOCIManifest, manifestJSON(t, `{"schemaVersion":"3","kind":"mixin"}`))

		_, err := f.client().FetchWorkload(t.Context(), f.ref(asked), amd64)

		require.ErrorIs(t, err, ErrDigestMismatch)
	})
	t.Run("the header of the registry", func(t *testing.T) {
		f := newFakeRegistry(t)
		index, _ := publish(t, f)
		f.override = func(w http.ResponseWriter, r *http.Request) bool {
			if !strings.HasSuffix(r.URL.Path, index) {
				return false
			}
			w.Header().Set("Content-Type", mediaOCIIndex)
			w.Header().Set("Docker-Content-Digest", "sha256:"+strings.Repeat("0", 64))
			_, _ = w.Write(f.docs[index].body)
			return true
		}

		_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

		require.ErrorIs(t, err, ErrDigestMismatch)
	})
}

func TestVerifyManifest(t *testing.T) {
	body := []byte("hello")
	require.NoError(t, verifyManifest(sha256Digest(body), body, ""))
	require.NoError(t, verifyManifest(sha256Digest(body), body, sha256Digest(body)))
	for _, bad := range []string{"", "sha256:", "sha256:zz", "sha512:" + strings.Repeat("a", 128), strings.ToUpper(sha256Digest(body)), sha256Digest(body)[:20], sha256Digest([]byte("other"))} {
		require.ErrorIs(t, verifyManifest(bad, body, ""), ErrDigestMismatch, bad)
	}
	require.ErrorIs(t, verifyManifest(sha256Digest(body), body, sha256Digest([]byte("other"))), ErrDigestMismatch, "the registry's header names other content")
}

func TestGuardRefusesAnHTTPURL(t *testing.T) {
	var hits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	t.Cleanup(plain.Close)
	g := newGuard(NewClient().transport, "example.com")
	for _, u := range []string{plain.URL + "/x", "ftp://example.com/a", "https://user:pw@example.com/a", "https:///a", "//example.com/a"} {
		t.Run(u, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, nil)
			require.NoError(t, err)

			_, err = g.RoundTrip(req)

			require.ErrorIs(t, err, ErrInsecure)
		})
	}
	t.Run("the plain ping the library also sends", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, plain.URL+"/v2/", nil)
		require.NoError(t, err)

		_, err = g.RoundTrip(req)

		require.ErrorIs(t, err, errHTTPFallback)
	})
	assert.Zero(t, hits.Load(), "nothing reached the plain server")
}

func TestGuardRefusesATokenRealmOverHTTP(t *testing.T) {
	var hits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"token":"x"}`))
	}))
	t.Cleanup(plain.Close)
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Www-Authenticate", `Bearer realm="`+plain.URL+`/token"`)
		w.WriteHeader(http.StatusUnauthorized)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrInsecure)
	_ = shapeOf(t, err, f)
	assert.Zero(t, hits.Load())
}

func TestGuardRefusesAnHTTPSToHTTPRedirect(t *testing.T) {
	var hits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(plain.Close)
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrRedirect)
	_ = shapeOf(t, err, f)
	assert.Zero(t, hits.Load(), "the plain server was never asked")
}

func TestGuardFollowsAnHTTPSRedirectAFewTimes(t *testing.T) {
	f := newFakeRegistry(t)
	index, _ := publish(t, f)
	var hops atomic.Int32
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, index) && hops.Add(1) == 1 {
			http.Redirect(w, r, "/v2/org/kit/manifests/"+index+"?again=1", http.StatusFound)
			return true
		}
		return false
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
}

func TestGuardFollowsFiveRedirectsAndNotSix(t *testing.T) {
	for redirects, ok := range map[int]bool{maxRedirects: true, maxRedirects + 1: false} {
		t.Run(strconv.Itoa(redirects), func(t *testing.T) {
			f := newFakeRegistry(t)
			index, _ := publish(t, f)
			var hops atomic.Int32
			f.override = func(w http.ResponseWriter, r *http.Request) bool {
				if strings.HasSuffix(r.URL.Path, index) {
					if n := hops.Add(1); int(n) <= redirects {
						http.Redirect(w, r, "/v2/org/kit/manifests/"+index+"?hop="+strconv.Itoa(int(n)), http.StatusFound)
						return true
					}
				}
				return false
			}

			_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

			if ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrRedirect)
		})
	}
	assert.Equal(t, 5, maxRedirects, "the limit the PR states")
}

func TestGuardRefusesTooManyRedirects(t *testing.T) {
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrRedirect)
	_ = shapeOf(t, err, f)
}

func TestGuardRefusesARedirectToACredentialURL(t *testing.T) {
	f := newFakeRegistry(t)
	f.override = func(w http.ResponseWriter, r *http.Request) bool {
		http.Redirect(w, r, "https://user:pw@"+r.Host+r.URL.Path, http.StatusFound)
		return true
	}

	_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

	require.ErrorIs(t, err, ErrRedirect)
}

func TestGuardCapsEveryResponse(t *testing.T) {
	big := strings.Repeat("a", MaxManifestSize+1)
	t.Run("a manifest that says its length", func(t *testing.T) {
		f := newFakeRegistry(t)
		f.override = func(w http.ResponseWriter, _ *http.Request) bool {
			w.Header().Set("Content-Length", strconv.Itoa(len(big)))
			_, _ = w.Write([]byte(big))
			return true
		}

		_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		require.ErrorIs(t, err, ErrTooLarge)
		_ = shapeOf(t, err, f)
	})
	t.Run("a manifest that does not", func(t *testing.T) {
		f := newFakeRegistry(t)
		f.override = func(w http.ResponseWriter, _ *http.Request) bool {
			fl := w.(http.Flusher)
			chunk := strings.Repeat("a", 1<<16)
			for range (MaxManifestSize / len(chunk)) + 1 {
				_, _ = w.Write([]byte(chunk))
				fl.Flush()
			}
			return true
		}

		_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		require.ErrorIs(t, err, ErrTooLarge)
	})
	t.Run("a token response", func(t *testing.T) {
		f := newFakeRegistry(t)
		f.needToken = true
		f.tokenBody = `{"token":"` + strings.Repeat("a", MaxTokenSize) + `"}`

		_, err := f.client().FetchWorkload(t.Context(), f.ref("sha256:"+strings.Repeat("a", 64)), amd64)

		require.ErrorIs(t, err, ErrTooLarge)
	})
	t.Run("a manifest at the cap is read", func(t *testing.T) {
		f := newFakeRegistry(t)
		pad := strings.Repeat("a", MaxManifestSize-200)
		body := `{"schemaVersion":2,"mediaType":"` + mediaOCIManifest + `","pad":"` + pad + `","annotations":{"` + DescriptorAnnotation + `":"{\"schemaVersion\":\"3\",\"kind\":\"workload\"}"}}`
		require.LessOrEqual(t, len(body), MaxManifestSize)
		m := f.add(mediaOCIManifest, []byte(body))

		got, err := f.client().FetchWorkload(t.Context(), f.ref(m), amd64)

		require.NoError(t, err)
		assert.Equal(t, "workload", got.Kit.Capabilities.Kind)
	})
}

func TestGuardRefusesADescriptorOverItsCap(t *testing.T) {
	f := newFakeRegistry(t)
	ann := `{"schemaVersion":"3","kind":"mixin","description":"` + strings.Repeat("a", MaxDescriptorSize) + `"}`
	m := f.add(mediaOCIManifest, manifestJSON(t, ann))

	_, err := f.client().FetchWorkload(t.Context(), f.ref(m), amd64)

	require.Error(t, err)
}
