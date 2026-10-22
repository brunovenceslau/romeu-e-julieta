// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package coverage is the coverage check of 10 10.2: the thresholds of
// S9, per package, over the cover profile that the race step of all
// writes, and over a profile a developer names (the coverage
// subcommand).
//
// The profile is read by golang.org/x/tools/cover (approved on
// 2026-10-21), which parses the format of the go command
// (https://pkg.go.dev/cmd/cover) and merges a block that several test
// binaries list into one. The go command's own output gives no
// per-package share from the profile: "go tool cover -func" reports per
// function and a total, and the "coverage: N%" line of "go test" is not
// in the profile and is rounded to one decimal, where this check
// compares statement counts exactly and prints its own share rounded
// down.
package coverage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/cover"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The thresholds of S9, in percent of statements, per package.
const (
	coverFloor  = 80
	coverStrict = 90
)

// coverTrees are the directories whose packages S9 measures, each with
// every package below it.
var coverTrees = []string{"cmd", "internal", "tools", "e2e/probes", "e2e/fakesbx"}

// coverStrictModules are the modules S9 holds to coverStrict: the
// package internal/<module> and every package below it.
var coverStrictModules = []string{"gitsafe", "gate", "spec", "render", "egress", "termsafe", "state", "canon"}

// coverPackage is the count of one package.
type coverPackage struct {
	stmts, covered int
}

// coverage applies the thresholds of S9 to a cover profile, as "go test
// -coverprofile" writes it (https://pkg.go.dev/cmd/cover), for the
// module whose path is module. It returns one finding for each package
// below its threshold, in the order of their paths.
//
// A block that the profile lists more than once (one test binary per
// package can each list a package they share) counts once, as covered
// when any listing ran it: cover merges the listings. A package with no
// test file is in the profile with every block at 0, so it fails
// instead of being left out.
// A package with no statement has nothing to measure and passes.
func coverage(profile []byte, module string) ([]check.Finding, error) {
	if !bytes.HasPrefix(profile, []byte("mode: ")) {
		return nil, errors.New("the cover profile does not start with a mode line")
	}
	profiles, err := cover.ParseProfilesFromReader(bytes.NewReader(profile))
	if err != nil {
		return nil, fmt.Errorf("the cover profile: %w", err)
	}
	packages := map[string]*coverPackage{}
	for _, prof := range profiles {
		rel, ok := strings.CutPrefix(path.Dir(prof.FileName), module+"/")
		if !ok || !inCoverTrees(rel) {
			continue
		}
		p := packages[rel]
		if p == nil {
			p = &coverPackage{}
			packages[rel] = p
		}
		for _, b := range prof.Blocks {
			p.stmts += b.NumStmt
			if b.Count > 0 {
				p.covered += b.NumStmt
			}
		}
	}
	var findings []check.Finding
	for _, rel := range slices.Sorted(maps.Keys(packages)) {
		p := packages[rel]
		want := threshold(rel)
		if p.stmts == 0 || p.covered*100 >= want*p.stmts {
			continue
		}
		msg := fmt.Sprintf("%s of %d statements covered, below %d%% (S9)", percent(p.covered, p.stmts), p.stmts, want)
		findings = append(findings, check.Finding{Where: rel, Rule: "coverage", Msg: msg})
	}
	return findings, nil
}

// inCoverTrees reports whether the package at rel, relative to the
// module root, is one S9 measures.
func inCoverTrees(rel string) bool {
	return slices.ContainsFunc(coverTrees, func(tree string) bool { return below(rel, tree) })
}

// threshold returns the percentage S9 asks of the package at rel.
func threshold(rel string) int {
	for _, m := range coverStrictModules {
		if below(rel, "internal/"+m) {
			return coverStrict
		}
	}
	return coverFloor
}

// below reports whether rel is dir or a path inside it.
func below(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

// percent prints a share with one decimal, rounded down, so that a
// share below a threshold never prints as the threshold itself.
func percent(part, whole int) string {
	tenths := part * 1000 / whole
	return fmt.Sprintf("%d.%d%%", tenths/10, tenths%10)
}

// modulePath reads the module path from the module line of a go.mod
// file.
func modulePath(goMod []byte) (string, error) {
	for line := range strings.SplitSeq(string(goMod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", errors.New("go.mod has no module line")
}

// At applies coverage to the profile at path, for the module of
// the go.mod in root.
func At(root, path string) ([]check.Finding, error) {
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	module, err := modulePath(goMod)
	if err != nil {
		return nil, err
	}
	profile, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the cover profile of the race step: %w", err)
	}
	return coverage(profile, module)
}

// Run applies the thresholds of S9 to a cover profile a
// developer wrote, as "go test -coverprofile=cover.out ./..." does.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 1 {
		return false, fmt.Errorf("coverage takes the path of a cover profile\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	path := args[0]
	if !filepath.IsAbs(path) {
		path = filepath.Join(e.Dir, path)
	}
	findings, err := At(root, path)
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("coverage", len(findings)))
	return len(findings) == 0, nil
}

// After is the coverage check of all, on the profile the race step
// wrote, for the module of the go.mod in root (At).
func After(root, profile string) check.Verdict {
	findings, err := At(root, profile)
	return check.Verdict{Name: "coverage", Findings: findings, Err: err}
}
