// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

var _ = registerBuilder("env-exec", func(fx Fixture) []Call {
	return []Call{
		mustCall(EnvExec(fx.Dir, procgroup.JulietaExec, nil, []string{fx.Julieta, "version", "--json"})),
		mustCall(EnvExec(fx.Dir, procgroup.JulietaSetup, []string{"JULIETA_MANIFEST=e30"}, []string{fx.Julieta, "setup", "--json"})),
	}
})
var _ = registerBuilder("env-exec-tty", func(fx Fixture) []Call {
	return []Call{
		mustCall(EnvExecTTY(fx.Dir, []string{"JULIETA_MANIFEST=e30"}, []string{fx.Julieta, "layout", "up"})),
		mustCall(EnvExecTTY(fx.Dir, nil, []string{"/bin/bash", "-l"})),
	}
})

// EnvExec is sbx env exec <dir> [--env NAME=VALUE]... -- <command>: a
// command run in the sandbox without a terminal, such as the calls of
// julieta (04 4.2, run steps 5 and 6), captured. class is the row of the
// timeout table of the command: julieta-setup for julieta setup (mise
// downloads inside it), julieta-exec for every other call of julieta,
// and unbounded for the salvage exec, which stops only on an interrupt
// (04 4.1, Subprocesses, item 5). Anything else is refused.
//
// The argv before "--" is what the fake matches, and the command after
// it is what it forwards (10 10.3, Replay only). A stopped sandbox "is
// started first"
// (https://docs.docker.com/reference/cli/sbx/env/exec/).
func EnvExec(dir string, class procgroup.Class, env, command []string) (Call, error) {
	switch class {
	case procgroup.JulietaSetup, procgroup.JulietaExec, procgroup.Unbounded:
	default:
		return Call{}, fmt.Errorf("sbxdrv: env exec runs under julieta-setup, julieta-exec or unbounded, not %s", class)
	}
	b := (&argvBuilder{}).lit("env", "exec").slot(envDir, dir)
	for _, e := range env {
		b.lit("--env").slot(envAssign, e)
	}
	return b.lit("--").rest(command).call("env-exec", class, Captured)
}

// EnvExecTTY is sbx env exec -it <dir> [--env NAME=VALUE]... --
// <command>: a command run in the sandbox on the terminal, interactive,
// such as julieta layout up (run step 7) and the shell the hint of
// layout-failed names (04 4.2). Nothing bounds it but the user, so its
// class is unbounded.
func EnvExecTTY(dir string, env, command []string) (Call, error) {
	b := (&argvBuilder{}).lit("env", "exec", "-it").slot(envDir, dir)
	for _, e := range env {
		b.lit("--env").slot(envAssign, e)
	}
	return b.lit("--").rest(command).call("env-exec-tty", procgroup.Unbounded, Interactive)
}
