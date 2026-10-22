// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// This file holds the two guards of sbxdrv, which are on the sbxdrv
// surface of .github/ask-first.yaml: I4, romeu never approves for sbx,
// and I25, romeu never handles a secret value (05 5.2). Both run on
// every call a builder hands out (argvBuilder.call) and again in Run,
// and the fake sbx holds the same two rules by itself (e2e/fakesbx),
// whatever its recording holds.

// valueFlags are the flags of the sbx calls that take a value as the
// next word, so that the scan does not read the value as a flag. The
// words after "--" are the command of an env exec and are not scanned.
var valueFlags = []string{
	"--env", "-e", "--command", "--sandbox", "--name", "--refresh", "--resource", "--id", "--ref",
	"--token", "-t", "--protocol", "--registry", "--username", "--registry-auth-endpoint",
	"--env-arg", "--env-args-file", "--kit-arg", "--kit-args-file", "--env-file",
	"--format", "--workdir", "-w", "--user", "-u", "--detach-keys", "--ttl", "--on-timeout",
}

// flagWords returns the flags of an argv: each word before "--" that
// starts with a dash, whole for a long flag with any "=value" cut, and
// as the cluster of letters for a short one. A word that is the value
// of a flag that takes one is left out.
func flagWords(argv []string) []string {
	var flags []string
	skip := false
	for _, w := range argv {
		if w == "--" {
			break
		}
		if skip {
			skip = false
			continue
		}
		if !strings.HasPrefix(w, "-") || w == "-" {
			continue
		}
		name, _, hasValue := strings.Cut(w, "=")
		flags = append(flags, name)
		if !hasValue && slices.Contains(valueFlags, name) {
			skip = true
		}
	}
	return flags
}

// hasShort reports whether one of the flags is a cluster of short
// flags that holds the letter.
func hasShort(flags []string, letter rune) bool {
	return slices.ContainsFunc(flags, func(f string) bool {
		return !strings.HasPrefix(f, "--") && strings.ContainsRune(f[1:], letter)
	})
}

// approvesPlan reports whether the argv is the creation or the start of
// an environment, the calls that sbx asks to approve the plan of.
func approvesPlan(argv []string) bool {
	return len(argv) >= 2 && argv[0] == "env" && (argv[1] == "run" || argv[1] == "create")
}

// secretSetFlags are the flags sbx secret set may carry: the source of
// the value is a command that sbx runs on the host, scoped to a sandbox,
// and refreshed as the policy says.
var secretSetFlags = []string{"--command", "--sandbox", "--refresh"}

// Guard returns an error when the call breaks I4 or I25.
//
// I4: no call approves for sbx. --auto-approve is refused whatever
// the call, and so is -y, its short form; sbx v0.48.0 renamed the
// approval of an environment plan to --force (-f) on env run and env
// create (https://docs.docker.com/ai/sandboxes/workflows/automation/,
// "Update scripts for v0.48.0"), so those two are refused on those two
// calls. --force on env rm, which skips the confirmation of a removal
// that romeu has salvaged and confirmed itself, is not an approval of a
// plan and is left to the builder of env rm.
//
// I25: sbx secret set is only called with --command: the secret is
// fetched by sbx on the host, and romeu never handles a value. Every
// other form of the call is refused: a value as a flag (-t, --token), a
// value on stdin (--password-stdin, or no source at all), a flow
// (--oauth) or a reference (--ref), by allowing the three flags of
// secretSetFlags and none else.
//
//romeu:invariant I4 guard
//romeu:invariant I25 guard
func Guard(c Call) error {
	flags := flagWords(c.Argv)
	if slices.Contains(flags, "--auto-approve") {
		return guardRefusal(c, "I4", "--auto-approve approves for sbx")
	}
	if hasShort(flags, 'y') {
		return guardRefusal(c, "I4", "-y approves for sbx")
	}
	if approvesPlan(c.Argv) {
		if slices.Contains(flags, "--force") || hasShort(flags, 'f') {
			return guardRefusal(c, "I4", "--force approves the plan of the environment without asking")
		}
	}
	if len(c.Argv) >= 2 && c.Argv[0] == "secret" && c.Argv[1] == "set" {
		return guardSecretSet(c, flags)
	}
	return nil
}

// guardSecretSet holds the I25 rule on the call sbx secret set.
func guardSecretSet(c Call, flags []string) error {
	for _, f := range flags {
		if !slices.Contains(secretSetFlags, f) {
			return guardRefusal(c, "I25", fmt.Sprintf("the flag %s is not a form of sbx secret set that romeu calls", f))
		}
	}
	if !slices.Contains(flags, "--command") {
		return guardRefusal(c, "I25", "sbx secret set without --command takes the value from a flag, stdin or a prompt")
	}
	for i, w := range c.Argv {
		if (w == "--command" && (i+1 >= len(c.Argv) || c.Argv[i+1] == "")) || w == "--command=" {
			return guardRefusal(c, "I25", "--command names no command")
		}
	}
	return nil
}

// guardRefusal is the error of a call that breaks a guard: a defect of
// the code, since no input of romeu gives a builder such a call.
func guardRefusal(c Call, invariant, why string) error {
	return cli.Internal.New(fmt.Sprintf("sbxdrv: the call %q breaks %s: %s", c.Name, invariant, why),
		cli.Detail{Key: "builder", Value: c.Name},
		cli.Detail{Key: "invariant", Value: invariant})
}
