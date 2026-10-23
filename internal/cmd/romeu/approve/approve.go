// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package approve is the command romeu approve --toolchain, gate 1 (01
// 1.4; 04 4.2, REQ-0310): it reads the live sbx version, prints what
// changed in the toolchain of this machine since it was acknowledged,
// asks on the terminal, and writes toolchain.json (03 3.8, REQ-0228).
// It also holds gate 1's check, Check, which the preflight and sync
// call, and Changes, which status and doctor report.
package approve

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The process's environment and standard input; a test replaces them.
var (
	getenv = os.Getenv
	stdin  = func() io.Reader { return os.Stdin }
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "approve"
	c.NoJourney = "J1 step 4 and J9 run it as an [O] step at a terminal, a precondition the scenario functions make outside a Step " +
		"(their World runs lines without a terminal); e2e/approve_test.go drives it through a /dev/ptmx pair"
	c.New = func(env *cli.Env) *cobra.Command {
		var toolchain bool
		cmd := &cobra.Command{
			Use:     "approve --toolchain",
			Short:   "acknowledge the toolchain of this machine (gate 1)",
			Example: "romeu approve --toolchain",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return run(c, env)
			},
		}
		cmd.Flags().BoolVar(&toolchain, "toolchain", false, "acknowledge the sbx version, romeu, the julieta it embeds and its catalog (required)")
		_ = cmd.MarkFlagRequired("toolchain")
		return cmd
	}
	return c
}())

// usage returns the error usage (RJ-349) for a precondition the command
// stops at, named by the slug of the rule it breaks (REQ-1258: a rule
// with no row of 04 4.4 has no id of its own), with exit 2 and the hint
// of the help of the command.
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " approve"}
	return e
}

// noTerminal is the refusal without a terminal on stdin (exit 2, 04
// 4.2): gate 1 is TTY-only (01 1.4, REQ-0059), and stdin decides the
// TTY (04 4.1, REQ-0272).
func noTerminal() error {
	return usage("no-tty", "romeu approve --toolchain needs a terminal on its standard input; run it in one")
}

// run is the command.
func run(cmd *cobra.Command, env *cli.Env) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
	in := stdin()
	if f, ok := in.(*os.File); !ok || !term.IsTerminal(int(f.Fd())) {
		return noTerminal()
	}
	h, err := loadSettings(env)
	if err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage("home", "HOME is not an absolute path")
	}
	sbx := &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: &h}
	release, err := sbx.RequireFloor(ctx, sbxdrv.Opts{Step: "approve --toolchain"})
	if err != nil {
		return err
	}
	live, err := Measure(release, env.Version)
	if err != nil {
		return err
	}

	stateDir, err := state.DefaultDir(getenv)
	if err != nil {
		return usage("state", err.Error())
	}
	st, err := state.Open(stateDir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	lk, err := st.Lock()
	if err != nil {
		return err
	}
	defer lk.Unlock()
	rec, err := Load(st)
	if err != nil {
		return err
	}
	if rec != nil && len(Changes(rec, live)) == 0 {
		_, err := fmt.Fprintf(env.Stdout, "gate 1: the toolchain of this machine is acknowledged already (sbx %s, romeu %s)\n", live.Sbx, live.Romeu.Version)
		return err
	}
	projects, err := appliedEgress(st)
	if err != nil {
		return err
	}
	text, err := Text(rec, live, projects)
	if err != nil {
		return err
	}
	ok, err := gate.Confirm(in, env.Stderr, text)
	switch {
	case errors.Is(err, gate.ErrNoTerminal):
		return noTerminal()
	case err != nil:
		return err
	case !ok:
		_, err := fmt.Fprintln(env.Stderr, "declined: toolchain.json is unchanged")
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := st.UpdateToolchain(lk, func(t *state.Toolchain) error {
		return acknowledge(t, live, env.Now())
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "acknowledged  %s\n", termsafe.Escape(filepath.Join(stateDir, "toolchain.json")))
	return err
}

// appliedEgress returns, for each project with a record, the domains
// romeu applied to its sandbox: what tells which projects a catalog
// change affects. A record that does not decode is record-unreadable
// (RJ-334); a file under projects/ that is not a record is skipped.
func appliedEgress(st *state.Store) (map[string][]string, error) {
	entries, err := os.ReadDir(filepath.Join(st.Dir(), "projects"))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string][]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gate 1: the project records: %w", err)
	}
	out := map[string][]string{}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		p, err := st.LoadProject(name)
		if errors.Is(err, state.ErrInvalidName) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[name] = p.EgressApplied
	}
	return out, nil
}
