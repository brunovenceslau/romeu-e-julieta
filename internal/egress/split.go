// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// Split returns the domains of the network hosts a kit declares, as its
// descriptor writes them, in the form Input.Kit takes (REQ-0069, 07 7.5
// step 6): a numeric port is stripped and the host is lower-cased. A
// host that is then not an egress domain by the rule of 03 3.1 (a
// wildcard, an IP literal, a name spec.ValidDomain refuses) or whose port
// is not a number from 1 to 65535 is an error naming the kit and the
// host, and no domain is returned with it.
//
// Split only reads the form of each host. Which phase a host belongs to,
// whether it is gated, and the catalog's upload flags are decided by the
// caller and by Derive.
func Split(kit string, hosts []string) ([]string, error) {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		dom, err := splitHost(h)
		if err != nil {
			return nil, fmt.Errorf("kit %s declares the network host %q: %w", kit, h, err)
		}
		out = append(out, dom)
	}
	return out, nil
}

// splitHost returns the domain of one declared host.
func splitHost(h string) (string, error) {
	name := h
	// SplitHostPort fails on a host with no port and on a bare IPv6
	// literal; both are then read whole, and ValidDomain refuses the
	// literal.
	if host, port, err := net.SplitHostPort(h); err == nil {
		if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 {
			return "", fmt.Errorf("the port %q is not a number from 1 to 65535", port)
		}
		name = host
	}
	name = strings.ToLower(name)
	if err := spec.ValidDomain(name); err != nil {
		return "", err
	}
	return name, nil
}
