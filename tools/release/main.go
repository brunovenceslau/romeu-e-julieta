// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command release builds the release archives (10 10.2, Release and
// bootstrap):
//
//	go run ./tools/release build --version v1.0.0 --dry-run [--out dir]
//
// It runs from the root of the repository. Without --dry-run the build
// asks the latest scheduled fuzz.yml run first, through the GitHub API,
// with GH_TOKEN and GITHUB_REPOSITORY (owner/name) in its environment;
// --dry-run reads no network and makes no such check. --out defaults to
// a new temporary directory. The commands are in
// tools/release/internal/build, whose doc comment says what the build
// does; this package parses the command line and calls it, under
// procgroup.Main. The exit status is 0 when the archives are written, 1
// when the build refuses or fails, and 2 for a wrong command line.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/release/internal/build"
)

const (
	exitOK    = 0 // the archives are written
	exitError = 1 // the build refuses or fails
	exitUsage = 2 // the command line is wrong
)

const defaultAPI = "https://api.github.com"

func main() { os.Exit(start(os.Args[1:], os.Stdout, os.Stderr)) }

// start runs args under the signal rule of 04 4.1.
func start(args []string, stdout, stderr io.Writer) int {
	return procgroup.Main("release", stderr, func(ctx context.Context) int {
		return run(ctx, args, stdout, stderr)
	})
}

const usage = "usage:\n  go run ./tools/release build --version <v> [--dry-run] [--out <dir>]\n"

// run parses args and calls the command they name.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "build" {
		say(stderr, "release: expected the command build\n%s", usage)
		return exitUsage
	}
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	version := fs.String("version", "", "the tag to build, v<major>.<minor>.<patch>")
	dry := fs.Bool("dry-run", false, "build without the fuzz gate and without network")
	out := fs.String("out", "", "where the archives go")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 || *version == "" {
		say(stderr, "release build: --version is required and takes no argument but its flags\n%s", usage)
		return exitUsage
	}
	root, err := os.Getwd()
	if err != nil {
		say(stderr, "release build: %v\n", err)
		return exitError
	}
	goBin, err := build.ResolveGo(ctx)
	if err != nil {
		say(stderr, "release build: no go command: %v\n", err)
		return exitError
	}
	opts := build.Options{Root: root, Version: *version, Out: *out, DryRun: *dry, GoBin: goBin}
	if !*dry {
		api := os.Getenv("GITHUB_API_URL")
		if api == "" {
			api = defaultAPI
		}
		opts.Gate = &build.FuzzGate{API: api, Repo: os.Getenv("GITHUB_REPOSITORY"), Token: os.Getenv("GH_TOKEN")}
	}
	res, err := build.Build(ctx, opts)
	if err != nil {
		// After a signal procgroup.Main says the run was interrupted.
		if ctx.Err() == nil {
			say(stderr, "release build: %v\n", err)
		}
		return exitError
	}
	for _, path := range res.Archives {
		say(stdout, "%s\n", path)
	}
	say(stdout, "%s\n", res.Checksums)
	return exitOK
}

func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
