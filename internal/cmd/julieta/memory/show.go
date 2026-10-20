// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "memory", "show", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		cmd := &cobra.Command{
			Use:     "show <id>",
			Short:   "print one memory entry",
			Example: "julieta memory show 01j9z3k8m6t2q4w5e7r9y1v3k5\njulieta memory show 01j9z3k8m6t2q4w5e7r9y1v3k5 --json",
			Args:    cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := checkID(cmd, args[0]); err != nil {
					return err
				}
				return show(env, repo, args[0])
			},
		}
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// show prints the entry id: its fields, one per line, then its body.
func show(env *cli.Env, repo, id string) error {
	t, err := resolve(repo)
	if err != nil {
		return err
	}
	r, err := memstore.Open(t.repo.Memory)
	if err != nil {
		return readErr(err, t)
	}
	defer func() { _ = r.Close() }()
	e, err := r.Get(id)
	if err != nil {
		return readErr(err, t)
	}
	rep := Report{Schema: Schema, Repo: t.repo.Dir, Entries: []Entry{entryOf(r, e)}}
	return output(env, rep, func(out *cli.Writer) error {
		v := rep.Entries[0]
		_, err := fmt.Fprintf(out, "id       %s\ntitle    %s\nstatus   %s\ntags     %s\ncreated  %s\nupdated  %s\npath     %s\nrepo     %s\n\n%s",
			v.ID, esc(v.Title), v.Status, esc(strings.Join(v.Tags, " ")), v.CreatedAt.Format(time.RFC3339), v.UpdatedAt.Format(time.RFC3339),
			esc(v.Path), esc(rep.Repo), v.Body)
		return err
	})
}
