// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package imports

import (
	"path"
	"strings"
)

// Binaries is a set of the two binaries, as the second column of the
// table of 10 10.7 names them: the binaries that may link a package.
type Binaries uint8

// The two binaries. A row whose second column is "tools/ci only" lets
// neither link its packages, and so has the empty set.
const (
	Romeu Binaries = 1 << iota
	Julieta
)

// Names returns the binaries of b, in the order romeu, julieta.
func (b Binaries) Names() []string {
	var names []string
	if b&Romeu != 0 {
		names = append(names, "romeu")
	}
	if b&Julieta != 0 {
		names = append(names, "julieta")
	}
	return names
}

// Row is one row of the table of 10 10.7.
type Row struct {
	// Req is the requirement id the row holds.
	Req string
	// Names are the packages of the first column, each a path below
	// internal/; "agent/*" names each package one level below
	// internal/agent (path.Match).
	Names []string
	// LinkedBy is the second column: the binaries that may link a
	// package of the row.
	LinkedBy Binaries
	// MustNot is the third column: the packages below internal/ that a
	// package of the row must not import, directly or through another.
	MustNot []string
	// TestOnly says that the second column is "test packages only": only
	// a test file imports a package of the row (testOnlyFindings).
	TestOnly bool
}

// Rows is the table of 10 10.7 (REQ-0968, REQ-1233, REQ-0969, REQ-1247,
// REQ-1248, REQ-1256), in its order. The tools/ci of the second column of the
// first row and the "tools/ci only" of the fourth are read through
// REQ-0971, which the tools rule (toolFindings) holds: the rows govern
// the two binaries.
//
// The second column, not the third, is the source of the strict
// reading: "The second column says which binary may link a package, so
// a package of the last row imports none of the second: romeu links
// it." A package imports only packages that every binary that may link
// it may link too, so a julieta package that imports a romeu-only
// package is flagged even where no third column names it.
var Rows = []Row{
	{
		Req:      "REQ-0968",
		Names:    []string{"sbxdrv", "render", "gate", "state", "egress", "signing", "preflight"},
		LinkedBy: Romeu,
		MustNot:  []string{"mise", "hooks", "layout"},
	},
	{
		Req:      "REQ-1233",
		Names:    []string{"mise", "hooks", "hooks/actions", "layout", "agent/*", "memstore/write"},
		LinkedBy: Julieta,
		MustNot:  []string{"sbxdrv"},
	},
	{
		Req: "REQ-0969",
		Names: []string{"gitsafe", "spec", "canon", "catalog", "oci", "manifest", "memstore", "handoff",
			"salvage", "shquote", "termsafe", "cli", "procgroup"},
		LinkedBy: Romeu | Julieta,
		MustNot:  []string{"sbxdrv", "layout"},
	},
	{
		Req:      "REQ-1247",
		Names:    []string{"cli/clidoc"},
		LinkedBy: 0,
		MustNot:  []string{"sbxdrv", "layout"},
	},
	{
		Req:      "REQ-1248",
		Names:    []string{"cli/testsupport"},
		LinkedBy: 0,
		MustNot:  []string{"sbxdrv", "layout"},
		TestOnly: true,
	},
	{
		Req:      "REQ-1256",
		Names:    []string{"moduleroot"},
		LinkedBy: 0,
		MustNot:  []string{"sbxdrv", "layout"},
	},
}

// commandRows maps the directory of the command packages of each binary
// to the id of the row they follow (REQ-0970): a command package under
// internal/cmd/romeu/ follows the row of romeu, one under
// internal/cmd/julieta/ the row of julieta, internal/cmd/<binary>/all
// among them.
var commandRows = map[string]string{
	"cmd/romeu/":   "REQ-0968",
	"cmd/julieta/": "REQ-1233",
}

// rowByReq returns the row whose id is req.
func rowByReq(req string) Row {
	for _, row := range Rows {
		if row.Req == req {
			return row
		}
	}
	panic("no row of 10 10.7 has the id " + req)
}

// RowOf returns the row that names name, a path below internal/, and
// false when no row names it (REQ-0972). A subpackage follows its own
// row, never its parent's (REQ-0971), so only a name written out, or
// matched by "agent/*", counts.
func RowOf(name string) (Row, bool) {
	for prefix, req := range commandRows {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" {
			return rowByReq(req), true
		}
	}
	for _, row := range Rows {
		for _, pattern := range row.Names {
			if ok, _ := path.Match(pattern, name); ok {
				return row, true
			}
		}
	}
	return Row{}, false
}

// toolImports are the packages below internal/ that tools/** and the
// packages under e2e/** may import directly (REQ-0971): procgroup, and
// the one package per binary that links every command in.
var toolImports = []string{"procgroup", "cmd/romeu/all", "cmd/julieta/all"}

// toolExtra are the further packages below internal/ that one tree of
// tools imports directly (REQ-0971): tools/ci the clidoc package, which
// no binary links, and the manifest package, for the type of the
// manifest schema its generator writes; tools/new the cli package, for
// the command list and the error table its command and error kinds
// read; tools/planviewer the gitsafe package, for the read-only
// history its Trends view shows; and tools/ci and e2e the moduleroot
// package, which finds the module root for their tests (REQ-1256).
// A tree is the directory and every package below it.
var toolExtra = map[string][]string{
	"tools/ci":         {"cli/clidoc", "manifest", "moduleroot"},
	"e2e":              {"moduleroot"},
	"tools/new":        {"cli"},
	"tools/planviewer": {"gitsafe"},
}

// The packages below internal/ that cmd/romeu links none of: the three
// the I1 row of 05 5.2 lists, the one I2 names because it configures
// hooks, and the writer of the memory store that I24 names (10 10.7).
var (
	hostTerminal = []string{"layout", "hooks", "mise"}
	hostHooks    = []string{"hooks"}
	hostMemWrite = []string{"memstore/write"}
)

// renderForbidden is the one package internal/render must not reach
// (I11): YAML and JSON are written only through marshallers.
const renderForbidden = "text/template"

// cobraModule is the module whose packages the I11 rule exempts by
// name: internal/cli links cobra, and cobra's template help is not used
// (ADR 0015, rule 5).
const cobraModule = "github.com/spf13/cobra"
