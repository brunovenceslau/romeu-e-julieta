// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// Split returns the domains of the network hosts a kit declares, as its
// descriptor writes them, in the form Input.Kit takes (REQ-0069, 07 7.5
// step 6): a port is stripped and the ASCII letters are lower-cased. A
// host with a byte outside ASCII, a name in brackets, a port that is not
// a number from 1 to 65535 written without a leading zero, or a host
// that is then not an egress domain by the rule of 03 3.1 (a wildcard,
// an IP literal, a name spec.ValidDomain refuses) is an error naming the
// kit and the host, and no domain is returned with it. Two hosts that
// differ only in their port give the same domain twice; Derive keeps
// each domain once.
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
	// Unicode case folding maps some letters outside ASCII onto ASCII
	// ones (the Kelvin sign onto k), so a host the descriptor writes in
	// one form would be allowed in another; only ASCII is lowered.
	if strings.ContainsFunc(h, func(r rune) bool { return r > unicode.MaxASCII }) {
		return "", errors.New("has a byte outside ASCII")
	}
	name := h
	// SplitHostPort fails on a host with no port and on a bare IPv6
	// literal; both are then read whole, and ValidDomain refuses the
	// literal. It also takes the brackets off a name, which is no form a
	// descriptor writes for one, so a name in brackets is refused here.
	if host, port, err := net.SplitHostPort(h); err == nil {
		if n, err := strconv.ParseUint(port, 10, 16); err != nil || n == 0 || port != strconv.FormatUint(n, 10) {
			return "", fmt.Errorf("the port %q is not a number from 1 to 65535 without a leading zero", port)
		}
		if strings.HasPrefix(h, "[") && net.ParseIP(host) == nil {
			return "", errors.New("puts a name in brackets, which only an IPv6 literal takes")
		}
		name = host
	}
	name = strings.ToLower(name)
	if err := spec.ValidDomain(name); err != nil {
		return "", err
	}
	return name, nil
}
