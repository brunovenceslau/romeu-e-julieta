// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The scrubbed environment of an sbx call (04 4.1, Subprocesses, item
// 1): PATH=/usr/bin:/bin, HOME, LANG=C, and TERM for the interactive
// calls. What a call carries beyond these comes from the contributions
// below, which are the one place a variable is added: a project that
// uses git-ssh-sign adds SSH_AUTH_SOCK there (T080, env_signing.go).
const (
	scrubbedPath = "/usr/bin:/bin"
	scrubbedLang = "C"
)

// EnvInput is what a contribution reads to decide what it adds: the
// call, the host settings and the project the call is for. Host and
// Project are nil for a call that has none; a contribution does not
// write what it reads.
type EnvInput struct {
	Call    Call
	Host    *spec.HostSettings
	Project *spec.Project
}

// contribution is a set of variables a feature adds to the environment
// of an sbx call.
type contribution struct {
	name string
	vars func(EnvInput) []string
}

// contributions is the registry of the environment contributions,
// filled during initialization by the file of each contribution (rule
// 14 of docs/plan.md) and read-only after.
var contributions []contribution

// register adds a contribution to the registry. Its vars return
// NAME=VALUE entries, or none; a name that is already one of the base
// environment, that two contributions both give, or that is not an upper
// case name is a defect, and every call that reaches it fails with the
// error internal.
func register(name string, vars func(EnvInput) []string) struct{} {
	contributions = append(contributions, contribution{name: name, vars: vars})
	return struct{}{}
}

// variableName is the name of a variable a contribution may add.
var variableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// reserved are the variables of the base environment, which no
// contribution sets.
var reserved = []string{"PATH", "HOME", "LANG", "TERM"}

// environment returns the whole environment of an sbx call: the base
// and the entries of the contributions, sorted by the name they were
// registered under, so that the same input gives the same environment.
func environment(home, term string, in EnvInput, regs []contribution) ([]string, error) {
	env := []string{"PATH=" + scrubbedPath, "HOME=" + home, "LANG=" + scrubbedLang}
	if in.Call.Mode == Interactive && term != "" {
		env = append(env, "TERM="+term)
	}
	seen := map[string]string{}
	for _, r := range slices.SortedStableFunc(slices.Values(regs), func(a, b contribution) int { return strings.Compare(a.name, b.name) }) {
		for _, entry := range r.vars(in) {
			name, value, ok := strings.Cut(entry, "=")
			switch {
			case !ok || !variableName.MatchString(name):
				return nil, cli.Internal.New(fmt.Sprintf("sbxdrv: the environment contribution %s gives %q, which is not NAME=VALUE with an upper case name", r.name, name))
			case slices.Contains(reserved, name):
				return nil, cli.Internal.New(fmt.Sprintf("sbxdrv: the environment contribution %s sets %s, which the base environment sets", r.name, name))
			case seen[name] != "":
				return nil, cli.Internal.New(fmt.Sprintf("sbxdrv: the environment contributions %s and %s both set %s", seen[name], r.name, name))
			case noControl(value) != nil:
				return nil, cli.Internal.New(fmt.Sprintf("sbxdrv: the environment contribution %s gives %s a value with a control character", r.name, name))
			}
			seen[name] = r.name
			env = append(env, entry)
		}
	}
	return env, nil
}
