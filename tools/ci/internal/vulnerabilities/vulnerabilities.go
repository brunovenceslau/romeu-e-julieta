// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package vulnerabilities is the vulnerabilities step of 10 10.2:
// govulncheck over the module. govulncheck runs in the environment of
// the steps with check.NetworkPassThrough, since it reads the Go
// vulnerability database, from the path mise.WhichPinned finds below
// the directory where mise installs it and at the version mise.lock
// locks, never through "mise exec". A govulncheck that is not found
// fails this step and no other. It runs once per target of
// lint.Targets, with the target in GOOS and GOARCH (govulncheck reads
// them as the go command does), so the Linux job analyses the code that
// builds on darwin alone, which no job of the macOS runner covers. It
// reads the tree and runs none of its code, so it runs before race.
package vulnerabilities

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// fetchFailed starts the line govulncheck prints when it cannot fetch
// the vulnerability database, whatever the cause: an unreachable
// network, a proxy that refuses, a server error, a certificate.
const fetchFailed = "govulncheck: fetching vulnerabilities:"

// notice is the line a failed fetch adds: a run that could not read the
// database is not the equivalent run (10 10.2).
const notice = "vulnerabilities: could not fetch the vulnerability database, so this all is not the equivalent run (10 10.2)"

// Steps returns the vulnerabilities steps for the tree at p.Root, one
// per lint target.
func Steps(ctx context.Context, p check.Plan) []check.Step {
	path, err := mise.WhichPinned(ctx, p.Root, "govulncheck", mise.GovulncheckLock, mise.GovulncheckDir)
	var steps []check.Step
	for _, target := range lint.Targets {
		s := check.Step{Name: "vulnerabilities " + target.String(), Environ: check.WithProcessEnv(lint.Env(target, p.Tools.GoDir), check.NetworkPassThrough), NoticeOn: fetchFailed, Notice: notice}
		if err != nil {
			s.Unavailable = err
		} else {
			s.Argv = []string{path, "./..."}
		}
		steps = append(steps, s)
	}
	return steps
}
