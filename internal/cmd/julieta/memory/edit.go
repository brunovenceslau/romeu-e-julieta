// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "memory", "edit", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo, title, body, status string
		var tags []string
		cmd := &cobra.Command{
			Use:   "edit <id>",
			Short: "change a memory entry; --status done closes it",
			Example: "julieta memory edit 01j9z3k8m6t2q4w5e7r9y1v3k5 --status done\n" +
				"julieta memory edit 01j9z3k8m6t2q4w5e7r9y1v3k5 --title 'Flaky test on arm64 runners' --json",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var ch write.Change
				f := cmd.Flags()
				if f.Changed("title") {
					ch.Title = &title
				}
				if f.Changed("body") {
					ch.Body = &body
				}
				if f.Changed("tag") {
					ch.Tags = &tags
				}
				if f.Changed("status") {
					ch.Status = &status
				}
				if err := checkID(cmd, args[0]); err != nil {
					return err
				}
				if err := checkChange(ch); err != nil {
					return usage(cmd, err.Error())
				}
				if ch.Tags != nil {
					if err := memstore.CheckType(*ch.Tags); err != nil {
						return usage(cmd, err.Error())
					}
				}
				return edit(env, cmd, repo, args[0], ch)
			},
		}
		cmd.Flags().StringVar(&title, "title", "", "the new title")
		cmd.Flags().StringVar(&body, "body", "", "the new body")
		cmd.Flags().StringArrayVar(&tags, "tag", nil, "a tag; the tags given replace the entry's, and exactly one of them is a type tag: "+typeTags)
		cmd.Flags().StringVar(&status, "status", "", "open or done; done closes the entry")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// edit applies ch to the entry id and prints it; an edit that changes
// nothing writes nothing and says so. An edit that names no tag keeps
// the entry's, so on an entry whose tags hold no type tag, or two, it
// is the error usage too (03 3.9, REQ-1287): the edit that types such
// an entry names its tags (REQ-1292).
func edit(env *cli.Env, cmd *cobra.Command, repo, id string, ch write.Change) error {
	t, err := resolve(repo)
	if err != nil {
		return err
	}
	s, err := openStore(env, t)
	if err != nil {
		return readErr(err, t)
	}
	defer func() { _ = s.Close() }()
	if ch.Tags == nil {
		if have, err := s.Get(id); err == nil {
			if err := memstore.CheckType(have.Tags); err != nil {
				return usage(cmd, err.Error()+"; name the tags with --tag")
			}
		}
	}
	e, wrote, err := s.Edit(id, ch)
	if err != nil {
		return readErr(err, t)
	}
	rep := Report{Schema: Schema, Repo: t.repo.Dir, Wrote: wrote, Entries: []Entry{entryOf(s.Reader, e)}}
	return output(env, rep, func(out *cli.Writer) error {
		if wrote {
			return written(out, "edited", rep.Entries[0], rep.Repo)
		}
		return written(out, "unchanged", rep.Entries[0], rep.Repo)
	})
}
