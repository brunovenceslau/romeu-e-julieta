// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import "context"

// UncheckedVSCodeFolders is the entry of Result.Unchecked for the trusted
// folders of VS Code, which are read only through
// Deps.VSCodeTrustedFolders. Deferred decision: the trust store is VS
// Code's SQLite state database, which this package does not read; the
// decision reopens at the first caller that can read it, or at T049 or
// T134.
const UncheckedVSCodeFolders = "vscode-trusted-folders"

// hostEnv is what a host check reads.
type hostEnv struct {
	deps *Deps
	in   *Input
	// unchecked collects the checks that ran in part (Result.Unchecked).
	unchecked *[]string
}

// hostCheck is a row of step 5 (REQ-1206): a check shared with doctor
// whose failure widens a sandbox or makes agent output executable on the
// host, marked pre in 04 4.2. It returns nil, or the error of its row.
type hostCheck struct {
	// slug is the slug of the row, as doctor reports it.
	slug string
	run  func(ctx context.Context, h *hostEnv) error
}

// hostChecks are the checks of step 5 in their order, the seam of rule
// 14 of docs/plan.md: a task that adds a pre check adds one line. The
// signing socket row for a project that uses git-ssh-sign lands with T080,
// and the breadth of the rows with T134.
var hostChecks = []hostCheck{
	{"sbx-global-secret", checkGlobalSecrets},
	{"sbx-broad-allow", checkBroadAllow},
	{"home-symlink", checkHomeSymlinks},
	{"auto-trust", checkAutoTrust},
	{"vscode-trust", checkVSCodeTrust},
}

// hostStep is step 5: the first host check that fails stops the step. A
// listing whose shape is not a recorded shape, or that cannot be parsed,
// fails it with upstream-shape.
func hostStep(ctx context.Context, d *Deps, in *Input, res *Result) error {
	for _, c := range hostChecks {
		if err := c.run(ctx, &hostEnv{deps: d, in: in, unchecked: &res.Unchecked}); err != nil {
			return err
		}
	}
	return nil
}

// HostFindings runs every host check and returns the error of each that
// fails, in the order of step 5, for doctor, which reports them all, and
// the checks that ran in part (Result.Unchecked), which it reports as
// not checked.
func HostFindings(ctx context.Context, d Deps, in Input) (findings []error, unchecked []string) {
	for _, c := range hostChecks {
		if err := c.run(ctx, &hostEnv{deps: &d, in: &in, unchecked: &unchecked}); err != nil {
			findings = append(findings, err)
		}
	}
	return findings, unchecked
}
