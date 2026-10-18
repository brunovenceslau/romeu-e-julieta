// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses of tools/ci. A run that SIGINT or SIGTERM cut short
// exits with 128 plus the signal number (130 and 143), see
// procgroup.Main.
const (
	ExitOK    = 0 // every check passed
	ExitFail  = 1 // a check found something
	ExitError = 2 // a check could not run, or the command line is wrong
)

// Env is what a subcommand reads and writes. Tests fill it with a
// fixture; main fills it with the process's own.
type Env struct {
	Dir    string    // where the command was started
	GitEnv []string  // the environment of git; nil means this process's
	Stdin  io.Reader // the hook's input, or the terminal of "hygiene add"
	Stdout io.Writer
	Stderr io.Writer
	// Steps are the command steps of fast or all; nil means the steps
	// of the registered checks, with the tools mise resolves.
	Steps []Step
	// Profile is the cover profile that coverage reads in all when
	// Steps is set; with the real steps, all writes one of its own.
	Profile string
	// Machine returns what "uname -m" prints; nil means the command.
	Machine func(context.Context) string
	// Translated reports whether the process runs under Rosetta; nil
	// means the probe of all.
	Translated func(context.Context) bool
	// Now returns the time; nil means time.Now. The durations of the
	// verdict lines and the budget of fast read it (10 10.4).
	Now func() time.Time
}

// Repo returns the repository the command was started in.
func (e Env) Repo() git.Repo {
	return git.Repo{Dir: e.Dir, Env: e.GitEnv}
}

// RepoRoot returns the top of the working tree the command runs in.
// The steps of fast run there, and "hygiene add" writes there.
func RepoRoot(ctx context.Context, e Env) (string, error) {
	out, err := e.Repo().Run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// Status returns the exit status of a subcommand that returned ok and
// err, and prints err, made safe, to stderr.
func Status(stderr io.Writer, ok bool, err error) int {
	switch {
	case err != nil:
		Say(stderr, "ci: %s\n", git.SafeLines(strings.TrimSpace(err.Error())))
		return ExitError
	case !ok:
		return ExitFail
	}
	return ExitOK
}

// Clock returns e.Now, or time.Now when it is nil.
func (e Env) Clock() func() time.Time {
	if e.Now == nil {
		return time.Now
	}
	return e.Now
}
