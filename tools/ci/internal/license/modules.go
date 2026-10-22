// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package license

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/taggedsuites"
)

// AllowlistPath is the committed allowlist of the licenses of the
// module graph (REQ-0848), with a reason for each license, from the
// root of the repository.
const AllowlistPath = "tools/ci/internal/license/allowlist.yaml"

// License is one license of the allowlist: its SPDX id, whether a
// module may carry it, why, and how a license file is recognized as it.
type License struct {
	ID      string   `yaml:"id"`
	Allowed bool     `yaml:"allowed"`
	Reason  string   `yaml:"reason"`
	Markers []string `yaml:"markers"`
	Unless  []string `yaml:"unless"`
}

// Rider is a marker of terms added to a license, which the markers of
// a license do not read: a file that holds one fails, whatever it
// names otherwise.
type Rider struct {
	Marker string `yaml:"marker"`
	Reason string `yaml:"reason"`
}

// Allowlist is the list of known licenses and riders.
type Allowlist struct {
	Licenses []License `yaml:"licenses"`
	Riders   []Rider   `yaml:"riders"`
}

// spdxID is the form of an SPDX license id.
var spdxID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]*$`)

// ParseAllowlist decodes an allowlist strictly: an unknown key, a
// license without an id, a reason or a marker, an id that is not of the
// SPDX form, an id listed twice, or a rider without a marker or a
// reason is an error.
func ParseAllowlist(src []byte) (Allowlist, error) {
	var a Allowlist
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	if err := dec.Decode(&a); err != nil {
		return Allowlist{}, fmt.Errorf("the license allowlist: %w", err)
	}
	seen := map[string]bool{}
	for i, l := range a.Licenses {
		switch {
		case !spdxID.MatchString(l.ID):
			return Allowlist{}, fmt.Errorf("the license allowlist: entry %d has no SPDX id", i+1)
		case seen[l.ID]:
			return Allowlist{}, fmt.Errorf("the license allowlist: %s is listed twice", l.ID)
		case strings.TrimSpace(l.Reason) == "":
			return Allowlist{}, fmt.Errorf("the license allowlist: %s has no reason", l.ID)
		case len(l.Markers) == 0 || slices.ContainsFunc(slices.Concat(l.Markers, l.Unless), func(m string) bool { return words(m) == "" }):
			return Allowlist{}, fmt.Errorf("the license allowlist: %s needs markers, none of them empty", l.ID)
		}
		seen[l.ID] = true
	}
	for i, r := range a.Riders {
		if words(r.Marker) == "" || strings.TrimSpace(r.Reason) == "" {
			return Allowlist{}, fmt.Errorf("the license allowlist: rider %d needs a marker and a reason", i+1)
		}
	}
	if len(a.Licenses) == 0 {
		return Allowlist{}, errors.New("the license allowlist lists no license")
	}
	return a, nil
}

// words returns s lowercased with every run of characters that are not
// letters or digits made one space, so a marker matches whatever the
// case, the line breaks, the indentation and the punctuation around it.
func words(s string) string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	return strings.Join(f, " ")
}

// contains reports whether the words of text hold the words of marker,
// on word boundaries.
func contains(text, marker string) bool {
	return strings.Contains(" "+text+" ", " "+words(marker)+" ")
}

// Identify returns the ids of the licenses whose markers all appear in
// text and whose unless markers do not, in the order of the list.
func (a Allowlist) Identify(text []byte) []string {
	norm := words(string(text))
	var ids []string
	for _, l := range a.Licenses {
		if !slices.ContainsFunc(l.Markers, func(m string) bool { return !contains(norm, m) }) &&
			!slices.ContainsFunc(l.Unless, func(m string) bool { return contains(norm, m) }) {
			ids = append(ids, l.ID)
		}
	}
	return ids
}

// RidersOf returns the markers of the riders text holds, in the order of
// the list.
func (a Allowlist) RidersOf(text []byte) []string {
	norm := words(string(text))
	var out []string
	for _, r := range a.Riders {
		if contains(norm, r.Marker) {
			out = append(out, r.Marker)
		}
	}
	return out
}

// allowed reports whether the license of id may be carried.
func (a Allowlist) allowed(id string) bool {
	i := slices.IndexFunc(a.Licenses, func(l License) bool { return l.ID == id })
	return i >= 0 && a.Licenses[i].Allowed
}

// Module is one module the builds of this module compile, as the
// module cache holds its source.
type Module struct {
	Path, Version string
	// Dir is the directory of its source in the module cache.
	Dir string
}

// listed is the part of "go list -json" the step reads: the module of
// each package.
type listed struct {
	ImportPath string
	Module     *struct {
		Path, Version string
		Main          bool
	}
	Error *struct{ Err string }
}

// tagSets are the build tags of the listings: none, and those of the
// tagged suites of 10 10.2, whose test files a listing without tags
// leaves out (as the imports check lists them).
var tagSets = [][]string{nil, {"-tags", taggedsuites.Tags()}}

// CompiledModules returns, as "path@version" and sorted, the modules
// whose packages some build of the module at dir compiles: "go list
// -deps -test ./..." with goBin, for each of lint.Targets and each of
// tagSets, in the environment of the steps (env, with GOPROXY=off and
// GOFLAGS=-mod=readonly), the main module and the standard library left
// out. These are the modules the "go mod download" of setup fetches; a
// module go.sum pins for the tests of a dependency alone, which no
// build of this module compiles, is not judged.
func CompiledModules(ctx context.Context, goBin, dir string, env []string) ([]string, error) {
	seen := map[string]bool{}
	for _, tags := range tagSets {
		for _, target := range lint.Targets {
			var stdout, stderr bytes.Buffer
			err := procgroup.Run(ctx, procgroup.Command{
				Class:  procgroup.ToolStep,
				Argv:   slices.Concat([]string{goBin, "list", "-deps", "-test", "-e", "-json=ImportPath,Module,Error"}, tags, []string{"./..."}),
				Dir:    dir,
				Env:    append(slices.Clone(env), "GOOS="+target.GOOS, "GOARCH="+target.GOARCH, "CGO_ENABLED=0"),
				Stdout: &stdout,
				Stderr: &stderr,
			})
			if err != nil {
				return nil, fmt.Errorf("go list for %s: %w\n%s", target, err, bytes.TrimSpace(stderr.Bytes()))
			}
			if err := modulesOf(&stdout, seen); err != nil {
				return nil, fmt.Errorf("go list for %s: %w", target, err)
			}
		}
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// modulesOf adds to seen the module of each package of a "go list
// -json" stream that belongs to a module other than the main one. A
// package that does not load, or a module path or version not of its
// form, is an error.
func modulesOf(r io.Reader, seen map[string]bool) error {
	dec := jsontext.NewDecoder(r)
	for {
		var p listed
		if err := json.UnmarshalDecode(dec, &p); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		if p.Error != nil {
			return fmt.Errorf("%s: %s", p.ImportPath, p.Error.Err)
		}
		if p.Module == nil || p.Module.Main {
			continue
		}
		if !modulePathForm.MatchString(p.Module.Path) || !versionForm.MatchString(p.Module.Version) {
			return fmt.Errorf("%s: the module %q at %q is not of the form of a module path and a version", p.ImportPath, p.Module.Path, p.Module.Version)
		}
		seen[p.Module.Path+"@"+p.Module.Version] = true
	}
}

// The forms of a module path and a version: a path starts with a
// letter or a digit, so no listing can pass a flag to the go command,
// and a version starts with "v" and a digit.
var (
	modulePathForm = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~+/-]*$`)
	versionForm    = regexp.MustCompile(`^v[0-9][0-9A-Za-z.+-]*$`)
)

// licenseFile matches the names of the files the step reads: LICENSE,
// LICENCE or COPYING, alone or followed by "-", "." or "_" and more.
var licenseFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying)([-._].*)?$`)

// Finding rules of the module step.
const (
	ruleDisallowed = "license disallowed"
	ruleUnknown    = "license unknown"
	ruleRider      = "license rider"
	ruleGoMod      = "license go.mod"
)

// JudgeModules reads the license files of each module and returns a
// finding for each license file that holds a rider, names a license
// the allowlist does not allow, or names none it knows, and for each
// module without a license file. Every file named LICENSE*, LICENCE* or COPYING* at any
// depth of the module counts, so the license of code a module copies
// in (as testify does) is read too.
func JudgeModules(mods []Module, a Allowlist) ([]check.Finding, error) {
	var findings []check.Finding
	for _, m := range mods {
		where := m.Path + "@" + m.Version
		files, err := licenseFiles(m.Dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if len(files) == 0 {
			findings = append(findings, check.Finding{Where: where, Rule: ruleUnknown, Msg: "has no LICENSE, LICENCE or COPYING file"})
			continue
		}
		for _, f := range files {
			if riders := a.RidersOf(f.content); len(riders) > 0 {
				findings = append(findings, check.Finding{Where: where, Rule: ruleRider, Msg: f.name + " holds the rider " + strings.Join(riders, ", ") + ", whose terms the allowlist does not read"})
				continue
			}
			ids := a.Identify(f.content)
			var bad []string
			for _, id := range ids {
				if !a.allowed(id) {
					bad = append(bad, id)
				}
			}
			switch {
			case len(ids) == 0:
				findings = append(findings, check.Finding{Where: where, Rule: ruleUnknown, Msg: f.name + " names no license of the allowlist"})
			case len(bad) > 0:
				findings = append(findings, check.Finding{Where: where, Rule: ruleDisallowed, Msg: f.name + " is under " + strings.Join(bad, ", ") + ", which the allowlist does not allow with GPL-3.0-only"})
			}
		}
	}
	return findings, nil
}

// file is one license file of a module: its slash path in the module,
// and its content.
type file struct {
	name    string
	content []byte
}

// licenseFiles returns the license files below dir, sorted by path,
// read through an os.Root, so no link leads the step out of the module.
// Go files and testdata directories are left out: no build ships them
// as a license.
func licenseFiles(dir string) ([]file, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	var files []file
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return fs.SkipDir
		}
		if !d.Type().IsRegular() || !licenseFile.MatchString(d.Name()) || path.Ext(p) == ".go" {
			return nil
		}
		content, err := root.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, file{name: p, content: content})
		return nil
	})
	return files, err
}

// Download returns the module cache directory of each of mods
// ("path@version"), from "go mod download -json" run with goBin in dir
// and env. With the GOPROXY=off of check.StepEnv it reads the cache
// alone, which setup fills, and it fails on a module the cache lacks or
// whose content does not match go.sum.
func Download(ctx context.Context, goBin, dir string, env, mods []string) ([]Module, error) {
	if len(mods) == 0 {
		return nil, nil
	}
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   slices.Concat([]string{goBin, "mod", "download", "-json", "--"}, mods),
		Dir:    dir,
		Env:    env,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	out, decodeErr := decodeDownload(&stdout)
	switch {
	case decodeErr != nil:
		return nil, decodeErr
	case err != nil:
		return nil, fmt.Errorf("go mod download: %w\n%s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

// decodeDownload decodes the stream of objects "go mod download -json"
// prints; a module it could not download is an error that names it.
func decodeDownload(r io.Reader) ([]Module, error) {
	var out []Module
	dec := jsontext.NewDecoder(r)
	for {
		var m struct {
			Path, Version, Dir, Error string
		}
		if err := json.UnmarshalDecode(dec, &m); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, fmt.Errorf("go mod download: %w", err)
		}
		if m.Error != "" {
			return nil, fmt.Errorf("go mod download: %s@%s: %s", m.Path, m.Version, m.Error)
		}
		if m.Dir == "" {
			return nil, fmt.Errorf("go mod download: %s@%s: no directory in the module cache", m.Path, m.Version)
		}
		out = append(out, Module{Path: m.Path, Version: m.Version, Dir: m.Dir})
	}
}

// goModFindings returns a finding for each replace or exclude directive
// of go.mod: with one, the module the build uses is not the one go.sum
// names by its path, so the step would read the license of another.
func goModFindings(gomod []byte) []check.Finding {
	var findings []check.Finding
	for n, line := range strings.Split(string(gomod), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && (f[0] == "replace" || f[0] == "exclude") {
			findings = append(findings, check.Finding{Where: "go.mod", Rule: ruleGoMod,
				Msg: fmt.Sprintf("line %d holds a %s directive, so the step cannot read the license of the module the build uses", n+1, f[0])})
		}
	}
	return findings
}

// Verify runs "go mod verify" with goBin in dir and env: it checks,
// offline, that each module of the cache is the one go.sum names, its
// extracted tree included, so an edited tree does not reach the step.
func Verify(ctx context.Context, goBin, dir string, env []string) error {
	out, err := procgroup.CombinedOutput(ctx, procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{goBin, "mod", "verify"},
		Dir:   dir,
		Env:   env,
	})
	if err != nil {
		return fmt.Errorf("go mod verify: %w\n%s", err, bytes.TrimSpace(out))
	}
	return nil
}

// ModulesStep is the second step of the license check: the licenses
// subcommand, run with the pinned go in the environment of the steps.
func ModulesStep(p check.Plan) check.Step {
	return check.Step{
		Name:    "license modules",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "run", "./tools/ci", "licenses"},
		Environ: check.StepEnv(p.Tools.GoDir),
	}
}

// RunModules is the licenses subcommand: it refuses a replace or an
// exclude directive of go.mod, verifies the module cache against go.sum
// ("go mod verify"), and reads the licenses of the modules the builds
// of this module compile (CompiledModules), from the module cache,
// against the committed allowlist.
func RunModules(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("licenses takes no argument\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	goBin, err := mise.WhichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return false, err
	}
	return reportModules(ctx, e.Stdout, goBin, root, check.StepEnv(filepath.Dir(goBin)))
}

// reportModules judges the modules the builds of root compile, read
// from the module cache with goBin in env, prints the findings and the summary line to out, and
// reports whether there were none.
func reportModules(ctx context.Context, out io.Writer, goBin, root string, env []string) (bool, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(AllowlistPath)))
	if err != nil {
		return false, err
	}
	a, err := ParseAllowlist(src)
	if err != nil {
		return false, err
	}
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false, err
	}
	if findings := goModFindings(gomod); len(findings) > 0 {
		check.Say(out, "%s%s\n", check.Lines(findings), check.Summary("licenses", len(findings)))
		return false, nil
	}
	if err := Verify(ctx, goBin, root, env); err != nil {
		return false, err
	}
	names, err := CompiledModules(ctx, goBin, root, env)
	if err != nil {
		return false, err
	}
	mods, err := Download(ctx, goBin, root, env, names)
	if err != nil {
		return false, err
	}
	findings, err := JudgeModules(mods, a)
	if err != nil {
		return false, err
	}
	check.Say(out, "%s%s\n", check.Lines(findings), check.Summary("licenses", len(findings)))
	return len(findings) == 0, nil
}
