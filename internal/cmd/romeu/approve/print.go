// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// Question is the question gate 1 asks after its text.
const Question = "acknowledge this toolchain? [y/N] "

// shortLen is the number of hex digits a digest or a sha256 is shown
// with: enough to tell two apart on the screen, which the record holds
// whole.
const shortLen = 12

// Text returns what gate 1 writes to stderr before the answer, its
// question last (01 1.4, REQ-0061): the sbx version recorded and live,
// beside the newest version with a recording set and the tested window
// of 10 10.3 (REQ-0063, REQ-0933); romeu's version and sha256; the
// julieta version change; and the catalog diff, the domains added or
// removed per backend:tool and the upload flags changed, each with the
// projects it affects. Any version lower than the recorded one is
// marked downgrade. With a record whose julieta and catalog the live
// toolchain keeps, the text is one line (REQ-0065).
//
// projects maps each project with a record to the domains romeu applied
// to its sandbox (egressApplied); a change affects the projects whose
// applied egress holds one of its domains. A change to the meta or base
// set, and one to a tool the record lists with no domain or not at all,
// affects every project, since nothing on the host tells which projects
// use that tool. The text is escaped by the CLI writer it is written to.
func Text(rec *state.Toolchain, live Live, projects map[string][]string) (string, error) {
	var r state.Toolchain
	if rec != nil {
		r = *rec
	}
	sbx := "sbx " + versionChange(r.SbxVersion, live.Sbx) + " (newest recorded " + newestRecorded() +
		"; tested window " + window(live.Sbx) + ")"
	romeu := versionChange(r.Romeu.Version, live.Romeu.Version) + ", sha256 " +
		change(short(sumText(r.Romeu.SHA256)), short(live.Romeu.SHA256.String()))
	if rec != nil && r.Julieta == live.Julieta && r.CatalogDigest == live.CatalogDigest {
		return "gate 1: " + sbx + ", romeu " + romeu + "; julieta and the catalog are unchanged; " + Question, nil
	}

	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("gate 1: the toolchain of this machine")
	line("  %s", sbx)
	line("  romeu %s", romeu)
	line("  julieta %s, protocol %s, sha256 %s",
		versionChange(r.Julieta.Version, live.Julieta.Version),
		protocolChange(r.Julieta.Protocol, live.Julieta.Protocol),
		change(short(sumText(r.Julieta.SHA256)), short(live.Julieta.SHA256.String())))
	if rec != nil && r.CatalogDigest == live.CatalogDigest {
		line("  catalog %s, unchanged", short(live.CatalogDigest.String()))
	} else {
		var old Catalog
		if rec != nil {
			var err error
			if old, err = recordedCatalog(r.Catalog); err != nil {
				return "", fmt.Errorf("gate 1: the catalog of toolchain.json: %w", err)
			}
		}
		line("  catalog %s", change(short(digestText(r.CatalogDigest)), short(live.CatalogDigest.String())))
		for _, l := range catalogDiff(old, live.Catalog, projects) {
			line("    %s", l)
		}
	}
	b.WriteString(Question)
	return b.String(), nil
}

// change returns "old -> new", "new" when they are equal, and
// "new (none recorded)" when nothing was recorded.
func change(old, new string) string {
	switch old {
	case new:
		return new
	case "":
		return new + " (none recorded)"
	}
	return old + " -> " + new
}

// versionChange is change for two versions, with downgrade after a new
// version lower than the old one.
func versionChange(old, new string) string {
	s := change(old, new)
	if lower(new, old) {
		s += " downgrade"
	}
	return s
}

// protocolChange is change for two protocol numbers, with downgrade
// after a lower one.
func protocolChange(old, new int) string {
	s := change(protocolText(old), protocolText(new))
	if old != 0 && new < old {
		s += " downgrade"
	}
	return s
}

// lower reports whether the version a is lower than b. Both are read by
// the version grammar of sbxdrv (major.minor.patch, an optional leading
// v, prerelease and build metadata); a version that does not read is
// lower than nothing, so it is never marked.
func lower(a, b string) bool {
	va, err := sbxdrv.ParseVersion([]byte(a))
	if err != nil {
		return false
	}
	vb, err := sbxdrv.ParseVersion([]byte(b))
	if err != nil {
		return false
	}
	return va.Compare(vb) < 0
}

// short cuts a hex text to shortLen digits.
func short(s string) string {
	if len(s) > shortLen {
		return s[:shortLen]
	}
	return s
}

// newestRecorded returns the newest sbx version with a recording set
// under e2e/testdata/sbx/ (sbxdrv.RecordedVersions).
func newestRecorded() string {
	return slices.MaxFunc(sbxdrv.RecordedVersions, func(a, b string) int {
		switch {
		case lower(a, b):
			return -1
		case lower(b, a):
			return 1
		}
		return strings.Compare(a, b)
	})
}

// window names the tested window of 10 10.3 (REQ-0932): the versions
// with a recording set, and whether live is one of them.
func window(live string) string {
	s := strings.Join(sbxdrv.RecordedVersions, ", ")
	if !slices.Contains(sbxdrv.RecordedVersions, live) {
		s += "; " + live + " is outside it, untested"
	}
	return s
}

// catalogDiff returns the lines of the diff from the catalog old to new:
// the meta and base sets, each tool whose domains changed, and each
// upload flag that changed, each with the projects it affects.
func catalogDiff(old, new Catalog, projects map[string][]string) []string {
	all := sortedKeys(projects)
	affected := func(domains []string) []string {
		var out []string
		for _, p := range all {
			if slices.ContainsFunc(projects[p], func(d string) bool { return slices.Contains(domains, d) }) {
				out = append(out, p)
			}
		}
		return out
	}
	var lines []string
	add := func(what string, delta string, ps []string) {
		names := "none"
		if len(ps) > 0 {
			names = strings.Join(ps, ", ")
		}
		lines = append(lines, what+": "+delta+"; projects: "+names)
	}
	for _, set := range []struct {
		name     string
		old, new []string
	}{{"meta", old.Meta, new.Meta}, {"base", old.Base, new.Base}} {
		if d := domainsDelta(set.old, set.new, new); d != "" {
			add(set.name, d, all)
		}
	}
	for _, k := range sortedKeys(mergeKeys(old.Tools, new.Tools)) {
		o, inOld := old.Tools[k]
		n, inNew := new.Tools[k]
		d := domainsDelta(o, n, new)
		what := "tool " + k
		switch {
		case !inOld:
			what += " (new)"
		case !inNew:
			what += " (removed)"
		}
		if d == "" {
			if inOld == inNew {
				continue
			}
			d = "no domain"
		}
		ps := all
		if len(o) > 0 {
			ps = affected(slices.Concat(o, n))
		}
		add(what, d, ps)
	}
	for _, dom := range sortedKeys(new.Domains) {
		o, ok := old.Domains[dom]
		if ok && o.Upload != new.Domains[dom].Upload {
			add("upload flag of "+dom, fmt.Sprintf("%t -> %t", o.Upload, new.Domains[dom].Upload), affected([]string{dom}))
		}
	}
	return lines
}

// domainsDelta returns "+ a (upload), - b" for the domains added to and
// removed from a list, an added domain marked when new gates it, or ""
// when the lists hold the same domains.
func domainsDelta(old, new []string, c Catalog) string {
	var parts []string
	for _, d := range new {
		if !slices.Contains(old, d) {
			p := "+ " + d
			if c.Domains[d].Upload {
				p += " (upload)"
			}
			parts = append(parts, p)
		}
	}
	for _, d := range old {
		if !slices.Contains(new, d) {
			parts = append(parts, "- "+d)
		}
	}
	return strings.Join(parts, ", ")
}

// mergeKeys returns a map with the keys of a and b.
func mergeKeys(a, b map[string][]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
