// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// forward runs the command of an env exec in place of the recorded
// answer (10 10.3, Replay only): the argv before "--" was matched against
// the recording, and the command after it is forwarded as the words
// added to cfg.Forward, with the variables of the --env flags in its
// environment. It returns the exit status of the command.
func forward(ctx context.Context, cfg config, argv, environ []string, stdin io.Reader, stdout, stderr io.Writer) int {
	dash := slices.Index(argv, "--")
	if dash < 0 || dash == len(argv)-1 {
		_, _ = fmt.Fprintln(stderr, "fakesbx: no command to forward after --")
		return exitBroken
	}
	env := slices.Clone(environ)
	for i, w := range argv[:dash] {
		if (w == "--env" || w == "-e") && i+1 < dash {
			env = append(env, argv[i+1])
		}
	}
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Unbounded,
		Argv:   append(slices.Clone(cfg.Forward), argv[dash+1:]...),
		Env:    env,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	var exit *procgroup.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit) && exit.Signal == 0:
		return exit.Code
	}
	_, _ = fmt.Fprintf(stderr, "fakesbx: the forwarded command: %v\n", err)
	return exitBroken
}
