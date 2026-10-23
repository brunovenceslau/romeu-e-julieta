// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Port is one published port of sbxenv.yaml, the object form of the
// env-file schema of sbx: the sandbox port, the host port, the protocol
// and the host address.
type Port struct {
	// Sandbox is the port inside the sandbox.
	Sandbox int `yaml:"sandbox"`
	// Host is the port on the host; 0, left out, lets sbx choose.
	Host int `yaml:"host,omitempty"`
	// Protocol is one of the protocols of 03 3.1; DefaultProtocol when
	// the spec names none.
	Protocol string `yaml:"protocol"`
	// HostIP is the host address: 127.0.0.1 always, written out even
	// when the spec names none, so that the file itself says the port
	// binds loopback alone rather than resting on the default of sbx.
	HostIP string `yaml:"hostIP"`
}

// DefaultProtocol is the protocol sbx gives a port spec that names none
// (T037, re-plan of 2026-10-22): tcp4, tcp6 being only for an IPv6 host
// address, which the spec does not accept yet (03 3.1). render writes
// it, so the file says what sbx does.
const DefaultProtocol = "tcp4"

// protocols are the protocols a port spec may name (03 3.1, REQ-1262).
var protocols = []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"}

// loopback is the one host address a port spec may name (REQ-1262).
const loopback = "127.0.0.1"

// ParsePort reads a port spec of the project spec,
// [[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL] (03 3.1, REQ-1262),
// into the object sbxenv.yaml holds. It applies the rule of 03 3.1 again
// rather than trust that the spec was validated: each port from 1 to
// 65535 with no leading zero, HOST_IP only 127.0.0.1, and a protocol of
// the list.
//
// Written by hand (ADR 0038): the grammar is three fields split on ":"
// and one on "/". The parser of the Docker engine
// (github.com/docker/go-connections/nat.ParsePortSpec) reads a wider
// grammar, port ranges and any host address among it, and would be a
// new dependency.
func ParsePort(s string) (Port, error) {
	spec, proto, slash := strings.Cut(s, "/")
	if !slash {
		proto = DefaultProtocol
	} else if !slices.Contains(protocols, proto) {
		return Port{}, fmt.Errorf("the port spec %q names a protocol that is none of %s", s, strings.Join(protocols, ", "))
	}
	fields := strings.Split(spec, ":")
	p := Port{HostIP: loopback}
	var err error
	switch len(fields) {
	case 3:
		if fields[0] != loopback {
			return Port{}, fmt.Errorf("the port spec %q binds a host address other than %s", s, loopback)
		}
		fields = fields[1:]
		fallthrough
	case 2:
		if p.Host, err = portNumber(fields[0]); err != nil {
			return Port{}, fmt.Errorf("the port spec %q: %w", s, err)
		}
		fields = fields[1:]
		fallthrough
	case 1:
		if p.Sandbox, err = portNumber(fields[0]); err != nil {
			return Port{}, fmt.Errorf("the port spec %q: %w", s, err)
		}
	default:
		return Port{}, fmt.Errorf("the port spec %q is not [[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL]", s)
	}
	p.Protocol = proto
	return p, nil
}

// portNumber reads one port: decimal digits with no leading zero, from 1
// to 65535.
func portNumber(s string) (int, error) {
	if s == "" || s[0] == '0' || strings.Trim(s, "0123456789") != "" || len(s) > 5 {
		return 0, fmt.Errorf("%q is not a port from 1 to 65535", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil || n > 65535 {
		return 0, fmt.Errorf("%q is not a port from 1 to 65535", s)
	}
	return n, nil
}
