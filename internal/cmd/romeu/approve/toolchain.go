// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// Catalog is the decoded catalog (03 3.12, the kind catalog): the input
// of the catalog digest gate 1 compares, and the form toolchain.json
// keeps so that the next gate 1 can print a diff (REQ-0228). Domains
// holds each domain the meta set, the base set or a tool uses, with the
// flag catalog.Catalog.Gated gives it: a domain the catalog declares
// and nothing uses has no consumer, and an undeclared one, which the
// check tools/ci catalog refuses, reads as gated.
type Catalog struct {
	// Meta are the domains of mise itself, sorted.
	Meta []string `json:"meta"`
	// Base are the domains of git over HTTPS, sorted.
	Base []string `json:"base"`
	// Domains are the upload flags, by domain.
	Domains map[string]Domain `json:"domains"`
	// Tools are the domains each backend:tool needs, sorted.
	Tools map[string][]string `json:"tools"`
}

// Domain is the entry of one domain of the decoded catalog.
type Domain struct {
	// Upload is the domain's upload flag.
	Upload bool `json:"upload"`
}

// CanonKind declares the one kind of the decoded catalog.
func (Catalog) CanonKind() canon.CatalogKind { return canon.Catalog }

// DecodeCatalog returns the decoded form of c. Every list is non-nil,
// so a tool that needs no domain is [] in the digest and the record,
// never null.
func DecodeCatalog(c catalog.Catalog) Catalog {
	out := Catalog{
		Meta:    orEmpty(c.Meta()),
		Base:    orEmpty(c.Base()),
		Domains: map[string]Domain{},
		Tools:   map[string][]string{},
	}
	used := slices.Concat(out.Meta, out.Base)
	for _, k := range c.Tools() {
		d, _ := c.Tool(k)
		out.Tools[k] = orEmpty(d)
		used = append(used, d...)
	}
	for _, d := range used {
		out.Domains[d] = Domain{Upload: c.Gated(d)}
	}
	return out
}

// orEmpty returns list, or an empty list for nil.
func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// recordCatalog returns c as toolchain.json holds it.
func recordCatalog(c Catalog) (state.Catalog, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m state.Catalog
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// recordedCatalog reads the catalog a record holds back into its decoded
// form, for the diff only: its digest is the record's catalogDigest and
// is never recomputed from the map, whose numbers went through float64.
func recordedCatalog(m state.Catalog) (Catalog, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return Catalog{}, err
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

// Live is the toolchain this machine runs now: the members gate 1
// records (03 3.8, REQ-0228) and compares (REQ-0062).
type Live struct {
	// Sbx is the live sbx version, as the record writes it.
	Sbx string
	// Romeu is this binary: its version and the sha256 of its file.
	Romeu state.Binary
	// Julieta is the julieta this binary embeds.
	Julieta state.JulietaBinary
	// Catalog is the embedded catalog, decoded.
	Catalog Catalog
	// CatalogDigest is the digest of Catalog.
	CatalogDigest canon.Sum[canon.CatalogKind]
}

// The seams of Measure; a test replaces them.
var (
	// executable returns the path of the running binary.
	executable = os.Executable
	// embeddedCatalog returns the catalog this binary embeds.
	embeddedCatalog = catalog.Embedded
	// buildInfo returns the build information of the binary.
	buildInfo = debug.ReadBuildInfo
)

// placeholderSHA256 is the sha256 of the julieta a source build embeds:
// the empty placeholder of 02 2.1 (REQ-0130).
var placeholderSHA256 = state.SHA256(sha256.Sum256(nil))

// embeddedJulieta returns the julieta this binary embeds. No package
// reads the embedded julieta yet (tools/release build maps it with an
// overlay, and the package that consumes it lands with sync, T043), so
// every build holds the placeholder of a source build (REQ-0130): its
// sha256 is the one of the empty file, and its version and protocol are
// romeu's own, since one build stamps both binaries and the two
// protocol constants are held equal (cmd/romeu, TestProtocolIsJulietas).
// The embed package replaces this function.
var embeddedJulieta = func(v cli.Version) state.JulietaBinary {
	return state.JulietaBinary{Version: Version(v), Protocol: v.Protocol, SHA256: placeholderSHA256}
}

// develVersion is the version stamp of a build that tools/release did
// not stamp (cmd/romeu, main.version).
const develVersion = "devel"

// Version returns the version gate 1 records for the stamp v: the
// stamp's version, or for a build that was not stamped, a dev build,
// v0.0.0-dev+<commit> (01 1.4, REQ-0065), the commit taken from the
// stamp or from the VCS information the go command wrote into the
// binary, and "unknown" when neither has one. Two dev builds of one
// commit share the version; their sha256 tells them apart, so a dev
// build is acknowledged once per build per machine.
func Version(v cli.Version) string {
	if v.Version != "" && v.Version != develVersion {
		return v.Version
	}
	commit := v.Commit
	if commit == "" || commit == "unknown" {
		commit = "unknown"
		if bi, ok := buildInfo(); ok {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" && s.Value != "" {
					commit = s.Value
				}
			}
		}
	}
	return "v0.0.0-dev+" + commit
}

// Measure returns the live toolchain: the sbx version the floor check
// read (sbxdrv.Runner.RequireFloor), this binary's version and sha256,
// the julieta it embeds and the digest of its embedded catalog. It
// reads no state and calls no sbx. A binary that cannot read itself,
// or a catalog that does not build, is the error internal (RJ-348): no
// healthy build reaches either.
func Measure(sbx sbxdrv.Release, v cli.Version) (Live, error) {
	sum, err := executableSHA256()
	if err != nil {
		return Live{}, cli.Internal.Wrap(err, "gate 1: romeu cannot read its own file")
	}
	c, err := embeddedCatalog()
	if err != nil {
		return Live{}, cli.Internal.Wrap(err, "gate 1: the embedded catalog does not build")
	}
	decoded := DecodeCatalog(c)
	digest, err := canon.Digest(canon.Catalog, decoded)
	if err != nil {
		return Live{}, cli.Internal.Wrap(err, "gate 1: the catalog digest")
	}
	return Live{
		Sbx:           sbx.String(),
		Romeu:         state.Binary{Version: Version(v), SHA256: sum},
		Julieta:       embeddedJulieta(v),
		Catalog:       decoded,
		CatalogDigest: digest,
	}, nil
}

// executableSHA256 returns the sha256 of the file of the running binary.
func executableSHA256() (state.SHA256, error) {
	path, err := executable()
	if err != nil {
		return state.SHA256{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return state.SHA256{}, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return state.SHA256{}, err
	}
	return state.SHA256(h.Sum(nil)), nil
}

// Change is one member of the toolchain whose live value is not the
// recorded one.
type Change struct {
	// Member names it by its path in toolchain.json, such as
	// "sbxVersion" or "romeu.sha256".
	Member string
	// Recorded and Live are the two values, as the record writes them;
	// Recorded is "" when no toolchain was acknowledged.
	Recorded, Live string
}

// Changes returns the members of the toolchain whose live value differs
// from the record rec, in the order of 03 3.8; a nil rec, no
// acknowledgement, changes every member. It is gate 1's comparison
// (REQ-0062): the sbx version, romeu's version and sha256, the embedded
// julieta's version, protocol and sha256, and the catalog digest.
// status and doctor print what it returns (they report a mismatch and
// do not stop); Check makes it the error of every other command.
//
//romeu:invariant I7 guard
func Changes(rec *state.Toolchain, live Live) []Change {
	var r state.Toolchain
	if rec != nil {
		r = *rec
	}
	pairs := []Change{
		{"sbxVersion", r.SbxVersion, live.Sbx},
		{"romeu.version", r.Romeu.Version, live.Romeu.Version},
		{"romeu.sha256", sumText(r.Romeu.SHA256), live.Romeu.SHA256.String()},
		{"julieta.version", r.Julieta.Version, live.Julieta.Version},
		{"julieta.protocol", protocolText(r.Julieta.Protocol), protocolText(live.Julieta.Protocol)},
		{"julieta.sha256", sumText(r.Julieta.SHA256), live.Julieta.SHA256.String()},
		{"catalogDigest", digestText(r.CatalogDigest), live.CatalogDigest.String()},
	}
	var out []Change
	for _, p := range pairs {
		if rec == nil || p.Recorded != p.Live {
			out = append(out, p)
		}
	}
	return out
}

// sumText is the text of a sha256, "" for none.
func sumText(s state.SHA256) string {
	if s == (state.SHA256{}) {
		return ""
	}
	return s.String()
}

// digestText is the text of a catalog digest, "" for none.
func digestText(s canon.Sum[canon.CatalogKind]) string {
	if s == (canon.Sum[canon.CatalogKind]{}) {
		return ""
	}
	return s.String()
}

// protocolText is the text of a protocol number, "" for none.
func protocolText(p int) string {
	if p == 0 {
		return ""
	}
	return fmt.Sprint(p)
}

// acknowledge writes live into the record rec, acknowledged at now:
// what gate 1 records once the operator approves (REQ-0060).
func acknowledge(rec *state.Toolchain, live Live, now time.Time) error {
	c, err := recordCatalog(live.Catalog)
	if err != nil {
		return err
	}
	rec.Schema = state.ToolchainField
	rec.SbxVersion = live.Sbx
	rec.Romeu = live.Romeu
	rec.Julieta = live.Julieta
	rec.CatalogDigest = live.CatalogDigest
	rec.Catalog = c
	rec.AcknowledgedAt = now.UTC()
	return nil
}

// Load returns the record of gate 1 in st, or nil when none was
// written. A record that does not decode is record-unreadable (RJ-334).
func Load(st *state.Store) (*state.Toolchain, error) {
	rec, err := st.LoadToolchain()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return rec, err
}

// Check is gate 1's check, the one the preflight (01 1.5, step 1) and
// sync call (REQ-0062, REQ-0075): nil when the record in st holds the
// live toolchain, and otherwise the error toolchain-approval-required
// (RJ-302, exit 3), whose details name each member that changed with
// its recorded and live values. No record is a change of every member;
// a record that does not decode is record-unreadable (RJ-334). It reads
// and writes nothing else.
//
//romeu:invariant I7 guard
func Check(st *state.Store, live Live) error {
	rec, err := Load(st)
	if err != nil {
		return err
	}
	changes := Changes(rec, live)
	if len(changes) == 0 {
		return nil
	}
	if rec == nil {
		return ToolchainApprovalRequired.New("no toolchain is acknowledged on this machine (gate 1)")
	}
	var details []cli.Detail
	var names []string
	for _, c := range changes {
		names = append(names, c.Member)
		details = append(details, cli.Detail{Key: c.Member, Value: c.Recorded + " -> " + c.Live})
	}
	return ToolchainApprovalRequired.New("the toolchain changed since it was acknowledged (gate 1): "+strings.Join(names, ", "), details...)
}

// sortedKeys returns the keys of m, sorted.
func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }
