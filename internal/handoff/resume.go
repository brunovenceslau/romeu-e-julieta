// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// CountFunc returns the number of commits of to that from does not
// have, in the repo dir, and false when it cannot tell (a commit that
// is gone after a rebase, a repo that is not there). The command that
// has git supplies it; a test fixes it.
type CountFunc func(repo, from, to string) (n int, ok bool)

// Changes lists, in the order of the repos of old and then of the repos
// only now has, what differs between the git facts old, stamped in a
// narrative, and now, stamped in a later facts file (REQ-0681): the
// count of new commits, a moved branch and a changed dirty count, which
// are the three things 08 8.3 says the gap line lists, and a repo added
// to or removed from the manifest. The stashes, the upstream and the
// ahead and behind counts are stamped but do not count as a difference:
// a push changes them and says nothing about the work. It returns none
// when nothing differs. count may be nil.
func Changes(old, now []RepoFacts, count CountFunc) []string {
	var out []string
	for _, o := range old {
		i := slices.IndexFunc(now, func(n RepoFacts) bool { return n.Dir == o.Dir })
		if i < 0 {
			out = append(out, o.Dir+": removed from the manifest")
			continue
		}
		n := now[i]
		if n.Head != o.Head {
			out = append(out, o.Dir+": "+commits(o, n, count))
		}
		if n.Branch != o.Branch {
			out = append(out, fmt.Sprintf("%s: branch %s -> %s", o.Dir, shown(o.Branch, "(detached)"), shown(n.Branch, "(detached)")))
		}
		if n.Dirty != o.Dirty {
			out = append(out, fmt.Sprintf("%s: dirty %d -> %d", o.Dir, o.Dirty, n.Dirty))
		}
	}
	for _, n := range now {
		if !slices.ContainsFunc(old, func(o RepoFacts) bool { return o.Dir == n.Dir }) {
			out = append(out, n.Dir+": added to the manifest")
		}
	}
	return out
}

// commits words the move of a head from o to n.
func commits(o, n RepoFacts, count CountFunc) string {
	if count != nil && o.Head != "" && n.Head != "" {
		switch c, ok := count(o.Dir, o.Head, n.Head); {
		case ok && c == 1:
			return "1 new commit"
		case ok && c > 1:
			return fmt.Sprintf("%d new commits", c)
		}
	}
	return "the head moved"
}

// shown returns s, or none when s is empty.
func shown(s, none string) string {
	if s == "" {
		return none
	}
	return s
}

// GapLine returns the gap line of 08 8.3 (REQ-0681), without its
// newline, or "" when there is none: "the newest facts differ from the
// latest narrative of <time>: <changes>". It is there only when a
// narrative and a later facts file exist and the git facts of the facts
// file differ from those stamped in the narrative; a newer facts file
// with the same git facts, such as the one SessionEnd writes at /clear
// after /handoff, gives none. A facts file older than the narrative says
// nothing about what happened since it.
func GapLine(l Latest, count CountFunc) string {
	if l.Narrative == nil || l.Facts == nil || bytes.Compare(l.Facts.ID[:], l.Narrative.ID[:]) <= 0 {
		return ""
	}
	changes := Changes(l.Narrative.File.Repos, l.Facts.File.Repos, count)
	if len(changes) == 0 {
		return ""
	}
	return "the newest facts differ from the latest narrative of " + l.Narrative.File.CreatedAt.UTC().Format(time.RFC3339) +
		": " + oneLine(strings.Join(changes, "; "))
}

// oneLine escapes s and keeps it on one line.
func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(termsafe.Escape(s))
}
