// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command release builds the release archives (10 10.2, Release and
// bootstrap):
//
//	go run ./tools/release build --version v1.0.0 --dry-run [--out dir]
//	go run ./tools/release build --host --out path
//
// --host is the host-native build: one romeu for the platform it runs
// on, embedding the julieta linux binary of the same GOARCH, written to
// path, which must not exist (build.Host). It takes no other flag,
// writes no archive and reads no network; the e2e of a command that
// needs julieta builds its romeu with it (10 10.2, the e2e row).
//
// It runs from the root of the repository. Without --dry-run or --host
// the build asks the latest scheduled fuzz.yml run first, through the
// GitHub API, with GH_TOKEN and GITHUB_REPOSITORY (owner/name) in its
// environment; --dry-run reads no network and makes no such check. The
// API is https and api.github.com unless --api-host names another one,
// as host:port. The tree must be clean; a dry run builds a dirty one
// only with --allow-dirty and stamps it dirty. A release build also needs
// the tag to name HEAD and each romeu binary to carry the bytes of its
// julieta, which a package of romeu consumes, and a dry run says
// plainly when one does not. --out must be absent or empty and is
// written in one rename; it defaults to a new temporary directory. The
// commands are in tools/release/internal/build, whose doc comment says
// what the build does; this package parses the command line and calls
// it, under procgroup.Main. The exit status is 0 when the archives, or the host
// build's binary, are written, 1 when the build refuses or fails, and 2
// for a wrong command line.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/release/internal/build"
)

const (
	exitOK    = 0 // the archives, or the host binary, are written
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

const usage = "usage:\n  go run ./tools/release build --version <v> [--dry-run [--allow-dirty]] [--out <dir>] [--api-host <host>]\n" +
	"  go run ./tools/release build --host --out <path>\n"

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
	out := fs.String("out", "", "where the archives go; absent or empty")
	dirty := fs.Bool("allow-dirty", false, "a dry run may build a tree with changes")
	apiHost := fs.String("api-host", "", "the one API host:port to use besides api.github.com")
	host := fs.Bool("host", false, "build one romeu for this platform, embedding julieta, to --out")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		say(stderr, "release build: unknown flag or argument\n%s", usage)
		return exitUsage
	}
	if *host {
		if *out == "" || otherThan(fs, "host", "out") {
			say(stderr, "release build: --host needs --out and takes no other flag\n%s", usage)
			return exitUsage
		}
	} else if *version == "" {
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
	if *host {
		return runHost(ctx, build.HostOptions{Root: root, Out: *out, GoBin: goBin}, stdout, stderr)
	}
	opts := build.Options{Root: root, Version: *version, Out: *out, DryRun: *dry, AllowDirty: *dirty, GoBin: goBin}
	if !*dry {
		api := os.Getenv("GITHUB_API_URL")
		if api == "" {
			api = defaultAPI
		}
		opts.Gate = &build.FuzzGate{API: api, Repo: os.Getenv("GITHUB_REPOSITORY"), Token: os.Getenv("GH_TOKEN"), AllowHost: *apiHost}
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
	notes(stdout, res)
	return exitOK
}

// otherThan reports whether fs set a flag not named in names.
func otherThan(fs *flag.FlagSet, names ...string) bool {
	other := false
	fs.Visit(func(f *flag.Flag) { other = other || !slices.Contains(names, f.Name) })
	return other
}

// runHost runs the host build and names the binary on stdout.
func runHost(ctx context.Context, o build.HostOptions, stdout, stderr io.Writer) int {
	if err := build.Host(ctx, o); err != nil {
		// After a signal procgroup.Main says the run was interrupted.
		if ctx.Err() == nil {
			say(stderr, "release build: %v\n", err)
		}
		return exitError
	}
	say(stdout, "%s\n", o.Out)
	return exitOK
}

// notes says what a build that wrote its archives left out: a dirty
// commit stamp, and a romeu that carries no julieta.
func notes(w io.Writer, res build.Result) {
	if res.Dirty {
		say(w, "note: the tree has changes; the commit stamp is <commit>-dirty, not a clean commit\n")
	}
	if !res.Embedded {
		say(w, "note: the archives carry no julieta: no package of romeu consumes %s, or the overlay did not replace its placeholder, so a romeu from this build cannot pass julieta-not-embedded\n", build.EmbedPath)
	}
}

func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
