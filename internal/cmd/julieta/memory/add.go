// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.Appends("a memory entry"), cli.SnapshotNone, cli.AppendsEachRun)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "memory", "add", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var in memstore.Content
		cmd := &cobra.Command{
			Use:   "add",
			Short: "add a memory entry to the repo's memory dir",
			Example: "julieta memory add --title 'Flaky test on arm64' --tag project --tag ci\n" +
				"julieta memory add --title 'Signing waits' --body 'Ask the operator.' --tag feedback --json",
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if err := in.Normal().Check(); err != nil {
					return usage(cmd, err.Error())
				}
				if err := memstore.CheckType(in.Tags); err != nil {
					return usage(cmd, err.Error())
				}
				return add(env, repo, in)
			},
		}
		cmd.Flags().StringVar(&in.Title, "title", "", "the entry's title, one line")
		cmd.Flags().StringVar(&in.Body, "body", "", "the entry's markdown body")
		cmd.Flags().StringArrayVar(&in.Tags, "tag", nil, "a tag of the entry; repeat for more; exactly one is a type tag: "+typeTags)
		cmd.Flags().StringVar(&in.Status, "status", memstore.StatusOpen, "open or done")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// add adds an entry of the content in, or prints the open entry whose
// memory-content digest is the same and writes nothing (04 4.1,
// Idempotency); either way it prints the id, the path and the repo,
// and warns memory-derivable on stderr when the body repeats a line of
// the repo's instruction files (REQ-1290).
func add(env *cli.Env, repo string, in memstore.Content) error {
	t, err := resolve(repo)
	if err != nil {
		return err
	}
	s, err := openStore(env, t)
	if err != nil {
		return readErr(err, t)
	}
	defer func() { _ = s.Close() }()
	e, wrote, err := s.Add(in)
	if err != nil {
		return readErr(err, t)
	}
	rep := Report{Schema: Schema, Repo: t.repo.Dir, Wrote: wrote, Entries: []Entry{entryOf(s.Reader, e)}}
	for _, m := range derivable(t.repo.Path, in.Body) {
		cli.Warn(env.Stderr, "memory-derivable", m)
	}
	return output(env, rep, func(out *cli.Writer) error {
		if wrote {
			return written(out, "added", rep.Entries[0], rep.Repo)
		}
		return written(out, "exists", rep.Entries[0], rep.Repo+" (an open entry holds the same content; nothing written)")
	})
}
