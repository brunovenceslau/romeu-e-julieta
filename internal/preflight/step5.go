// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import "context"

// hostEnv is what a host check reads.
type hostEnv struct {
	deps *Deps
	in   *Input
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
func hostStep(ctx context.Context, d *Deps, in *Input) error {
	for _, c := range hostChecks {
		if err := c.run(ctx, &hostEnv{deps: d, in: in}); err != nil {
			return err
		}
	}
	return nil
}

// HostFindings runs every host check and returns the error of each that
// fails, in the order of step 5, for doctor, which reports them all.
func HostFindings(ctx context.Context, d Deps, in Input) []error {
	var out []error
	for _, c := range hostChecks {
		if err := c.run(ctx, &hostEnv{deps: &d, in: &in}); err != nil {
			out = append(out, err)
		}
	}
	return out
}
