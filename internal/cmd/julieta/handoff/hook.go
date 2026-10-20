// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"fmt"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// hookRun is what a hook runs: it returns the text to print on success,
// the repo it ran for (empty when it failed before it knew), and the
// error that fails it.
type hookRun func() (out, repo string, err error)

// runHook runs a session hook (08 8.2). Whatever happens it returns nil,
// so the hook exits 0 (REQ-0672). It records the time of the run
// (REQ-0669). On success it prints the text of the run and forgets the
// recorded failures of the hook, and applies after to the state setup
// wrote; on a failure it prints one stdout line naming the failure and
// julieta status, and records the failure for julieta status and the
// next SessionStart (REQ-0672, REQ-0675).
func runHook(env *cli.Env, hook, command string, run hookRun, after func(*setup.State)) error {
	start := memstore.Stamp(env.Now())
	out, repo, err := run()
	home, herr := setup.Home()
	stamp := func(h *Hooks) {
		if hook == HookSessionStart {
			h.SessionStartAt = start
		} else {
			h.SessionEndAt = start
		}
	}
	if err == nil {
		if herr == nil {
			herr = update(home, func(h *Hooks, s *setup.State) {
				stamp(h)
				h.Failures = slices.DeleteFunc(h.Failures, func(f Failure) bool { return f.Hook == hook })
				if after != nil {
					after(s)
				}
			})
		}
		_, werr := env.Stdout.Write([]byte(out))
		if herr != nil {
			failed(env, command, fmt.Errorf("the state of the hooks was not recorded: %w", herr))
		}
		_ = werr
		return nil
	}
	if herr == nil {
		herr = update(home, func(h *Hooks, _ *setup.State) {
			stamp(h)
			f := Failure{At: start, Hook: hook, Message: err.Error(), Repo: repo, Status: cli.ExitCode(err)}
			h.Failures = slices.DeleteFunc(h.Failures, func(o Failure) bool { return o.Hook == hook && o.Repo == repo })
			h.Failures = append(h.Failures, f)
		})
	}
	if herr != nil {
		err = fmt.Errorf("%w (and the failure was not recorded: %w)", err, herr)
	}
	failed(env, command, err)
	return nil
}

// failed prints the one stdout line of a failed hook (REQ-0672).
func failed(env *cli.Env, command string, err error) {
	_, _ = fmt.Fprintf(env.Stdout, "%s failed: %s; run julieta status\n", command, oneLine(err.Error()))
}
