// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package vulnerabilities is the vulnerabilities step of 10 10.2:
// govulncheck over the module. govulncheck runs in the environment of
// the steps with check.NetworkPassThrough, since it reads the Go
// vulnerability database, from the path mise.WhichPinned finds below
// the directory where mise installs it and at the version mise.lock
// locks, never through "mise exec". A govulncheck that is not found
// fails this step and no other. It reads the tree and runs none of its
// code, so it runs before unit.
package vulnerabilities

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// fetchFailed starts the line govulncheck prints when it cannot fetch
// the vulnerability database, whatever the cause: an unreachable
// network, a proxy that refuses, a server error, a certificate.
const fetchFailed = "govulncheck: fetching vulnerabilities:"

// notice is the line a failed fetch adds: a run that could not read the
// database is not the equivalent run (10 10.2).
const notice = "vulnerabilities: could not fetch the vulnerability database, so this all is not the equivalent run (10 10.2)"

// Steps returns the vulnerabilities step for the tree at p.Root.
func Steps(ctx context.Context, p check.Plan) []check.Step {
	s := check.Step{Name: "vulnerabilities", Environ: check.WithProcessEnv(check.StepEnv(p.Tools.GoDir), check.NetworkPassThrough), NoticeOn: fetchFailed, Notice: notice}
	if path, err := mise.WhichPinned(ctx, p.Root, "govulncheck", mise.GovulncheckLock, mise.GovulncheckDir); err != nil {
		s.Unavailable = err
	} else {
		s.Argv = []string{path, "./..."}
	}
	return []check.Step{s}
}
