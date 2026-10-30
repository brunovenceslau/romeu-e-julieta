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
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// lockFetcher returns the mise.lock of the repo at url as it is at the
// tip of its branch ref on origin, the default branch when ref is
// empty, and whether that commit holds one. A lock it refuses to read
// (over the cap, a link) is an error that wraps gitsafe.ErrFileRefused.
type lockFetcher func(ctx context.Context, url, ref string) (data []byte, found bool, err error)

// gitFetcher returns the lockFetcher of julieta: a clone of the origin,
// through gitsafe in sandbox mode (05 5.1), into a temporary directory
// it removes, and the lock read from the commit of origin's ref with
// git cat-file under the file cap of mise, never from a work tree.
func gitFetcher(g *gitsafe.Runner) lockFetcher {
	return func(ctx context.Context, url, ref string) ([]byte, bool, error) {
		tmp, err := os.MkdirTemp("", "julieta-spec-")
		if err != nil {
			return nil, false, err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		dest := filepath.Join(tmp, "repo")
		if err := g.Clone(ctx, url, dest); err != nil {
			return nil, false, err
		}
		if ref == "" {
			if ref, err = g.DefaultBranch(ctx, dest); err != nil {
				return nil, false, err
			}
			if ref == "" {
				return nil, false, errors.New("origin names no default branch")
			}
		}
		oid, ok, err := g.RefOID(ctx, dest, "refs/remotes/origin/"+ref)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, fmt.Errorf("origin has no branch %s", ref)
		}
		return g.CommitFile(ctx, dest, oid, "mise.lock", mise.MaxFileSize)
	}
}

// checkCatalog is --catalog for the spec p of file (REQ-0422,
// REQ-0639): it reads the mise.lock of each repo at its ref and looks
// each entry's backend:tool up in the embedded catalog cat, then in the
// spec's egress.tools, as romeu sync does (07 7.5, step 2): one found
// in neither is the finding catalog-unknown-tool. Each artifact host of
// the lock that the catalog does not produce for these tools, nor its
// meta and base sets, is a note, since sync gates it (step 3). A lock
// that does not parse is the finding lock-unparsed. The domains found
// are added to eg. A repo whose lock cannot be fetched stops the
// command.
func checkCatalog(ctx context.Context, fetch lockFetcher, cat catalog.Catalog, file string, p projectspec.Project, eg egressSet) ([]finding, []string, error) {
	produced := egressSet{}
	produced.add(cat.Meta()...)
	produced.add(cat.Base()...)
	var findings []finding
	var hosts []string
	for _, r := range p.Repos {
		data, found, err := fetch(ctx, r.URL, r.Ref)
		if errors.Is(err, gitsafe.ErrFileRefused) {
			findings = append(findings, finding{rule: "lock-unparsed", text: fmt.Sprintf("%s: the mise.lock of %s cannot be read: %v", file, r.URL, err)})
			continue
		}
		if err != nil {
			return nil, nil, cli.Internal.Wrap(err, "julieta spec validate --catalog could not read the mise.lock of "+
				termsafe.Escape(r.URL)+" at "+termsafe.Escape(refName(r.Ref))+" from origin: "+termsafe.Escape(err.Error()))
		}
		if !found {
			continue
		}
		lock, err := mise.ParseLock(data)
		if err != nil {
			findings = append(findings, finding{rule: "lock-unparsed", text: fmt.Sprintf("%s: the mise.lock of %s does not parse: %v", file, r.URL, err)})
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
			findings = append(findings, finding{rule: "catalog-unknown-tool", text: fmt.Sprintf(
				"%s: %s in the mise.lock of %s is neither in the embedded egress catalog nor in egress.tools of the spec: "+
					"add it to the catalog (internal/catalog/egress/) or declare it in egress.tools", file, tool, r.URL)})
		}
		for _, key := range slices.Sorted(maps.Keys(lock.Tools)) {
			for _, e := range lock.Tools[key] {
				for _, platform := range e.PlatformNames() {
					if h := hostOf(e.Platforms[platform].URL); h != "" && !slices.Contains(hosts, h) {
						hosts = append(hosts, h)
					}
				}
			}
		}
	}
	var notes []string
	slices.Sort(hosts)
	for _, h := range hosts {
		eg.add(h)
		if !produced[h] {
			notes = append(notes, "gated "+termsafe.Escape(file)+" "+termsafe.Escape(h)+
				": a host of a mise.lock that the embedded egress catalog does not produce; romeu sync gates it")
		}
	}
	return findings, notes, nil
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

// refName returns the name of the ref of a repo, for a message.
func refName(ref string) string {
	if ref == "" {
		return "its default branch"
	}
	return ref
}
