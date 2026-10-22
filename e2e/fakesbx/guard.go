// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"slices"
	"strings"
)

// This file is on the sbxdrv surface of .github/ask-first.yaml. The
// fake refuses what the guards of internal/sbxdrv refuse, by its own
// rules and whatever the recording holds (REQ-0485, REQ-0509): it
// exits 99 on --auto-approve (I4) and on a value form of sbx secret
// set (I25). It does not import the guards, so that the one cannot
// pass what the other would not.

// valueFlags are the flags that take the next word as their value, so
// that the scan does not read the value as a flag.
var valueFlags = []string{
	"--env", "-e", "--command", "--sandbox", "--name", "--refresh", "--resource", "--id", "--ref",
	"--token", "-t", "--protocol", "--registry", "--username", "--registry-auth-endpoint",
	"--env-arg", "--env-args-file", "--kit-arg", "--kit-args-file", "--env-file",
	"--format", "--workdir", "-w", "--user", "-u", "--detach-keys", "--ttl", "--on-timeout",
}

// secretSetFlags are the flags sbx secret set may carry.
var secretSetFlags = []string{"--command", "--sandbox", "--refresh"}

// flagsOf returns the flags of an argv: each word before "--" that starts
// with a dash, with any "=value" cut, and not the value of a flag that
// takes one.
func flagsOf(argv []string) []string {
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

// hasShort reports whether a flag is a cluster of short flags with the
// letter in it.
func hasShort(flags []string, letter rune) bool {
	return slices.ContainsFunc(flags, func(f string) bool {
		return !strings.HasPrefix(f, "--") && strings.ContainsRune(f[1:], letter)
	})
}

// refusal returns why the argv breaks I4 or I25, or "".
func refusal(argv []string) string {
	flags := flagsOf(argv)
	if slices.Contains(flags, "--auto-approve") || hasShort(flags, 'y') {
		return "I4: --auto-approve approves for sbx"
	}
	if len(argv) >= 2 && argv[0] == "env" && (argv[1] == "run" || argv[1] == "create") {
		if slices.Contains(flags, "--force") || hasShort(flags, 'f') {
			return "I4: --force approves the plan of the environment without asking"
		}
	}
	if len(argv) >= 2 && argv[0] == "secret" && argv[1] == "set" {
		for _, f := range flags {
			if !slices.Contains(secretSetFlags, f) {
				return "I25: sbx secret set carries the flag " + f + ", a form that is not --command"
			}
		}
		if !slices.Contains(flags, "--command") {
			return "I25: sbx secret set without --command takes the value from a flag, stdin or a prompt"
		}
		for i, w := range argv {
			if (w == "--command" && (i+1 >= len(argv) || argv[i+1] == "")) || w == "--command=" {
				return "I25: sbx secret set --command names no command"
			}
		}
	}
	return ""
}
