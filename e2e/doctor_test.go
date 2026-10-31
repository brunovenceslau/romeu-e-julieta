// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"bytes"
	"encoding/json/v2"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// doctorReport is the --json document of romeu doctor, as a program
// reads it.
type doctorReport struct {
	Checks []struct {
		Slug   string `json:"slug"`
		Result string `json:"result"`
		Error  *struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"error"`
		Message string `json:"message"`
	} `json:"checks"`
	Leftovers []string `json:"leftovers"`
}

// result returns the result of slug and the slug of its error.
func (r doctorReport) result(t *testing.T, slug string) (string, string) {
	t.Helper()
	for _, c := range r.Checks {
		if c.Slug == slug {
			if c.Error == nil {
				return c.Result, ""
			}
			return c.Result, c.Error.Slug
		}
	}
	t.Fatalf("no check %s in the report", slug)
	return "", ""
}

// sessionLines returns the lines of a recorded session of sbx 0.49.2:
// its header and the entries whose argv is one of argvs.
func sessionLines(t *testing.T, name string, argvs ...[]string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "sbx", "0.49.2", name))
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	out := []string{lines[0]}
	for _, l := range lines[1:] {
		var e struct {
			Argv []string `json:"argv"`
		}
		require.NoError(t, json.Unmarshal([]byte(l), &e))
		if slices.ContainsFunc(argvs, func(a []string) bool { return slices.Equal(a, e.Argv) }) {
			out = append(out, l)
		}
	}
	require.Len(t, out, 1+len(argvs), "%s holds each argv once", name)
	return out
}

// listing returns the session entry of the read argv with stdout as its
// output, exit 0.
func listing(t *testing.T, argv []string, stdout string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"argv": argv, "exit": 0, "stdout": stdout, "stderr": ""})
	require.NoError(t, err)
	return string(b)
}

// syntheticListing returns a synthesized listing of T046, its first
// line, the synthetic marker, left out.
func syntheticListing(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "internal", "preflight", "testdata", "synthetic", name))
	require.NoError(t, err)
	first, rest, _ := strings.Cut(string(b), "\n")
	require.True(t, strings.HasPrefix(first, "# synthetic: true"), name)
	return rest
}

var (
	argvSecretList = []string{"secret", "ls"}
	argvPolicyList = []string{"policy", "ls", "--json"}
)

// doctorEnv returns a plain build's Env with settings whose one secret
// entry is for service and runs secretBody, and a session of the
// recorded sbx reads doctor makes, the recorded A9 listings (the secret
// listing of secrets-settings, the policy listing of handshake), each
// replaced by the body given when it is not empty.
func doctorEnv(t *testing.T, service, secretBody, secrets, policies string) *romeu.Env {
	t.Helper()
	e := romeu.New(t, nil)
	e.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	argv := filepath.Join(e.Bin, "secret-"+service)
	require.NoError(t, os.WriteFile(argv, []byte("#!/bin/sh\n"+secretBody+"\n"), 0o700))
	e.AppendSettings(t, "secrets:\n  token@shop:\n    service: "+service+"\n    argv: ["+argv+"]\n")

	lines := sessionLines(t, "handshake.jsonl", []string{"version"}, []string{"ls", "--json"}, argvPolicyList)
	lines = append(lines, sessionLines(t, "secrets-settings.jsonl", argvSecretList)[1:]...)
	for i, l := range lines {
		switch {
		case secrets != "" && strings.Contains(l, `"argv":["secret","ls"]`):
			lines[i] = listing(t, argvSecretList, secrets)
		case policies != "" && strings.Contains(l, `"argv":["policy","ls","--json"]`):
			lines[i] = listing(t, argvPolicyList, policies)
		}
	}
	session := filepath.Join(filepath.Dir(e.Bin), "doctor-session.jsonl")
	require.NoError(t, os.WriteFile(session, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	require.NoError(t, e.SetSbxSession(session))
	return e
}

// runDoctor runs romeu doctor --json and decodes its report.
func runDoctor(t *testing.T, e *romeu.Env) (doctorReport, romeu.Result) {
	t.Helper()
	res := e.Run(t, []string{"doctor", "--json"})
	var rep doctorReport
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &rep), "stdout %q, stderr %q", res.Stdout, res.Stderr)
	return rep, res
}

// TestDoctorI22OnTheRecordedSessions: the romeu binary, on the fake sbx,
// passes the two I22 rows on the A9 recording, fails them with exit 6 on
// each synthesized fixture of T046, and reports upstream-shape on both
// with exit 1 on the two shape fixtures, the same files the preflight's
// tests read (REQ-0371, REQ-0372, REQ-0331).
func TestDoctorI22OnTheRecordedSessions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, secrets, policies string
		global, broad, errSlug  string
		exit                    int
	}{
		{"the A9 recording", "", "", "pass", "pass", "", 6},
		{"a global secret of a used service", "secrets-global-used.txt", "", "fail", "pass", "", 6},
		{"a global allow of **", "", "policies-broad-doublestar.json", "pass", "fail", "", 6},
		{"a wildcard over a top-level domain", "", "policies-broad-tld.json", "pass", "fail", "", 6},
		{"a permissive default policy", "", "policies-broad-default.json", "pass", "fail", "", 6},
		{"a changed container shape", "secrets-changed-container.txt", "policies-changed-container.json", "error", "error", "upstream-shape", 1},
		{"an unparseable body", "secrets-unparseable.txt", "policies-unparseable.json", "error", "error", "upstream-shape", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var secrets, policies string
			if tc.secrets != "" {
				secrets = syntheticListing(t, tc.secrets)
			}
			if tc.policies != "" {
				policies = syntheticListing(t, tc.policies)
			}
			e := doctorEnv(t, "svc-alpha", "echo value", secrets, policies)

			rep, res := runDoctor(t, e)

			assert.Equal(t, tc.exit, res.Code, res.Stderr)
			for slug, want := range map[string]string{"sbx-global-secret": tc.global, "sbx-broad-allow": tc.broad} {
				got, errSlug := rep.result(t, slug)
				assert.Equal(t, want, got, slug)
				assert.Equal(t, tc.errSlug, errSlug, slug)
			}
			// The one finding every case shares: no toolchain is
			// acknowledged on a fresh machine, so 6 is the exit of a
			// report with no error.
			got, _ := rep.result(t, "toolchain-approval-required")
			assert.Equal(t, "fail", got)
		})
	}
}

// TestDoctorOnAPlainBuild: a romeu built with plain go build, which
// embeds no julieta, runs doctor as the embedded build does: every
// check runs, none reports an error, and julieta-not-embedded appears
// nowhere (04 4.2; moved from T043 on 2026-10-30). doctor writes
// nothing: no state directory and no root.
func TestDoctorOnAPlainBuild(t *testing.T) {
	t.Parallel()
	e := doctorEnv(t, "github", "echo value", "", "")

	rep, res := runDoctor(t, e)

	assert.NotContains(t, res.Stdout+res.Stderr, "julieta-not-embedded")
	for _, c := range rep.Checks {
		assert.NotEqual(t, "error", c.Result, "%s: %v %s", c.Slug, c.Error, c.Message)
	}
	assert.Equal(t, 6, res.Code, "the fresh machine has no acknowledged toolchain: %s", res.Stderr)
	assert.NoDirExists(t, e.StateDir(), "doctor makes no state directory")
	assert.NoDirExists(t, e.Root, "doctor makes no root")

	text := e.Run(t, []string{"doctor"})
	assert.Equal(t, 6, text.Code, text.Stderr)
	assert.Contains(t, text.Stdout, "fail  toolchain-approval-required")
	assert.Contains(t, text.Stderr, "check-findings")
}

// doctorSentinel is what the secret argv stub of the sentinel case
// prints.
const doctorSentinel = "SENTINEL-2b7d41c8-e2e-secret-value"

// TestDoctorNeverShowsASecretArgvOutput is the case "doctor never shows
// a secret argv's output" on the binary (I25): a stub that prints the
// sentinel on stdout and on stderr and exits 0, and one that prints it
// and exits 1. The sentinel appears in no stdout, stderr or --json
// output of doctor, nor in any file under HOME and the XDG directories
// after the run; the positive control is the stub's own output.
func TestDoctorNeverShowsASecretArgvOutput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ exit, want string }{{"exit 0", "pass"}, {"exit 1", "fail"}} {
		t.Run(tc.exit, func(t *testing.T) {
			t.Parallel()
			body := "echo " + doctorSentinel + "; echo " + doctorSentinel + " >&2; " + tc.exit
			e := doctorEnv(t, "github", body, "", "")
			stub := filepath.Join(e.Bin, "secret-github")
			control, _ := procgroup.CombinedOutput(t.Context(), procgroup.Command{Class: procgroup.Secret, Argv: []string{stub}})
			require.Contains(t, string(control), doctorSentinel, "positive control: the stub prints the sentinel")

			rep, res := runDoctor(t, e)
			text := e.Run(t, []string{"doctor"})

			got, _ := rep.result(t, "secret-argv")
			assert.Equal(t, tc.want, got)
			for _, out := range []string{res.Stdout, res.Stderr, text.Stdout, text.Stderr} {
				assert.NotContains(t, out, doctorSentinel)
			}
			for _, dir := range []string{e.Home, e.ConfigHome, e.StateHome} {
				require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
					if err != nil || !d.Type().IsRegular() {
						return err
					}
					b, err := os.ReadFile(p)
					if err == nil {
						assert.False(t, bytes.Contains(b, []byte(doctorSentinel)), "%s holds the sentinel", p)
					}
					return nil
				}))
			}
		})
	}
}
