// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

package git_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

func TestRun(t *testing.T) {
	r := gittest.New(t)
	r.Write("a.txt", "a\n")
	sha := r.Commit("a")

	tests := []struct {
		name    string
		stdin   string
		args    []string
		want    string
		wantErr string
	}{
		{"standard output is returned", "", []string{"rev-parse", "HEAD"}, sha + "\n", ""},
		{"standard input is passed", sha + "\n", []string{"cat-file", "--batch-check"}, sha + " commit ", ""},
		{"a failing command carries what git said", "", []string{"rev-parse", "--verify", "nonesuch"}, "", "git rev-parse:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := r.Run(context.Background(), []byte(tt.stdin), tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "fatal") {
					t.Fatalf("error = %v, want one that holds %q and git's message", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(out), tt.want) {
				t.Errorf("output = %q, want prefix %q", out, tt.want)
			}
		})
	}
}

func TestRunStopsWithItsContext(t *testing.T) {
	r := gittest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Run(ctx, nil, "rev-parse", "--git-dir"); err == nil {
		t.Error("Run succeeded under a cancelled context")
	}
}
