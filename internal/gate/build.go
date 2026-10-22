// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// setMembers maps each top-level field of spec.Project tagged
// gate:"widening", by JSON name, to the member of Set that holds it.
// The generator fails when a gated field is missing here or a key here
// is not a gated field (Generated).
var setMembers = map[string]string{
	"name":     "project",
	"repos":    "repos",
	"secrets":  "secrets",
	"workload": "workload",
	"kits":     "kits",
	"egress":   "egressGated",
	"ports":    "ports",
	"agent":    "agent",
	"salvage":  "salvageExcludes",
}

// Resolved is what the widening set holds beyond the spec and the host
// settings: what romeu read or derived for this render.
type Resolved struct {
	// WorkloadCapabilities is the normalized capability set of the
	// workload descriptor.
	WorkloadCapabilities Capabilities
	// Kits are the files and capabilities of each kit of the spec, in
	// the spec's order.
	Kits []ResolvedKit
	// EgressGated are the gated domains derived from the pinned
	// commits, the catalog and the kits (07 7.5); Build adds the spec's
	// own extras and tool overrides.
	EgressGated []GatedDomain
	// Render is the rendered sbxenv.yaml, decoded.
	Render Render
}

// ResolvedKit is the content of one kit.
type ResolvedKit struct {
	// Files are the kit's files.
	Files []File
	// Capabilities is the normalized capability set of the kit's
	// descriptor.
	Capabilities Capabilities
}

// Build returns the widening set of the validated project spec p, with
// the secret commands of the host settings hs and the resolved inputs
// r, in its normal form. It fails, rather than leave a member out, on a
// secret with no host-settings entry <name>@<project>, a workload that
// is not pinned by digest, a kit that is not exactly one of product and
// personal, a count of resolved kits other than the spec's, and a repo
// without a dir.
//
//romeu:invariant I7 guard
func Build(p spec.Project, hs spec.HostSettings, r Resolved) (Set, error) {
	s := Set{Project: p.Name, Agent: p.Agent, Ports: p.Ports, EgressGated: slices.Clone(r.EgressGated)}
	for _, repo := range p.Repos {
		if repo.Dir == "" {
			return Set{}, fmt.Errorf("gate: the repo %q has no dir; the spec is validated first", repo.URL)
		}
		s.Repos = append(s.Repos, Repo{Dir: repo.Dir, URL: repo.URL, Primary: repo.Primary, Ref: repo.Ref})
	}
	for _, name := range p.Secrets {
		key := name + "@" + p.Name
		entry, ok := hs.Secrets[key]
		if !ok {
			return Set{}, fmt.Errorf("gate: the secret %q has no host-settings entry %q", name, key)
		}
		sum, err := SecretArgvDigest(entry.Argv)
		if err != nil {
			return Set{}, fmt.Errorf("gate: the secret %q: %w", name, err)
		}
		s.Secrets = append(s.Secrets, Secret{Name: name, Service: entry.Service, ArgvDigest: sum})
	}
	repository, digest, ok := strings.Cut(p.Workload, "@")
	if !ok || repository == "" || digest == "" {
		return Set{}, fmt.Errorf("gate: the workload %q is not pinned by digest", p.Workload)
	}
	s.Workload = Workload{Repository: repository, Digest: digest, Capabilities: r.WorkloadCapabilities}
	if len(r.Kits) != len(p.Kits) {
		return Set{}, fmt.Errorf("gate: the spec has %d kits and %d were resolved", len(p.Kits), len(r.Kits))
	}
	for i, k := range p.Kits {
		kit, err := buildKit(k, r.Kits[i])
		if err != nil {
			return Set{}, err
		}
		s.Kits = append(s.Kits, kit)
	}
	if p.Egress != nil {
		for _, d := range p.Egress.Extra {
			s.EgressGated = appendDomain(s.EgressGated, GatedDomain{Domain: d, Reason: ReasonSpecExtra})
		}
		for _, tool := range slices.Sorted(maps.Keys(p.Egress.Tools)) {
			for _, d := range p.Egress.Tools[tool] {
				s.EgressGated = appendDomain(s.EgressGated, GatedDomain{Domain: d, Reason: ReasonSpecToolOverride})
			}
		}
	}
	if p.Salvage != nil {
		s.SalvageExcludes = p.Salvage.ExcludeIgnored
	}
	sum, err := RenderDigest(r.Render)
	if err != nil {
		return Set{}, err
	}
	s.RenderDigest = sum
	return normal(s)
}

// buildKit returns the kit of the set for the spec kit k and its
// content r.
func buildKit(k spec.Kit, r ResolvedKit) (Kit, error) {
	var kit Kit
	switch {
	case k.Product != "" && k.Personal == "":
		kit.ID, kit.Source = k.Product, SourceProduct
	case k.Personal != "" && k.Product == "":
		kit.ID, kit.Source = k.Personal, SourcePersonal
	default:
		return Kit{}, fmt.Errorf("gate: a kit names product %q and personal %q; it names exactly one", k.Product, k.Personal)
	}
	sum, err := KitTreeDigest(r.Files)
	if err != nil {
		return Kit{}, fmt.Errorf("gate: the kit %s: %w", kit.key(), err)
	}
	kit.ContentDigest, kit.Args, kit.Capabilities = sum, k.Args, r.Capabilities
	return kit, nil
}

// appendDomain appends d to list unless list holds it: a domain the
// spec names twice for one reason is one gated domain.
func appendDomain(list []GatedDomain, d GatedDomain) []GatedDomain {
	if slices.Contains(list, d) {
		return list
	}
	return append(list, d)
}

// Render is the rendered sbxenv.yaml, decoded (01 1.4).
type Render map[string]any

// CanonKind declares the one kind of the normalized render.
func (Render) CanonKind() canon.RenderKind { return canon.Render }

// RenderDigest returns the widening set's renderDigest (01 1.4): the
// digest of the render r stripped of every top-level key named by a
// field of spec.Project that carries no gate:"widening" tag
// (ungatedFields, today sandboxOptions and run). It does not change r,
// and it refuses an absent render.
func RenderDigest(r Render) (canon.Sum[canon.RenderKind], error) {
	if r == nil {
		return canon.Sum[canon.RenderKind]{}, errors.New("gate: no render to digest")
	}
	stripped := maps.Clone(r)
	for _, f := range ungatedFields {
		delete(stripped, f)
	}
	return canon.Digest(canon.Render, stripped)
}

// recreateInput is the input of the recreate digest: each field of
// spec.Project tagged apply:"recreate", by JSON name.
type recreateInput map[string]any

// CanonKind declares the one kind of the recreate input.
func (recreateInput) CanonKind() canon.RecreateKind { return canon.Recreate }

// RecreateDigest returns the recreate digest (03 3.12) of the project
// spec p and its widening set s: every field tagged apply:"recreate"
// (recreateFields), a gated one as the member of s that holds it,
// resolved, so a change of a kit's content or of the workload's
// capabilities needs a recreate too, and an ungated one, such as
// sandboxOptions, as the spec holds it.
func RecreateDigest(p spec.Project, s Set) (canon.Sum[canon.RecreateKind], error) {
	n, err := normal(s)
	if err != nil {
		return canon.Sum[canon.RecreateKind]{}, err
	}
	in := recreateInput{}
	for _, f := range recreateFields {
		var v any
		var ok bool
		if m, gated := setMembers[f]; gated {
			v, ok = memberByJSON(reflect.ValueOf(n), m)
		} else {
			v, ok = memberByJSON(reflect.ValueOf(p), f)
		}
		if !ok {
			return canon.Sum[canon.RecreateKind]{}, fmt.Errorf("gate: no member holds the recreate field %s", f)
		}
		in[f] = v
	}
	return canon.Digest(canon.Recreate, in)
}

// memberByJSON returns the field of the struct v whose JSON name is
// name.
func memberByJSON(v reflect.Value, name string) (any, bool) {
	t := v.Type()
	for i := range t.NumField() {
		tag, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if tag == name {
			return v.Field(i).Interface(), true
		}
	}
	return nil, false
}
