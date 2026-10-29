// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
	"github.com/brunovenceslau/romeu-e-julieta/kits"
)

// The seams of step 6; a test replaces them.
var (
	// fetchWorkload reads a workload's descriptor from its registry,
	// by digest. A unit test replaces it with a fake registry: the
	// guarded client reads public HTTPS registries alone.
	fetchWorkload = func(ctx context.Context, ref string, p oci.Platform) (oci.Fetched, error) {
		return oci.NewClient().FetchWorkload(ctx, ref, p)
	}
	// productKits are the product kits romeu renders from (06 6.4).
	productKits fs.FS = kits.FS
	// seams are the clock and the ULID source of the candidate ids.
	seams = state.Seams{Now: time.Now, IDs: canon.NewULIDSource(rand.Reader)}
)

// romeuDir is the dir of <name>-env/ that holds what romeu renders
// (02 2.3), the candidates among it.
const romeuDir = ".romeu"

// candidate is what step 6 resolved and rendered of one target, in
// the order of targets: what step 8 (gate 2 and the promotion) reads.
type candidate struct {
	// project is the target's record of host state, or a new one
	// before its first sync; step 8 records the approval into it.
	project *state.Project
	// dir is the candidate dir of the candidate machine (01 1.6).
	dir *state.Candidate
	// workload is the capability set of the workload descriptor.
	workload oci.Capabilities
	// kits are the kits materialized into dir, in the spec's order,
	// and files the files of each, read back from dir.
	kits  []render.MaterializedKit
	files [][]gate.File
	// egress is the derived egress (07 7.5).
	egress egress.Result

	// The rest is filled once the rules pass: the widening set, its
	// digest, the candidate side of the gate 2 diff and the record
	// written as the candidate's render.json.
	set      gate.Set
	widening canon.Sum[canon.WideningKind]
	side     gate.Side
	record   render.Record
}

// renderCandidates is step 6 of 04 4.2 (REQ-0341), for each target:
// the project record; the workload descriptor, from the descriptor
// cache of host state first and from its registry by digest only on a
// miss, which is then written to the cache; .romeu/ of the env dir,
// made when missing; the candidate dir, made by the event "sync
// renders" of the candidate machine, which deletes every candidate dir
// a killed command left (after step 1's recovery no record names
// one); the kits, product kits from romeu's embedded copy and personal
// kits from the config commit's tree through git fsck --strict and the
// validated walk (I27), materialized into the candidate dir; and the
// egress, derived from the pinned locks, the spec and the network
// domains the descriptors declare. Then the render-time rules run, on
// every target, and only then is sbxenv.yaml rendered and the widening
// set built, so a rule refuses before any sbxenv.yaml is rendered;
// what is written before it is .romeu/, the descriptor cache and the
// candidate dir with its kits. Last, render.json is written with the
// gate's digests. It fills candidates.
//
// On any error once a candidate dir is made, each candidate of the run
// is declined (the event "operator declines" of 01 1.6), which deletes
// its dir, so a command that returns leaves no candidate dir; only a
// killed one does, and the next sync deletes it.
//
// The egress is derived after the kits are materialized, where 04 4.2
// lists it before: the kits' network domains are read from their
// descriptors as the materializer parses them.
func renderCandidates(ctx context.Context, r *run) error {
	if err := renderAll(ctx, r); err != nil {
		return errors.Join(err, decline(r.candidates))
	}
	return nil
}

// renderAll is step 6 without the decline of its candidates on an
// error.
func renderAll(ctx context.Context, r *run) error {
	cat, err := catalog.Embedded()
	if err != nil {
		return fmt.Errorf("the embedded catalog: %w", err)
	}
	for i, p := range r.targets {
		c, err := resolve(ctx, r, p, r.trees[i], cat)
		if err != nil {
			return err
		}
		r.candidates = append(r.candidates, c)
	}
	if err := checkRules(ctx, r); err != nil {
		return err
	}
	digest, err := canon.Digest(canon.Catalog, approve.DecodeCatalog(cat))
	if err != nil {
		return fmt.Errorf("the catalog digest: %w", err)
	}
	for i, p := range r.targets {
		if err := finish(ctx, r, p, r.trees[i], &r.candidates[i], digest); err != nil {
			return err
		}
	}
	return nil
}

// decline declines each candidate of cs, which deletes its dir
// (REQ-1209).
func decline(cs []candidate) error {
	var errs []error
	for _, c := range cs {
		if _, err := c.dir.Apply(state.CandidateDecline, nil, state.Approval{}); err != nil {
			errs = append(errs, fmt.Errorf("decline the candidate %s: %w", c.dir.ID(), err))
		}
	}
	return errors.Join(errs...)
}

// resolve is the part of step 6 for the target p, whose tree step 4
// made, that comes before the rules. On an error once its candidate
// dir is made, it declines that candidate.
func resolve(ctx context.Context, r *run, p spec.Project, t tree, cat catalog.Catalog) (_ candidate, err error) {
	project, err := r.store.LoadProject(p.Name)
	if errors.Is(err, fs.ErrNotExist) {
		project, err = state.NewProject(), nil
	}
	if err != nil {
		return candidate{}, err
	}
	workload, err := workloadCapabilities(ctx, r, p.Workload)
	if err != nil {
		return candidate{}, err
	}
	dir, err := newCandidate(t.env)
	if err != nil {
		return candidate{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, decline([]candidate{{dir: dir}}))
		}
	}()
	m := render.KitMaterializer{
		FS: productKits, EnvDir: t.env, GOARCH: runtime.GOARCH,
		Git: r.git, ConfigDir: r.configClone, ConfigCommit: r.specCommit,
	}
	kits, err := m.Materialize(ctx, render.Candidate{Dir: dir.Path()}, p)
	if err != nil {
		return candidate{}, kitRefused(err)
	}
	files, err := readKits(dir.Path(), kits)
	if err != nil {
		return candidate{}, err
	}
	in := egress.Input{Spec: p.Egress, Kit: declared(workload, kits), Catalog: cat}
	for _, rt := range t.repos {
		in.Repos = append(in.Repos, egress.Repo{Dir: rt.dir, URL: repoURL(p, rt.dir), Read: egress.GitReader(r.git, rt.clone, rt.egressCommit)})
	}
	res, err := egress.Derive(ctx, in)
	if err != nil {
		return candidate{}, err
	}
	return candidate{project: project, dir: dir, workload: workload, kits: kits, files: files, egress: res}, nil
}

// newCandidate makes .romeu/ of the env dir env when it is missing,
// refusing one that is not a plain directory, a link included (I12),
// then the candidate dir below .romeu/candidates/.
func newCandidate(env string) (*state.Candidate, error) {
	rt, err := os.OpenRoot(env)
	if err != nil {
		return nil, fmt.Errorf("the host tree: %w", err)
	}
	defer func() { _ = rt.Close() }()
	if err := makeDir(rt.Mkdir, rt.Lstat, romeuDir, filepath.Join(env, romeuDir), "tree-unsafe"); err != nil {
		return nil, err
	}
	return state.RenderCandidate(filepath.Join(env, filepath.FromSlash(render.CandidatesDir)), seams)
}

// workloadCapabilities returns the capability set of the descriptor of
// the workload ref (06 6.1, REQ-0571): the entry of the descriptor
// cache under the ref's digest when there is one for romeu's platform,
// and otherwise the descriptor read from the registry by digest, which
// is then cached under the digest of the manifest it was read from.
//
// A cache entry is keyed by that manifest's digest (03 3.8, REQ-0240),
// which is the ref's own for a workload of one manifest; for an index,
// the platform manifest's digest is known only once the registry has
// been read, so the cache answers a workload of one manifest alone, and
// an index is read from its registry at each sync until the store can
// find an entry by the index digest and the platform.
func workloadCapabilities(ctx context.Context, r *run, ref string) (oci.Capabilities, error) {
	w, err := spec.ParseWorkload(ref)
	if err != nil {
		return oci.Capabilities{}, err
	}
	platform := oci.Platform{OS: "linux", Architecture: runtime.GOARCH}
	d, err := r.store.GetDescriptor(w.Digest)
	switch {
	case err == nil && d.Workload == w.Digest && d.Platform == platform.String():
		return *d.Capabilities, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return oci.Capabilities{}, err
	}
	f, err := fetchWorkload(ctx, ref, platform)
	if errors.Is(err, oci.ErrInvalid) {
		return oci.Capabilities{}, usage("descriptor-invalid", "the descriptor of the workload "+termsafe.Escape(ref)+": "+termsafe.Escape(err.Error()))
	}
	if err != nil {
		return oci.Capabilities{}, err
	}
	caps := f.Kit.Capabilities
	entry := state.Descriptor{Digest: f.ManifestDigest, Workload: w.Digest, Platform: platform.String(), Capabilities: &caps}
	if err := r.store.PutDescriptor(r.lock, entry); err != nil {
		return oci.Capabilities{}, err
	}
	return caps, nil
}

// kitRefused returns, as usage with the slug kit-refused, an error of
// the materializer that the spec or the config commit causes: a
// personal kit of a refused kind or capability (REQ-0574), a
// descriptor the grammar refuses (REQ-0573), and a tree fsck or the
// walk refuses (I27). Any other error is returned as it is.
func kitRefused(err error) error {
	if errors.Is(err, render.ErrPersonalCapability) || errors.Is(err, render.ErrPersonalKind) || errors.Is(err, oci.ErrInvalid) {
		return usage("kit-refused", "the kits of the spec: "+termsafe.Escape(err.Error()))
	}
	return refused(err, "kit-refused", "the kits of the spec")
}

// readKits returns the files of each kit of kits, read back from the
// candidate dir: the gate shows them and digests them (01 1.4). Each
// file is read as the kit-tree digest reads it: a regular file, with
// the executable mode when any execute bit is set, and anything else
// refused. A kit whose files are not its digest fails closed.
func readKits(dir string, kits []render.MaterializedKit) ([][]gate.File, error) {
	if len(kits) == 0 {
		return nil, nil
	}
	rt, err := os.OpenRoot(filepath.Join(dir, render.CandidateKitsDir))
	if err != nil {
		return nil, fmt.Errorf("the candidate's kits: %w", err)
	}
	defer func() { _ = rt.Close() }()
	out := make([][]gate.File, 0, len(kits))
	for _, m := range kits {
		files, err := kitFiles(rt, m.Dir())
		if err != nil {
			return nil, fmt.Errorf("the candidate's kit %s: %w", termsafe.Escape(m.ID), err)
		}
		sum, err := gate.KitTreeDigest(files)
		if err != nil {
			return nil, fmt.Errorf("the candidate's kit %s: %w", termsafe.Escape(m.ID), err)
		}
		if sum != m.Digest {
			return nil, fmt.Errorf("the candidate's kit %s is not the kit materialized", termsafe.Escape(m.ID))
		}
		out = append(out, files)
	}
	return out, nil
}

// kitFiles returns the files below the dir name of rt.
func kitFiles(rt *os.Root, name string) ([]gate.File, error) {
	sub, err := rt.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()
	fsys := sub.FS()
	var files []gate.File
	err = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", termsafe.Escape(p))
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		mode := gate.ModeRegular
		if info.Mode().Perm()&0o111 != 0 {
			mode = gate.ModeExecutable
		}
		files = append(files, gate.File{Path: p, Mode: mode, Data: data})
		return nil
	})
	return files, err
}

// declared returns the domains the network capability of the workload
// and of each kit allows: the kits' input of the egress derivation (07
// 7.5, step 6), which gates each one the catalog does not allow (I10).
// The hosts of the install phase and of the runtime phase are merged
// into the one list egress.Input.Kit takes, so a host a kit allows at
// install alone is allowed to the sandbox at runtime too. Whether
// egress keeps the phases apart is open: kits/julieta/julieta.yaml
// names the install phase so the runtime does not open github.com
// (REQ-0592), and 07 7.5 does not say. Each host is passed as the
// descriptor writes it; a host:port entry fails the derivation until
// egress.Split (REQ-0069, 06 6.1) splits it.
func declared(workload oci.Capabilities, kits []render.MaterializedKit) []string {
	var out []string
	add := func(c oci.Capabilities) {
		if c.Network == nil {
			return
		}
		for _, rules := range []oci.Rules{c.Network.Install, c.Network.Runtime} {
			for _, e := range rules.Allow {
				out = append(out, e.Hosts...)
			}
		}
	}
	add(workload)
	for _, k := range kits {
		add(k.Capabilities)
	}
	return out
}

// repoURL returns the URL of the repo of p whose dir is dir.
func repoURL(p spec.Project, dir string) string {
	for _, repo := range p.Repos {
		if repo.Dir == dir {
			return repo.URL
		}
	}
	return ""
}

// materialized is the Materializer of kits already written into the
// candidate: render renders sbxenv.yaml from them and writes no kit
// again.
type materialized []render.MaterializedKit

// Materialize returns the kits.
func (m materialized) Materialize(context.Context, render.Candidate, spec.Project) ([]render.MaterializedKit, error) {
	return m, nil
}

// gateReasons are the reasons of the gated domains the derivation
// gives, as the widening set names them; gate.Build adds the spec's
// extras and tool overrides itself, so those two are left to it.
var gateReasons = map[egress.Reason]string{
	egress.ReasonUpload:   gate.ReasonUpload,
	egress.ReasonKit:      gate.ReasonKitDeclared,
	egress.ReasonLockHost: gate.ReasonNonCatalogLock,
}

// finish is the part of step 6 for the target p that comes after the
// rules: sbxenv.yaml, the widening set and its digests, the candidate
// side of the gate 2 diff, and render.json.
func finish(ctx context.Context, r *run, p spec.Project, t tree, c *candidate, catalogDigest canon.Sum[canon.CatalogKind]) error {
	cand := render.Candidate{Dir: c.dir.Path()}
	out, err := render.Render(ctx, cand, render.Input{Project: p, Host: r.settings, Home: r.home}, materialized(c.kits))
	if err != nil {
		return err
	}
	var doc gate.Render
	if err := yaml.Unmarshal(out.Env, &doc); err != nil {
		return fmt.Errorf("the rendered %s: %w", render.CandidateEnvFile, err)
	}
	resolved := gate.Resolved{WorkloadCapabilities: c.workload, Render: doc}
	side := gate.Side{Render: doc, KitFiles: map[string][]gate.File{}, SecretArgv: map[string][]string{}}
	for i, k := range p.Kits {
		source := gate.SourceProduct
		if k.Personal != "" {
			source = gate.SourcePersonal
		}
		resolved.Kits = append(resolved.Kits, gate.ResolvedKit{Source: source, ID: c.kits[i].ID, Files: c.files[i], Capabilities: c.kits[i].Capabilities})
		side.KitFiles[source+":"+c.kits[i].ID] = c.files[i]
	}
	for _, g := range c.egress.Gated {
		reason, ok := gateReasons[g.Reason]
		switch {
		case g.Reason == egress.ReasonExtra || g.Reason == egress.ReasonSpecTool:
			continue
		case !ok:
			return fmt.Errorf("the gated domain %s has the reason %q, which the widening set does not name", termsafe.Escape(g.Domain), g.Reason)
		}
		resolved.EgressGated = append(resolved.EgressGated, gate.GatedDomain{Domain: g.Domain, Reason: reason})
	}
	for _, name := range p.Secrets {
		side.SecretArgv[name] = r.settings.Secrets[name+"@"+p.Name].Argv
	}
	if c.set, err = gate.Build(p, r.settings, resolved); err != nil {
		return err
	}
	side.Set = c.set
	if c.widening, err = gate.Digest(c.set); err != nil {
		return err
	}
	recreate, err := gate.RecreateDigest(p, c.set)
	if err != nil {
		return err
	}
	rec, err := record(ctx, r, p, t, c, out.Env)
	if err != nil {
		return err
	}
	rec.WideningDigest, rec.RecreateDigest, rec.CatalogDigest = c.widening, recreate, catalogDigest
	if err := render.WriteRecord(cand, rec); err != nil {
		return err
	}
	c.side, c.record = side, rec
	return nil
}

// record returns the render.json of the candidate c of p and its tree t,
// whose sbxenv.yaml is env, without the gate's digests: where the spec
// was read, each repo's egress commit and the sha256 of its mise.lock
// there, the digest of each file a promotion writes, and the egress.
func record(ctx context.Context, r *run, p spec.Project, t tree, c *candidate, env []byte) (render.Record, error) {
	eg, err := render.EgressOf(c.egress)
	if err != nil {
		return render.Record{}, err
	}
	rec := render.Record{
		RomeuVersion: approve.Version(r.env.Version),
		SpecSource: render.SpecSource{
			Kind:   render.SpecSourceOrigin,
			Ref:    "refs/romeu/origin/" + filepath.Base(r.configClone) + "/" + r.settings.Config.Ref,
			Commit: r.specCommit,
		},
		Files:  r.bin.Files(),
		Egress: eg,
	}
	rec.Files[render.LiveEnvFile] = plainSum(env)
	for _, k := range c.kits {
		rec.Files[k.LivePath()] = k.Digest.String()
	}
	for _, rt := range t.repos {
		repo := render.RecordRepo{Dir: rt.dir, URL: repoURL(p, rt.dir), EgressRef: rt.egressRef, EgressCommit: rt.egressCommit}
		lock, found, err := egress.GitReader(r.git, rt.clone, rt.egressCommit)(ctx, "mise.lock")
		if err != nil {
			return render.Record{}, fmt.Errorf("the mise.lock of %s: %w", termsafe.Escape(rt.dir), err)
		}
		if found {
			repo.LockSha256 = plainSum(lock)
		}
		rec.Repos = append(rec.Repos, repo)
	}
	return rec, nil
}

// plainSum returns the plain sha256 of data, lowercase hex: the digest
// render.json holds for a file (REQ-0256).
func plainSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
