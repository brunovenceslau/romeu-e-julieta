// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
)

// MaxPreToolUse is the most bytes of a PreToolUse document julieta
// decodes; a larger one is refused as undecodable.
const MaxPreToolUse = 1 << 20

// PreToolUseDoc is what julieta hooks guard reads of the document the
// agent sends a PreToolUse hook on stdin (04 4.3, REQ-0410): the tool
// and, for Bash, the command it is about to run.
type PreToolUseDoc struct {
	Tool    string
	Command string
}

// DecodePreToolUse decodes a PreToolUse document, of the shape the
// pinned version sends (the fixture e2e/testdata/agent/2.1.296): a JSON
// object with tool_name, and for the tool Bash tool_input.command. A
// member julieta does not read is skipped; a document that is not that
// shape, holds a duplicate member or invalid UTF-8, names another
// event, or is over MaxPreToolUse bytes is an error, which the guard
// refuses (fail closed).
func DecodePreToolUse(data []byte) (PreToolUseDoc, error) {
	if len(data) > MaxPreToolUse {
		return PreToolUseDoc{}, fmt.Errorf("the document is over %d bytes", MaxPreToolUse)
	}
	var doc struct {
		Event     *string        `json:"hook_event_name"`
		Tool      *string        `json:"tool_name"`
		ToolInput jsontext.Value `json:"tool_input"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return PreToolUseDoc{}, err
	}
	if doc.Event != nil && *doc.Event != PreToolUse {
		return PreToolUseDoc{}, fmt.Errorf("the document is of the event %q, not %s", *doc.Event, PreToolUse)
	}
	if doc.Tool == nil {
		return PreToolUseDoc{}, errors.New("the document has no tool_name")
	}
	p := PreToolUseDoc{Tool: *doc.Tool}
	if p.Tool != "Bash" {
		return p, nil
	}
	if len(doc.ToolInput) == 0 {
		return PreToolUseDoc{}, errors.New("the Bash call has no tool_input")
	}
	var in struct {
		Command *string `json:"command"`
	}
	if err := json.Unmarshal(doc.ToolInput, &in); err != nil {
		return PreToolUseDoc{}, fmt.Errorf("tool_input: %w", err)
	}
	if in.Command == nil {
		return PreToolUseDoc{}, errors.New("the Bash call has no tool_input.command")
	}
	p.Command = *in.Command
	return p, nil
}
