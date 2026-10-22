// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// Placeholder is a type of value that varies from call to call. A
// session under e2e/testdata/sbx/<version>/ holds its Token in place of
// the value (10 10.3, Replay only), the fake sbx matches an argv
// against it, and a builder gives a value to its slot only when Check
// accepts it, since no string from outside becomes an argv element
// without a validator (04 4.1, Output, REQ-0271).
//
// The tokens are the placeholder types the sessions use; a recorder
// (T018) writes the same tokens, and a placeholder parses as the type it
// stands for, so a recorded listing still runs through the parsers.
type Placeholder struct {
	// Token is what a session holds, such as "<name>".
	Token string
	// What says what the placeholder stands for.
	What string
	// Check returns why a value is not of the type, or nil.
	Check func(string) error
}

// projectName is the pattern of the rule project-name of the project
// spec (internal/spec/rules.go), which a sandbox name follows, so that
// the pattern is written once. A spec without the rule is a defect found
// at start.
var projectName = func() *regexp.Regexp {
	for _, r := range spec.Rules() {
		if r.Format == spec.FormatProject && r.Slug == "project-name" && r.Pattern != "" {
			return regexp.MustCompile(r.Pattern)
		}
	}
	panic("sbxdrv: internal/spec has no rule project-name")
}()

// envName is the name of a variable of the environment.
var envName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// The placeholder types.
var (
	// sandboxName is the name of a sandbox, which is the project's name
	// (01 1.1).
	sandboxName = Placeholder{Token: "<name>", What: "the name of a sandbox, which is its project's name", Check: func(v string) error {
		if !projectName.MatchString(v) {
			return errors.New("is not a project name")
		}
		return nil
	}}
	// envDir is the directory that holds sbxenv.yaml, which sbx env
	// commands take as PATH.
	envDir = Placeholder{Token: "<dir>", What: "the absolute path of the directory that holds sbxenv.yaml", Check: func(v string) error {
		if !filepath.IsAbs(v) || filepath.Clean(v) != v {
			return errors.New("is not a clean absolute path")
		}
		return noControl(v)
	}}
	// domain is the resource of a network rule: a host name, by the rule
	// of 03 3.1 (REQ-0188).
	domain = Placeholder{Token: "<domain>", What: "a domain of a network rule", Check: func(v string) error { return spec.ValidDomain(v) }}
	// service is a secret service of sbx, such as github.
	service = Placeholder{Token: "<service>", What: "the service a secret is for", Check: matches(regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`), "is not a service name")}
	// hostCommand is the command sbx runs on the host to fetch a secret:
	// the rendered argv of a secret binding, which starts with an
	// absolute path (sbx secret set, --command).
	hostCommand = Placeholder{Token: "<command>", What: "the host command that fetches a secret", Check: func(v string) error {
		if !strings.HasPrefix(v, "/") {
			return errors.New("does not start with an absolute path")
		}
		return noControl(v)
	}}
	// refresh is the refresh policy of a command secret.
	refresh = Placeholder{Token: "<duration>", What: "how long sbx keeps the value of a command secret", Check: matches(regexp.MustCompile(`^(?:on-demand|[1-9][0-9]*[smh])$`), "is not on-demand or a duration of seconds, minutes or hours")}
	// settingKey is the key of an sbx setting.
	settingKey = Placeholder{Token: "<key>", What: "the key of an sbx setting", Check: matches(regexp.MustCompile(`^[a-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*$`), "is not a setting key")}
	// settingValue is the value of an sbx setting: a bool, a number or
	// a word.
	settingValue = Placeholder{Token: "<value>", What: "the value of an sbx setting", Check: matches(regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:-]{0,127}$`), "is not a setting value")}
	// envAssign is one NAME=VALUE variable for the command of an env
	// exec.
	envAssign = Placeholder{Token: "<env>", What: "a NAME=VALUE variable for the command of an env exec", Check: func(v string) error {
		name, _, ok := strings.Cut(v, "=")
		if !ok || !envName.MatchString(name) {
			return errors.New("is not NAME=VALUE with an upper case name")
		}
		return noControl(v)
	}}
	// commandWords is the command an env exec runs, after "--": the
	// words the fake forwards and does not match.
	commandWords = Placeholder{Token: "<command...>", What: "the command after --, forwarded and not matched", Check: func(v string) error {
		if v == "" || strings.ContainsRune(v, 0) {
			return errors.New("is empty or holds a NUL byte")
		}
		return nil
	}}
)

// placeholders is the list of the placeholder types, in the order the
// sessions use them.
var placeholders = []Placeholder{sandboxName, envDir, domain, service, hostCommand, refresh, settingKey, settingValue, envAssign, commandWords}

// PlaceholderTokens returns the tokens of the placeholder types, which
// are the only angle-bracket words an argv in a session may hold.
func PlaceholderTokens() []string {
	out := make([]string, 0, len(placeholders))
	for _, p := range placeholders {
		out = append(out, p.Token)
	}
	return slices.Clone(out)
}

// matches returns a check that a value matches re.
func matches(re *regexp.Regexp, why string) func(string) error {
	return func(v string) error {
		if !re.MatchString(v) {
			return errors.New(why)
		}
		return nil
	}
}
