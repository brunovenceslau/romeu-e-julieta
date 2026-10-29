// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package callsites is the call-site check of 05 5.2 (I16, REQ-0481,
// REQ-0499, REQ-0500) and of 04 4.1 (Subprocesses, item 4). It reads
// the Go files of the module with go/parser and fails:
//
//   - in the packages cmd/romeu links, shared ones included, each call
//     of the kinds of I16 (os.Remove*, rename-over, O_TRUNC, update-ref
//     -d, branch -D, gc --prune, sbx env rm) outside a function the
//     admitted-call-site table (Admitted) names;
//   - in the packages either binary links, each call of the runner
//     (procgroup.Run) outside a package the table names;
//   - in the module, exec.Command and exec.CommandContext outside the
//     function the table names, and the imports and calls that start a
//     process or map a signal around internal/procgroup (the scan of
//     procgroup.go).
//
// Raw system calls (syscall and unix Syscall, Syscall6, RawSyscall and
// RawSyscall6) are a kind of their own: the scan cannot read what a
// trap number does, so one outside the table fails. A dot import of a
// package whose names the lists hold makes its bare identifiers count.
//
// What it does not see:
//   - a package a binary links through a build constraint it does not
//     read is read anyway, which only widens the scan;
//   - a call through an argv built from variables;
//   - a deletion that the argument of "update-ref --stdin" carries;
//   - the git deletions 05 5.2 does not list: tag -d, push --delete,
//     symbolic-ref --delete, reflog expire, worktree remove and prune;
//   - the truncation of os.Create, os.WriteFile and (*os.Root).WriteFile,
//     which name no flag and which the tree uses widely for new files,
//     and of the Truncate method of an os.File, whose name a time.Time
//     shares;
//   - a method of another type whose name is one of methodHits (the
//     Remove of container/list), which is a false finding.
package callsites

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The entry packages of the two binaries.
const (
	romeuEntry   = "cmd/romeu"
	julietaEntry = "cmd/julieta"
)

// i16Kinds are the kinds the scan of I16 reads in the packages romeu
// links.
var i16Kinds = []Kind{KindRemove, KindRename, KindTruncate, KindSyscall, KindUpdateRefDelete, KindBranchDelete, KindGCPrune, KindEnvRm}

// Scan judges the module at root, whose path is module, against the
// table, and returns the findings sorted by place.
func Scan(root, module string, table []Site) ([]check.Finding, error) {
	t, err := parseModule(root, module)
	if err != nil {
		return nil, err
	}
	if len(t.dirs[romeuEntry]) == 0 || len(t.dirs[julietaEntry]) == 0 {
		return nil, fmt.Errorf("%s and %s are not both packages of the module", romeuEntry, julietaEntry)
	}
	romeu := t.closure(romeuEntry)
	both := maps.Clone(romeu)
	maps.Copy(both, t.closure(julietaEntry))
	all := map[string]bool{}
	for dir := range t.dirs {
		all[dir] = true
	}
	findings := slices.Concat(
		execFindings(t),
		siteFindings(t, all, []Kind{KindExec}, table),
		siteFindings(t, both, []Kind{KindRunner}, table),
		siteFindings(t, romeu, i16Kinds, table),
	)
	slices.SortStableFunc(findings, func(a, b check.Finding) int {
		return strings.Compare(a.Where+a.Msg, b.Where+b.Msg)
	})
	return findings, nil
}

// modulePath returns the path of the module line of the go.mod at root.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	module := modfile.ModulePath(data)
	if module == "" {
		return "", fmt.Errorf("go.mod has no module line")
	}
	return module, nil
}

// Steps returns the one step of the check: the subcommand, run with the
// pinned go command.
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "callsites",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "run", "./tools/ci", "callsites"},
		Environ: check.StepEnv(p.Tools.GoDir),
	}}
}

// Run is the subcommand "callsites".
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("callsites takes no argument\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	return report(e.Stdout, root)
}

// report scans the module at root and prints the findings and the
// summary line to out.
func report(out io.Writer, root string) (bool, error) {
	module, err := modulePath(root)
	if err != nil {
		return false, err
	}
	findings, err := Scan(root, module, Admitted)
	if err != nil {
		return false, err
	}
	check.Say(out, "%s%s\n", check.Lines(findings), check.Summary("callsites", len(findings)))
	return len(findings) == 0, nil
}
