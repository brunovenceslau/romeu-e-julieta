// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package setup is the setup subcommand of tools/ci, which prepares a
// machine to run the checks: it installs the mise that ci.yml pins when
// another one is found, trusts mise.toml, installs the tools mise.lock
// locks, and fills the module cache. It registers no check.
package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// Run prepares a machine to run the checks, in three steps: it
// trusts the repository's mise.toml, installs the tools mise.lock
// locks, and fills the module cache with the modules of go.sum, which
// the steps read with GOPROXY=off. The first two are what a new
// machine runs by hand otherwise ("mise trust", "mise install"). The
// hosted workflow runs it before all: there the mise action trusts the
// configuration only through MISE_TRUSTED_CONFIG_PATHS, which no step
// of tools/ci keeps (goenv.PassThrough), and installs the tools already, so
// the second step finds nothing to do.
//
// Before them, when the mise that tools/ci starts (mise.Path) names
// another version than the one ci.yml pins, or none, setup installs
// that pin in the user cache directory (install), after it checks the
// download against the sha256 of the matrix of ci.yml (mise.ReadPin);
// every mise command then starts that copy. It still refuses a mise of
// another version (mise.CheckVersion) before any mise step.
//
// Both mise steps run with mise.MiseEnv, at the top of the tree. In the
// hosted workflow, setup first checks that its own environment holds
// mise.MiseEnv, as the workflow's env table gives every step
// (mise.HostedEnv).
//
// The download runs the pinned go command in check.StepEnv with the proxy
// the go command uses by default, the one step of tools/ci that
// fetches modules; go.sum checks what it fetches. It and "mise
// install" keep check.NetworkPassThrough.
//
// "mise install" downloads and builds the pinned tools, so it runs
// under the class procgroup.Mise, the limit the table gives every mise
// install; "mise trust" and the download are command steps, under
// procgroup.ToolStep.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("setup takes no argument\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	if err := mise.HostedEnv(); err != nil {
		return false, err
	}
	pin, err := mise.ReadPin(root)
	if err != nil {
		return false, err
	}
	if version, err := mise.Version(ctx, root); err != nil || version != pin.Version {
		path, err := install(ctx, pin)
		if err != nil {
			return false, fmt.Errorf("setup: install mise %s: %w", pin.Version, err)
		}
		check.Say(e.Stdout, "installed mise %s at %s\n", pin.Version, path)
	}
	if err := mise.CheckVersion(ctx, root); err != nil {
		return false, err
	}
	misePath, err := mise.Path(root)
	if err != nil {
		return false, err
	}
	c := check.NewPrinter(e.Stdout)
	miseSteps := []check.Step{
		{Name: "mise trust", Argv: []string{misePath, "trust", filepath.Join(root, "mise.toml")}, Environ: mise.RunEnviron(check.PassThroughEnv(), root)},
		{Name: "mise install", Argv: []string{misePath, "install"}, Environ: mise.RunEnviron(check.WithProcessEnv(check.PassThroughEnv(), check.NetworkPassThrough), root), Class: procgroup.Mise},
	}
	c.Run(ctx, root, miseSteps)
	if !c.OK {
		return false, nil
	}
	tools, err := mise.ResolveLintTools(ctx, root)
	if err != nil {
		return false, err
	}
	download := check.Step{
		Name:    "go mod download",
		Argv:    []string{filepath.Join(tools.GoDir, "go"), "mod", "download"},
		Environ: check.WithProcessEnv(slices.DeleteFunc(check.StepEnv(tools.GoDir), func(kv string) bool { return kv == "GOPROXY=off" }), check.NetworkPassThrough),
	}
	c.Run(ctx, root, []check.Step{download})
	return c.OK, nil
}
