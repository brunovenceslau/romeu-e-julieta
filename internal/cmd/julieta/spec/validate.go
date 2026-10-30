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
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "spec", "validate", true
	c.New = func(env *cli.Env) *cobra.Command {
		var withCatalog bool
		cmd := &cobra.Command{
			Use:   "validate <file>...",
			Short: "check project specs without the host settings",
			Example: "julieta spec validate projects/foo.yaml\n" +
				"julieta spec validate --catalog projects/*.yaml",
			Long: "Decodes each project spec strictly and checks it against the schema and the name and path rules of 03 3.1, " +
				"each file alone and the files together, and prints one line naming the rules only romeu checks, since they read host settings. " +
				"Every entry of a projects/ directory that holds a file given and is not *.yaml, a subdirectory or a dotfile among them, is a finding. " +
				"So is a spec whose manifest would be at least 32 KiB over the cap: julieta measures it with stand-ins for what only romeu knows, " +
				"a lower bound, and romeu sync measures it again. " +
				"With --catalog it also reads each repo's mise.lock at the tip of its ref with a shallow fetch from origin and reports each backend:tool " +
				"the embedded egress catalog and the spec's egress.tools lack, a finding, and a lock that does not parse or that gitsafe refuses, a finding too; " +
				"it warns lock-host-gated for each lock host the catalog does not produce, and for an artifact URL with no host it can read. " +
				"A pass is no gate on those hosts: romeu sync gates them. --catalog fetches from origins the specs name, so the config repo's CI " +
				"runs it for trusted authors only, or under restricted egress. " +
				"An origin it cannot read, or one that lacks the ref or a default branch, is fetch-failed, exit 1. " +
				"Any finding is exit 6; a file that cannot be read is exit 2. " +
				"With --json it prints one document of the format julieta-spec.v1, the warnings among its members, and stderr is then empty but for an error.",
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

// finding is one finding of the command: the file, the place of the
// value in it (empty for the whole spec), the slug of its rule and why.
type finding struct {
	file, path, rule, message string
}

// String returns the finding as the text output names it:
// "<file>: <path>: <message>", or "<file>: <message>" without a path.
func (f finding) String() string {
	if f.path == "" {
		return f.file + ": " + f.message
	}
	return f.file + ": " + f.path + ": " + f.message
}

// run validates the files of args; fetch, when it is not nil, reads the
// mise.lock of a repo for --catalog.
func run(ctx context.Context, env *cli.Env, args []string, fetch lockFetcher) error {
	files, added, err := readFiles(args)
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
			findings = append(findings, finding{file: f.File, path: f.Path, rule: f.Rule, message: f.Message})
		}
	}
	cat, err := catalog.Embedded()
	if err != nil {
		return fmt.Errorf("the embedded egress catalog: %w", err)
	}
	if fetch != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, catalogTimeout)
		defer cancel()
	}
	var warnings []setup.Warning
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
			warnings = append(warnings, gated...)
		}
		size, err := manifestFinding(f.Path, p, eg)
		if err != nil {
			return err
		}
		findings = append(findings, size...)
	}
	rep := newReport(files, findings, warnings)
	if env.JSON {
		if err := writeJSON(env, rep); err != nil {
			return err
		}
	} else if err := writeText(env, rep, findings, warnings, added); err != nil {
		return err
	}
	return findingsError(findings)
}

// writeText prints each warning on stderr, then each finding, the line
// of the rules not checked and a summary line on stdout. The summary
// counts the files judged, and among them the entries of projects/ the
// command added (REQ-0423).
func writeText(env *cli.Env, rep Report, findings []finding, warnings []setup.Warning, added int) error {
	for _, w := range warnings {
		// lock-host-gated is the one warning of the command, and
		// TestWarningSlugs reads the slug of every call as a literal of
		// the list of 04 4.1, so the call names it rather than w.Slug.
		cli.Warn(env.Stderr, "lock-host-gated", termsafe.Escape(w.Message))
	}
	for _, f := range findings {
		if _, err := fmt.Fprintf(env.Stdout, "finding %s %s\n", f.rule, termsafe.Escape(f.String())); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(env.Stdout, notChecked()); err != nil {
		return err
	}
	from := ""
	if added > 0 {
		from = fmt.Sprintf(" (%d from %s/)", added, projectsDir)
	}
	_, err := fmt.Fprintf(env.Stdout, "spec validate %d files%s %d findings\n", len(rep.Files), from, len(rep.Findings))
	return err
}

// findingsError returns nil for no finding, and otherwise
// check-findings (RJ-328), exit 6, whose details list each finding by
// its slug (REQ-0435).
func findingsError(findings []finding) error {
	if len(findings) == 0 {
		return nil
	}
	details := make([]cli.Detail, len(findings))
	var slugs []string
	for i, f := range findings {
		details[i] = cli.Detail{Key: termsafe.Escape(f.String()), Value: f.rule}
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
// config repo's CI never names one (REQ-0423). Every such entry is a
// finding, whatever it is: a subdirectory, a dotfile such as .gitkeep,
// or a link, as sync judges the tree of projects/ (03 3.1). A file that cannot be
// read, or a directory given, is usage, exit 2.
func readFiles(args []string) (files []projectspec.File, added int, err error) {
	given := map[string]bool{}
	var dirs []string
	for _, a := range args {
		data, err := readCapped(a)
		if err != nil {
			return nil, 0, unreadable(a, err)
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
			return nil, 0, unreadable(d, err)
		}
		for _, e := range entries {
			p := filepath.ToSlash(filepath.Join(d, e.Name()))
			if path.Ext(p) != ".yaml" && !given[p] {
				given[p] = true
				files = append(files, projectspec.File{Path: p})
				added++
			}
		}
	}
	return files, added, nil
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
