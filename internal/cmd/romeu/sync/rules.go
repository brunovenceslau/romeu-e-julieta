// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// rule is a render-time rule: a check step 6 runs before it renders, in the order of the rules' names.
// An error it returns stops the command (rule 14 of docs/plan.md: an
// unordered set, one rule per file, each joined by a package-level
// var _ = register(rule{...})).
type rule struct {
	// name is the rule's slug, unique in the registry.
	name string
	// check returns why the run breaks the rule, or nil.
	check func(ctx context.Context, r *run) error
}

// rules is the registry register fills during initialization; nothing
// writes it later.
var rules []rule

// register adds ru to the registry and returns it, for a package-level
// variable. It panics on a rule with no name or no check and on a name
// already registered, so the mistake fails at init, in every test.
func register(ru rule) rule {
	if ru.name == "" || ru.check == nil {
		panic(fmt.Sprintf("sync: rule %q lacks a name or a check", ru.name))
	}
	if slices.ContainsFunc(rules, func(have rule) bool { return have.name == ru.name }) {
		panic(fmt.Sprintf("sync: rule %q is registered twice", ru.name))
	}
	rules = append(rules, ru)
	return ru
}

// checkRules runs every registered rule, in the order of the
// rules' names, and returns the first error.
func checkRules(ctx context.Context, r *run) error {
	sorted := slices.SortedFunc(slices.Values(rules), func(a, b rule) int {
		return strings.Compare(a.name, b.name)
	})
	for _, ru := range sorted {
		if err := ru.check(ctx, r); err != nil {
			return err
		}
	}
	return nil
}
