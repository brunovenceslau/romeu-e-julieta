// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "handoff", "list", true
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "list",
			Short:   "list the handoff files of every repo's memory dir, with their kind",
			Example: "julieta handoff list\njulieta handoff list --json",
			Args:    cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return list(env)
			},
		}
	}
	return c
}())

// list prints one line per handoff file, oldest first: its id, its kind,
// its repo and its path; with --json one document (REQ-0415).
func list(env *cli.Env) error {
	sb, err := load()
	if err != nil {
		return err
	}
	var files []File
	for _, r := range sb.m.Repos {
		m, err := memstore.Open(r.Memory)
		if err != nil {
			return fmt.Errorf("the memory dir of %s: %w", esc(r.Dir), err)
		}
		names, err := hf.Names(m.Root())
		_ = m.Close()
		if err != nil {
			return fmt.Errorf("the memory dir of %s: %w", esc(r.Dir), err)
		}
		for _, n := range names {
			files = append(files, fileOf(r.Dir, r.Memory, n.ID, n.Kind))
		}
	}
	slices.SortStableFunc(files, func(a, b File) int {
		if c := strings.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return bytes.Compare([]byte(a.Repo), []byte(b.Repo))
	})
	return output(env, Report{Schema: Schema, Files: files}, func(out *cli.Writer) error {
		for _, f := range files {
			if _, err := fmt.Fprintf(out, "%s  %s  %s  %s\n", f.ID, f.Kind, esc(f.Repo), esc(f.Path)); err != nil {
				return err
			}
		}
		return nil
	})
}
