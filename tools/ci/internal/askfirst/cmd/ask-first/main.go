// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command pr is the step of the ask-first job (12 12.4, REQ-1150): it judges
// the pull request of the event payload file it is given, the one
// GITHUB_EVENT_PATH names, from the base branch's checkout it runs in.
//
//	go run ./tools/ci/internal/askfirst/cmd/ask-first "$GITHUB_EVENT_PATH"
//
// It is the code of "go run ./tools/ci ask-first" without the framework of
// tools/ci: it imports the approval package and what that imports, every
// package of it on the approvals surface (TestPRImports), so no change
// that a grant covers changes the code that judges a pull request. Unlike
// the local command, it fetches head.sha when the clone lacks it, without
// credentials (REQ-1152).
//
// It exits 0 when the pull request passes, 1 when it has a finding, 2
// when it could not judge it, and 128 plus the signal number when SIGINT
// or SIGTERM cut it short (procgroup.Main).
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst/approval"
)

// usage is the one form of the command line.
const usage = "usage: pr <event payload file>"

func main() {
	os.Exit(procgroup.Main("ask-first", os.Stderr, func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	}))
}

// run judges the pull request of the one payload file args names, in
// the repository of the working directory, and returns the exit status.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	ok, err := judge(ctx, args[0], stdout)
	switch {
	case err != nil:
		_, _ = fmt.Fprintln(stderr, git.SafeLines("pr: "+err.Error()))
		return 2
	case !ok:
		return 1
	}
	return 0
}

// judge reads the payload, judges the pull request with the fetch of a
// missing head on, and prints the findings. Git runs in an environment
// cleared of the variables that choose another repository or
// configuration (git.CleanEnv).
func judge(ctx context.Context, payload string, stdout io.Writer) (bool, error) {
	pr, err := approval.ReadPayload(payload)
	if err != nil {
		return false, err
	}
	findings, err := approval.Judge(ctx, git.Repo{Dir: ".", Env: git.CleanEnv(nil)}, pr, true)
	if err != nil {
		return false, err
	}
	return approval.Print(stdout, findings), nil
}
