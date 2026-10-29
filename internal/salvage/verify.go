// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The skipped reasons the host half adds (03 3.11, REQ-0249); each
// one's recovery is the table of 08 8.5 (REQ-0251).
const (
	// ReasonFsckFailed: a bundle failed its checks, a ref name the
	// validator refuses among them (REQ-0715).
	ReasonFsckFailed = "fsck-failed"
	// ReasonDaemonURLRefused: gitsafe refuses the daemon's URL
	// (REQ-0716).
	ReasonDaemonURLRefused = "daemon-url-refused"
	// ReasonRepoMissing: a repo of the generation's recorded set has no
	// manifest this build reads (REQ-0714).
	ReasonRepoMissing = "repo-missing"
	// ReasonDaemonHeadsMissing: a daemon head is absent from the bundle
	// refs or has another SHA (REQ-0716).
	ReasonDaemonHeadsMissing = "daemon-heads-missing"
	// ReasonPayloadMismatch: a payload is absent, or its size or sha256
	// differs from the manifest (REQ-0714).
	ReasonPayloadMismatch = "payload-mismatch"
)

// VerifyRepo is one repo of the generation's recorded repo set
// (REQ-0693), as romeu knows it on the host.
type VerifyRepo struct {
	// Dir is the repo's dir in the project, a name of 03 3.1.
	Dir string
	// Memory is its memory dir, an absolute and clean path.
	Memory string
	// Clone is its host clone, where the refs are created.
	Clone string
	// Origin is its origin URL, fetched into refs/romeu/origin/<dir>/
	// when a bundle's prerequisites are missing; empty, no fetch.
	Origin string
}

// VerifyConfig is the host half of one salvage (08 8.5 steps 1, 2 and
// 4): the salvage romeu passed to julieta, and where its refs go.
type VerifyConfig struct {
	// Git is romeu's runner, in host mode.
	Git *gitsafe.Runner
	// Name is the project's name, which names the refs.
	Name string
	// Generation and SalvageID are the ids romeu passed (REQ-0247).
	Generation, SalvageID string
	// Repos is the generation's recorded repo set.
	Repos []VerifyRepo
	// CapBytes is salvage.capBytes, the most payload bytes the
	// verification reads (REQ-0253).
	CapBytes int64
	// Work is an existing directory of romeu's, 0700 and below no
	// memory dir or other mount of the sandbox, where each bundle is
	// copied for git to read. The copies stay: the caller removes the
	// directory, so this package removes nothing on the host (I16).
	Work string
}

// Item is one skipped item of a salvage: the manifests' own and the
// ones the host half adds.
type Item struct {
	// Repo is the dir of the repo whose salvage left it out.
	Repo string
	// Item names what was left out, escaped.
	Item string
	// Reason is one of the skipped reasons of 03 3.11.
	Reason string
	// Detail is what failed, escaped; empty when the reason says it all.
	Detail string
}

// RepoResult is what the host half read and imported of one repo.
type RepoResult struct {
	// Dir is the repo's dir.
	Dir string
	// Manifest is its manifest; the zero value when the repo is
	// repo-missing.
	Manifest Manifest
	// ManifestSHA256 is the plain sha256 of the manifest's bytes
	// (REQ-0234, REQ-0256); empty when the repo is repo-missing.
	ManifestSHA256 string
	// Imported are the refs created in its host clone.
	Imported []gitsafe.Head
}

// Verification is the result of the host half's steps 1 and 2.
type Verification struct {
	// RefsPrefix is refs/romeu/salvage/<name>/<generation>/<salvage-id>/,
	// below which every imported ref is.
	RefsPrefix string
	// Repos are the repos, in the order of the config.
	Repos []RepoResult
	// Skipped are the items of the manifests and of the host, in the
	// order of the repos.
	Skipped []Item
	// SecretBearing says that a verified payload holds ignored files or
	// agent state (REQ-0711).
	SecretBearing bool
}

// Complete reports whether items hold only the reasons that leave a
// salvage complete, excluded-by-spec and transcript-excluded
// (REQ-0717). The manifest's own complete is not read: the caller adds
// the items of its later steps, such as daemon-heads-missing, before it
// asks.
func Complete(items []Item) bool {
	return !slices.ContainsFunc(items, func(i Item) bool { return !slices.Contains(acceptedReasons, i.Reason) })
}

// Reasons returns the reasons of items that make a salvage incomplete,
// sorted, each once: the reasons[] of the salvage record (03 3.8).
func Reasons(items []Item) []string {
	var out []string
	for _, i := range items {
		if !slices.Contains(acceptedReasons, i.Reason) && !slices.Contains(out, i.Reason) {
			out = append(out, i.Reason)
		}
	}
	slices.Sort(out)
	return out
}

// HostRefPrefix returns refs/romeu/salvage/<name>/<generation>/
// <salvage-id>/, the namespace of the refs a salvage imports
// (REQ-0715).
func HostRefPrefix(name, generation, salvageID string) string {
	return "refs/romeu/salvage/" + name + "/" + generation + "/" + salvageID + "/"
}

// Verify runs steps 1 and 2 of the host half of 08 8.5 (REQ-0714,
// REQ-0715) over the generation's recorded repo set: for each repo it
// reads the manifest of memory/<dir>/salvage/<generation>/<salvage-id>/
// through an os.Root (I24); a repo with none, or with one this build
// does not decode or that names another generation or repo, is the
// item repo-missing. A salvageId other than the one passed is
// salvage-id-mismatch (RJ-339), and a path that breaks the rule of
// REQ-0252 is salvage-path (RJ-307): both are errors, not items. Then,
// bundles first and within salvage.capBytes, each bundle is verified
// and imported (importBundle) and each file's size and sha256 checked;
// a payload beyond the cap is not read and is over-cap (REQ-0253). The
// manifests' items are kept and their complete is not read (REQ-0717).
// Any other error is a verification romeu could not run.
func Verify(ctx context.Context, c VerifyConfig) (Verification, error) {
	if c.Git == nil {
		return Verification{}, errors.New("salvage: a verification needs a runner")
	}
	if _, err := canon.ParseULID(c.Generation); err != nil {
		return Verification{}, fmt.Errorf("the generation: %w", err)
	}
	if _, err := canon.ParseULID(c.SalvageID); err != nil {
		return Verification{}, fmt.Errorf("the salvage id: %w", err)
	}
	if err := checkComponent("the project name", c.Name); err != nil {
		return Verification{}, err
	}
	v := &verifier{c: c, budget: max(c.CapBytes, 0), prefix: HostRefPrefix(c.Name, c.Generation, c.SalvageID)}
	out := Verification{RefsPrefix: v.prefix}
	for _, r := range c.Repos {
		if err := checkComponent("the repo dir", r.Dir); err != nil {
			return Verification{}, err
		}
	}
	for _, r := range c.Repos {
		res, err := v.repo(ctx, r)
		if err != nil {
			return Verification{}, fmt.Errorf("the salvage of the repo %s: %w", r.Dir, err)
		}
		out.Repos = append(out.Repos, res)
	}
	out.Skipped, out.SecretBearing = v.items, v.secret
	return out, nil
}

// checkComponent returns an error for a name that cannot be one
// component of a ref name.
func checkComponent(what, name string) error {
	if name == "" || strings.Contains(name, "/") || CheckRef("refs/heads/"+name) != nil {
		return fmt.Errorf("%s %q cannot be a component of a ref name", what, clip(name))
	}
	return nil
}

// verifier is one run of Verify.
type verifier struct {
	c      VerifyConfig
	budget int64
	prefix string
	items  []Item
	secret bool
}

// skip records an item of the repo dir.
func (v *verifier) skip(dir, item, reason string, err error) {
	i := Item{Repo: dir, Item: termsafe.Escape(item), Reason: reason}
	if err != nil {
		i.Detail = termsafe.Escape(err.Error())
	}
	v.items = append(v.items, i)
}

// repo verifies and imports the salvage of the repo r.
func (v *verifier) repo(ctx context.Context, r VerifyRepo) (RepoResult, error) {
	res := RepoResult{Dir: r.Dir}
	root, err := openSalvageDir(r.Memory, v.c.Generation, v.c.SalvageID)
	if errors.Is(err, fs.ErrNotExist) {
		v.skip(r.Dir, "the salvage dir", ReasonRepoMissing, errors.New("absent"))
		return res, nil
	}
	if err != nil {
		return res, err
	}
	defer func() { _ = root.Close() }()
	data, err := readManifest(root)
	if errors.Is(err, fs.ErrNotExist) {
		// A salvage dir without its manifest, which an interrupted
		// salvage leaves, is ignored and kept (REQ-0708).
		v.skip(r.Dir, ManifestFile, ReasonRepoMissing, errors.New("absent"))
		return res, nil
	}
	if err != nil {
		return res, err
	}
	m, err := DecodeManifest(data)
	if errors.Is(err, ErrManifest) {
		v.skip(r.Dir, ManifestFile, ReasonRepoMissing, err)
		return res, nil
	}
	if err != nil {
		return res, err
	}
	if m.SalvageID != v.c.SalvageID {
		return res, SalvageIDMismatch.New(fmt.Sprintf("the manifest of the repo %s names the salvage %q, not %s, the id romeu passed",
			r.Dir, termsafe.Escape(clip(m.SalvageID)), v.c.SalvageID))
	}
	if m.Generation != v.c.Generation || m.Repo != r.Dir {
		v.skip(r.Dir, ManifestFile, ReasonRepoMissing, fmt.Errorf("it names the generation %q and the repo %q",
			clip(m.Generation), clip(m.Repo)))
		return res, nil
	}
	if err := CheckPaths(m); err != nil {
		return res, err
	}
	sum := sha256.Sum256(data)
	res.Manifest, res.ManifestSHA256 = m, hex.EncodeToString(sum[:])
	for _, s := range m.Skipped {
		v.items = append(v.items, Item{Repo: r.Dir, Item: termsafe.Escape(s.Item), Reason: s.Reason, Detail: termsafe.Escape(s.Detail)})
	}
	for _, ref := range m.Refs {
		if !slices.ContainsFunc(m.Bundles, func(b Bundle) bool { return b.Nested == ref.Nested }) {
			v.skip(r.Dir, ref.Ref, ReasonPayloadMismatch, fmt.Errorf("no bundle holds the nested repository %q", clip(ref.Nested)))
		}
	}
	nested := 0
	for _, b := range m.Bundles {
		if b.Nested != "" {
			nested++
		}
		if !v.take(r.Dir, b.File, b.Size) {
			continue
		}
		imported, err := v.importBundle(ctx, root, r, m, b, nested)
		if err != nil {
			return res, err
		}
		res.Imported = append(res.Imported, imported...)
	}
	for _, f := range m.Files {
		if !v.take(r.Dir, f.Path, f.Size) {
			continue
		}
		err := readPayload(root, f.Path, f.Size, f.SHA256, nil)
		if errors.Is(err, errMismatch) {
			v.skip(r.Dir, f.Path, ReasonPayloadMismatch, err)
			continue
		}
		if err != nil {
			return res, err
		}
		v.secret = true
	}
	return res, nil
}

// take reports whether the payload p of size bytes fits in what is left
// of the cap, and takes it; one that does not is over-cap and is not
// read (REQ-0253).
func (v *verifier) take(dir, p string, size int64) bool {
	if size > v.budget {
		v.skip(dir, p, ReasonOverCap, fmt.Errorf("%d bytes pass what is left of salvage.capBytes, %d", size, v.budget))
		return false
	}
	v.budget -= size
	return true
}

// readManifest returns the bytes of the manifest of the salvage dir
// root, at most MaxManifestSize, read as readPayload reads; an absent
// manifest is fs.ErrNotExist.
func readManifest(root *os.Root) ([]byte, error) {
	f, err := openRegular(root, ManifestFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, MaxManifestSize+1))
}
