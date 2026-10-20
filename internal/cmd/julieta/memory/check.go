// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub = "julieta", "memory", "check"
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "check",
			Short:   "check that every memory dir is mounted, writable and in its layout",
			Example: "julieta memory check",
			Args:    cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return check(env)
			},
		}
	}
	return c
}())

// check checks the memory dir of every repo of the manifest (04 4.3,
// REQ-0412): one line per dir, then one line per finding, "finding
// <rule> <dir>/<path>", on stdout, and one line per warning on stderr,
// a leftover temporary file or a handoff/ over its cap, which refuses
// nothing (REQ-0650, REQ-0654). A dir that is not mounted or not
// writable fails the command (exit 1), and so does a finding.
//
// A finding is the error check-findings (RJ-328, exit 6, REQ-0435) by
// 04 4.3. That row is not declared in code yet: it lands in this
// package's errors.go, on the contracts surface of
// .github/ask-first.yaml, and until then a finding fails the command
// with an error that carries no id, exit 1.
func check(env *cli.Env) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	var failed, findings []string
	for _, r := range m.Repos {
		rep, err := memstore.CheckDir(r.Memory)
		if err != nil {
			failed = append(failed, r.Dir)
			if _, err := fmt.Fprintf(env.Stdout, "memory %s %s error: %s\n", esc(r.Dir), esc(r.Memory), esc(err.Error())); err != nil {
				return err
			}
			continue
		}
		state := "ok"
		if len(rep.Findings) > 0 {
			state = fmt.Sprintf("%d findings", len(rep.Findings))
		}
		if _, err := fmt.Fprintf(env.Stdout, "memory %s %s %s\n", esc(r.Dir), esc(r.Memory), state); err != nil {
			return err
		}
		for _, f := range rep.Findings {
			findings = append(findings, f.Rule)
			if _, err := fmt.Fprintf(env.Stdout, "finding %s %s\n", f.Rule, esc(filepath.Join(r.Memory, f.Path))); err != nil {
				return err
			}
		}
		for _, w := range rep.Warnings {
			if _, err := fmt.Fprintf(env.Stderr, "note %s %s: %s\n", w.Rule, esc(filepath.Join(r.Memory, w.Path)), note(w.Rule)); err != nil {
				return err
			}
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("the memory dir of %s is not mounted or not writable", esc(strings.Join(failed, ", ")))
	}
	if len(findings) > 0 {
		return fmt.Errorf("the memory dirs break the layout of 08 8.1: %d findings: %s", len(findings), strings.Join(findings, ", "))
	}
	return nil
}

// note says what a warning of Check means for the reader.
func note(rule string) string {
	if rule == memstore.RuleTemp {
		return "a leftover julieta temporary file, never read; julieta setup removes it"
	}
	return "handoff/ is over its cap of " + fmt.Sprint(memstore.MaxHandoffFiles) + " files"
}
