// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// hookRun is what a hook runs: it returns the text to print on success,
// the repo it ran for (empty when it failed before it knew), and the
// error that fails it.
type hookRun func() (out, repo string, err error)

// runHook runs a session hook (08 8.2). Whatever happens it returns nil,
// so the hook exits 0 (REQ-0672). It records the time of the run
// (REQ-0669). On success it prints the text of the run and forgets the
// recorded failures of the hook for that repo, and applies after to the state setup
// wrote through setup.UpdateState; on a failure it prints one stdout line naming the failure and
// julieta status, and records the failure for julieta status and the
// next SessionStart (REQ-0672, REQ-0675).
func runHook(env *cli.Env, hook, command string, run hookRun, after func() func(*setup.State) error) error {
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
			herr = updateHooks(home, stamp)
		}
		if herr == nil && after != nil {
			if f := after(); f != nil {
				herr = setup.UpdateState(home, f)
			}
		}
		if herr == nil {
			// Only this hook in this repo is forgotten, and a failure
			// recorded before the repo was known.
			herr = errors.Join(hooks.Clear(failuresDir(home), repo, hook), hooks.Clear(failuresDir(home), "", hook))
		}
		_, _ = env.Stdout.Write([]byte(out))
		if herr != nil {
			failed(env, command, fmt.Errorf("the state of the hooks was not recorded: %w", herr))
		}
		return nil
	}
	if herr == nil {
		herr = errors.Join(updateHooks(home, stamp),
			hooks.Record(failuresDir(home), hooks.Failure{At: start, ExitCode: cli.ExitCode(err), Hook: hook, Message: capMessage(err.Error()), Repo: repo, Stage: command}))
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

// maxMessage is the most bytes of a failure message the store keeps; the
// print cuts at 300 as well.
const maxMessage = 300

// capMessage cuts s to maxMessage bytes at a character.
func capMessage(s string) string {
	if len(s) <= maxMessage {
		return s
	}
	cut := maxMessage
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
