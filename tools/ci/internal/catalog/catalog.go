// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package catalog is the catalog check of 10 10.2 (REQ-0842): the files
// of internal/catalog/egress/ are read as the binaries read them
// (strictly, one file per tool family, merged), and every domain used
// anywhere in them, by the meta and base sets or by a tool, must have an
// explicit upload entry (REQ-0212, REQ-0641). A malformed tool key, a
// domain two files declare with different entries and a file the embed
// glob egress/*.yaml would not read are findings too. It judges the
// commit fast and all read, and is the catalog subcommand.
//
// A tree without the directory and without the package has no catalog to
// judge, as a tree without kits has none to hold to the kit rules. A tree
// that has the package and no file below the directory is a finding: the
// embed pattern would match nothing.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Dir is the directory of the catalog files, from the root of the
// repository.
const Dir = "internal/catalog/egress/"

// Ext is the extension of the files the embed pattern of internal/catalog
// matches.
const Ext = ".yaml"

// Package is the file of internal/catalog that holds the embed directive.
const Package = "internal/catalog/catalog.go"

const rule = "catalog"

// Of applies the rules to the catalog files among files, the tree of one
// commit as check.HeadTree reads it.
func Of(files []check.TreeFile) []check.Finding {
	var findings []check.Finding
	add := func(where, format string, args ...any) {
		findings = append(findings, check.Finding{Where: git.Printable(where), Rule: rule, Msg: fmt.Sprintf(format, args...)})
	}
	var sources []all.CatalogSource
	seen, hasPackage := false, false
	for _, f := range files {
		if f.Path == Package {
			hasPackage = true
		}
		rest, ok := strings.CutPrefix(f.Path, Dir)
		if !ok || f.Content == nil {
			continue
		}
		seen = true
		if strings.Contains(rest, "/") || path.Ext(rest) != Ext {
			add(f.Path, "is not embedded: the files of the catalog are %s*.yaml, one directory deep", Dir)
			continue
		}
		sources = append(sources, all.CatalogSource{Name: rest, Data: f.Content})
	}
	if !seen {
		if hasPackage {
			add(Dir, "no file below %s, so the embed pattern of %s matches nothing", Dir, Package)
		}
		return findings
	}
	c, err := all.CatalogBuild(sources)
	if err != nil {
		for _, e := range unjoin(err) {
			add(Dir, "%v", e)
		}
		return findings
	}
	for _, d := range c.Undeclared() {
		add(Dir, "the domain %s has no upload entry in any file's domains (REQ-0212, REQ-0641)", d)
	}
	return findings
}

// unjoin returns the errors err joins, or err itself.
func unjoin(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range j.Unwrap() {
			out = append(out, unjoin(e)...)
		}
		return out
	}
	return []error{err}
}

// catalogAt judges the tree of head.
func catalogAt(ctx context.Context, repo git.Repo, head string) ([]check.Finding, error) {
	files, err := check.HeadTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	return Of(files), nil
}

// Run is the catalog subcommand.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, errors.New("catalog takes no argument\n" + check.Usage())
	}
	findings, err := catalogAt(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("catalog", len(findings)))
	return len(findings) == 0, nil
}

// Judge is the catalog check of the commit fast and all read before any
// step (Of).
func Judge(_ context.Context, c check.Commit) ([]check.Verdict, error) {
	return []check.Verdict{{Name: "catalog", Findings: Of(c.Files)}}, nil
}
