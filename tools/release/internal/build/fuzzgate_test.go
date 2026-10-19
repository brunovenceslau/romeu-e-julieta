// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// history is a repository with a fuzz commit and a release commit after
// it: the fuzz commit holds a guard-tagged file and an untagged one.
type history struct {
	root, fuzz, release string
}

func newHistory(t *testing.T, change map[string]string) history {
	t.Helper()
	root := initRepo(t)
	write(t, root, map[string]string{
		"guard.go": "package p\n\n//romeu:invariant I1 guard\nfunc f() {}\n",
		"plain.go": "package p\n",
	})
	fuzz := commit(t, root, "the fuzz commit", commitTime)
	write(t, root, change)
	release := commit(t, root, "the release commit", commitTime.Add(time.Hour))
	return history{root, fuzz, release}
}

// gateFor answers the API with the body at status, and records the
// request it got.
func gateFor(t *testing.T, status int, body string, got *http.Request) *FuzzGate {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			*got = *r.Clone(r.Context())
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return &FuzzGate{API: server.URL, Repo: "owner/romeu-e-julieta", Token: "secret-token"}
}

func runs(event, branch, sha, conclusion string) string {
	return fmt.Sprintf(`{"workflow_runs":[{"event":%q,"head_branch":%q,"head_sha":%q,"status":"completed","conclusion":%q}]}`,
		event, branch, sha, conclusion)
}

// TestFuzzGate holds the fixtures of step 1 of the release steps (10
// 10.2), by their titles there, and "workflow_dispatch fuzz run
// refuses" (round 18, ship gate round 1, F10).
func TestFuzzGate(t *testing.T) {
	plainChange := map[string]string{"plain.go": "package p\n\nvar changed = 1\n"}
	guardChange := map[string]string{"guard.go": "package p\n\n//romeu:invariant I1 guard\nfunc f() { _ = 1 }\n"}

	tests := []struct {
		name    string
		change  map[string]string
		status  int
		body    func(h history) string
		wantErr string
	}{
		{"fuzz run success builds", plainChange, 200,
			func(h history) string { return runs("schedule", "main", h.fuzz, "success") }, ""},
		{"fuzz run of another conclusion refuses", plainChange, 200,
			func(h history) string { return runs("schedule", "main", h.fuzz, "failure") }, "is failure, not success"},
		{"fuzz run still running refuses", plainChange, 200,
			func(h history) string { return runs("schedule", "main", h.fuzz, "") }, "is unknown, not success"},
		{"no fuzz run found refuses", plainChange, 200,
			func(history) string { return `{"workflow_runs":[]}` }, "no scheduled fuzz.yml run on main found"},
		{"fuzz run API error refuses", plainChange, 500,
			func(history) string { return `{"message":"boom"}` }, "the API answered 500"},
		{"fuzz run API answer that is not JSON refuses", plainChange, 200,
			func(history) string { return `<html>` }, "the API answer"},
		{"fuzz run head not an ancestor refuses", plainChange, 200,
			func(history) string {
				return runs("schedule", "main", "0123456789abcdef0123456789abcdef01234567", "success")
			}, "the fuzz run's head"},
		{"fuzz run head that is not a commit id refuses", plainChange, 200,
			func(history) string { return runs("schedule", "main", "--help", "success") }, "is not a commit id"},
		{"guard-tagged file changed since the fuzz run refuses", guardChange, 200,
			func(h history) string { return runs("schedule", "main", h.fuzz, "success") }, "guard.go"},
		{"workflow_dispatch fuzz run refuses", plainChange, 200,
			func(h history) string { return runs("workflow_dispatch", "main", h.fuzz, "success") }, "no scheduled fuzz.yml run on main found"},
		{"scheduled fuzz run of another branch refuses", plainChange, 200,
			func(h history) string { return runs("schedule", "other", h.fuzz, "success") }, "no scheduled fuzz.yml run on main found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHistory(t, tt.change)
			gate := gateFor(t, tt.status, tt.body(h), nil)
			err := gate.Check(t.Context(), h.root)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestFuzzGateSideBranchHeadIsNotAnAncestor: a real commit that the
// release commit does not contain refuses.
func TestFuzzGateSideBranchHeadIsNotAnAncestor(t *testing.T) {
	h := newHistory(t, map[string]string{"plain.go": "package p\n\nvar changed = 1\n"})
	_, err := git(t.Context(), h.root, "checkout", "-q", "-b", "side", h.fuzz)
	require.NoError(t, err)
	write(t, h.root, map[string]string{"side.go": "package p\n"})
	side := commit(t, h.root, "a commit on a side branch", commitTime.Add(2*time.Hour))
	_, err = git(t.Context(), h.root, "checkout", "-q", "main")
	require.NoError(t, err)

	gate := gateFor(t, 200, runs("schedule", "main", side, "success"), nil)
	require.ErrorContains(t, gate.Check(t.Context(), h.root), "is not an ancestor of the release commit")
}

// TestFuzzGateAsksForScheduledRunsOnMain: the request names the
// workflow, the branch and the event, and carries the token.
func TestFuzzGateAsksForScheduledRunsOnMain(t *testing.T) {
	h := newHistory(t, map[string]string{"plain.go": "package p\n\nvar changed = 1\n"})
	var got http.Request
	gate := gateFor(t, 200, runs("schedule", "main", h.fuzz, "success"), &got)
	require.NoError(t, gate.Check(t.Context(), h.root))
	assert.Equal(t, "/repos/owner/romeu-e-julieta/actions/workflows/fuzz.yml/runs", got.URL.Path)
	assert.Equal(t, "main", got.URL.Query().Get("branch"))
	assert.Equal(t, "schedule", got.URL.Query().Get("event"))
	assert.Equal(t, "Bearer secret-token", got.Header.Get("Authorization"))
}

// TestFuzzGateInputs: a repository that is not owner/name and an empty
// token refuse before any request.
func TestFuzzGateInputs(t *testing.T) {
	h := newHistory(t, map[string]string{"plain.go": "package p\n\nvar changed = 1\n"})
	good := runs("schedule", "main", h.fuzz, "success")

	bad := gateFor(t, 200, good, nil)
	bad.Repo = "owner/../x?y"
	require.ErrorContains(t, bad.Check(t.Context(), h.root), "is not owner/name")

	empty := gateFor(t, 200, good, nil)
	empty.Token = ""
	require.ErrorContains(t, empty.Check(t.Context(), h.root), "GH_TOKEN is empty")
}

// TestGitExitCodeOneIsNotAnError documents what ancestor relies on.
func TestGitExitCodeOneIsNotAnError(t *testing.T) {
	h := newHistory(t, map[string]string{"plain.go": "package p\n\nvar changed = 1\n"})
	_, err := git(t.Context(), h.root, "merge-base", "--is-ancestor", h.release, h.fuzz)
	exit, ok := errors.AsType[*procgroup.ExitError](err)
	require.True(t, ok)
	assert.Equal(t, 1, exit.Code)
}
