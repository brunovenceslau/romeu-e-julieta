// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"math"
	"path"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The rule manifest-size (03 3.6, REQ-0216): the manifest romeu run
// would pass for a target must fit the cap of the env value, 32 KiB,
// by the size function of internal/manifest; one over it is
// manifest-size (RJ-313, exit 2), before gate 2.
var _ = register(rule{name: "manifest-size", check: checkManifestSize})

// The stand-ins of the members of the manifest that sync cannot know,
// each as long as the value run passes, or longer, so a manifest that
// fits here fits at run.
const (
	// standInGeneration is a ULID, the form of every generation id
	// and salvage id.
	standInGeneration = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	// sandboxHome is the home of the workload user, where a secondary
	// repo is cloned (REQ-0221): probe A4 measures it and kits/pins.yaml
	// will hold it with the workload pin; until then it is the home of
	// the in-sandbox paths of 02 2.5.
	sandboxHome = "/home/agent"
)

// checkManifestSize refuses the first target whose manifest is over
// the cap.
func checkManifestSize(_ context.Context, r *run) error {
	for i, c := range r.candidates {
		p := r.targets[i]
		m, err := sizedManifest(p, r.trees[i], c.egress, r.specCommit)
		if err != nil {
			return err
		}
		n, err := manifest.EncodedSize(m)
		if err != nil {
			return fmt.Errorf("the manifest of %s: %w", termsafe.Escape(p.Name), err)
		}
		if n > manifest.MaxEncodedSize {
			e := ManifestSize.New(fmt.Sprintf("the manifest of %s is %d bytes encoded, over the cap of %d bytes", termsafe.Escape(p.Name), n, manifest.MaxEncodedSize))
			e.Fill = map[string]string{"<name>": p.Name}
			return e
		}
	}
	return nil
}

// sizedManifest returns the manifest of the target p with the tree t
// and the egress eg, as large as the one romeu passes on any exec, a
// salvage call's included: every gated domain counts as approved, each
// repo has one base, and the generation, the salvage id of salvageRun
// (REQ-0223) and the salvage cap are stand-ins of the longest form.
//
// It holds no sharedMemory, which 03 3.6 lists, since
// internal/manifest has no such member yet (the re-plan of
// 2026-10-28): until it has, the size measured lacks that member.
func sizedManifest(p spec.Project, t tree, eg egress.Result, commit string) (manifest.Manifest, error) {
	m := manifest.Manifest{
		Agent: p.Agent, Egress: eg.Effective(), Generation: standInGeneration, Project: p.Name,
		Protocol: manifest.Protocol, Schema: manifest.Field, SpecCommit: commit,
		Salvage:    manifest.Salvage{CapBytes: math.MaxInt64},
		SalvageRun: &manifest.SalvageRun{ID: standInGeneration},
	}
	if p.Salvage != nil {
		m.Salvage.ExcludeIgnored = p.Salvage.ExcludeIgnored
	}
	for _, rt := range t.repos {
		var repo spec.Repo
		for _, sr := range p.Repos {
			if sr.Dir == rt.dir {
				repo = sr
			}
		}
		where := rt.clone
		if !repo.Primary {
			where = path.Join(sandboxHome, "src", p.Name, rt.dir)
		}
		m.Repos = append(m.Repos, manifest.Repo{
			Base: []string{rt.egressCommit}, Dir: rt.dir, Memory: rt.memory, Path: where, Primary: repo.Primary,
			Ref: strings.TrimPrefix(rt.egressRef, "refs/romeu/origin/"+rt.dir+"/"), URL: repo.URL,
		})
	}
	m.Run = manifest.Run{}
	if p.Run != nil {
		data, err := json.Marshal(p.Run)
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("the run layout of %s: %w", termsafe.Escape(p.Name), err)
		}
		if err := json.Unmarshal(data, &m.Run); err != nil {
			return manifest.Manifest{}, fmt.Errorf("the run layout of %s: %w", termsafe.Escape(p.Name), err)
		}
	}
	sum, err := canon.Digest(canon.Run, m.Run)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("the run layout of %s: %w", termsafe.Escape(p.Name), err)
	}
	m.RunDigest = sum.String()
	return m, nil
}
