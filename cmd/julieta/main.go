// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command julieta is the sandbox binary (linux)
// (02 2.1). This file is its composition root: it builds the Env of
// internal/cli from the real clock, entropy, the version stamp and the
// process streams, runs under the signal handler of internal/procgroup,
// links every command in through internal/cmd/julieta/all and calls
// cli.Main. It holds no other logic.
package main

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The version stamp. The release build sets the three with -ldflags
// "-X main.version=... -X main.commit=... -X main.date=..."; any other
// build prints these values.
var (
	version = "devel"
	commit  = "unknown"
	date    = "unknown"
)

// protocol is the romeu/julieta contract number (03 3.6, REQ-0218),
// the same in both binaries.
const protocol = 1

func main() { os.Exit(start(os.Args[1:], os.Stdout, os.Stderr)) }

// start runs args under the signal rule of procgroup.Main (04 4.1,
// Signals) and returns the exit status.
func start(args []string, stdout, stderr io.Writer) int {
	return procgroup.Main("julieta", stderr, func(ctx context.Context) int {
		return run(ctx, args, stdout, stderr)
	})
}

// run runs args, the arguments after the binary's name, with the Env of
// the process.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	env := &cli.Env{
		Binary:  "julieta",
		Stdout:  cli.NewWriter(stdout),
		Stderr:  cli.NewWriter(stderr),
		Now:     time.Now,
		ULIDs:   canon.NewULIDSource(rand.Reader),
		Version: cli.Version{Version: version, Commit: commit, Date: date, Protocol: protocol},
	}
	return cli.Main(ctx, args, env)
}
