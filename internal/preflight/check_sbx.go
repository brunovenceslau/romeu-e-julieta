// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// checkGlobalSecrets is the row sbx-global-secret of I22 (REQ-0371): a
// secret whose scope is not one sandbox, for a service a project uses,
// would reach every sandbox. A global secret for a service no project
// uses passes, and so does a listing with no global row.
func checkGlobalSecrets(ctx context.Context, h *hostEnv) error {
	out, err := h.deps.Sbx.SecretList(ctx)
	if err != nil {
		return err
	}
	rows, err := parseSecrets(out)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, one := scopeOf(r.scope); one || !slices.Contains(h.in.UsedServices, r.service) {
			continue
		}
		e := SbxGlobalSecret.New(fmt.Sprintf("sbx holds a secret for the service %s that is not limited to one sandbox", r.service),
			cli.Detail{Key: "service", Value: r.service}, cli.Detail{Key: "scope", Value: r.scope})
		e.Fill = map[string]string{"<service>": r.service, "<name>": h.in.Project}
		return e
	}
	return nil
}

// checkBroadAllow is the row sbx-broad-allow of I22 (REQ-0372): a
// global allow rule of ** or another broad wildcard, or a permissive
// default network policy. A rule scoped to one sandbox is the business
// of the egress derivation, not of this check.
func checkBroadAllow(ctx context.Context, h *hostEnv) error {
	out, err := h.deps.Sbx.PolicyList(ctx)
	if err != nil {
		return err
	}
	rows, err := parsePolicies(out)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if _, one := scopeOf(r.scope); one || r.effect != "allow" {
			continue
		}
		switch {
		case r.kind == "default":
			return SbxBroadAllow.New("the default network policy of sbx allows every destination",
				cli.Detail{Key: "rule", Value: "default network policy: allow"})
		default:
			broad, err := broadPattern(r.resource)
			if err != nil {
				return err
			}
			if broad {
				return SbxBroadAllow.New("a global allow rule of sbx is broad: "+r.resource,
					cli.Detail{Key: "rule", Value: r.resource})
			}
		}
	}
	return nil
}

// broadPattern reports whether an allow pattern covers a whole class of
// hosts: **, *, a wildcard over a whole top-level domain (*.com), a
// pattern whose top-level label is a wildcard (example.*). A trailing
// dot (the root label) and a port suffix are ignored. A resource that is
// not host[:port], a URL among them, is not a shape romeu reads, and the
// error is upstream-shape (RJ-310): a pattern it cannot judge is never
// read as narrow. A wildcard over a public suffix of two labels
// (*.co.uk) is not caught: that needs the public suffix list, which is
// not a dependency of the module (deferred with the dependency).
func broadPattern(resource string) (bool, error) {
	p := strings.ToLower(strings.TrimSpace(resource))
	if host, port, ok := strings.Cut(p, ":"); ok {
		if port == "" || strings.Trim(port, "0123456789") != "" {
			return false, upstreamShape("policy ls --json", "a rule resource is not host[:port]")
		}
		p = host
	}
	p = strings.TrimSuffix(p, ".")
	if strings.Trim(p, "abcdefghijklmnopqrstuvwxyz0123456789.*-") != "" {
		return false, upstreamShape("policy ls --json", "a rule resource is not host[:port]")
	}
	labels := strings.Split(p, ".")
	if strings.Contains(labels[len(labels)-1], "*") {
		return true, nil
	}
	rest := labels
	for len(rest) > 0 && strings.Trim(rest[0], "*") == "" {
		rest = rest[1:]
	}
	return len(rest) < len(labels) && len(rest) <= 1, nil
}
