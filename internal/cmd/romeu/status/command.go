// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// getenv reads the environment; a test replaces it.
var getenv = os.Getenv

// The command romeu status (04 4.2): read only, so it runs no
// preflight, takes no lock, appends nothing and takes no snapshot
// (REQ-0328). It needs no embedded julieta: a plain go build runs it as
// the release build does (T048, from T043).
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "romeu", "status", true
	c.NoJourney = "the J10 scenario of T068 adds its line, step 1 of J10; until then its own e2e cases assert its output"
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "status [--json] [<name>]",
			Short:   "report each project's sandbox, generations, tree and toolchain, read only",
			Example: "romeu status shop",
			Args:    cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, names []string) error {
				h, closeStore, err := newHost(env)
				if err != nil {
					return err
				}
				defer closeStore()
				return run(c.Context(), env, h, names)
			},
		}
	}
	return c
}())

// usage returns the error usage (RJ-349) for a precondition status stops
// at, named by the slug of the rule it breaks, with the hint of the
// help of the command.
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " status"}
	return e
}

// newHost builds what status reads from the host settings (through
// hostpaths, the one reader of their path): the state directory open to
// read, or none when it does not exist yet, the root of the trees, sbx,
// and the measure of this build's toolchain. The returned function
// closes the store.
func newHost(env *cli.Env) (host, func(), error) {
	settings, err := hostpaths.LoadSettings(env, getenv, usage)
	if err != nil {
		return host{}, nil, err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return host{}, nil, usage("home", "HOME is not an absolute path")
	}
	dir, err := hostpaths.StateDir(getenv)
	if err != nil {
		return host{}, nil, usage("state", err.Error())
	}
	h := host{
		root:    settings.Root,
		sbx:     &sbxdrv.Runner{Sbx: settings.Tools.Sbx, Home: home, Host: &settings},
		measure: func(v sbxdrv.Release) (approve.Live, error) { return approve.Measure(v, env.Version) },
	}
	store, err := state.OpenRead(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return h, func() {}, nil
	case err != nil:
		return host{}, nil, StatusIncomplete.Wrap(err, "the host state cannot be opened")
	}
	h.store = store
	return h, func() { _ = store.Close() }, nil
}
