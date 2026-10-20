// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// The section of julieta's warnings: those the last setup recorded in
// julieta state, fetch-failed and default-branch-moved among them (04
// 4.3, REQ-0399).
var _ = register(section{
	name:  "warnings",
	order: 90,
	fill: func(_ context.Context, src *source, rep *Report) {
		rep.Warnings = append(rep.Warnings, src.state.Warnings...)
	},
	text: func(out *cli.Writer, rep *Report) error {
		for _, w := range rep.Warnings {
			setup.PrintWarning(out, w)
		}
		return nil
	},
})
