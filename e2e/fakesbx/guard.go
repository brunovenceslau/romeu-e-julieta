// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"regexp"
	"slices"
	"strings"
)

// This file is on the sbxdrv surface of .github/ask-first.yaml. The
// fake refuses what the guards of internal/sbxdrv refuse, by its own
// rules and whatever the recording holds (REQ-0485, REQ-0509): it
// exits 99 on an approval (I4) and on any form of sbx secret set that
// is not the command form (I25). It imports nothing from that package,
// and its rules are written from the help of sbx v0.49.2 (the flags of
// each verb and whether they take a value), not from the guard's table:
// a test of each package runs the one table of cases
// (e2e/testdata/sbx/guard-cases.jsonl) through both, so that the one
// cannot pass what the other would not.

// valueFlagsOf are, by verb, the flags that take the next word as their
// value, so that the scan does not read the value as a flag. A verb that
// is not listed has none that romeu knows.
var valueFlagsOf = map[string][]string{
	"secret set": {"--command", "--ref", "--refresh", "--registry", "--registry-auth-endpoint", "--sandbox", "-t", "--token", "--username"},
	"env exec":   {"--detach-keys", "-e", "--env", "--env-arg", "--env-args-file", "--env-file", "--name", "-u", "--user", "-w", "--workdir"},
	"env run":    {"--env-arg", "--env-args-file", "--kit-arg", "--kit-args-file", "--name", "--on-timeout", "--ttl"},
	"env create": {"--env-arg", "--env-args-file", "--kit-arg", "--kit-args-file", "--name", "--on-timeout", "--ttl"},
	"policy":     {"--sandbox", "--resource", "--id", "--protocol", "--format"},
}

// verbOf returns the verb of an argv, found after the global flags (-D,
// --debug, --cloud: every word that starts with a dash before it), as
// "word" or, for the verbs that have subcommands, "word word"; and the
// words after it. Top-level run and create are env run and env create:
// they apply the same plan.
func verbOf(argv []string) (verb string, rest []string) {
	i := 0
	for i < len(argv) && strings.HasPrefix(argv[i], "-") {
		i++
	}
	if i >= len(argv) {
		return "", nil
	}
	verb, i = argv[i], i+1
	switch verb {
	case "run", "create":
		return "env " + verb, argv[i:]
	case "env", "secret", "settings", "policy":
		if i < len(argv) {
			sub := argv[i]
			if verb == "policy" {
				return "policy", argv[i:]
			}
			return verb + " " + sub, argv[i+1:]
		}
	}
	return verb, argv[i:]
}

// scan returns what an approval would be in the words of a verb: the
// flags it finds before "--", whole for a long one and letter by letter
// for a cluster of short ones, and not the value of a flag that takes
// one.
func scan(verb string, rest []string) (long []string, short []rune) {
	values := valueFlagsOf[verb]
	skip := false
	for _, w := range rest {
		if w == "--" {
			break
		}
		if skip {
			skip = false
			continue
		}
		switch {
		case !strings.HasPrefix(w, "-") || w == "-":
		case strings.HasPrefix(w, "--"):
			name, _, hasValue := strings.Cut(w, "=")
			long = append(long, name)
			skip = !hasValue && slices.Contains(values, name)
		default:
			for j, c := range w[1:] {
				short = append(short, c)
				if slices.Contains(values, "-"+string(c)) {
					skip = j+2 >= len(w) // the value is the next word unless it follows in the cluster
					break
				}
			}
		}
	}
	return long, short
}

// refusal returns why the argv breaks I4 or I25, or "".
func refusal(argv []string) string {
	verb, rest := verbOf(argv)
	long, short := scan(verb, rest)
	if slices.Contains(long, "--auto-approve") || slices.Contains(short, 'y') {
		return "I4: --auto-approve approves for sbx"
	}
	if verb == "env run" || verb == "env create" {
		if slices.Contains(long, "--force") || slices.Contains(short, 'f') {
			return "I4: --force approves the plan of the environment without asking"
		}
	}
	if verb == "secret set" && !isCommandForm(rest) {
		return "I25: sbx secret set is only <service> --sandbox <name> --command <command> [--refresh <duration>], and this call is not"
	}
	return ""
}

// isCommandForm reports whether the words after "secret set" are
// exactly <service> --sandbox <name> --command <command>, with
// --refresh <duration> after, and nothing else.
func isCommandForm(rest []string) bool {
	if len(rest) != 5 && len(rest) != 7 {
		return false
	}
	if rest[1] != "--sandbox" || rest[3] != "--command" || (len(rest) == 7 && rest[5] != "--refresh") {
		return false
	}
	for i, w := range rest {
		if w == "" || (i == 0 && strings.HasPrefix(w, "-")) {
			return false
		}
	}
	return true
}

// baseEnv are the variables of the scrubbed environment of an sbx call
// (04 4.1), and the one the fake reads its configuration from.
var baseEnv = []string{"PATH", "HOME", "LANG", "TERM", configEnv}

// tokenValue is the shape of the access tokens the fake refuses in any
// variable, besides the sentinels of the configuration.
var tokenValue = regexp.MustCompile(`(?:gh[opsu]_|github_pat_)[A-Za-z0-9_]{6,}|sk-[A-Za-z0-9-]{10,}`)

// envRefusal returns why the child environment is not the scrubbed one
// of 04 4.1, or "": a name outside PATH, HOME, LANG, TERM and the
// declared contributions, or a value that holds a sentinel or a token.
func envRefusal(environ []string, cfg config) string {
	for _, kv := range environ {
		name, value, _ := strings.Cut(kv, "=")
		if !slices.Contains(baseEnv, name) && !slices.Contains(cfg.AllowEnv, name) {
			return "the variable " + name + " is not in the scrubbed environment"
		}
		if tokenValue.MatchString(value) || slices.ContainsFunc(cfg.Sentinels, func(s string) bool { return strings.Contains(value, s) }) {
			return "the variable " + name + " holds a secret"
		}
	}
	return ""
}

// recordRefusal writes the refusal into the state file, so that a test
// reads it even when the caller hides the exit status. It is best
// effort: the exit is 99 whatever happens here.
func recordRefusal(cfg config, argv []string, why string) {
	sf, err := openState(cfg.State)
	if err != nil {
		return
	}
	defer sf.close()
	sf.st.Calls = append(sf.st.Calls, argv)
	sf.st.Failures = append(sf.st.Failures, failure{Argv: argv, Reason: why})
	_ = sf.save()
}
