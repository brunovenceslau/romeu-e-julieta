// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package install holds the command julieta install (04 4.3, REQ-0403,
// REQ-0404): mise install, locked, in every repo of the manifest, or in
// the one --repo names, skipped when the lock-set digest equals the last
// success. The run is setup.Install, the one the install step of julieta
// setup runs too; this package prints its result.
package install

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

var _ = cli.Register(func() cli.Command {
	// A second install with no change of a lock does nothing; it writes
	// the digest of the last success, which a repeat leaves as it is.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "julieta", "install"
	c.New = func(env *cli.Env) *cobra.Command {
		var opt setup.InstallOptions
		cmd := &cobra.Command{
			Use:   "install",
			Short: "install the locked tools of every repo with mise",
			Example: "julieta install\n" +
				"julieta install --repo api\n" +
				"julieta install --force",
			Long: "Runs mise install, locked, in every repo of the manifest that has a mise.toml, or in the repo --repo names. " +
				"It does nothing when the lock-set digest of the mise.lock files equals the last success; --force installs anyway. " +
				"A repo with a mise.toml and no mise.lock is the error lock-missing, and a download the egress policy blocks is install-blocked.",
			Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return run(c, env, opt)
			},
		}
		cmd.Flags().StringVar(&opt.Repo, "repo", "", "the dir of the one repo to install, as the manifest names it")
		cmd.Flags().BoolVar(&opt.Force, "force", false, "install although the lock-set digest equals the last success")
		return cmd
	}
	return c
}())

// run installs and prints one line for each repo, then the warnings.
func run(c *cobra.Command, env *cli.Env, opt setup.InstallOptions) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	res, err := setup.Install(c.Context(), home, m, opt)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		cli.Warn(env.Stderr, "version-only-tool", w.Message)
	}
	for _, r := range res.Repos {
		if _, err := fmt.Fprintf(env.Stdout, "install %s %s\n", termsafe.Escape(r.Dir), r.State); err != nil {
			return err
		}
	}
	return nil
}
