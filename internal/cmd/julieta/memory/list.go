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
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "memory", "list", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo, since string
		var f memstore.Filter
		cmd := &cobra.Command{
			Use:   "list",
			Short: "list the memory entries of the repo, one index line each, newest first",
			Example: "julieta memory list\n" +
				"julieta memory list --tag feedback --since 2026-10-28 --limit 20\n" +
				"julieta memory list --query arm64 --status open --json",
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if f.Status != "" && f.Status != memstore.StatusOpen && f.Status != memstore.StatusDone {
					return usage(cmd, fmt.Sprintf("--status \"%s\" is not %s or %s", esc(f.Status), memstore.StatusOpen, memstore.StatusDone))
				}
				if f.Sort != memstore.SortUpdated && f.Sort != memstore.SortCreated {
					return usage(cmd, fmt.Sprintf("--sort \"%s\" is not %s or %s", esc(f.Sort), memstore.SortUpdated, memstore.SortCreated))
				}
				if cmd.Flags().Changed("limit") && f.Limit < 1 {
					return usage(cmd, fmt.Sprintf("--limit %d is not a number of entries above zero", f.Limit))
				}
				if cmd.Flags().Changed("since") {
					t, err := parseSince(since)
					if err != nil {
						return usage(cmd, err.Error())
					}
					f.Since = t
				}
				return list(env, repo, f)
			},
		}
		fl := cmd.Flags()
		fl.StringVar(&f.Query, "query", "", "list the entries whose title, body or a tag holds this text, ignoring case")
		fl.StringVar(&f.Status, "status", "", "list the entries of this status only: open or done")
		fl.StringArrayVar(&f.Tags, "tag", nil, "list the entries that carry this tag; repeat for more, and every one must hold")
		fl.StringVar(&since, "since", "", "list the entries updated at or after this RFC 3339 date or timestamp; a date is midnight UTC")
		fl.IntVar(&f.Limit, "limit", 0, "list at most this many entries")
		fl.StringVar(&f.Sort, "sort", memstore.SortUpdated, "order newest first by updated or by created")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// parseSince reads the value of --since: an RFC 3339 date, read as
// midnight UTC, or an RFC 3339 timestamp (04 4.3, REQ-1293).
func parseSince(s string) (time.Time, error) {
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("--since \"%s\" is not an RFC 3339 date (2026-10-28) or timestamp (2026-10-28T10:00:00Z)", esc(s))
}

// list prints the entries f selects, one index line each in text
// (REQ-1288): the id, the repo, the status, the type, the date of
// updatedAt and the title.
func list(env *cli.Env, repo string, f memstore.Filter) error {
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
			if _, err := fmt.Fprintln(out, indexLine(e, rep.Repo)); err != nil {
				return err
			}
		}
		return nil
	})
}

// indexLine returns the index line of the entry e of the repo dir repo
// (04 4.3, REQ-1288): its id, the repo, its status, its type tag or -,
// the date of its updatedAt and its title, the repo and the title
// escaped, so no text an agent wrote breaks the line (REQ-0670).
func indexLine(e Entry, repo string) string {
	return strings.Join([]string{e.ID, esc(repo), e.Status, memstore.Type(e.Tags), e.UpdatedAt.UTC().Format(time.DateOnly), esc(e.Title)}, " ")
}
