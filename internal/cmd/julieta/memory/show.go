// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"errors"
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
			Use:   "show <id>...",
			Short: "print memory entries in full, in the order given",
			Example: "julieta memory show 01j9z3k8m6t2q4w5e7r9y1v3k5\n" +
				"julieta memory show 01j9z3k8m6t2q4w5e7r9y1v3k5 01j9z3m2a7b4c6d8e0f2g4h6j8 --json",
			Args: cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				for _, id := range args {
					if err := checkID(cmd, id); err != nil {
						return err
					}
				}
				return show(env, repo, args)
			},
		}
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// show prints the entries ids, in the order given (REQ-1289): the
// fields of each, one per line, then its body. An id with no entry is
// passed over and fails the command, naming it, once the known ones
// are printed; any other read error fails it at once.
func show(env *cli.Env, repo string, ids []string) error {
	t, err := resolve(repo)
	if err != nil {
		return err
	}
	r, err := memstore.Open(t.repo.Memory)
	if err != nil {
		return readErr(err, t)
	}
	defer func() { _ = r.Close() }()
	rep := Report{Schema: Schema, Repo: t.repo.Dir}
	var missing []string
	for _, id := range ids {
		e, err := r.Get(id)
		if errors.Is(err, memstore.ErrNotFound) {
			missing = append(missing, id)
			continue
		}
		if err != nil {
			return readErr(err, t)
		}
		rep.Entries = append(rep.Entries, entryOf(r, e))
	}
	err = output(env, rep, func(out *cli.Writer) error {
		for i, v := range rep.Entries {
			sep := ""
			if i > 0 {
				sep = "\n"
				if !strings.HasSuffix(rep.Entries[i-1].Body, "\n") {
					sep = "\n\n"
				}
			}
			_, err := fmt.Fprintf(out, "%sid       %s\ntitle    %s\nstatus   %s\ntags     %s\ncreated  %s\nupdated  %s\npath     %s\nrepo     %s\n\n%s",
				sep, v.ID, esc(v.Title), v.Status, esc(strings.Join(v.Tags, " ")), v.CreatedAt.Format(time.RFC3339), v.UpdatedAt.Format(time.RFC3339),
				esc(v.Path), esc(rep.Repo), v.Body)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("the memory dir %s of the repo %s has no entry %s", t.repo.Memory, t.repo.Dir, strings.Join(missing, ", "))
	}
	return nil
}
