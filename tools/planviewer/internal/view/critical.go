// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"slices"
	"strings"
)

// open are the fates of a task that is not built yet, as in
// tools/ci/internal/plan: a merged or closed task adds nothing to the
// critical path.
var open = []string{"kept", "new", "deferred"}

// first returns the first task of each id, the one every rule reads.
func first(tasks []Task) map[string]Task {
	byID := map[string]Task{}
	for _, t := range tasks {
		if _, ok := byID[t.ID]; !ok && t.ID != "" {
			byID[t.ID] = t
		}
	}
	return byID
}

// waits returns the ids a task waits for, over needs and after, each
// once, naming a task that exists and not the task itself.
func waits(t Task, byID map[string]Task) []string {
	var out []string
	for _, id := range slices.Concat(t.Needs, t.After) {
		if _, ok := byID[id]; ok && id != t.ID && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// less orders two chains of one length by their ids.
func less(a, b []string) bool { return strings.Join(a, " ") < strings.Join(b, " ") }

// criticalPath is the rule of CriticalPath in tools/ci/internal/plan:
// the longest chain of open tasks over needs and after, from the task
// that starts first to the one that ends it. Of two chains of one length
// the one whose ids come first wins; a cycle ends a chain where it
// closes. It never returns nil, so the page reads an array.
func criticalPath(tasks []Task) []string {
	byID := first(tasks)
	isOpen := func(t Task) bool { return slices.Contains(open, t.Fate) }
	memo := map[string][]string{}
	var longest func(id string, path []string) []string
	longest = func(id string, path []string) []string {
		if c, ok := memo[id]; ok {
			return c
		}
		var best []string
		for _, w := range waits(byID[id], byID) {
			if !isOpen(byID[w]) || slices.Contains(path, w) {
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
	best := []string{}
	for _, t := range tasks {
		if !isOpen(t) || byID[t.ID].Path != t.Path {
			continue
		}
		c := longest(t.ID, nil)
		if len(c) > len(best) || (len(c) == len(best) && less(c, best)) {
			best = c
		}
	}
	return best
}
