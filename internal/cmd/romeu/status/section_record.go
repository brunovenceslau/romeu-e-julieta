// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"context"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The record section: the state of the project's record. The text says
// it only when the record is not present, so a project named on the
// command line that was never synced, or a misspelled name, is not a
// silent block.
var _ = register(section{
	name:  "record",
	order: 5,
	fill: func(_ context.Context, src *source, p *Project) {
		p.Record = src.recorded
	},
	text: func(out *cli.Writer, p *Project) error {
		var err error
		switch p.Record {
		case RecordAbsent:
			_, err = fmt.Fprintln(out, "  record: absent, the project was never synced")
		case RecordUnreadable:
			_, err = fmt.Fprintln(out, "  record: unreadable, see its error below")
		}
		return err
	},
})
