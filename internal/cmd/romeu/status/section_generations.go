// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The generations section: each generation of the record with its
// state, `adopted, not gated` for an adopted one, and its salvage
// records (04 4.2, REQ-0327; 01 1.6: status is a projection of these
// records, REQ-0107).
var _ = register(section{
	name:  "generations",
	order: 20,
	fill: func(_ context.Context, src *source, p *Project) {
		if src.record == nil {
			return
		}
		for _, g := range src.record.Generations {
			out := Generation{
				ID:        g.ID.String(),
				State:     string(g.State),
				Adopted:   g.Adopted,
				CreatedAt: g.CreatedAt,
				ClosedAt:  g.ClosedAt,
				Salvage:   []Salvage{},
			}
			for _, s := range g.Salvage {
				out.Salvage = append(out.Salvage, Salvage{
					ID:         s.ID.String(),
					CreatedAt:  s.CreatedAt,
					Result:     s.Result,
					Reasons:    append([]string{}, s.Reasons...),
					RefsPrefix: s.RefsPrefix,
				})
			}
			p.Generations = append(p.Generations, out)
		}
	},
	text: func(out *cli.Writer, p *Project) error {
		var b strings.Builder
		for _, g := range p.Generations {
			fmt.Fprintf(&b, "  generation %s: %s", g.ID, line(g.State))
			if g.Adopted {
				b.WriteString(" (adopted, not gated)")
			}
			fmt.Fprintf(&b, ", created %s", g.CreatedAt.UTC().Format(time.RFC3339))
			if !g.ClosedAt.IsZero() {
				fmt.Fprintf(&b, ", closed %s", g.ClosedAt.UTC().Format(time.RFC3339))
			}
			b.WriteString("\n")
			for _, s := range g.Salvage {
				fmt.Fprintf(&b, "    salvage %s: %s, %s, %s", s.ID, line(s.Result), s.CreatedAt.UTC().Format(time.RFC3339), line(s.RefsPrefix))
				if len(s.Reasons) > 0 {
					fmt.Fprintf(&b, "; %s", line(strings.Join(s.Reasons, ", ")))
				}
				b.WriteString("\n")
			}
		}
		_, err := out.Write([]byte(b.String()))
		return err
	},
})
