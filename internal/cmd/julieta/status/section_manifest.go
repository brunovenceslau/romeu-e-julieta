// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The section of the manifest: the project, the live spec commit
// (REQ-0220) and the egress set romeu applied (REQ-0224, 07 7.5).
var _ = register(section{
	name:  "manifest",
	order: 10,
	fill: func(_ context.Context, src *source, rep *Report) {
		rep.Project = src.m.Project
		rep.SpecCommit = src.m.SpecCommit
		rep.Egress = append(rep.Egress, src.m.Egress...)
		slices.Sort(rep.Egress)
	},
	text: func(out, _ *cli.Writer, rep *Report) error {
		_, err := fmt.Fprintf(out, "project  %s\nspec     %s\negress   %s\n", rep.Project, rep.SpecCommit, strings.Join(rep.Egress, " "))
		return err
	},
})
