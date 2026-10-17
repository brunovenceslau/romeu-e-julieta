// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package plan

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Run is the "plan" subcommand over the tree at HEAD: the check, or with
// --critical-path or --ready one of the two views, one id per line.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	view := ""
	switch {
	case len(args) == 0:
	case len(args) == 1 && (args[0] == "--critical-path" || args[0] == "--ready"):
		view = args[0]
	default:
		return false, fmt.Errorf("plan takes no argument, --critical-path or --ready\n%s", check.Usage())
	}
	files, err := check.HeadTree(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	if view == "" {
		findings, err := Of(files, Generated)
		if err != nil {
			return false, err
		}
		check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("plan", len(findings)))
		return len(findings) == 0, nil
	}
	tasks, _, err := Read(files)
	if err != nil {
		return false, err
	}
	ids := Ready(tasks)
	if view == "--critical-path" {
		ids = CriticalPath(tasks)
	}
	for _, id := range ids {
		check.Say(e.Stdout, "%s\n", id)
	}
	return true, nil
}

// Ready returns, in id order, the open tasks that wait only for their
// edges (fate kept or new) and whose needs and after are all merged.
func Ready(tasks []Task) []string {
	byID := index(tasks)
	var ready []string
	for _, t := range tasks {
		if !t.is(startable) || byID[t.ID].Path != t.Path {
			continue
		}
		if !slices.ContainsFunc(slices.Concat(t.Needs, t.After), func(id string) bool { return byID[id].Fate != "merged" }) {
			ready = append(ready, t.ID)
		}
	}
	slices.Sort(ready)
	return ready
}

// CriticalPath returns the longest chain of open tasks over needs and
// after, both kinds counted (rule 2 of docs/plan.md), from the task
// that starts first to the one that ends it. Merged and closed tasks
// add nothing to it. Of two chains of one length, the one whose ids
// come first in order wins, so the output is the same on every run. A
// cycle, which the check reports, ends a chain where it closes.
func CriticalPath(tasks []Task) []string {
	byID := index(tasks)
	memo := map[string][]string{}
	var longest func(id string, path []string) []string
	// longest returns the longest chain that ends at id: the chain of
	// the task it waits for that is longest, then id.
	longest = func(id string, path []string) []string {
		if c, ok := memo[id]; ok {
			return c
		}
		var best []string
		for _, w := range waits(byID[id], byID) {
			if !byID[w].is(open) || slices.Contains(path, w) {
				continue
			}
			c := longest(w, append(path, id))
			if len(c) > len(best) || (len(c) == len(best) && less(c, best)) {
				best = c
			}
		}
		chain := append(slices.Clone(best), id)
		memo[id] = chain
		return chain
	}
	var best []string
	for _, t := range tasks {
		if !t.is(open) || byID[t.ID].Path != t.Path {
			continue
		}
		c := longest(t.ID, nil)
		if len(c) > len(best) || (len(c) == len(best) && less(c, best)) {
			best = c
		}
	}
	return best
}

// less orders two chains of one length by their ids.
func less(a, b []string) bool { return strings.Join(a, " ") < strings.Join(b, " ") }
