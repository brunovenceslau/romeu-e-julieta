// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package status holds the command julieta status (04 4.3, REQ-0424):
// the state of each repo of the manifest, the live spec commit, the
// egress set romeu applied, the recorded hook failures and julieta's
// warnings. Each part of the
// report is a section of its own file, registered in sections.go. It
// writes nothing, and a read that fails is shown in the report, never
// the exit status: status exits 0, or 2 without a manifest (REQ-0425).
package status

import (
	"context"
	"encoding/json/v2"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "julieta", "status", true
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "status",
			Short:   "print the state of the repos, the manifest, the recorded hook failures and julieta's warnings",
			Example: "julieta status\njulieta status --json",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return status(c.Context(), env)
			},
		}
	}
	return c
}())

// status reads every section and prints the report.
func status(ctx context.Context, env *cli.Env) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	git, err := setup.NewGit(home)
	if err != nil {
		return err
	}
	rep := Report{Schema: Schema, Egress: []string{}, Errors: []string{}, Repos: []Repo{}, Warnings: []setup.Warning{}}
	src := &source{home: home, m: m, git: git}
	if src.state, err = setup.ReadState(home); err != nil {
		rep.Errors = append(rep.Errors, err.Error())
	}
	for _, s := range sections {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.fill(ctx, src, &rep)
	}
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	for _, e := range rep.Errors {
		if _, err := env.Stdout.Write([]byte("error  " + e + "\n")); err != nil {
			return err
		}
	}
	for _, s := range sections {
		if err := s.text(env.Stdout, env.Stderr, &rep); err != nil {
			return err
		}
	}
	return nil
}
