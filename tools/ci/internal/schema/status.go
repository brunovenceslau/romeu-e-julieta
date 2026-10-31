// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The --json document of romeu status (04 4.2), whose Go type lives in
// its command package; tools/ci reaches it through
// internal/cmd/romeu/all (REQ-0971). The schema admits the states of a
// project's record and of the toolchain alone; the sandbox's state is
// the text sbx prints, so it stays a string.
func init() {
	schemagen.Register(schemagen.Schema{Name: "romeu-status.v1", Type: reflect.TypeFor[all.StatusReport](),
		Keywords: []schemagen.Keyword{
			{Path: "projects[].record", Values: map[string]any{"enum": anyOf(all.StatusRecordStates)}},
			{Path: "toolchain.state", Values: map[string]any{"enum": anyOf(all.StatusToolchainStates)}},
		}})
}

// anyOf returns the values of an enum keyword.
func anyOf(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}
