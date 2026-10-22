// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScrubberReplace replaces the four values of 04 4.1, Error ids,
// item 7.
func TestScrubberReplace(t *testing.T) {
	s := Scrubber{Home: "/home/agent", User: "agent", Host: "agent-mbp", Root: "/home/agent/dev"}
	tests := []struct {
		name, in, want string
	}{
		{"the root inside the home", "open /home/agent/dev/shop-env: denied", "open <root>/shop-env: denied"},
		{"the home", "read /home/agent/.config/x", "read <home>/.config/x"},
		{"the user as a word", "owner agent, group staff", "owner <user>, group staff"},
		{"the host as a word", "host agent-mbp refused", "host <host> refused"},
		{"the user inside another word", "reagent and agent-mbp2 and agent_x", "reagent and agent-mbp2 and agent_x"},
		{"twice in a row", "agent agent", "<user> <user>"},
		{"at the ends", "agent", "<user>"},
		{"nothing to replace", "plain text", "plain text"},
		{"more than once", "/home/agent/dev and /home/agent/dev", "<root> and <root>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, s.Replace(tt.in))
		})
	}
}

// TestScrubberEmptyValues replaces nothing for a value left empty, so
// that an unknown host name does not touch the text.
func TestScrubberEmptyValues(t *testing.T) {
	assert.Equal(t, "agent at /home/agent", Scrubber{}.Replace("agent at /home/agent"))
	assert.Equal(t, "agent at <home>", Scrubber{Home: "/home/agent"}.Replace("agent at /home/agent"))
}

// TestScrubberLeavesItsPlaceholdersAlone holds that a replacement is
// never read again: a user named home or root, or a home of /, does not
// turn <home> and <root> into something else.
func TestScrubberLeavesItsPlaceholdersAlone(t *testing.T) {
	tests := []struct {
		name string
		s    Scrubber
		in   string
		want string
	}{
		{"a user named home", Scrubber{Home: "/srv/home", User: "home"}, "home owns /srv/home/x", "<user> owns <home>/x"},
		{"a user named root", Scrubber{Root: "/srv/r", User: "root"}, "root wrote /srv/r/a", "<user> wrote <root>/a"},
		{"a host named user", Scrubber{Host: "user", User: "bob"}, "user and bob", "<host> and <user>"},
		{"a home of /", Scrubber{Home: "/", User: "bob"}, "/usr/bin and bob", "/usr/bin and <user>"},
		{"a root of /", Scrubber{Root: "/"}, "/etc/x", "/etc/x"},
		{"a user that is a placeholder's word", Scrubber{Host: "host", User: "user"}, "host user", "<host> <user>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.s.Replace(tt.in))
		})
	}
}
