// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// ChecksPath is the inventory of the checks that generate writes from
// the registry (12 12.3).
const ChecksPath = "docs/reference/checks.md"

// ChecksPage renders the inventory of checks, one line of its table per
// check, in the order of the registry: the name, what it guards, its
// usage line, where it runs, the job of the hosted workflow it runs in
// (a check that runs in all alone, or in all and fast), its order key, whether it runs code of the
// change, the requirement id of its row in the step table of 10 10.2,
// and a link to its package.
func ChecksPage(checks []Check) []byte {
	var b bytes.Buffer
	b.WriteString("---\ntype: reference\nreader: a contributor who adds or changes a check of tools/ci, or who reads which checks a change meets\n---\n\n")
	b.WriteString(HTMLComment(GeneratedHeader("tools/ci/check_*.go (the registry of tools/ci)")) + "\n")
	b.WriteString("# Checks\n\n")
	b.WriteString("Each check of tools/ci registers itself from one file,\n`tools/ci/check_<check>.go`, and lives in one package below\n`tools/ci/internal/`\n([12 12.2](../spec/12-engineering.md#122-development-commands-and-capability-map)).\n")
	b.WriteString("fast and all run the checks they hold in the order below, except that\nevery check that judges the tree runs before the first one that runs\ncode of the change, and the checks of the commit judge the commit read\nbefore any step.\n")
	b.WriteString("Job is the job of the hosted workflow the check runs in, and `tools/ci <job>` runs the checks of that job alone: `static` judges the commit and the tree and runs none of the tests, `race` is the one test run with the race detector and the coverage check after it, and `e2e` is the container e2e. `tools/ci all` runs the union of the three jobs; `-` marks a check that runs in no job.\n")
	b.WriteString("The requirement id is the one of the row of the check in the step table of\n[10 10.2](../spec/10-testing-style.md#102-ci); `-` marks a row that holds none.\n\n")
	b.WriteString("| Check | Guards | Usage | Runs in | Job | Order | Runs code of the change | Requirement | Package |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range checks {
		req := c.Req
		if req == "" {
			req = "-"
		}
		runsCode := "no"
		if c.RunsCode {
			runsCode = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | `%s` | %s | %s | %d | %s | %s | [%s](../../%s/) |\n",
			c.Name, cell(c.Guards), cell(c.Usage), runsIn(c), jobOf(c), c.Order, runsCode, req, c.Package, c.Package)
	}
	return b.Bytes()
}

// runsIn says where a check runs: fast, all, or the pre-push run of
// fast alone.
func runsIn(c Check) string {
	var in []string
	switch {
	case c.Fast:
		in = append(in, "fast")
	case c.Hook:
		in = append(in, "fast, in the pre-push hook only")
	}
	if c.All {
		in = append(in, "all")
	}
	if len(in) == 0 {
		return "-"
	}
	return strings.Join(in, ", ")
}

// jobOf says the job of a check that runs in all, and "-" for one that
// does not (the pushed range, which runs in the pre-push hook only).
func jobOf(c Check) string {
	if !c.All {
		return "-"
	}
	return c.JobName()
}

// cell escapes the one character of a table cell that Markdown reads as
// its end.
func cell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// ChecksPageOutput is the generator of the inventory of the checks of
// this binary's registry, docs/reference/checks.md (ChecksPage).
func ChecksPageOutput(*os.Root) ([]Output, error) {
	return []Output{{Path: ChecksPath, Content: ChecksPage(Registered())}}, nil
}
