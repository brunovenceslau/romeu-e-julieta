// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// This file is the placeholder normalizer of 10 10.1 (REQ-0766): the
// argv a test gives the fake holds real names, paths and domains, and a
// recording holds typed placeholders in their place (10 10.3, Replay
// only). A placeholder token matches the values of its type, so an argv
// has the shape of a recording when each word is the same literal or a
// value of the placeholder type.
//
// The types are the ones internal/sbxdrv's placeholders.go lists; the
// fake does not import that package, so that the rule it holds is its
// own (REQ-0971 keeps e2e from linking it), and a test of sbxdrv runs
// the builders against this fake to hold the two together.

// commandToken stands for the command of an env exec, after "--": the
// words are forwarded, never matched.
const commandToken = "<command...>"

// typeMatchers are the placeholder types, by token.
var typeMatchers = map[string]func(string) bool{
	"<name>":     isName,
	"<dir>":      isAbsolute,
	"<domain>":   isDomain,
	"<service>":  matchRegexp(`^[a-z][a-z0-9-]{0,31}$`),
	"<command>":  isHostCommand,
	"<duration>": matchRegexp(`^(?:on-demand|[1-9][0-9]*[smh])$`),
	"<key>":      matchRegexp(`^[a-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*$`),
	"<value>":    matchRegexp(`^[A-Za-z0-9][A-Za-z0-9._/:-]{0,127}$`),
	"<env>":      isEnvAssign,
}

// matchRegexp returns a matcher for the expression.
func matchRegexp(expr string) func(string) bool {
	re := regexp.MustCompile(expr)
	return re.MatchString
}

// isName accepts the name of a sandbox: a project name, lowercase
// letters, digits and hyphens.
var isName = matchRegexp(`^[a-z][a-z0-9-]{0,38}[a-z0-9]$`)

// isAbsolute accepts a clean absolute path with no control character.
func isAbsolute(v string) bool {
	return filepath.IsAbs(v) && filepath.Clean(v) == v && !hasControl(v)
}

// isDomain accepts the resource of a network rule as sbx takes it: a
// host name, with the wildcards, port suffix and CIDR forms of
// "sbx policy allow network".
var isDomain = matchRegexp(`^[A-Za-z0-9*?.:\[\]!/-]{1,253}$`)

// isHostCommand accepts the command of a command secret: an absolute
// path first, with no control character.
func isHostCommand(v string) bool { return strings.HasPrefix(v, "/") && !hasControl(v) }

// isEnvAssign accepts NAME=VALUE with an upper case name.
func isEnvAssign(v string) bool {
	name, _, ok := strings.Cut(v, "=")
	return ok && regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(name) && !hasControl(v)
}

// hasControl reports whether v holds a control character.
func hasControl(v string) bool {
	return strings.IndexFunc(v, unicode.IsControl) >= 0
}

// matchShape reports whether argv has the shape of a recorded argv: the
// same number of words, each the same literal as the shape's or a value
// of the placeholder type the shape holds; a shape that ends in the
// command token also takes any words after "--" in the argv, at least
// one.
func matchShape(shape, argv []string) bool {
	if n := len(shape); n >= 2 && shape[n-1] == commandToken {
		if len(argv) < n || argv[n-2] != "--" || shape[n-2] != "--" {
			return false
		}
		shape, argv = shape[:n-1], argv[:n-1]
	}
	if len(shape) != len(argv) {
		return false
	}
	for i, want := range shape {
		if matcher, ok := typeMatchers[want]; ok {
			if !matcher(argv[i]) {
				return false
			}
			continue
		}
		if want != argv[i] {
			return false
		}
	}
	return true
}

// tokens returns the placeholder tokens of the shape, in order.
func tokens(shape []string) []string {
	var out []string
	for _, w := range shape {
		if _, ok := typeMatchers[w]; ok || w == commandToken {
			out = append(out, w)
		}
	}
	return slices.Clone(out)
}
