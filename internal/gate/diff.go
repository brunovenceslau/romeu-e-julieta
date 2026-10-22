// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Side is one side of the gate 2 diff: a widening set and what the set
// holds only by digest, so the diff can show it in full.
type Side struct {
	// Set is the widening set.
	Set Set
	// KitFiles are the files of each kit of the set, by "<source>:<id>".
	KitFiles map[string][]File
	// SecretArgv is the host-settings argv of each secret of the set,
	// by name. It is read on the candidate side only: the host settings
	// hold the current argv, and the approval its digest.
	SecretArgv map[string][]string
}

// Diff is the gate 2 diff of 01 1.4 (REQ-0071): a summary, then every
// change in full, never truncated. Its text is escaped line by line
// (termsafe, I29).
type Diff struct {
	project string
	summary []string
	body    []string
}

// PromptLine is the question Ask writes after the diff.
const PromptLine = "approve this widening? [y/N] "

// NewDiff returns the diff from the approved side to the candidate; a
// nil approved is the first approval of the project, where everything
// is added. It fails, rather than show less than the set holds, when a
// kit of a set has no files or files whose kit-tree digest is not the
// set's, when a secret of the candidate has no argv or one whose digest
// is not the set's, and when a side names a kit or a secret its set
// does not hold.
func NewDiff(approved *Side, candidate Side) (Diff, error) {
	newSet, err := checkSide(candidate, true)
	if err != nil {
		return Diff{}, fmt.Errorf("gate: the candidate: %w", err)
	}
	var oldSet Set
	oldKits := map[string][]File{}
	first := approved == nil
	if !first {
		if oldSet, err = checkSide(*approved, false); err != nil {
			return Diff{}, fmt.Errorf("gate: the approval: %w", err)
		}
		oldKits = approved.KitFiles
	}
	d := Diff{project: newSet.Project}
	c := comparison{old: oldSet, new: newSet, first: first, oldKits: oldKits, newKits: candidate.KitFiles, argv: candidate.SecretArgv}
	d.summary = c.summary()
	d.body = c.body()
	return d, nil
}

// checkSide returns the set of s in its normal form, after checking
// that the files and argv of s are those the set digests.
func checkSide(s Side, withArgv bool) (Set, error) {
	n, err := normal(s.Set)
	if err != nil {
		return Set{}, err
	}
	keys := map[string]bool{}
	for _, k := range n.Kits {
		keys[k.key()] = true
		files, ok := s.KitFiles[k.key()]
		if !ok {
			return Set{}, fmt.Errorf("the kit %s has no files", k.key())
		}
		sum, err := KitTreeDigest(files)
		if err != nil {
			return Set{}, fmt.Errorf("the kit %s: %w", k.key(), err)
		}
		if sum != k.ContentDigest {
			return Set{}, fmt.Errorf("the files of the kit %s have the digest %s, and the set holds %s", k.key(), sum, k.ContentDigest)
		}
	}
	for key := range s.KitFiles {
		if !keys[key] {
			return Set{}, fmt.Errorf("files are given for the kit %s, which the set does not hold", key)
		}
	}
	if !withArgv {
		return n, nil
	}
	names := map[string]bool{}
	for _, sec := range n.Secrets {
		names[sec.Name] = true
		argv, ok := s.SecretArgv[sec.Name]
		if !ok {
			return Set{}, fmt.Errorf("the secret %s has no argv", sec.Name)
		}
		sum, err := SecretArgvDigest(argv)
		if err != nil {
			return Set{}, fmt.Errorf("the secret %s: %w", sec.Name, err)
		}
		if sum != sec.ArgvDigest {
			return Set{}, fmt.Errorf("the argv of the secret %s has the digest %s, and the set holds %s", sec.Name, sum, sec.ArgvDigest)
		}
	}
	for name := range s.SecretArgv {
		if !names[name] {
			return Set{}, fmt.Errorf("an argv is given for the secret %s, which the set does not hold", name)
		}
	}
	return n, nil
}

// comparison holds both sides while the diff is written.
type comparison struct {
	old, new         Set
	first            bool
	oldKits, newKits map[string][]File
	argv             map[string][]string
}

// mark opens each line of the summary that names a widening of the
// classes 01 1.4 lists first.
const mark = "! "

// summary returns the lines of the summary, in the order of 01 1.4:
// the changed capabilities, gated domains, secret argv and workload
// repository, marked; then the content changes of the kits with file
// and line counts; then the name of every changed member of the set.
func (c comparison) summary() []string {
	var out []string
	for _, l := range capabilityChanges("workload", c.old.Workload.Capabilities, c.new.Workload.Capabilities) {
		out = append(out, mark+"capability "+l)
	}
	oldKits, newKits := kitsByKey(c.old.Kits), kitsByKey(c.new.Kits)
	for _, key := range unionKeys(oldKits, newKits) {
		for _, l := range capabilityChanges("kit "+key, oldKits[key].Capabilities, newKits[key].Capabilities) {
			out = append(out, mark+"capability "+l)
		}
	}
	added, removed := listDiff(c.old.EgressGated, c.new.EgressGated)
	for _, g := range added {
		out = append(out, mark+"gated domain added: "+g.Domain+" ("+g.Reason+")")
	}
	for _, g := range removed {
		out = append(out, mark+"gated domain removed: "+g.Domain+" ("+g.Reason+")")
	}
	oldSecrets := map[string]Secret{}
	for _, s := range c.old.Secrets {
		oldSecrets[s.Name] = s
	}
	for _, s := range c.new.Secrets {
		switch o, ok := oldSecrets[s.Name]; {
		case !ok:
			out = append(out, mark+"secret argv added: "+s.Name)
		case o.ArgvDigest != s.ArgvDigest || o.Service != s.Service:
			out = append(out, mark+"secret argv changed: "+s.Name)
		}
	}
	if c.first || c.old.Workload.Repository != c.new.Workload.Repository {
		out = append(out, mark+"workload repository: "+c.workloadChange())
	}
	for _, key := range unionKeys(oldKits, newKits) {
		if oldKits[key].ContentDigest == newKits[key].ContentDigest {
			continue
		}
		files := fileChanges(c.oldKits[key], c.newKits[key])
		var nAdded, nRemoved, nChanged, plus, minus int
		for _, f := range files {
			switch {
			case f.same():
				continue
			case f.old == nil:
				nAdded++
			case f.new == nil:
				nRemoved++
			default:
				nChanged++
			}
			p, m := f.lineCounts()
			plus, minus = plus+p, minus+m
		}
		out = append(out, fmt.Sprintf("kit content %s: %d file(s) changed, %d added, %d removed; +%d -%d lines",
			key, nChanged, nAdded, nRemoved, plus, minus))
	}
	if names := c.changedMembers(); len(names) > 0 {
		out = append(out, "changed: "+strings.Join(names, ", "))
	} else {
		out = append(out, "changed: nothing")
	}
	return out
}

// workloadChange returns "<old> -> <new>", or "(none) -> <new>" on a
// first approval.
func (c comparison) workloadChange() string {
	old := c.old.Workload.Repository
	if c.first {
		old = "(none)"
	}
	return old + " -> " + c.new.Workload.Repository
}

// changedMembers returns the JSON names of the members of the set whose
// canonical JSON differs, in the order of Set; on a first approval,
// every member.
func (c comparison) changedMembers() []string {
	var out []string
	t := reflect.TypeFor[Set]()
	ov, nv := reflect.ValueOf(c.old), reflect.ValueOf(c.new)
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if c.first || canonical(ov.Field(i).Interface()) != canonical(nv.Field(i).Interface()) {
			out = append(out, name)
		}
	}
	return out
}

// body returns the lines that follow the summary: every changed member
// of the set, the kit files with their modes and full text diffs, the
// workload repository highlighted when it changed, and the argv behind
// each secret name.
func (c comparison) body() []string {
	var out []string
	out = append(out, "changed members of the widening set:")
	t := reflect.TypeFor[Set]()
	ov, nv := reflect.ValueOf(c.old), reflect.ValueOf(c.new)
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		o, n := ov.Field(i).Interface(), nv.Field(i).Interface()
		if !c.first && canonical(o) == canonical(n) {
			continue
		}
		out = append(out, "  "+name+":")
		out = append(out, indent("    ", memberLines(o, n, c.first))...)
	}

	oldKits, newKits := kitsByKey(c.old.Kits), kitsByKey(c.new.Kits)
	out = append(out, "", "kit files:")
	for _, key := range unionKeys(oldKits, newKits) {
		files := fileChanges(c.oldKits[key], c.newKits[key])
		changed := slices.ContainsFunc(files, func(f fileChange) bool { return !f.same() })
		state := "unchanged"
		switch {
		case oldKits[key].ID == "":
			state = "added"
		case newKits[key].ID == "":
			state = "removed"
		case changed:
			state = "changed"
		}
		out = append(out, "  "+key+" ("+state+"):")
		for _, f := range files {
			out = append(out, "    "+f.listLine())
		}
		for _, f := range files {
			if !f.same() {
				out = append(out, indent("      ", f.diffLines())...)
			}
		}
	}

	out = append(out, "", "workload repository:")
	if c.first || c.old.Workload.Repository != c.new.Workload.Repository {
		out = append(out, "  !!! changed: "+c.workloadChange()+" !!!")
	} else {
		out = append(out, "  "+c.new.Workload.Repository+" (unchanged)")
	}

	out = append(out, "", "secret argv (host settings):")
	if len(c.new.Secrets) == 0 {
		out = append(out, "  (no secrets)")
	}
	for _, s := range c.new.Secrets {
		out = append(out, "  "+s.Name+"@"+c.new.Project+" (service "+s.Service+"): "+canonical(c.argv[s.Name]))
	}
	return out
}

// memberLines returns the lines of one changed member of the set: for
// a list, each element removed or added; for any other value, the old
// and the new value; each element or value as its canonical JSON.
func memberLines(o, n any, first bool) []string {
	ov, nv := reflect.ValueOf(o), reflect.ValueOf(n)
	if ov.Kind() == reflect.Slice {
		olds, news := elements(ov), elements(nv)
		if first {
			olds = nil
		}
		var out []string
		for _, e := range olds {
			if !slices.Contains(news, e) {
				out = append(out, "- "+e)
			}
		}
		for _, e := range news {
			if !slices.Contains(olds, e) {
				out = append(out, "+ "+e)
			}
		}
		if len(out) == 0 {
			// The same elements in another order: the kits.
			out = append(out, "- "+canonical(o), "+ "+canonical(n))
		}
		return out
	}
	if first {
		return []string{"+ " + canonical(n)}
	}
	return []string{"- " + canonical(o), "+ " + canonical(n)}
}

// elements returns the canonical JSON of each element of the slice v.
func elements(v reflect.Value) []string {
	out := make([]string, v.Len())
	for i := range v.Len() {
		out[i] = canonical(v.Index(i).Interface())
	}
	return out
}

// canonical returns the canonical JSON of v (canon.CanonicalJSON). A
// value of a normal set always has one; a value it refuses is shown as
// the error, so the line is never empty.
func canonical(v any) string {
	data, err := canon.CanonicalJSON(v)
	if err != nil {
		return "(" + err.Error() + ")"
	}
	return string(data)
}

// capabilityChanges returns a line for each capability type that the
// capability set of owner gains, loses or changes.
func capabilityChanges(owner string, old, new Capabilities) []string {
	var out []string
	for _, k := range unionKeys(old, new) {
		o, inOld := old[k]
		n, inNew := new[k]
		switch {
		case !inOld:
			out = append(out, owner+": "+k+" added")
		case !inNew:
			out = append(out, owner+": "+k+" removed")
		case canonical(o) != canonical(n):
			out = append(out, owner+": "+k+" changed")
		}
	}
	return out
}

// kitsByKey returns the kits by "<source>:<id>".
func kitsByKey(kits []Kit) map[string]Kit {
	out := make(map[string]Kit, len(kits))
	for _, k := range kits {
		out[k.key()] = k
	}
	return out
}

// unionKeys returns the keys of a and b, sorted.
func unionKeys[V any](a, b map[string]V) []string {
	keys := slices.Collect(maps.Keys(a))
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// listDiff returns the elements of new that old lacks and those of old
// that new lacks.
func listDiff[E comparable](old, new []E) (added, removed []E) {
	for _, e := range new {
		if !slices.Contains(old, e) {
			added = append(added, e)
		}
	}
	for _, e := range old {
		if !slices.Contains(new, e) {
			removed = append(removed, e)
		}
	}
	return added, removed
}

// fileChange is one path of a kit on both sides; old or new is nil
// where the path is absent.
type fileChange struct {
	path     string
	old, new *File
}

// fileChanges pairs the files of one kit by path, sorted.
func fileChanges(old, new []File) []fileChange {
	byPath := map[string]*fileChange{}
	for i := range old {
		byPath[old[i].Path] = &fileChange{path: old[i].Path, old: &old[i]}
	}
	for i := range new {
		f, ok := byPath[new[i].Path]
		if !ok {
			f = &fileChange{path: new[i].Path}
			byPath[new[i].Path] = f
		}
		f.new = &new[i]
	}
	out := make([]fileChange, 0, len(byPath))
	for _, f := range byPath {
		out = append(out, *f)
	}
	slices.SortFunc(out, func(a, b fileChange) int { return cmp.Compare(a.path, b.path) })
	return out
}

// same reports whether the file is on both sides with one mode and one
// content.
func (f fileChange) same() bool {
	return f.old != nil && f.new != nil && f.old.Mode == f.new.Mode && bytes.Equal(f.old.Data, f.new.Data)
}

// listLine returns the line of the kit file list: a status letter (A
// added, D removed, M changed, = unchanged), the mode and the path.
func (f fileChange) listLine() string {
	switch {
	case f.old == nil:
		return "A " + f.new.Mode + " " + f.path
	case f.new == nil:
		return "D " + f.old.Mode + " " + f.path
	case f.same():
		return "= " + f.new.Mode + " " + f.path
	case f.old.Mode != f.new.Mode:
		return "M " + f.old.Mode + " -> " + f.new.Mode + " " + f.path
	}
	return "M " + f.new.Mode + " " + f.path
}

// lineCounts returns the lines the diff of the file adds and removes; a
// binary file counts none.
func (f fileChange) lineCounts() (plus, minus int) {
	if f.binary() {
		return 0, 0
	}
	for _, l := range f.textDiff() {
		switch l[0] {
		case '+':
			plus++
		case '-':
			minus++
		}
	}
	return plus, minus
}

// binary reports whether either side of the file is binary: not UTF-8,
// or holding a NUL byte.
func (f fileChange) binary() bool {
	return isBinary(f.old) || isBinary(f.new)
}

// isBinary reports whether the file is present and binary.
func isBinary(f *File) bool {
	return f != nil && (!utf8.Valid(f.Data) || bytes.IndexByte(f.Data, 0) >= 0)
}

// diffLines returns the lines that show the change of the file: for a
// binary file, the size and sha256 of each side, flagged binary; for a
// text file, its full text diff.
func (f fileChange) diffLines() []string {
	out := []string{"--- " + f.path}
	if f.binary() {
		for _, side := range []struct {
			sign string
			file *File
		}{{"-", f.old}, {"+", f.new}} {
			if side.file == nil {
				continue
			}
			sum := sha256.Sum256(side.file.Data)
			out = append(out, side.sign+" binary, "+strconv.Itoa(len(side.file.Data))+" bytes, sha256 "+hex.EncodeToString(sum[:]))
		}
		return out
	}
	return append(out, f.textDiff()...)
}

// textDiff returns the full line diff of a text file: the lines both
// sides share at the start and at the end are left out, and every line
// between is shown, removed then added, under a hunk header. Each line
// keeps its end, so a change of the last newline alone is shown, as
// "(no newline at end of file)".
//
// It is not a minimal diff: two changes far apart show every line
// between them, as removed and added. It never leaves a changed line
// out, which is what the gate needs (REQ-0071).
func (f fileChange) textDiff() []string {
	var old, new []string
	if f.old != nil {
		old = splitLines(f.old.Data)
	}
	if f.new != nil {
		new = splitLines(f.new.Data)
	}
	prefix := 0
	for prefix < len(old) && prefix < len(new) && old[prefix] == new[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(new)-prefix && old[len(old)-1-suffix] == new[len(new)-1-suffix] {
		suffix++
	}
	removed, added := old[prefix:len(old)-suffix], new[prefix:len(new)-suffix]
	if len(removed) == 0 && len(added) == 0 {
		return nil
	}
	// The hunk header of a unified diff: a side with no lines starts at
	// the line before the hunk.
	start := func(lines []string) int {
		if len(lines) == 0 {
			return prefix
		}
		return prefix + 1
	}
	out := []string{fmt.Sprintf("@@ -%d,%d +%d,%d @@", start(removed), len(removed), start(added), len(added))}
	for _, l := range removed {
		out = append(out, "-"+showLine(l))
	}
	for _, l := range added {
		out = append(out, "+"+showLine(l))
	}
	return out
}

// splitLines returns the lines of data, each with its newline; the last
// one lacks it when data does not end with one.
func splitLines(data []byte) []string {
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// showLine returns the line l without its newline, or marked when it
// has none.
func showLine(l string) string {
	if s, ok := strings.CutSuffix(l, "\n"); ok {
		return s
	}
	return l + " (no newline at end of file)"
}

// indent returns lines, each after prefix.
func indent(prefix string, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = prefix + l
	}
	return out
}

// WriteTo writes the diff to w: a title, the summary, the body, the
// summary again right above the prompt, so a long diff cannot push it
// out of view, then PromptLine. Every line is escaped (termsafe, I29).
//
//romeu:invariant I29 guard
func (d Diff) WriteTo(w io.Writer) (int64, error) {
	var b strings.Builder
	line := func(s string) { b.WriteString(termsafe.Escape(s) + "\n") }
	title := "gate 2: widening of the project " + d.project
	line(title)
	line("summary:")
	for _, l := range d.summary {
		line("  " + l)
	}
	line("")
	for _, l := range d.body {
		line(l)
	}
	line("")
	line("summary, again (" + title + "):")
	for _, l := range d.summary {
		line("  " + l)
	}
	b.WriteString(PromptLine)
	n, err := io.WriteString(w, b.String())
	return int64(n), err
}
