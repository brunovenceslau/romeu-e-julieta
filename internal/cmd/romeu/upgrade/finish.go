// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"context"
	"crypto/subtle"
	"fmt"
	"path/filepath"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// syncAll is the command line finish runs after gate 1: the steps of
// romeu sync --all, gate 2 included (REQ-1330).
var syncAll = []string{"sync", "--all"}

// finish is romeu upgrade --finish <running>, which the swap step runs
// in place of the process once the operator confirmed the preview
// (REQ-1330): it writes toolchain.json with no second prompt, gate 1
// acknowledged by that confirmation (01 1.4, REQ-0061), then runs the
// steps of romeu sync --all through the command tree of this binary, and
// returns what sync returns. Since that write is gate 1's, the finish
// first needs what the swap leaves it: a terminal on its standard input
// and its standard error, which the exec keeps, and the finish token of
// the swap (token.go), which it consumes; without either it is the
// error usage and writes nothing.
func finish(ctx context.Context, env *cli.Env, running string) error {
	if !onTerminal(stdin()) {
		return noTerminal()
	}
	stateDir, err := hostpaths.StateDir(getenv)
	if err != nil {
		return usage("state", err.Error())
	}
	if err := checkToken(env, stateDir, running); err != nil {
		return err
	}
	h, err := hostpaths.LoadSettings(env, getenv, usage)
	if err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage("home", "HOME is not an absolute path")
	}
	sbx := &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: &h}
	release, err := sbx.RequireFloor(ctx, sbxdrv.Opts{Step: "upgrade --finish"})
	if err != nil {
		return err
	}
	live, err := approve.Measure(release, env.Version)
	if err != nil {
		return err
	}
	if err := acknowledge(env, stateDir, live); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(env.Stdout, "upgraded      romeu %s -> %s\nacknowledged  %s\n",
		termsafe.Escape(running), termsafe.Escape(live.Romeu.Version), termsafe.Escape(filepath.Join(stateDir, "toolchain.json"))); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root := cli.Tree(env)
	root.SetArgs(append([]string{}, syncAll...))
	return root.ExecuteContext(ctx)
}

// acknowledge writes live into toolchain.json of the state directory
// under the state lock, which it releases before sync takes it again.
func acknowledge(env *cli.Env, stateDir string, live approve.Live) error {
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
	return approve.Acknowledge(st, lk, live, env.Now())
}

// checkToken consumes the finish token of stateDir and returns the error
// usage unless it holds: not expired, nor further ahead than a swap
// writes it, made for the sha256 of the running executable, and by the
// romeu of the version running, the argument of --finish.
func checkToken(env *cli.Env, stateDir, running string) error {
	refuse := func(why string) error {
		return usage("finish", "romeu upgrade --finish runs only in place of romeu upgrade, right after its swap: "+why+"; run romeu upgrade")
	}
	tok, err := consumeToken(stateDir)
	if err != nil {
		return refuse(termsafe.Escape(err.Error()))
	}
	now := env.Now()
	if !now.Before(tok.expires) || tok.expires.After(now.Add(tokenLife)) {
		return refuse("the token of the swap expired at " + tok.expires.UTC().Format(time.RFC3339) + ", or is not of this swap")
	}
	exe, err := executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return cli.Internal.Wrap(err, "upgrade: the path of the running binary")
	}
	sum, err := fileSum(exe)
	if err != nil {
		return cli.Internal.Wrap(err, "upgrade: the sha256 of the running binary")
	}
	if subtle.ConstantTimeCompare(sum[:], tok.sum[:]) != 1 {
		return refuse("the token of the swap names another binary than the running one")
	}
	if tok.version != running {
		return refuse("the token of the swap names the version " + tok.version + ", not " + termsafe.Escape(running))
	}
	return nil
}
