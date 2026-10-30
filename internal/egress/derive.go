// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The files of a repo this package reads (07 7.5, step 1).
const (
	configFile = "mise.toml"
	lockFile   = "mise.lock"
)

// ErrUnreadable is wrapped by a Reader for a file it refuses to give: one
// over the cap of MaxFileSize, or one that is not a regular file. Derive
// reports it as lock-unparsed. A refusal of the repository, the commit or
// the name is not this error: it is a defect of the caller.
var ErrUnreadable = errors.New("egress: a file of the repo cannot be read as one")

// Reader returns the bytes of the file name of a repo, a path from the
// root of its tree with forward slashes, at the commit egress derives
// from, and whether the commit holds the file. A file it refuses (over
// MaxFileSize, a link) is an error that wraps ErrUnreadable.
type Reader func(ctx context.Context, name string) (data []byte, found bool, err error)

// GitReader returns the Reader of the commit in the repository in dir:
// git cat-file at the full object id commit, through g, under the file
// cap of 07 7.5, step 1. It reads no work tree and starts no mise.
func GitReader(g *gitsafe.Runner, dir, commit string) Reader {
	return func(ctx context.Context, name string) ([]byte, bool, error) {
		data, found, err := g.CommitFile(ctx, dir, commit, name, MaxFileSize)
		if errors.Is(err, gitsafe.ErrFileRefused) {
			err = fmt.Errorf("%w: %w", ErrUnreadable, err)
		}
		return data, found, err
	}
}

// Repo is one repo of the project: its dir and URL, which the errors
// name, and the reader of its files at its egressCommit.
type Repo struct {
	Dir, URL string
	Read     Reader
}

// Input is what Derive reads (07 7.5): the repos, the egress of the
// project spec, the network domains the kits declare, and the catalog.
type Input struct {
	Repos []Repo
	// Spec is egress of the project spec, nil when it has none.
	Spec *spec.Egress
	// Kit are the network domains of the normalized capability sets of
	// the kits (06 6.1), step 6.
	Kit     []string
	Catalog catalog.Catalog
}

// Reason says why a domain is gated. A domain with several reasons keeps
// the first of this list:
//
//  1. ReasonUpload: the catalog, the base set or the meta set produced it,
//     and step 7 moves it because it can upload or has no upload entry.
//  2. ReasonKit: a kit declared it and no catalog set did, and step 7
//     moves it for the same cause.
//  3. ReasonExtra: egress.extra of the spec names it.
//  4. ReasonSpecTool: an egress.tools entry of the spec names it.
//  5. ReasonLockHost: it is the host of an artifact URL of a lock that
//     the catalog did not produce.
//
// The catalog does not say whether a domain can upload or only lacks an
// entry, and the approval it waits for is the same, so a domain that step
// 7 moves has one reason for both.
type Reason string

// The reasons a domain is gated (01 1.3, egressGated).
const (
	// ReasonUpload is a domain a catalog set produced that can take data
	// out, or that the catalog has no upload entry for (07 7.5, step 7).
	ReasonUpload Reason = "upload"
	// ReasonKit is a domain a kit declared that step 7 moves, and that no
	// catalog set produced (06 6.1, I10).
	ReasonKit Reason = "kit"
	// ReasonExtra is a domain of egress.extra of the spec (step 5).
	ReasonExtra Reason = "extra"
	// ReasonSpecTool is a domain of an egress.tools entry of the spec
	// (step 2).
	ReasonSpecTool Reason = "spec-tool"
	// ReasonLockHost is a host of an artifact URL of a lock that the
	// catalog did not produce (step 3).
	ReasonLockHost Reason = "lock-host"
)

// reasonOrder ranks the reasons: the lowest wins.
var reasonOrder = []Reason{ReasonUpload, ReasonKit, ReasonExtra, ReasonSpecTool, ReasonLockHost}

// Gate is a gated domain and why.
type Gate struct {
	Domain string
	Reason Reason
}

// HTTPSPort is the port of a rule for a domain the catalog, the base or
// meta set, egress.extra or egress.tools produced, and for a lock host
// on https (07 7.5, REQ-0634).
const HTTPSPort = 443

// HTTPPort is the port of a rule for a lock host on http.
const HTTPPort = 80

// Rule is a network rule romeu applies to a sandbox: one domain and one
// port, never every port of the domain (07 7.5, REQ-0634).
type Rule struct {
	Domain string
	Port   int
}

// Result is the derived egress: the domains that are allowed at once and
// the domains that wait for an approval, each sorted, and disjoint, and
// the rules romeu applies for them.
type Result struct {
	Auto  []string
	Gated []Gate
	// Rules are the rules of the domains of Auto and Gated that a source
	// of romeu produced (steps 2 to 5), sorted by domain then port; the
	// caller applies those of Auto and of the approved Gated (REQ-0634).
	// A domain only the descriptors declared (step 6) has no rule: sbx
	// applies it from the descriptor with its port, and romeu does not
	// apply, remove or check it (REQ-1316). It stays in Auto or Gated for
	// the gate's upload flag, the digest and the manifest's applied set.
	Rules []Rule
}

// Effective returns the sorted union of Auto and Gated, the input of the
// egress digest (step 8).
func (r Result) Effective() []string {
	out := slices.Clone(r.Auto)
	for _, g := range r.Gated {
		out = append(out, g.Domain)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// EffectiveSet is the sorted effective domain union as the egress
// digest takes it (03 3.12).
type EffectiveSet []string

// CanonKind returns the kind of the egress digest.
func (EffectiveSet) CanonKind() canon.EgressKind { return canon.Egress }

// Digest returns effectiveDigest of render.json (step 8): the egress
// digest of the sorted union of Auto and Gated.
func (r Result) Digest() (canon.Sum[canon.EgressKind], error) {
	return canon.Digest(canon.Egress, EffectiveSet(r.Effective()))
}

// Derive is the egress derivation of 07 7.5. It reads each repo's
// mise.lock through its Reader and starts no process of its own. It fails
// closed and gives no partial result: a mise.toml with no mise.lock is
// lock-missing (RJ-340), a lock that is not the pinned layout is
// lock-unparsed (RJ-325), a backend:tool in no catalog entry and no
// egress.tools entry is catalog-unknown-tool (RJ-327), and a lock host
// that fails the egress domain rule is lock-host-invalid (RJ-326). The
// repos are judged in order, and the first that fails is the error. A
// domain that the catalog can upload to, or has no entry for, is gated
// wherever it comes from; one that is already allowed by the catalog stays
// allowed when egress.extra repeats it.
//
// What the lock names is not always what mise contacts: for an http: tool
// the url of mise.toml beats the one of the lock in locked mode (the audit
// of 2026-10-22). Derive reads the lock alone (07 7.5, deliberate
// deviation), so the host it allows can differ from the host mise
// contacts. That fails closed: the host of the lock is gated, an http:
// tool being in no catalog entry unless a release ships one, and the host
// mise contacts is in no set, so the proxy of the sandbox blocks it and
// the install error names it (REQ-0637). It never allows more than the
// lock names.
func Derive(ctx context.Context, in Input) (Result, error) {
	d := derivation{in: in, auto: map[string]bool{}, produced: map[string]bool{}, kit: map[string]bool{}, gated: map[string]Reason{}, rules: map[Rule]bool{}}
	for _, repo := range in.Repos {
		if err := d.repo(ctx, repo); err != nil {
			return Result{}, err
		}
	}
	for _, set := range [][]string{in.Catalog.Meta(), in.Catalog.Base()} {
		for _, dom := range set {
			d.produce(dom)
		}
	}
	if in.Spec != nil {
		for _, dom := range in.Spec.Extra {
			if err := d.gate(dom, ReasonExtra); err != nil {
				return Result{}, fmt.Errorf("egress.extra: %w", err)
			}
		}
	}
	for _, dom := range in.Kit {
		if err := spec.ValidDomain(dom); err != nil {
			return Result{}, fmt.Errorf("a kit domain: %w", err)
		}
		d.auto[dom] = true
		d.kit[dom] = true
	}
	for _, h := range d.hosts {
		if !d.produced[h] {
			d.gated[h] = firstReason(d.gated[h], ReasonLockHost)
		}
	}
	return d.result(), nil
}

// derivation is the state of one Derive.
type derivation struct {
	in       Input
	auto     map[string]bool   // domains allowed unless step 7 gates them
	produced map[string]bool   // domains the catalog produced
	kit      map[string]bool   // domains a kit declared
	gated    map[string]Reason // domains gated by their source
	hosts    []string          // lock hosts, judged once every repo is read
	rules    map[Rule]bool     // the rules of the domains romeu produced
}

// produce adds a domain the catalog produced.
func (d *derivation) produce(dom string) {
	d.auto[dom] = true
	d.produced[dom] = true
	d.rules[Rule{Domain: dom, Port: HTTPSPort}] = true
}

// gate adds a domain of the spec to the gated set.
func (d *derivation) gate(dom string, r Reason) error {
	if err := spec.ValidDomain(dom); err != nil {
		return err
	}
	d.gated[dom] = firstReason(d.gated[dom], r)
	d.rules[Rule{Domain: dom, Port: HTTPSPort}] = true
	return nil
}

// firstReason returns the stronger of two reasons; the empty reason is
// the weakest.
func firstReason(a, b Reason) Reason {
	if a == "" {
		return b
	}
	if slices.Index(reasonOrder, b) < slices.Index(reasonOrder, a) {
		return b
	}
	return a
}

// result applies step 7 and sorts.
func (d *derivation) result() Result {
	gated := map[string]Reason{}
	res := Result{Auto: []string{}, Gated: []Gate{}, Rules: slices.AppendSeq([]Rule{}, maps.Keys(d.rules))}
	for dom := range d.auto {
		switch {
		case d.in.Catalog.Gated(dom) && d.kit[dom] && !d.produced[dom]:
			gated[dom] = ReasonKit
		case d.in.Catalog.Gated(dom):
			gated[dom] = ReasonUpload
		default:
			res.Auto = append(res.Auto, dom)
		}
	}
	for dom, r := range d.gated {
		switch {
		case gated[dom] != "":
			// The catalog, base, meta or kit produced it and step 7 moved
			// it: that reason stays.
		case d.auto[dom] && !d.in.Catalog.Gated(dom):
			// Already allowed by its source: the spec repeats it.
		default:
			gated[dom] = r
		}
	}
	slices.Sort(res.Auto)
	for dom, r := range gated {
		res.Gated = append(res.Gated, Gate{Domain: dom, Reason: r})
	}
	slices.SortFunc(res.Gated, func(a, b Gate) int { return strings.Compare(a.Domain, b.Domain) })
	slices.SortFunc(res.Rules, func(a, b Rule) int {
		return cmp.Or(strings.Compare(a.Domain, b.Domain), cmp.Compare(a.Port, b.Port))
	})
	return res
}

// repo reads the files of one repo at its commit and takes its tools and
// hosts (07 7.5, steps 1 to 3).
func (d *derivation) repo(ctx context.Context, repo Repo) error {
	dir := termsafe.Escape(repo.Dir)
	unparsed := func(err error) error {
		e := cli.LockUnparsed.Wrap(errors.New(termsafe.Escape(err.Error())),
			fmt.Sprintf("the mise files of %s (%s) cannot be read as the layout of the pinned mise: relock with julieta lock", dir, termsafe.Escape(repo.URL)))
		e.Fill = map[string]string{"<dir>": repo.Dir}
		return e
	}
	_, hasConfig, err := repo.Read(ctx, configFile)
	if err != nil {
		return readError(err, unparsed)
	}
	data, hasLock, err := repo.Read(ctx, lockFile)
	if err != nil {
		return readError(err, unparsed)
	}
	switch {
	case !hasLock && hasConfig:
		e := cli.LockMissing.New(fmt.Sprintf("%s (%s) has a mise.toml and no mise.lock at the commit egress derives from", dir, termsafe.Escape(repo.URL)))
		e.Fill = map[string]string{"<dir>": repo.Dir}
		return e
	case !hasLock:
		return nil // no mise files: the repo contributes nothing
	}
	lock, err := ParseLock(data)
	if err != nil {
		return unparsed(err)
	}
	var unknown []string
	for _, key := range slices.Sorted(maps.Keys(lock.Tools)) {
		for _, e := range lock.Tools[key] {
			if doms, ok := d.in.Catalog.Tool(e.Backend); ok {
				for _, dom := range doms {
					d.produce(dom)
				}
			} else if doms, ok := d.spec(e.Backend); ok {
				for _, dom := range doms {
					if err := d.gate(dom, ReasonSpecTool); err != nil {
						return fmt.Errorf("egress.tools[%q]: %w", e.Backend, err)
					}
				}
			} else if !slices.Contains(unknown, e.Backend) {
				unknown = append(unknown, e.Backend)
			}
		}
	}
	if len(unknown) > 0 {
		return unknownTools(dir, unknown)
	}
	for _, key := range slices.Sorted(maps.Keys(lock.Tools)) {
		for _, e := range lock.Tools[key] {
			for _, platform := range e.PlatformNames() {
				host, port, err := hostOf(e.URLs[platform])
				if err != nil {
					e := LockHostInvalid.Wrap(errors.New(termsafe.Escape(err.Error())),
						fmt.Sprintf("the mise.lock of %s (%s) records an artifact host for the tool %q, version %s, platform %s, that is not an egress domain",
							dir, termsafe.Escape(repo.URL), termsafe.Escape(key), termsafe.Escape(e.Version), termsafe.Escape(platform)))
					e.Fill = map[string]string{"<dir>": repo.Dir}
					return e
				}
				d.hosts = append(d.hosts, host)
				d.rules[Rule{Domain: host, Port: port}] = true
			}
		}
	}
	return nil
}

// spec returns the domains egress.tools gives a backend:tool.
func (d *derivation) spec(backend string) ([]string, bool) {
	if d.in.Spec == nil {
		return nil, false
	}
	doms, ok := d.in.Spec.Tools[backend]
	return doms, ok
}

// readError turns the error of a Reader into the error Derive returns.
func readError(err error, unparsed func(error) error) error {
	if errors.Is(err, ErrUnreadable) {
		return unparsed(err)
	}
	return err
}

// unknownTools is catalog-unknown-tool for the backends of a repo that no
// catalog entry and no egress.tools entry has.
func unknownTools(dir string, unknown []string) error {
	slices.Sort(unknown)
	var names []string
	var details []cli.Detail
	for _, u := range unknown {
		names = append(names, termsafe.Escape(u))
		details = append(details, cli.Detail{Key: termsafe.Escape(u), Value: dir})
	}
	return CatalogUnknownTool.New(fmt.Sprintf("the mise.lock of %s holds %s, which no entry of the egress catalog and no egress.tools entry of the spec has: "+
		"open a pull request that adds the tool to the catalog, or declare it in egress.tools of the project spec", dir, strings.Join(names, ", ")), details...)
}

// hostOf returns the host of an artifact URL of a lock, which must pass
// the egress domain rule of 03 3.1, and the port of its scheme: 443 for
// https and 80 for http. A URL that writes a port fails the domain rule.
// The URL is judged as written: only
// printable ASCII without a backslash, since a backslash is a path
// separator to some parsers and not to Go's, and no user information, so
// the host Derive allows is the one every parser reads.
func hostOf(raw string) (string, int, error) {
	if i := strings.IndexFunc(raw, func(r rune) bool { return r <= ' ' || r >= 0x7f || r == '\\' }); i >= 0 {
		return "", 0, fmt.Errorf("the url holds a character that is not printable ASCII, or a backslash, at byte %d", i)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, errors.New("the url does not parse")
	}
	port := HTTPSPort
	switch u.Scheme {
	case "https":
	case "http":
		port = HTTPPort
	default:
		return "", 0, fmt.Errorf("the url scheme is %q, not https or http", u.Scheme)
	}
	if u.User != nil {
		return "", 0, errors.New("the url holds user information")
	}
	if err := spec.ValidDomain(u.Host); err != nil {
		return "", 0, err
	}
	return u.Host, port, nil
}
