// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "memory", "list", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var f memstore.Filter
		cmd := &cobra.Command{
			Use:     "list",
			Short:   "list the memory entries of the repo; --query filters",
			Example: "julieta memory list\njulieta memory list --query arm64 --status open --json",
			Args:    cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return list(env, repo, f)
			},
		}
		cmd.Flags().StringVar(&f.Query, "query", "", "list the entries whose title, body or a tag holds this text, ignoring case")
		cmd.Flags().StringVar(&f.Status, "status", "", "list the entries of this status only: open or done")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// list prints the entries f selects, one line each in text: the id,
// the status, the title and the tags.
func list(env *cli.Env, repo string, f memstore.Filter) error {
	if f.Status != "" && f.Status != memstore.StatusOpen && f.Status != memstore.StatusDone {
		return fmt.Errorf("the status %q is not %s or %s", f.Status, memstore.StatusOpen, memstore.StatusDone)
	}
	t, err := resolve(repo)
	if err != nil {
		return err
	}
	r, err := memstore.Open(t.repo.Memory)
	if err != nil {
		return readErr(err, t)
	}
	defer func() { _ = r.Close() }()
	entries, err := r.List(f)
	if err != nil {
		return readErr(err, t)
	}
	rep := Report{Schema: Schema, Repo: t.repo.Dir}
	for _, e := range entries {
		rep.Entries = append(rep.Entries, entryOf(r, e))
	}
	return output(env, rep, func(out *cli.Writer) error {
		for _, e := range rep.Entries {
			line := e.ID + "  " + e.Status + "  " + termsafe.Escape(e.Title)
			if len(e.Tags) > 0 {
				line += "  [" + strings.Join(e.Tags, " ") + "]"
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
		return nil
	})
}
