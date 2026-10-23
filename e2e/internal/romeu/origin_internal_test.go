// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRequestOK admits /owner/repo and below, clean, and nothing else.
func TestRequestOK(t *testing.T) {
	for p, want := range map[string]bool{
		"/owner/repo/info/refs":       true,
		"/owner/repo/git-upload-pack": true,
		"/owner/repo":                 false, // no service: not a git request
		"/owner/../x/info/refs":       false,
		"/owner/repo/../../etc/x":     false,
		"//owner/repo/info/refs":      false,
		"/owner/repo/./info/refs":     false,
		"/../owner/repo/info/refs":    false,
		"owner/repo/info/refs":        false,
		"/owner//info/refs":           false,
		"/":                           false,
	} {
		assert.Equal(t, want, requestOK(p), p)
	}
}

// TestCGIBackendRefusesBadRequests: a path that leaves the project root
// and a body over the cap are refused before a program runs.
func TestCGIBackendRefusesBadRequests(t *testing.T) {
	b := cgiBackend{path: "/nonexistent", root: t.TempDir(), limit: 8}

	w := httptest.NewRecorder()
	b.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/owner/repo/../../etc/passwd", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	b.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/owner/repo/git-upload-pack", strings.NewReader("0123456789")))
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
