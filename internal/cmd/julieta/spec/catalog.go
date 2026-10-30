// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// lockFetcher returns the mise.lock of the repo at url as it is at the
// tip of its branch ref on origin, the default branch when ref is
// empty, and whether that commit holds one. A lock that is no regular
// file within the cap (a link, a directory, over the cap) is an error
// that wraps gitsafe.ErrFileRefused.
type lockFetcher func(ctx context.Context, url, ref string) (data []byte, found bool, err error)

// gitFetcher returns the lockFetcher of julieta: a shallow fetch of the
// one commit at the tip of the ref, through gitsafe in sandbox mode
// (05 5.1), into a temporary directory it removes, and the lock read
// from that commit with git cat-file under the file cap of mise, never
// from a work tree (gitsafe.Runner.TipFile).
func gitFetcher(g *gitsafe.Runner) lockFetcher {
	return func(ctx context.Context, url, ref string) ([]byte, bool, error) {
		work, err := os.MkdirTemp("", "julieta-spec-")
		if err != nil {
			return nil, false, err
		}
		defer func() { _ = os.RemoveAll(work) }()
		return g.TipFile(ctx, work, url, ref, "mise.lock", mise.MaxFileSize)
	}
}

// checkCatalog is --catalog for the spec p of file (REQ-0422,
// REQ-0639): it reads the mise.lock of each repo at its ref and looks
// each entry's backend:tool up in the embedded catalog cat, then in the
// spec's egress.tools, as romeu sync does (07 7.5, step 2): one found
// in neither is the finding catalog-unknown-tool. Each artifact host of
// the lock that the catalog does not produce for these tools, nor its
// meta and base sets, is the warning lock-host-gated, since sync gates
// it (step 3), and so is an artifact URL with no host it can read. A
// lock that does not parse, or that is no regular file within the cap
// (gitsafe.ErrFileRefused), is the finding lock-unparsed: a property of
// the repo, not of the network. The domains found are added to eg. A
// repo whose lock cannot be fetched otherwise (git fails or runs past
// repoTimeout, a redirect, the ref or a default branch missing) stops
// the command with fetch-failed (RJ-360), so nothing passes on a lock
// it did not read.
func checkCatalog(ctx context.Context, fetch lockFetcher, cat catalog.Catalog, file string, p projectspec.Project, eg egressSet) ([]finding, []setup.Warning, error) {
	produced := egressSet{}
	produced.add(cat.Meta()...)
	produced.add(cat.Base()...)
	var findings []finding
	var hosts []string
	var warnings []setup.Warning
	for i, r := range p.Repos {
		at := fmt.Sprintf("repos[%d]", i)
		repoCtx, cancel := context.WithTimeout(ctx, repoTimeout)
		data, found, err := fetch(repoCtx, r.URL, r.Ref)
		cancel()
		if errors.Is(err, gitsafe.ErrFileRefused) {
			findings = append(findings, finding{file: file, path: at, rule: "lock-unparsed",
				message: fmt.Sprintf("the mise.lock of %s at %s is no regular file within the cap of %d bytes: %v", r.URL, refName(r.Ref), mise.MaxFileSize, err)})
			continue
		}
		if err != nil {
			return nil, nil, FetchFailed.Wrap(err, "julieta spec validate --catalog could not read the mise.lock of "+
				termsafe.Escape(r.URL)+" at "+termsafe.Escape(refName(r.Ref))+" from origin: "+termsafe.Escape(err.Error()),
				cli.Detail{Key: "url", Value: termsafe.Escape(r.URL)}, cli.Detail{Key: "ref", Value: termsafe.Escape(refName(r.Ref))})
		}
		if !found {
			continue
		}
		lock, err := mise.ParseLock(data)
		if err != nil {
			findings = append(findings, finding{file: file, path: at, rule: "lock-unparsed",
				message: fmt.Sprintf("the mise.lock of %s does not parse: %v", r.URL, err)})
			continue
		}
		var unknown []string
		for _, key := range slices.Sorted(maps.Keys(lock.Tools)) {
			for _, e := range lock.Tools[key] {
				doms, ok := cat.Tool(e.Backend)
				if ok {
					produced.add(doms...)
					eg.add(doms...)
					continue
				}
				if p.Egress != nil {
					if doms, ok := p.Egress.Tools[e.Backend]; ok {
						eg.add(doms...)
						continue
					}
				}
				if !slices.Contains(unknown, e.Backend) {
					unknown = append(unknown, e.Backend)
				}
			}
		}
		for _, tool := range unknown {
			findings = append(findings, finding{file: file, path: at, rule: "catalog-unknown-tool", message: fmt.Sprintf(
				"%s in the mise.lock of %s is neither in the embedded egress catalog nor in egress.tools of the spec: "+
					"add it to the catalog (internal/catalog/egress/) or declare it in egress.tools", tool, r.URL)})
		}
		for _, key := range slices.Sorted(maps.Keys(lock.Tools)) {
			for _, e := range lock.Tools[key] {
				for _, platform := range e.PlatformNames() {
					raw := e.Platforms[platform].URL
					h := hostOf(raw)
					switch {
					case raw != "" && h == "":
						warnings = append(warnings, setup.Warning{Slug: lockHostGated, Message: fmt.Sprintf(
							"%s: the mise.lock of %s records an artifact URL for %s, platform %s, with no host julieta can read; romeu sync refuses or gates it",
							file, r.URL, key, platform)})
					case h != "" && !slices.Contains(hosts, h):
						hosts = append(hosts, h)
					}
				}
			}
		}
	}
	slices.Sort(hosts)
	for _, h := range hosts {
		eg.add(h)
		if !produced[h] {
			warnings = append(warnings, setup.Warning{Slug: lockHostGated, Message: file + ": " + h +
				" is an artifact host of a mise.lock that the embedded egress catalog does not produce; romeu sync gates it"})
		}
	}
	return findings, warnings, nil
}

// hostOf returns the lowercase host of an artifact URL of a lock, or ""
// for one with none. romeu sync refuses a host that is no egress
// domain (lock-host-invalid); the size measured here does not need it.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// The deadlines of --catalog. No section of the spec names one, so they
// are taken from the subprocess classes of 04 4.1 (internal/procgroup):
// a repo gets the limit of one julieta git call, JulietaGit, for its
// whole read (init, ls-remote, fetch and the reads of the file), and
// the whole check gets the limit of julieta's longest command,
// JulietaSetup, which also reaches every repo of a project.
var (
	repoTimeout    = procgroup.JulietaGit.Timeout()
	catalogTimeout = procgroup.JulietaSetup.Timeout()
)

// lockHostGated is the slug of the warning of a lock host the catalog
// does not produce (04 4.1, Warnings).
const lockHostGated = "lock-host-gated"

// refName returns the name of the ref of a repo, for a message.
func refName(ref string) string {
	if ref == "" {
		return "its default branch"
	}
	return ref
}
