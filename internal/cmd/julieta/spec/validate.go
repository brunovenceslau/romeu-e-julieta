// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package spec holds the command julieta spec validate (04 4.3,
// REQ-0422, REQ-0423): the checks of the config repo's project specs
// that need no host settings, which the config repo's CI runs (02 2.2,
// REQ-0134). It decodes and validates the files given, each alone and
// together, with the validators of internal/spec, names the rules of
// 03 3.1 only romeu checks, measures the manifest each spec would make
// (manifest.go), and with --catalog looks every tool of each repo's
// mise.lock up in the embedded egress catalog (catalog.go).
package spec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

var _ = cli.Register(func() cli.Command {
	// validate reads the files it is given and, with --catalog, the
	// origins of their repos; it writes nothing but a temporary clone.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.Sub = "julieta", "spec", "validate"
	c.New = func(env *cli.Env) *cobra.Command {
		var withCatalog bool
		cmd := &cobra.Command{
			Use:   "validate <file>...",
			Short: "check project specs without the host settings",
			Example: "julieta spec validate projects/foo.yaml\n" +
				"julieta spec validate --catalog projects/*.yaml",
			Long: "Decodes each project spec strictly and checks it against the schema and the name and path rules of 03 3.1, " +
				"each file alone and the files together, and prints one line naming the rules only romeu checks, since they read host settings. " +
				"A file under projects/ that is not *.yaml, and a spec whose manifest would pass 32 KiB, are findings too. " +
				"With --catalog it also reads each repo's mise.lock at its ref from origin and reports each backend:tool the embedded egress catalog " +
				"and the spec's egress.tools lack, a finding, and each lock host the catalog does not produce, which sync gates. " +
				"Any finding is exit 6; a file that cannot be read is exit 2.",
			Args: cobra.MinimumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				var fetch lockFetcher
				if withCatalog {
					home, err := setup.Home()
					if err != nil {
						return err
					}
					g, err := setup.NewGit(home)
					if err != nil {
						return err
					}
					fetch = gitFetcher(g)
				}
				return run(c.Context(), env, args, fetch)
			},
		}
		cmd.Flags().BoolVar(&withCatalog, "catalog", false, "also read each repo's mise.lock at its ref and look its tools up in the embedded egress catalog")
		return cmd
	}
	return c
}())

// projectsDir is the directory of the config repo that holds the specs
// (02 2.2).
const projectsDir = "projects"

// finding is one finding of the command: the slug of its rule and the
// line that names where it is and why.
type finding struct {
	rule, text string
}

// run validates the files of args; fetch, when it is not nil, reads the
// mise.lock of a repo for --catalog.
func run(ctx context.Context, env *cli.Env, args []string, fetch lockFetcher) error {
	files, err := readFiles(args)
	if err != nil {
		return err
	}
	var findings []finding
	if _, err := projectspec.LoadSet(files, nil); err != nil {
		inv, ok := errors.AsType[*projectspec.InvalidError](err)
		if !ok {
			return err
		}
		for _, f := range inv.Findings {
			findings = append(findings, finding{rule: f.Rule, text: strings.TrimSuffix(f.String(), " ("+f.Rule+")")})
		}
	}
	cat, err := catalog.Embedded()
	if err != nil {
		return fmt.Errorf("the embedded egress catalog: %w", err)
	}
	var notes []string
	for _, f := range files {
		if path.Ext(f.Path) != ".yaml" {
			continue
		}
		p, err := projectspec.DecodeProject(f.Data)
		if err != nil {
			continue // LoadSet has reported it
		}
		eg := baseEgress(cat, p)
		if fetch != nil {
			found, gated, err := checkCatalog(ctx, fetch, cat, f.Path, p, eg)
			if err != nil {
				return err
			}
			findings = append(findings, found...)
			notes = append(notes, gated...)
		}
		size, err := manifestFinding(f.Path, p, eg)
		if err != nil {
			return err
		}
		findings = append(findings, size...)
	}
	return report(env, len(files), findings, notes)
}

// report prints each finding, each note of --catalog and the line of
// the rules not checked on stdout, then a summary line; any finding is
// check-findings (RJ-328), exit 6, whose details list each by its slug
// (REQ-0435).
func report(env *cli.Env, files int, findings []finding, notes []string) error {
	for _, f := range findings {
		if _, err := fmt.Fprintf(env.Stdout, "finding %s %s\n", f.rule, termsafe.Escape(f.text)); err != nil {
			return err
		}
	}
	for _, n := range notes {
		if _, err := fmt.Fprintln(env.Stdout, n); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(env.Stdout, notChecked()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(env.Stdout, "spec validate %d files %d findings\n", files, len(findings)); err != nil {
		return err
	}
	if len(findings) == 0 {
		return nil
	}
	details := make([]cli.Detail, len(findings))
	var slugs []string
	for i, f := range findings {
		details[i] = cli.Detail{Key: termsafe.Escape(f.text), Value: f.rule}
		if !slices.Contains(slugs, f.rule) {
			slugs = append(slugs, f.rule)
		}
	}
	return cli.CheckFindings.New(fmt.Sprintf("the specs have %d findings: %s", len(findings), strings.Join(slugs, ", ")), details...)
}

// notChecked returns the line that names the rules of 03 3.1 only romeu
// checks, since they read host settings (REQ-0174).
func notChecked() string {
	var names []string
	for _, r := range projectspec.HostOnly() {
		names = append(names, r.Slug+" ("+r.Source+")")
	}
	return "not checked, since they read host settings: " + strings.Join(names, ", ")
}

// readFiles reads the files of args, each under the cap of a spec file
// and one byte more, so the decoder refuses a longer one as a finding.
// For an argument in a directory named projects, every other entry of
// that directory that is not *.yaml is added with no content, so the
// rule of REQ-0178 judges it: the shell glob projects/*.yaml of the
// config repo's CI never names one (REQ-0423). A file that cannot be
// read, or a directory given, is usage, exit 2.
func readFiles(args []string) ([]projectspec.File, error) {
	var files []projectspec.File
	given := map[string]bool{}
	var dirs []string
	for _, a := range args {
		data, err := readCapped(a)
		if err != nil {
			return nil, unreadable(a, err)
		}
		p := filepath.ToSlash(filepath.Clean(a))
		if given[p] {
			continue
		}
		given[p] = true
		files = append(files, projectspec.File{Path: p, Data: data})
		if d := filepath.Dir(a); filepath.Base(d) == projectsDir && !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			return nil, unreadable(d, err)
		}
		for _, e := range entries {
			p := filepath.ToSlash(filepath.Join(d, e.Name()))
			if path.Ext(p) != ".yaml" && !given[p] {
				given[p] = true
				files = append(files, projectspec.File{Path: p})
			}
		}
	}
	return files, nil
}

// readCapped reads the regular file name, at most one byte over the cap
// of a spec file. It is judged before it is opened, so a FIFO given in
// error does not block the open.
func readCapped(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, projectspec.MaxFileSize+1))
}

// unreadable returns usage, exit 2, for a file that cannot be read.
func unreadable(name string, err error) error {
	e := cli.Usage.New("julieta spec validate cannot read " + termsafe.Escape(name) + ": " + termsafe.Escape(err.Error()))
	e.Fill = map[string]string{"<binary>": "julieta", " <command>": " spec validate"}
	return e
}
