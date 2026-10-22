// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command fakesbx is the fake sbx of the end-to-end tests (10 10.3). It
// replays the sessions under e2e/testdata/sbx/<version>/ and nothing
// else: an argv shape absent from the session fails the test with the
// argv (Replay only, REQ-0927); a call out of the recorded order fails
// (Stateful replay, REQ-0928); on a create it writes the recorded
// remote stanza into the config of the primary clone (REQ-0929); for
// env exec, the argv before "--" is matched and the command after it is
// forwarded when the configuration names a command to forward to.
//
// It exits 99 on --auto-approve (I4) and on a value form of sbx secret
// set (I25), whatever its recording holds, 97 for a call it will not
// replay (the reason is on its stderr and in the failures of its state
// file, which the test reads even when romeu hides the exit), and 98
// when its own configuration or session is unusable.
//
// A test configures it with a JSON file (config.go) and starts it as the
// sbx of host settings.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// The exit statuses of the fake that are not sbx's.
const (
	exitUnrecorded = 97
	exitBroken     = 98
	exitGuard      = 99
)

func main() {
	os.Exit(run(os.Args, os.Environ(), os.Stdin, os.Stdout, os.Stderr, func() { time.Sleep(24 * time.Hour) }))
}

// run replays one call. hang is what the fake does for a call that
// recorded no answer; it does not return in the binary, which sleeps until
// the timeout of the class ends the process (a select with no case would
// stop the Go runtime with a deadlock error instead), and a test replaces
// it.
func run(args, environ []string, stdin io.Reader, stdout, stderr io.Writer, hang func()) int {
	argv := args[1:]
	if why := refusal(argv); why != "" {
		_, _ = fmt.Fprintf(stderr, "fakesbx: refused %q: %s\n", argv, why)
		return exitGuard
	}
	cfg, err := loadConfig(args[0], environ)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
		return exitBroken
	}
	sess, err := loadSession(cfg.Session)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
		return exitBroken
	}
	sf, err := openState(cfg.State)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
		return exitBroken
	}
	released := false
	release := func() {
		if !released {
			released = true
			sf.close()
		}
	}
	defer release()
	sf.st.Calls = append(sf.st.Calls, argv)
	i, err := answer(sess, &sf.st, argv)
	if err != nil {
		sf.st.Failures = append(sf.st.Failures, failure{Argv: argv, Reason: err.Error()})
		if saveErr := sf.save(); saveErr != nil {
			_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", saveErr)
		}
		_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
		return exitUnrecorded
	}
	e := sess.Entries[i]
	if e.Hang {
		if err := sf.save(); err != nil {
			_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
			return exitBroken
		}
		release()
		hang()
		return exitBroken
	}
	if *e.Exit == 0 {
		if err := applyEffects(e, argv, cfg); err != nil {
			_, _ = fmt.Fprintf(stderr, "fakesbx: the effects of %q: %v\n", e.Argv, err)
			return exitBroken
		}
	}
	if err := sf.save(); err != nil {
		_, _ = fmt.Fprintf(stderr, "fakesbx: %v\n", err)
		return exitBroken
	}
	release()
	if len(cfg.Forward) > 0 && e.Argv[len(e.Argv)-1] == commandToken {
		return forward(context.Background(), cfg, argv, environ, stdin, stdout, stderr)
	}
	_, _ = io.WriteString(stdout, replaceAll(e.Stdout, cfg.Replace))
	_, _ = io.WriteString(stderr, replaceAll(e.Stderr, cfg.Replace))
	return *e.Exit
}

// replaceAll applies the replacements, in order.
func replaceAll(text string, pairs []replacement) string {
	for _, p := range pairs {
		text = strings.ReplaceAll(text, p.From, p.To)
	}
	return text
}
