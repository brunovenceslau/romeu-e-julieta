// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package doctor is the command romeu doctor (04 4.2, REQ-0330): the
// host health and security checks of "Checks run by romeu doctor", one
// file per check, registered through rows.go. It reads host settings,
// sbx, git, the host tree and the state directory, runs each secret argv
// once, keeps no output of it, takes no lock and writes nothing. The
// checks marked pre are the functions of internal/preflight (step 5),
// called from here, so one implementation serves both (REQ-0365).
//
// Every check runs and is printed; the exit is 1 when one could not run,
// 6 when every one ran and one failed, and 0 otherwise (REQ-0331). A
// romeu built with plain go build, which embeds no julieta, runs it as
// the embedded build does: nothing here needs the embedded julieta.
//
// all/import_doctor.go links the command into romeu.
package doctor

import (
	"context"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "romeu", "doctor", true
	c.NoJourney = "J1 step 4 and J9 run it as an [O] step whose report the operator reads and acts on; " +
		"e2e/doctor_test.go runs it on the recorded sbx sessions"
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "doctor",
			Short:   "check the health and the security of this host",
			Example: "romeu doctor\nromeu doctor --json",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				ctx := c.Context()
				if err := ctx.Err(); err != nil {
					return err
				}
				h := newHost(env, hostSeams())
				defer h.close()
				return run(ctx, env, h)
			},
		}
	}
	return c
}())

// hostSeams returns the seams of the process: its environment, sbx and
// git by the absolute paths of the settings, and the secret argv run
// through internal/procgroup.
func hostSeams() seams {
	return seams{
		getenv: os.Getenv,
		goos:   runtime.GOOS,
		sbx: func(h *spec.HostSettings, home string) sbxReader {
			return runnerSbx{&sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: h}}
		},
		gitFloor: func(h *spec.HostSettings, home string) func(context.Context) error {
			return func(ctx context.Context) error {
				g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: h.Tools.Git, Home: home})
				if err != nil {
					return usage("settings-invalid", err.Error())
				}
				return g.CheckFloor(ctx)
			}
		},
		secret: runSecretArgv,
	}
}

// secretPath is the PATH a secret argv runs with (03 3.4, REQ-0208).
const secretPath = "PATH=/usr/bin:/bin"

// runSecretArgv runs one secret argv through internal/procgroup, under
// its class, with exactly HOME and PATH=/usr/bin:/bin (03 3.4; 04 4.1,
// Subprocesses, item 1), and its standard input empty. Its standard
// output is tested for emptiness only and its standard error is
// discarded: neither is kept, printed or written anywhere (I25). The
// error is procgroup's, whose text names the program and its status and
// holds none of its output.
func runSecretArgv(ctx context.Context, argv []string, home string) (bool, error) {
	var out nonEmpty
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Secret,
		Argv:   argv,
		Dir:    home,
		Env:    []string{"HOME=" + home, secretPath},
		Stdout: &out,
	})
	return bool(out), err
}

// nonEmpty is a writer that keeps nothing of what it is given and only
// remembers whether it was given a byte.
type nonEmpty bool

// Write records whether p holds a byte.
func (n *nonEmpty) Write(p []byte) (int, error) {
	if len(p) > 0 {
		*n = true
	}
	return len(p), nil
}
