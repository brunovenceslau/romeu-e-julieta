// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// notes say what each warning of memstore.Check means for the reader.
// 04 4.1, Warnings has no slug for them, so step 5 prints each as a
// note line, as julieta memory check does, and not with cli.Warn.
var notes = map[string]string{
	memstore.RuleTemp:       "a leftover julieta temporary file, never read; julieta setup removes it",
	memstore.RuleHandoffCap: fmt.Sprintf("handoff/ is over its cap of %d files", memstore.MaxHandoffFiles),
}

// checkMemory is step 5 of 04 4.2 (REQ-0340): every memory dir step 4
// made or found, each repo's and the shared one, is checked against
// the layout allowlist of 08 8.1 by memstore.CheckDir, which judges
// each node with Lstat through an os.Root and follows nothing (I12,
// I24). Any finding is memory-layout (exit 2), naming each offending
// path, escaped, and its rule; nothing is changed. A warning of the
// check, such as a leftover .tmp-<ULID>, refuses nothing and joins
// warnings.
func checkMemory(_ context.Context, r *run) error {
	var findings []string
	for _, t := range r.trees {
		dirs := []string{t.shared}
		for _, rt := range t.repos {
			dirs = append(dirs, rt.memory)
		}
		for _, dir := range dirs {
			rep, err := memstore.CheckDir(dir)
			if err != nil {
				return fmt.Errorf("the memory dir %s: %w", termsafe.Escape(dir), err)
			}
			for _, f := range rep.Findings {
				findings = append(findings, f.Rule+" "+termsafe.Escape(filepath.Join(dir, f.Path)))
			}
			for _, f := range rep.Warnings {
				line := "note " + f.Rule + " " + termsafe.Escape(filepath.Join(dir, f.Path))
				if why, ok := notes[f.Rule]; ok {
					line += ": " + why
				}
				r.warnings = append(r.warnings, func(w *cli.Writer) { _, _ = fmt.Fprintln(w, line) })
			}
		}
	}
	if findings == nil {
		return nil
	}
	return usage("memory-layout", "the memory dirs break the layout allowlist of 08 8.1, and romeu follows and changes nothing in them: "+strings.Join(findings, ", "))
}
