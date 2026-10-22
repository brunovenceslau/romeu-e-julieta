// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// Finding is one value a rule refuses. Its text names where the value is
// and why, never the value: a host settings value can be a secret.
type Finding struct {
	// File is the path of the file, when the validator knows it.
	File string
	// Path is the place of the value, as "repos[1].dir".
	Path string
	// Rule is the slug of the rule.
	Rule string
	// Message says what is wrong with the value.
	Message string
	// Row is the rule's error of 04 4.4, when it has one.
	Row *cli.Row
}

// String returns the finding as one line.
func (f Finding) String() string {
	where := f.Path
	if f.File != "" {
		where = f.File + ": " + where
	}
	return where + ": " + f.Message + " (" + f.Rule + ")"
}

// InvalidError holds the findings of a validator, in the order of the
// rules and then of the values. It is ErrInvalid (errors.Is).
type InvalidError struct {
	Findings []Finding
}

// Error returns one line per finding.
func (e *InvalidError) Error() string {
	lines := make([]string, len(e.Findings))
	for i, f := range e.Findings {
		lines[i] = f.String()
	}
	return ErrInvalid.Error() + ":\n" + strings.Join(lines, "\n")
}

// Is reports whether target is ErrInvalid.
func (e *InvalidError) Is(target error) bool { return target == ErrInvalid }

// errorOf returns nil for no findings, or the *InvalidError of the
// findings; when one of them has an error of 04 4.4, the first such
// error wraps it, so the command exits with that row's code.
func errorOf(findings []Finding) error {
	if len(findings) == 0 {
		return nil
	}
	inv := &InvalidError{Findings: findings}
	for _, f := range findings {
		if f.Row != nil {
			return f.Row.Wrap(inv, f.String(), cli.Detail{Key: "rule", Value: f.Rule})
		}
	}
	return inv
}

// Validate checks p by every rule of Rules for project.v1 that reads one
// spec: each value rule and each check across its values. It reads p
// with the defaults DecodeProject fills (a repo's dir), so a Project
// built by hand is checked as its file would be; p is left as it is. A
// spec from DecodeProject has passed it.
//
//romeu:invariant I12 guard
func (p *Project) Validate() error {
	q := *p
	q.Repos = slices.Clone(p.Repos)
	q.setDefaults()
	findings, err := valueFindings(FormatProject, reflect.ValueOf(&q).Elem())
	if err != nil {
		return err
	}
	for _, r := range rulesOf(FormatProject) {
		if r.inProject != nil {
			findings = append(findings, withRule(r, r.inProject(&q))...)
		}
	}
	return errorOf(findings)
}

// Validate checks h by every rule of Rules for host-settings.v1, the
// secret-in-argv guard among them (I25). Settings from
// DecodeHostSettings have passed it.
//
//romeu:invariant I25 guard
func (h *HostSettings) Validate() error {
	findings, err := valueFindings(FormatHostSettings, reflect.ValueOf(h).Elem())
	if err != nil {
		return err
	}
	return errorOf(findings)
}

// rulesOf returns the rules of format f.
func rulesOf(f Format) []Rule {
	var out []Rule
	for _, r := range rules {
		if r.Format == f {
			out = append(out, r)
		}
	}
	return out
}

// HostOnly returns the rules of Rules that romeu checks and julieta spec
// validate cannot, since they read host settings (REQ-0174).
func HostOnly() []Rule {
	var out []Rule
	for _, r := range Rules() {
		if r.Format == FormatProject && r.Scope == RomeuOnly {
			out = append(out, r)
		}
	}
	return out
}

// withRule returns findings with the rule's slug, message and row set.
func withRule(r Rule, findings []Finding) []Finding {
	for i := range findings {
		findings[i].Rule, findings[i].Row = r.Slug, r.Row
		if findings[i].Message == "" {
			findings[i].Message = r.Message
		}
	}
	return findings
}

// node is one value of a document the walk reaches: the path of the
// rules that apply to it, its own path, and the value.
type node struct {
	rule, at string
	v        reflect.Value
}

// walk returns every value of v, a value of a format's type, with its
// paths; a zero member the type marks optional is left out, with what
// is below it. A struct type reached a second time takes the rule path
// of its first place, so a rule of a recursive type applies at any
// depth.
func walk(v reflect.Value) []node {
	var out []node
	var visit func(v reflect.Value, rule, at string, first map[reflect.Type]string)
	visit = func(v reflect.Value, rule, at string, first map[reflect.Type]string) {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			if p, ok := first[v.Type()]; ok {
				rule = p
			} else {
				first = withType(first, v.Type(), rule)
			}
		}
		out = append(out, node{rule: rule, at: at, v: v})
		switch v.Kind() {
		case reflect.Struct:
			for i, f := range slices.Collect(v.Type().Fields()) {
				fv := v.Field(i)
				if optional(f) && fv.IsZero() {
					continue
				}
				name := jsonName(f)
				visit(fv, join(rule, name), join(at, name), first)
			}
		case reflect.Slice:
			for i := range v.Len() {
				idx := "[" + strconv.Itoa(i) + "]"
				visit(v.Index(i), rule+"[]", at+idx, first)
				if i == 0 {
					out = append(out, node{rule: rule + "[0]", at: at + idx, v: v.Index(i)})
				}
			}
		case reflect.Map:
			keys := v.MapKeys()
			slices.SortFunc(keys, func(a, b reflect.Value) int { return cmp.Compare(a.String(), b.String()) })
			for _, k := range keys {
				out = append(out, node{rule: rule + "{key}", at: at + "{" + strconv.Quote(k.String()) + "}", v: k})
				visit(v.MapIndex(k), rule+"{}", at+"["+strconv.Quote(k.String())+"]", first)
			}
		}
	}
	visit(v, "", "", map[reflect.Type]string{})
	return out
}

// withType returns a copy of m with t mapped to p.
func withType(m map[reflect.Type]string, t reflect.Type, p string) map[reflect.Type]string {
	out := make(map[reflect.Type]string, len(m)+1)
	maps.Copy(out, m)
	out[t] = p
	return out
}

// join returns the path of the member name of the value at p.
func join(p, name string) string {
	if p == "" {
		return name
	}
	return p + "." + name
}

// compiled holds the regular expressions of the rules, compiled once; a
// pattern that does not compile is an error of every validator, which
// TestRulesAreWellFormed keeps from being committed.
var compiled = sync.OnceValues(func() (map[string]*regexp.Regexp, error) {
	out := map[string]*regexp.Regexp{}
	for _, r := range rules {
		for _, p := range []string{r.Pattern, r.NotPattern} {
			if p == "" {
				continue
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("spec: the rule %s: %w", r.Slug, err)
			}
			out[p] = re
		}
	}
	return out, nil
})

// valueFindings applies each value rule of format f to the values of v
// its field path reaches.
func valueFindings(f Format, v reflect.Value) ([]Finding, error) {
	re, err := compiled()
	if err != nil {
		return nil, err
	}
	byField := map[string][]node{}
	for _, n := range walk(v) {
		byField[n.rule] = append(byField[n.rule], n)
	}
	var findings []Finding
	for _, r := range rulesOf(f) {
		for _, n := range byField[r.Field] {
			if why := check(r, n.v, re); why != "" {
				findings = append(findings, Finding{Path: label(n.at), Rule: r.Slug, Message: why, Row: r.Row})
			}
		}
	}
	return findings, nil
}

// check returns why the value v breaks the constraints of r, or "" when
// it keeps them; re holds the compiled patterns. The checks across
// values are not applied here.
func check(r Rule, v reflect.Value, re map[string]*regexp.Regexp) string {
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		switch {
		case r.Pattern != "" && !re[r.Pattern].MatchString(s),
			r.NotPattern != "" && re[r.NotPattern].MatchString(s),
			r.MaxBytes != 0 && len(s) > r.MaxBytes,
			slices.ContainsFunc(r.Reserved, func(x string) bool { return strings.EqualFold(x, s) }),
			len(r.Enum) > 0 && !slices.Contains(r.Enum, s):
			return r.Message
		}
		if r.value != nil {
			if err := r.value(s); err != nil {
				return r.Message + ": it " + err.Error()
			}
		}
	case reflect.Float64:
		if x := v.Float(); r.Range && !(x >= r.Min && x <= r.Max) {
			return r.Message
		}
	case reflect.Int:
		if x := float64(v.Int()); r.Range && !(x >= r.Min && x <= r.Max) {
			return r.Message
		}
	case reflect.Slice:
		if v.Len() < r.MinItems {
			return r.Message
		}
	case reflect.Struct:
		if len(r.OneOf) > 0 && !oneGroup(v, r.OneOf) {
			return r.Message
		}
	}
	return ""
}

// oneGroup reports whether exactly one of groups has each of its
// members present (not zero) in the struct value v, and no member of
// another group is present.
func oneGroup(v reflect.Value, groups [][]string) bool {
	present := map[string]bool{}
	for i, f := range slices.Collect(v.Type().Fields()) {
		present[jsonName(f)] = !v.Field(i).IsZero()
	}
	whole := 0
	for i, g := range groups {
		if !slices.ContainsFunc(g, func(m string) bool { return !present[m] }) {
			whole++
			for j, o := range groups {
				if j != i && slices.ContainsFunc(o, func(m string) bool { return present[m] }) {
					return false
				}
			}
		}
	}
	return whole == 1
}

// onePrimary refuses a project whose repos hold not exactly one primary.
func onePrimary(p *Project) []Finding {
	n := 0
	for _, r := range p.Repos {
		if r.Primary {
			n++
		}
	}
	if n != 1 {
		return []Finding{{Path: "repos"}}
	}
	return nil
}

// uniqueDirs refuses a repo dir another repo of the project has,
// compared case-insensitively (REQ-0176).
func uniqueDirs(p *Project) []Finding {
	var out []Finding
	seen := map[string]bool{}
	for i, r := range p.Repos {
		k := strings.ToLower(r.Dir)
		if seen[k] {
			out = append(out, Finding{Path: fmt.Sprintf("repos[%d].dir", i)})
		}
		seen[k] = true
	}
	return out
}

// placedOnce refuses a pane id declared twice, a declared pane no tab
// places or two places name, and a leaf that names no declared pane
// (REQ-0197).
func placedOnce(p *Project) []Finding {
	if p.Run == nil {
		return nil
	}
	var out []Finding
	declared := map[string]bool{}
	for i, pane := range p.Run.Panes {
		if declared[pane.ID] {
			out = append(out, Finding{Path: fmt.Sprintf("run.panes[%d].id", i), Message: "is declared twice"})
		}
		declared[pane.ID] = true
	}
	placed := map[string]int{}
	for i, t := range p.Run.Tabs {
		for at, id := range leaves(&t.Root, fmt.Sprintf("run.tabs[%d].root", i)) {
			placed[id]++
			if !declared[id] {
				out = append(out, Finding{Path: at + ".pane", Message: "names no declared pane"})
			}
		}
	}
	for i, pane := range p.Run.Panes {
		switch n := placed[pane.ID]; {
		case n == 0:
			out = append(out, Finding{Path: fmt.Sprintf("run.panes[%d].id", i), Message: "is placed in no tab"})
		case n > 1:
			out = append(out, Finding{Path: fmt.Sprintf("run.panes[%d].id", i), Message: "is placed " + strconv.Itoa(n) + " times"})
		}
	}
	slices.SortStableFunc(out, func(a, b Finding) int { return cmp.Compare(a.Path, b.Path) })
	return out
}

// leaves returns the pane id of each leaf below n, by its path from at.
func leaves(n *Node, at string) map[string]string {
	out := map[string]string{}
	var visit func(n *Node, at string)
	visit = func(n *Node, at string) {
		if n == nil {
			return
		}
		if n.Pane != "" {
			out[at] = n.Pane
		}
		visit(n.First, at+".first")
		visit(n.Second, at+".second")
	}
	visit(n, at)
	return out
}

// paneRepos refuses a pane whose repo is not the dir of a repo of the
// project.
func paneRepos(p *Project) []Finding {
	if p.Run == nil {
		return nil
	}
	var out []Finding
	for i, pane := range p.Run.Panes {
		if !slices.ContainsFunc(p.Repos, func(r Repo) bool { return r.Dir == pane.Repo }) {
			out = append(out, Finding{Path: fmt.Sprintf("run.panes[%d].repo", i)})
		}
	}
	return out
}

// focusPerTab refuses a second focused pane in one tab.
func focusPerTab(p *Project) []Finding {
	if p.Run == nil {
		return nil
	}
	focused := map[string]bool{}
	for _, pane := range p.Run.Panes {
		focused[pane.ID] = pane.Focus
	}
	var out []Finding
	for i, t := range p.Run.Tabs {
		at := fmt.Sprintf("run.tabs[%d]", i)
		n := 0
		for _, id := range leaves(&t.Root, at+".root") {
			if focused[id] {
				n++
			}
		}
		if n > 1 {
			out = append(out, Finding{Path: at, Message: "places " + strconv.Itoa(n) + " focused panes; at most one"})
		}
	}
	return out
}

// File is one file of a config repo's projects/ directory: its path from
// the repo's root, such as "projects/shop.yaml", and its content.
type File struct {
	Path string
	Data []byte
}

// set is what an InSet rule reads: the files of projects/, the specs
// that decoded, by file path, and the host settings, nil for julieta
// spec validate.
type set struct {
	files []File
	specs map[string]*Project
	host  *HostSettings
}

// LoadSet decodes and validates the files of a config repo's projects/
// directory together (04 4.2, sync step 3): each file as DecodeProject
// does, then the rules across files, and with host settings (romeu)
// the rules of RomeuOnly; julieta spec validate passes nil and names
// HostOnly. It returns every spec when no file breaks a rule, and
// otherwise no spec and every finding of every file at once, which sync
// step 3 lists before it exits 2 (04 4.2); a file that does not decode
// is one finding.
func LoadSet(files []File, host *HostSettings) ([]Project, error) {
	s := &set{files: files, specs: map[string]*Project{}, host: host}
	var findings []Finding
	var specs []Project
	for _, f := range files {
		if path.Ext(f.Path) != ".yaml" {
			continue
		}
		p, err := DecodeProject(f.Data)
		if err != nil {
			findings = append(findings, fileFindings(f.Path, err)...)
			continue
		}
		s.specs[f.Path] = &p
		specs = append(specs, p)
	}
	for _, r := range rulesOf(FormatProject) {
		if r.inSet != nil && (r.Scope == Both || host != nil) {
			findings = append(findings, withRule(r, r.inSet(s))...)
		}
	}
	if err := errorOf(findings); err != nil {
		return nil, err
	}
	return specs, nil
}

// fileFindings returns the findings of err, the refusal of the file at
// p, each with the file set; an error with no findings is one finding.
func fileFindings(p string, err error) []Finding {
	if inv, ok := errors.AsType[*InvalidError](err); ok {
		out := slices.Clone(inv.Findings)
		for i := range out {
			out[i].File = p
		}
		return out
	}
	f := Finding{File: p, Path: "the file", Rule: "decode", Message: err.Error()}
	if e, ok := errors.AsType[*cli.Error](err); ok {
		f.Rule, f.Row = e.Slug, &e.Row
	}
	return []Finding{f}
}

// sortedSpecs returns the specs of s by file path.
func (s *set) sortedSpecs() []string {
	var paths []string
	for p := range s.specs {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}

// yamlOnly refuses a file under projects/ that is not *.yaml (REQ-0178).
func yamlOnly(s *set) []Finding {
	var out []Finding
	for _, f := range s.files {
		if path.Ext(f.Path) != ".yaml" {
			out = append(out, Finding{File: f.Path, Path: "the file"})
		}
	}
	return out
}

// nameIsStem refuses a spec whose name is not its file stem (REQ-0133).
func nameIsStem(s *set) []Finding {
	var out []Finding
	for _, p := range s.sortedSpecs() {
		if s.specs[p].Name != strings.TrimSuffix(path.Base(p), ".yaml") {
			out = append(out, Finding{File: p, Path: "name"})
		}
	}
	return out
}

// uniqueNames refuses a name an earlier spec has.
func uniqueNames(s *set) []Finding {
	var out []Finding
	seen := map[string]bool{}
	for _, p := range s.sortedSpecs() {
		if seen[s.specs[p].Name] {
			out = append(out, Finding{File: p, Path: "name"})
		}
		seen[s.specs[p].Name] = true
	}
	return out
}

// uniqueURLs refuses a repo URL an earlier spec has, compared after
// NormalizeURL (REQ-0177).
func uniqueURLs(s *set) []Finding {
	var out []Finding
	owner := map[string]string{}
	for _, p := range s.sortedSpecs() {
		for i, r := range s.specs[p].Repos {
			k := NormalizeURL(r.URL)
			if o, ok := owner[k]; ok && o != p {
				out = append(out, Finding{File: p, Path: fmt.Sprintf("repos[%d].url", i)})
			}
			owner[k] = p
		}
	}
	return out
}

// configProject refuses a set in which not exactly one spec's primary
// repo URL equals config.url of host settings (REQ-0179).
func configProject(s *set) []Finding {
	want := NormalizeURL(s.host.Config.URL)
	n := 0
	for _, p := range s.specs {
		for _, r := range p.Repos {
			if r.Primary && NormalizeURL(r.URL) == want {
				n++
			}
		}
	}
	if n != 1 {
		return []Finding{{File: "settings.yaml", Path: "config.url"}}
	}
	return nil
}

// knownHosts refuses a repo URL whose host no gitHosts entry names
// (REQ-0180).
func knownHosts(s *set) []Finding {
	var out []Finding
	for _, p := range s.sortedSpecs() {
		for i, r := range s.specs[p].Repos {
			host, _, _ := strings.Cut(strings.TrimPrefix(r.URL, "https://"), "/")
			if !slices.ContainsFunc(s.host.GitHosts, func(g GitHost) bool { return g.Host == strings.ToLower(host) }) {
				out = append(out, Finding{File: p, Path: fmt.Sprintf("repos[%d].url", i)})
			}
		}
	}
	return out
}

// allowedWorkloads refuses a workload whose repository is not in
// workloadRepositories (REQ-0190).
func allowedWorkloads(s *set) []Finding {
	var out []Finding
	for _, p := range s.sortedSpecs() {
		repo, _, _ := strings.Cut(s.specs[p].Workload, "@")
		if !slices.Contains(s.host.WorkloadRepositories, repo) {
			out = append(out, Finding{File: p, Path: "workload"})
		}
	}
	return out
}
