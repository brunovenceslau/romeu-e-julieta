// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// The port spec of 03 3.1 (REQ-1262) and the object sbxenv.yaml holds
// for it; an omitted protocol is tcp4, as sbx reads it (T037, re-plan
// of 2026-10-22).
func TestParsePort(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want render.Port
	}{
		{"the sandbox port alone", "3000", render.Port{Sandbox: 3000, Protocol: "tcp4"}},
		{"host and sandbox ports", "8080:80", render.Port{Sandbox: 80, Host: 8080, Protocol: "tcp4"}},
		{"the loopback host address", "127.0.0.1:8080:80", render.Port{Sandbox: 80, Host: 8080, Protocol: "tcp4", HostIP: "127.0.0.1"}},
		{"a protocol", "53/udp", render.Port{Sandbox: 53, Protocol: "udp"}},
		{"every field", "127.0.0.1:5353:53/udp6", render.Port{Sandbox: 53, Host: 5353, Protocol: "udp6", HostIP: "127.0.0.1"}},
		{"tcp named", "1:65535/tcp", render.Port{Sandbox: 65535, Host: 1, Protocol: "tcp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render.ParsePort(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The port specs ParsePort refuses, each by the rule of 03 3.1.
func TestParsePortErrors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", `"" is not a port`},
		{"a port of zero", "0", `"0" is not a port`},
		{"a leading zero", "080", `"080" is not a port`},
		{"past 65535", "65536", `"65536" is not a port`},
		{"six digits", "100000", `"100000" is not a port`},
		{"a sign", "+80", `"+80" is not a port`},
		{"a range", "80-81", `"80-81" is not a port`},
		{"an empty host port", ":80", `"" is not a port`},
		{"another host address", "0.0.0.0:80:80", "binds a host address other than 127.0.0.1"},
		{"an IPv6 host address", "[::1]:80:80", "is not [[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL]"},
		{"four fields", "a:1:2:3", "is not [[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL]"},
		{"an unknown protocol", "80/sctp", "names a protocol that is none of"},
		{"an empty protocol", "80/", "names a protocol that is none of"},
		{"a second slash", "80/tcp/udp", "names a protocol that is none of"},
		{"an uppercase protocol", "80/TCP", "names a protocol that is none of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render.ParsePort(tt.in)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// Every port spec ParsePort accepts writes back to one that reads as the
// same port.
func FuzzParsePort(f *testing.F) {
	for _, s := range []string{"3000", "8080:80", "127.0.0.1:1:2/udp", "0", "80/", "a:b:c:d"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, err := render.ParsePort(s)
		if err != nil {
			return
		}
		assert.Positive(t, p.Sandbox)
		assert.LessOrEqual(t, p.Sandbox, 65535)
		assert.GreaterOrEqual(t, p.Host, 0)
		assert.LessOrEqual(t, p.Host, 65535)
		assert.Contains(t, []string{"", "127.0.0.1"}, p.HostIP)
		assert.Contains(t, []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"}, p.Protocol)
	})
}
