// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"crypto/subtle"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NewAuthOrigins is NewOrigins behind HTTP basic authentication: a
// request without the user and the password is answered 401, as a
// private repository of a forge answers it, and one with them is served
// as NewOrigins serves it. Push writes the bare repositories directly,
// so a test fills them as it does for NewOrigins.
func NewAuthOrigins(tb testing.TB, user, password string) *Origins {
	tb.Helper()
	open := NewOrigins(tb)
	o := *open
	o.srv = httptest.NewTLSServer(basicAuth{next: open.srv.Config.Handler, user: user, password: password})
	// The open server would serve the same repositories without the
	// credentials; nothing reaches it, and closing it makes sure. Its
	// handler lives on in the new server, and its own cleanup closes it
	// again, which httptest allows.
	open.srv.Close()
	tb.Cleanup(o.srv.Close)
	o.ca = filepath.Join(filepath.Dir(open.ca), "auth-ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: o.srv.Certificate().Raw})
	if err := os.WriteFile(o.ca, pemBytes, 0o600); err != nil {
		tb.Fatal(err)
	}
	return &o
}

// basicAuth serves next to a request that carries the user and the
// password, and answers any other with 401 and a Basic challenge.
type basicAuth struct {
	next           http.Handler
	user, password string
}

func (b basicAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(b.user)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(b.password)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="origins"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	b.next.ServeHTTP(w, r)
}

// WriteCredentialHelper writes, under the Env's home, a git credential
// helper that answers a get with user and password, and returns its
// absolute path: the value a test passes to --git-credential-helper.
func (e *Env) WriteCredentialHelper(tb testing.TB, user, password string) string {
	tb.Helper()
	for _, s := range []string{user, password} {
		if strings.ContainsAny(s, "'\n") {
			tb.Fatalf("%q cannot be quoted in the helper script", s)
		}
	}
	dir := filepath.Join(e.Home, "helpers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		tb.Fatal(err)
	}
	p := filepath.Join(dir, "git-credential-test")
	script := "#!/bin/sh\nif [ \"$1\" = get ]; then\n  printf 'username=%s\\npassword=%s\\n' '" + user + "' '" + password + "'\nfi\n"
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil {
		tb.Fatal(err)
	}
	return p
}

// RecordGit makes the git of the Env record the argv of every call
// before it runs, and returns a function that reads the calls made so
// far, each as its arguments without the program. The settings a
// command writes keep the same tools.git, since the recorder takes the
// place of the git at e.Git.
func (e *Env) RecordGit(tb testing.TB) func() [][]string {
	tb.Helper()
	real := e.Git + ".real"
	log := filepath.Join(filepath.Dir(e.Bin), "git-calls")
	for _, s := range []string{real, log} {
		if strings.ContainsAny(s, "'\n") {
			tb.Fatalf("%q cannot be quoted in the recorder script", s)
		}
	}
	if err := os.Rename(e.Git, real); err != nil {
		tb.Fatal(err)
	}
	// Each argument ends with US (0x1f) and each call with RS (0x1e).
	script := "#!/bin/sh\n{ for a in \"$@\"; do printf '%s\\037' \"$a\"; done; printf '\\036'; } >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(e.Git, []byte(script), 0o700); err != nil {
		tb.Fatal(err)
	}
	return func() [][]string {
		data, err := os.ReadFile(log)
		if err != nil {
			tb.Fatal(err)
		}
		var calls [][]string
		for call := range strings.SplitSeq(strings.TrimSuffix(string(data), "\x1e"), "\x1e") {
			calls = append(calls, strings.Split(strings.TrimSuffix(call, "\x1f"), "\x1f"))
		}
		return calls
	}
}
