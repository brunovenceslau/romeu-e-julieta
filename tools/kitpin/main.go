// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command kitpin writes kits/pins.yaml, the one pin file (02 2.1), and
// is the only writer of it:
//
//	go run ./tools/kitpin frontend
//	go run ./tools/kitpin workload <registry>/<repository>@sha256:<64 hex>
//	go run ./tools/kitpin herdr <version>
//	go run ./tools/kitpin mise <version>
//
// The commands are in tools/kitpin/internal/pin, whose doc comment says
// what each reads, checks and writes; this package parses the command
// line and calls them. It runs under procgroup.Main, so the first
// SIGINT or SIGTERM cancels a download in flight and the run exits with
// 128 plus the signal number, writing nothing. Otherwise the exit
// status is 0 when the file is written, 1 when the command refuses or
// fails, and 2 for a wrong command line.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/internal/pin"
)

const (
	exitOK    = 0 // the file is written
	exitError = 1 // the command refuses or fails
	exitUsage = 2 // the command line is wrong
)

func main() { os.Exit(start(pin.Default(), os.Args[1:], os.Stdout, os.Stderr)) }

// start runs the command of args under the signal rule of 04 4.1.
func start(e pin.Env, args []string, stdout, stderr io.Writer) int {
	return procgroup.Main("kitpin", stderr, func(ctx context.Context) int {
		return run(ctx, e, args, stdout, stderr)
	})
}

func usage() string {
	return `usage:
  go run ./tools/kitpin frontend
  go run ./tools/kitpin workload <registry>/<repository>@sha256:<64 hex>
  go run ./tools/kitpin herdr <version>
  go run ./tools/kitpin mise <version>
`
}

// run parses args and calls the command they name.
func run(ctx context.Context, e pin.Env, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		say(stderr, "%s", usage())
		return exitUsage
	}
	name, rest := args[0], args[1:]
	var want int
	var cmd func() (string, error)
	switch name {
	case "frontend":
		cmd = func() (string, error) { return pin.Frontend(ctx, e) }
	case "workload":
		want, cmd = 1, func() (string, error) { return pin.Workload(ctx, e, rest[0]) }
	case "herdr":
		want, cmd = 1, func() (string, error) { return pin.Herdr(ctx, e, rest[0]) }
	case "mise":
		want, cmd = 1, func() (string, error) { return pin.Mise(ctx, e, rest[0]) }
	default:
		say(stderr, "kitpin: unknown command %q\n%s", name, usage())
		return exitUsage
	}
	if len(rest) != want {
		say(stderr, "kitpin: %s takes %d argument(s), not %d\n%s", name, want, len(rest), usage())
		return exitUsage
	}
	done, err := cmd()
	if err != nil {
		// After a signal procgroup.Main says the run was interrupted;
		// the error of the cancelled request adds nothing to that line.
		if ctx.Err() == nil {
			say(stderr, "kitpin %s: %v\n", name, err)
		}
		return exitError
	}
	say(stdout, "%s\n", done)
	return exitOK
}

func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
