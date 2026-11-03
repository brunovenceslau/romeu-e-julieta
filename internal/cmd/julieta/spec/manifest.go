// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"encoding/json/v2"
	"fmt"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The stand-ins of the members of the manifest a spec alone does not
// give, each as long as the value romeu passes. The host root, under
// which the primary clone and each memory dir are, is the host's and
// unknown here: the shortest one, "/", stands in for it, so the size
// measured is a lower bound and romeu sync measures the manifest again
// with the root it has (03 3.6, REQ-0216).
const (
	// standInULID is a lowercase ULID, the form of the generation and
	// of a salvage id.
	standInULID = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	// sandboxHome is the home of the workload user, below which a
	// secondary repo is cloned (REQ-0221), as romeu sync measures it.
	sandboxHome = "/home/agent"
)

// standInCommit is a commit id: the spec commit and each base.
var standInCommit = strings.Repeat("0", 40)

// egressSet is the egress of a spec as far as julieta can derive it:
// the catalog's meta and base sets and the spec's egress.extra, and
// with --catalog the domains of each tool of the repos' locks and the
// hosts the locks record (07 7.5, steps 2 to 5). Kit-declared domains
// (step 6) need the descriptors, which romeu reads; they are left out.
type egressSet map[string]bool

// baseEgress returns the egress of p before any lock is read, with the
// embedded catalog cat.
func baseEgress(cat catalog.Catalog, p projectspec.Project) egressSet {
	eg := egressSet{}
	eg.add(cat.Meta()...)
	eg.add(cat.Base()...)
	if p.Egress != nil {
		eg.add(p.Egress.Extra...)
	}
	return eg
}

// add adds domains to the set.
func (e egressSet) add(domains ...string) {
	for _, d := range domains {
		e[d] = true
	}
}

// sorted returns the domains, sorted.
func (e egressSet) sorted() []string {
	out := make([]string, 0, len(e))
	for d := range e {
		out = append(out, d)
	}
	slices.Sort(out)
	return out
}

// manifestFinding returns the finding manifest-size (RJ-313, reported
// as a finding of RJ-328) when the manifest romeu would pass for p,
// with the egress eg, is over the cap of 32 KiB, by the size function
// of internal/manifest (03 3.6, REQ-0216).
func manifestFinding(file string, p projectspec.Project, eg egressSet) ([]finding, error) {
	m, err := sizedManifest(p, eg)
	if err != nil {
		return nil, fmt.Errorf("the manifest of %s: %w", file, err)
	}
	n, err := manifest.EncodedSize(m)
	if err != nil {
		return nil, fmt.Errorf("the manifest of %s: %w", file, err)
	}
	if n <= manifest.MaxEncodedSize {
		return nil, nil
	}
	return []finding{{file: file, rule: "manifest-size", message: fmt.Sprintf(
		"the manifest of the project is at least %d bytes encoded, over the cap of %d bytes; julieta measures a lower bound, and romeu sync measures it again",
		n, manifest.MaxEncodedSize)}}, nil
}

// sizedManifest returns the manifest of p with the egress eg and the
// stand-ins above, as romeu sync builds it for its own size check, a
// salvage call's members included.
func sizedManifest(p projectspec.Project, eg egressSet) (manifest.Manifest, error) {
	m := manifest.Manifest{
		Agent: p.Agent, Egress: eg.sorted(), Generation: standInULID, Project: p.Name,
		Protocol: manifest.Protocol, Schema: manifest.Field, SpecCommit: standInCommit,
		Salvage:    manifest.Salvage{CapBytes: math.MaxInt64},
		SalvageRun: &manifest.SalvageRun{ID: standInULID},
	}
	if p.Salvage != nil {
		m.Salvage.ExcludeIgnored = p.Salvage.ExcludeIgnored
	}
	env := "/" + p.Name + "-env"
	for _, r := range p.Repos {
		where := path.Join(env, r.Dir)
		if !r.Primary {
			where = path.Join(sandboxHome, "src", p.Name, r.Dir)
		}
		m.Repos = append(m.Repos, manifest.Repo{
			Base: []string{standInCommit}, Dir: r.Dir, Memory: path.Join(env, "memory", r.Dir), Path: where,
			Primary: r.Primary, Ref: r.Ref, URL: r.URL,
		})
	}
	m.Run = manifest.Run{}
	if p.Run != nil {
		data, err := json.Marshal(p.Run)
		if err != nil {
			return manifest.Manifest{}, err
		}
		if err := json.Unmarshal(data, &m.Run); err != nil {
			return manifest.Manifest{}, err
		}
	}
	sum, err := canon.Digest(canon.Run, m.Run)
	if err != nil {
		return manifest.Manifest{}, err
	}
	m.RunDigest = sum.String()
	return m, nil
}
