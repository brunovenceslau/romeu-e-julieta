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
// whatever its recording holds; a table of cases both are run over
// (e2e/testdata/sbx/guard-cases.jsonl) keeps the two together.
//
// The guard is an allowlist of shapes. It finds the verb after the
// global flags, reads the flags of that verb with the arity sbx gives
// them (the -t of env exec is a boolean, the -t of secret set takes a
// value), scans for the approvals, and then lets through only the
// shapes a builder emits: a flag no builder emits is refused whatever
// it is.

// flagSpec is a flag of a verb: its long name, its short letter (0 for
// none), whether it takes a value, and whether a builder emits it.
type flagSpec struct {
	long    string
	short   byte
	value   bool
	driven  bool
	noShort bool // a builder emits the long form alone
}

// verbSpec is a verb romeu knows of: its flags, how many positional
// words it takes, whether it has a command after "--", and whether a
// builder drives it at all.
type verbSpec struct {
	flags    []flagSpec
	min, max int
	command  bool
	driven   bool
}

// The flags every verb takes before or after it, and the approval; none
// is emitted by a builder.
var commonFlags = []flagSpec{
	{long: "--debug", short: 'D'},
	{long: "--cloud"},
	{long: "--auto-approve", short: 'y'},
}

// planFlags are the flags of sbx env run and env create, from the help
// of v0.49.2: of these a builder emits --detach and --clone on env run
// alone. --force applies the plan without asking (I4).
var planFlags = []flagSpec{
	{long: "--detach", short: 'd', driven: true}, {long: "--clone", driven: true},
	{long: "--force", short: 'f'}, {long: "--skip-host-commands"}, {long: "--env-arg", value: true},
	{long: "--env-args-file", value: true}, {long: "--kit-arg", value: true}, {long: "--kit-args-file", value: true},
	{long: "--name", value: true}, {long: "--on-timeout", value: true}, {long: "--ttl", value: true},
}

// verbs are the verbs of sbx that the guard knows, by their words. A
// verb not here is refused. The flags of the verbs romeu drives, other
// than the three whose help was read (env run, env create, env exec and
// secret set), are those their builders emit.
var verbs = map[string]verbSpec{
	"version":    {driven: true},
	"ls":         {flags: []flagSpec{{long: "--json", driven: true}}, driven: true},
	"stop":       {min: 1, max: 1, driven: true},
	"env plan":   {min: 1, max: 1, driven: true},
	"env run":    {flags: planFlags, min: 1, max: 1, driven: true},
	"env create": {flags: planFlags, min: 1, max: 1},
	"run":        {flags: planFlags, min: 1, max: 1},
	"create":     {flags: planFlags, min: 1, max: 1},
	"env exec": {flags: []flagSpec{
		{long: "--interactive", short: 'i', driven: true}, {long: "--tty", short: 't', driven: true}, {long: "--env", short: 'e', value: true, driven: true, noShort: true},
		{long: "--detach", short: 'd'}, {long: "--detach-keys", value: true}, {long: "--env-arg", value: true},
		{long: "--env-args-file", value: true}, {long: "--env-file", value: true}, {long: "--name", value: true},
		{long: "--privileged"}, {long: "--user", short: 'u', value: true}, {long: "--workdir", short: 'w', value: true},
	}, min: 1, max: 1, command: true, driven: true},
	"env rm":               {flags: []flagSpec{{long: "--force", driven: true}}, min: 1, max: 1, driven: true},
	"policy allow network": {flags: []flagSpec{{long: "--sandbox", value: true, driven: true}}, min: 1, max: 1, driven: true},
	"policy check network": {flags: []flagSpec{{long: "--sandbox", value: true, driven: true}}, min: 1, max: 1, driven: true},
	"policy rm network":    {flags: []flagSpec{{long: "--sandbox", value: true, driven: true}, {long: "--resource", value: true, driven: true}}, driven: true},
	"policy ls":            {flags: []flagSpec{{long: "--json", driven: true}}, driven: true},
	"settings set":         {min: 2, max: 2, driven: true},
	"secret ls":            {driven: true},
	"secret set": {flags: []flagSpec{
		{long: "--sandbox", value: true, driven: true}, {long: "--command", value: true, driven: true}, {long: "--refresh", value: true, driven: true},
		{long: "--all-sandboxes"}, {long: "--force", short: 'f'}, {long: "--no-verify"}, {long: "--oauth"}, {long: "--password-stdin"},
		{long: "--ref", value: true}, {long: "--registry", value: true}, {long: "--registry-auth-endpoint", value: true},
		{long: "--show-error"}, {long: "--token", short: 't', value: true}, {long: "--username", value: true},
	}, min: 1, max: 1, driven: true},
}

// verbDepth is how many words name a verb that has subcommands.
var verbDepth = map[string]int{"env": 2, "policy": 3, "secret": 2, "settings": 2}

// usedFlag is a flag found in an argv.
type usedFlag struct {
	spec    flagSpec
	value   string
	hasEq   bool // written --flag=value, or the value stuck to the letters
	short   bool // written as a letter
	missing bool // takes a value, and there is none
}

// parsed is an argv read with the arity of its verb.
type parsed struct {
	verb     string
	known    bool
	global   []string // the flags before the verb
	flags    []usedFlag
	unknown  []string // flags the verb does not have, taken as booleans
	words    []string // the positional words before "--"
	dash     bool
	command  []string // the words after "--"
	badStart string   // a flag before the verb that is not a global flag
	lead     []string // the approvals found before the verb, by long name
}

// parseArgv reads an argv: the global flags, the verb, and its flags
// with the arity the verb gives them.
func parseArgv(argv []string) parsed {
	var p parsed
	i := 0
	for ; i < len(argv) && strings.HasPrefix(argv[i], "-"); i++ {
		p.lead = append(p.lead, leadApprovals(argv[i])...)
		if g, ok := lookup(nil, argv[i]); ok && g.long != "--auto-approve" {
			p.global = append(p.global, argv[i])
		} else {
			p.badStart = argv[i]
		}
	}
	var name []string
	for ; i < len(argv) && !strings.HasPrefix(argv[i], "-"); i++ {
		name = append(name, argv[i])
		if _, ok := verbs[strings.Join(name, " ")]; ok || len(name) >= verbDepth[name[0]] || verbDepth[name[0]] == 0 {
			i++
			break
		}
	}
	p.verb = strings.Join(name, " ")
	vs, known := verbs[p.verb]
	p.known = known
	rest := argv[min(i, len(argv)):]
	for j := 0; j < len(rest); j++ {
		w := rest[j]
		switch {
		case w == "--":
			p.dash, p.command = true, rest[j+1:]
			return p
		case !strings.HasPrefix(w, "-") || w == "-":
			p.words = append(p.words, w)
		case strings.HasPrefix(w, "--"):
			flag, val, eq := strings.Cut(w, "=")
			spec, ok := lookup(vs.flags, flag)
			if !ok {
				p.unknown = append(p.unknown, flag)
				continue
			}
			u := usedFlag{spec: spec, hasEq: eq, value: val}
			if spec.value && !eq {
				if j+1 < len(rest) {
					j++
					u.value = rest[j]
				} else {
					u.missing = true
				}
			}
			p.flags = append(p.flags, u)
		default:
			for k := 1; k < len(w); k++ {
				spec, ok := lookup(vs.flags, "-"+string(w[k]))
				if !ok {
					p.unknown = append(p.unknown, "-"+string(w[k]))
					continue
				}
				u := usedFlag{spec: spec, short: true}
				if spec.value {
					switch {
					case k+1 < len(w):
						u.value, u.hasEq = w[k+1:], true
					case j+1 < len(rest):
						j++
						u.value = rest[j]
					default:
						u.missing = true
					}
					p.flags = append(p.flags, u)
					break
				}
				p.flags = append(p.flags, u)
			}
		}
	}
	return p
}

// leadApprovals returns the approvals a word before the verb holds, by
// long name: --auto-approve and -y are one, --force and -f the other,
// in a cluster of letters as well. Whether --force approves depends on
// the verb that follows.
func leadApprovals(w string) []string {
	switch {
	case strings.HasPrefix(w, "--"):
		if name, _, _ := strings.Cut(w, "="); name == "--auto-approve" || name == "--force" {
			return []string{name}
		}
	case len(w) > 1 && w[1] != '-':
		var out []string
		for _, c := range w[1:] {
			switch c {
			case 'y':
				out = append(out, "--auto-approve")
			case 'f':
				out = append(out, "--force")
			}
		}
		return out
	}
	return nil
}

// lookup finds a flag of the verb by its long name or, with one dash and
// one letter, its short; the common flags are on every verb.
func lookup(flags []flagSpec, name string) (flagSpec, bool) {
	for _, f := range slices.Concat(flags, commonFlags) {
		if name == f.long || (f.short != 0 && len(name) == 2 && name[0] == '-' && name[1] == f.short) {
			return f, true
		}
	}
	return flagSpec{}, false
}

// has reports whether the flag, by its long name, was used.
func (p parsed) has(long string) bool {
	return slices.ContainsFunc(p.flags, func(u usedFlag) bool { return u.spec.long == long })
}

// approvesPlan reports whether the argv is the creation or the start of
// an environment, the calls that sbx asks to approve the plan of: env
// run and env create, and the top-level run and create that do the same.
func approvesPlan(argv []string) bool {
	switch parseArgv(argv).verb {
	case "env run", "env create", "run", "create":
		return true
	}
	return false
}

// Guard returns an error when the call breaks I4 or I25, or is not a
// shape romeu drives.
//
// I4: no call approves for sbx. --auto-approve is refused whatever the
// call, and so is -y, its short form; sbx v0.48.0 renamed the approval
// of an environment plan to --force (-f) on env run and env create
// (https://docs.docker.com/ai/sandboxes/workflows/automation/, "Update
// scripts for v0.48.0"), so those are refused on those two calls and on
// the top-level run and create, which do the same. --force on env rm,
// which skips the confirmation of a removal that romeu has salvaged and
// confirmed itself, is not an approval of a plan and is left to the
// builder of env rm.
//
// I25: sbx secret set is only called as sbx secret set <service>
// --sandbox <name> --command <command> [--refresh <duration>], in that
// order, with no other flag and no other word: the secret is fetched by
// sbx on the host, and romeu never handles a value. A value as a flag
// (-t, --token), on stdin (--password-stdin, or no source at all), a flow
// (--oauth), a reference (--ref), --show-error (which "may contain
// secrets") and the words after "--" are all refused by it.
//
// Then only the shapes a builder emits pass: a verb or a flag romeu does
// not use is refused, among them --skip-host-commands, --privileged, -e
// and the files and arguments that feed an environment.
//
//romeu:invariant I4 guard
//romeu:invariant I25 guard
func Guard(c Call) error {
	p := parseArgv(c.Argv)
	if p.has("--auto-approve") || slices.Contains(p.lead, "--auto-approve") {
		return guardRefusal(c, "I4", "--auto-approve approves for sbx")
	}
	if approvesPlan(c.Argv) && (p.has("--force") || slices.Contains(p.lead, "--force")) {
		return guardRefusal(c, "I4", "--force approves the plan of the environment without asking")
	}
	if p.verb == "secret set" {
		if err := guardSecretSet(c, p); err != nil {
			return err
		}
	}
	return guardShape(c, p)
}

// guardSecretSet holds the I25 rule on the call sbx secret set.
func guardSecretSet(c Call, p parsed) error {
	for _, u := range p.flags {
		if !u.spec.driven || u.hasEq {
			return guardRefusal(c, "I25", fmt.Sprintf("the flag %s is not a form of sbx secret set that romeu calls", u.spec.long))
		}
	}
	if len(p.unknown) > 0 || p.dash || len(p.global) > 0 {
		return guardRefusal(c, "I25", "sbx secret set carries a flag or a double dash that is not a form romeu calls")
	}
	// The rest is the exact shape, which compares words, not flags.
	if p.badStart != "" || c.Argv[0] != "secret" {
		return guardRefusal(c, "I25", "sbx secret set follows no flag")
	}
	words := c.Argv[2:]
	n := len(words)
	if (n != 5 && n != 7) || words[1] != "--sandbox" || words[3] != "--command" || (n == 7 && words[5] != "--refresh") {
		return guardRefusal(c, "I25", "sbx secret set is only <service> --sandbox <name> --command <command> [--refresh <duration>]")
	}
	for i, w := range words {
		if w == "" || (i == 0 && strings.HasPrefix(w, "-")) {
			return guardRefusal(c, "I25", "sbx secret set names no service, sandbox, command or refresh")
		}
	}
	return nil
}

// guardShape lets through only the shapes a builder emits.
func guardShape(c Call, p parsed) error {
	switch vs, ok := verbs[p.verb]; {
	case p.badStart != "":
		return shapeRefusal(c, "the flag "+p.badStart+" before the verb is not one sbx has")
	case len(p.global) > 0:
		return shapeRefusal(c, "a global flag is never passed to sbx")
	case !ok || !vs.driven:
		return shapeRefusal(c, fmt.Sprintf("%q is not a verb romeu drives", p.verb))
	case len(p.unknown) > 0:
		return shapeRefusal(c, "the flag "+p.unknown[0]+" is not one romeu passes to "+p.verb)
	case len(p.words) < vs.min || len(p.words) > vs.max:
		return shapeRefusal(c, fmt.Sprintf("%s takes %d to %d words and the call has %d", p.verb, vs.min, vs.max, len(p.words)))
	case vs.command != p.dash || (vs.command && len(p.command) == 0):
		return shapeRefusal(c, "the double dash and the command after it do not fit "+p.verb)
	}
	for _, u := range p.flags {
		switch {
		case !u.spec.driven:
			return shapeRefusal(c, "the flag "+u.spec.long+" is not one romeu passes to "+p.verb)
		case u.hasEq || u.missing || (u.short && u.spec.noShort):
			return shapeRefusal(c, "the flag "+u.spec.long+" is not written as a flag and a value")
		}
	}
	if p.verb == "env rm" && !p.has("--force") {
		return shapeRefusal(c, "env rm is always called with --force, since its stdin is closed")
	}
	if p.verb == "ls" && !p.has("--json") {
		return shapeRefusal(c, "ls is always called with --json")
	}
	if p.verb == "policy ls" && !p.has("--json") {
		return shapeRefusal(c, "policy ls is always called with --json")
	}
	if strings.HasPrefix(p.verb, "policy ") && p.verb != "policy ls" && !p.has("--sandbox") {
		return shapeRefusal(c, "a policy rule is always scoped to a sandbox (I22)")
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

// shapeRefusal is the error of a call that is not a shape a builder
// emits.
func shapeRefusal(c Call, why string) error {
	return cli.Internal.New(fmt.Sprintf("sbxdrv: the call %q is not a shape romeu drives: %s", c.Name, why),
		cli.Detail{Key: "builder", Value: c.Name},
		cli.Detail{Key: "invariant", Value: "shape"})
}
