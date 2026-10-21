// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The section of the recorded hook failures (08 8.2, REQ-0675; 04 4.3,
// REQ-0424): every failure in the store below julieta state that julieta
// hooks run and the session hooks record in, and clear when the same
// hook next succeeds in the same repo. It reads the store and writes
// nothing; a file of the store it cannot read is one entry of errors.
var _ = register(section{
	name:  "hook-failures",
	order: 80,
	fill: func(_ context.Context, src *source, rep *Report) {
		failures, err := hooks.Failures(filepath.Join(setup.StateDir(src.home), "hook-failures"))
		if err != nil {
			rep.Errors = append(rep.Errors, "hook failures: "+err.Error())
		}
		rep.HookFailures = make([]HookFailure, 0, len(failures))
		for _, f := range failures {
			rep.HookFailures = append(rep.HookFailures, HookFailure(f))
		}
	},
	text: func(out, _ *cli.Writer, rep *Report) error {
		var b strings.Builder
		for _, f := range rep.HookFailures {
			repo, detail := f.Repo, f.Path
			if repo == "" {
				repo = "(none)"
			}
			if f.Message != "" {
				detail = f.Message
			}
			// Each value is escaped alone, so a line feed in it stays on
			// the failure's line (cli.Writer).
			fmt.Fprintf(&b, "hook failure  %s  repo %s  stage %s  exit %d  at %s  %s\n",
				termsafe.Escape(f.Hook), termsafe.Escape(repo), termsafe.Escape(f.Stage), f.ExitCode,
				f.At.UTC().Format(time.RFC3339), termsafe.Escape(detail))
		}
		_, err := out.Write([]byte(b.String()))
		return err
	},
})
