// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"
)

// maxListOutput is the most output of sbx ls --json a parser reads
// (REQ-0271): a document from a subprocess is decoded with a size cap.
// A host with a thousand sandboxes writes a few hundred KiB.
const maxListOutput = 1 << 20

// Sandbox is a sandbox as sbx ls --json lists it: the members romeu
// reads. Every other member is skipped (03, opening, REQ-0163).
//
// The members come from the migration table of the sbx v0.48.0 release,
// which says that sbx ls --json is a sandbox array in snake_case with
// created_at and last_used_at in RFC 3339 and a detached member
// (https://docs.docker.com/ai/sandboxes/workflows/automation/, "Update
// scripts for v0.48.0"); the name is the one its jq example reads
// (.[].name). The status, the workspaces and the id come from what
// daily use showed and are assumed, which probe A4 records.
type Sandbox struct {
	// Name is the name of the sandbox, which is its project's name.
	Name string
	// Status is what sbx says the sandbox is doing: running, stopped.
	Status string
	// Workspaces are the host paths mounted into it; the first is the
	// primary workspace (https://docs.docker.com/ai/sandboxes/usage/).
	Workspaces []string
	// ID is the id sbx gives the sandbox, when it lists one. I26 records
	// it when probe A4 shows sbx reports one.
	ID string
	// CreatedAt is when sbx created the sandbox, when it lists it.
	CreatedAt time.Time
	// Detached says that the sandbox was kept running in detached mode.
	Detached bool
}

// Running reports whether the sandbox is running.
func (s Sandbox) Running() bool { return s.Status == "running" }

// Stopped reports whether the sandbox is stopped.
func (s Sandbox) Stopped() bool { return s.Status == "stopped" }

// PrimaryWorkspace returns the first workspace, or "" for a sandbox
// with none.
func (s Sandbox) PrimaryWorkspace() string {
	if len(s.Workspaces) == 0 {
		return ""
	}
	return s.Workspaces[0]
}

// wireSandbox is the sandbox as sbx writes it. A member romeu reads is
// required: it is a pointer, so that a member that is missing and one
// that is null are both nil, and sandbox refuses a nil one.
type wireSandbox struct {
	Name       *string    `json:"name"`
	Status     *string    `json:"status"`
	Workspaces *[]string  `json:"workspaces"`
	ID         *string    `json:"id"`
	CreatedAt  *time.Time `json:"created_at"`
	Detached   *bool      `json:"detached"`
}

// ParseList reads the standard output of sbx ls --json: a JSON array of
// sandboxes. Unknown members are skipped; a duplicate member name, a
// missing or mistyped member that is read, a document cut short, a top
// level that is not an array (the object {"sandboxes": [...]} of sbx
// before v0.48.0 among them) or output over the cap is the error
// upstream-shape (RJ-310), never an empty list: only the array [] is
// an empty list (03, opening, REQ-0170; 01 1.6, REQ-0102).
func ParseList(out []byte) ([]Sandbox, error) {
	if len(out) > maxListOutput {
		return nil, upstreamShape("ls --json", fmt.Sprintf("the output is longer than %d bytes", maxListOutput))
	}
	if first := firstByte(out); first != '[' {
		return nil, upstreamShape("ls --json", "the document is not an array of sandboxes")
	}
	var wire []*wireSandbox
	if err := jsonv2.Unmarshal(out, &wire); err != nil {
		return nil, upstreamShape("ls --json", describe(err))
	}
	list := make([]Sandbox, 0, len(wire))
	for i, w := range wire {
		s, err := w.sandbox()
		if err != nil {
			return nil, upstreamShape("ls --json", fmt.Sprintf("sandbox %d: %v", i+1, err))
		}
		list = append(list, s)
	}
	return list, nil
}

// sandbox converts the wire form, refusing what a reader of it cannot
// use.
func (w *wireSandbox) sandbox() (Sandbox, error) {
	switch {
	case w == nil:
		return Sandbox{}, errors.New("the element is null")
	case w.Name == nil || *w.Name == "":
		return Sandbox{}, errors.New("no name")
	case w.Status == nil || *w.Status == "":
		return Sandbox{}, errors.New("no status")
	case w.Workspaces == nil:
		return Sandbox{}, errors.New("no workspaces")
	}
	s := Sandbox{Name: *w.Name, Status: *w.Status, Workspaces: *w.Workspaces}
	if w.ID != nil {
		s.ID = *w.ID
	}
	if w.CreatedAt != nil {
		s.CreatedAt = *w.CreatedAt
	}
	if w.Detached != nil {
		s.Detached = *w.Detached
	}
	return s, nil
}

// firstByte returns the first byte of out that is not white space, or 0.
func firstByte(out []byte) byte {
	for _, c := range out {
		switch c {
		case ' ', '\t', '\r', '\n':
		default:
			return c
		}
	}
	return 0
}

// describe says what the decoder found, without the text of the output.
func describe(err error) string {
	var syn *jsontext.SyntacticError
	var sem *jsonv2.SemanticError
	switch {
	case errors.As(err, &syn):
		return "the document is not JSON, or is cut short, or repeats a member name"
	case errors.As(err, &sem):
		return "a member has the wrong type or is missing: " + firstLine(err.Error())
	}
	return "the document does not decode"
}

// firstLine returns the first line of s, cut at 200 bytes.
func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
