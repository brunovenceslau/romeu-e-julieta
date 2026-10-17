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
// download against the sha256 of the matrix of ci.yml, or of x64.yml
// for an x64 runner (mise.ReadPin);
// every mise command then starts that copy. It still refuses a mise of
// another version (mise.CheckVersion) before any mise step.
//
// Both mise steps run with mise.MiseEnv, at the top of the tree. In the
// hosted workflow, setup first checks that its own environment holds
// mise.MiseEnv, as the workflow's env table gives every step
// (mise.HostedEnv).
//
// The download runs the pinned go command in check.StepEnv, the one
// step of tools/ci that fetches the modules of go.sum; go.sum and the
// checksum database check what it fetches. All three steps run with
// moduleSources, and the download and "mise install" keep
// check.NetworkPassThrough.
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
		{Name: "mise trust", Argv: []string{misePath, "trust", filepath.Join(root, "mise.toml")}, Environ: trustEnv(root)},
		{Name: "mise install", Argv: []string{misePath, "install"}, Environ: installEnv(root), Class: procgroup.Mise},
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
		Environ: downloadEnv(tools.GoDir),
	}
	c.Run(ctx, root, []check.Step{download})
	return c.OK, nil
}

// moduleSources are where the go commands of setup fetch a module from
// and the checksum database that checks it (REQ-0862): the module
// proxy alone, with no "direct" fallback to the module's own host, and
// sum.golang.org. GOENV=off, the third variable REQ-0862 names, comes
// with goenv.Pin, which both mise.MiseEnv and check.StepEnv hold. Every
// environment below is built from nothing, so no GO variable of the
// caller (GONOSUMDB, GOPRIVATE, GOINSECURE, GOFLAGS among them) reaches
// a step. TestSetupEnv holds each step to both rules.
var moduleSources = []string{
	"GOPROXY=https://proxy.golang.org",
	"GOSUMDB=sum.golang.org",
}

// trustEnv is the environment of "mise trust", run in root.
func trustEnv(root string) []string {
	return mise.RunEnviron(slices.Concat(check.PassThroughEnv(), moduleSources), root)
}

// installEnv is the environment of "mise install", run in root, which
// reaches the network to build the pinned go tools.
func installEnv(root string) []string {
	env := check.WithProcessEnv(check.PassThroughEnv(), check.NetworkPassThrough)
	return mise.RunEnviron(slices.Concat(env, moduleSources), root)
}

// downloadEnv is the environment of "go mod download" with the go
// command of goDir: the steps' environment with moduleSources in place
// of their GOPROXY=off.
func downloadEnv(goDir string) []string {
	env := slices.DeleteFunc(check.StepEnv(goDir), func(kv string) bool { return kv == "GOPROXY=off" })
	return check.WithProcessEnv(slices.Concat(env, moduleSources), check.NetworkPassThrough)
}
