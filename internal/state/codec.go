// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// MaxRecordSize is the most bytes of a record a reader takes: 16 MiB.
// doctor warns past 256 KiB (REQ-0231), so a record this large is a
// defect or damage, and the cap keeps a reader's memory bounded.
const MaxRecordSize = 16 << 20

// ErrInvalid is the error of a document that is not a record of the
// format this build reads: it does not decode, names another format or
// an older version, passes the size cap, or holds a value of the wrong
// form. A reader of a file turns it into record-unreadable (RJ-334).
var ErrInvalid = errors.New("state: not a record this build reads")

// The decoding rule of the state formats, on the strict side of ADR 0041.
// romeu writes every state record and the same binary reads it back, so
// the Consequences of the ADR keep it strict, and REQ-0163 does not list
// these files among the formats between binaries: an unknown member, a
// duplicate member name, text that is not UTF-8, trailing data, the size
// cap and a value of the wrong form each make the record unreadable
// (RJ-334), and the file is never written over. A newer version is read
// first from the schema field and is format-newer (RJ-311), also before
// any other check, so a record a newer romeu wrote says so instead of
// failing on its new members. The schemas keep additionalProperties:
// false for the same reason.
func decodeRecord[T any](data []byte, name string, version int, into *T) error {
	if len(data) > MaxRecordSize {
		return fmt.Errorf("%w: %d bytes pass the cap of %d", ErrInvalid, len(data), MaxRecordSize)
	}
	var head struct {
		Schema *string `json:"schema"`
	}
	if err := jsonv2.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if head.Schema == nil {
		return fmt.Errorf("%w: no schema member", ErrInvalid)
	}
	if err := acceptSchema(*head.Schema, name, version); err != nil {
		return err
	}
	if err := jsonv2.Unmarshal(data, into, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return nil
}

// acceptSchema checks the schema field of a record against the format
// name and the version this build reads, "<name>/v<n>" (REQ-0161): a
// newer version is format-newer, and another name, form or an older
// version is invalid. A state format has no previous version to read
// yet (REQ-0164), so the window is the current version alone.
func acceptSchema(got, name string, version int) error {
	want := name + "/v" + strconv.Itoa(version)
	n, ok := versionOf(got, name)
	switch {
	case !ok:
		return fmt.Errorf("%w: the schema %q is not %s/v<n>", ErrInvalid, clip(got), name)
	case n > version:
		return manifest.FormatNewer.New(fmt.Sprintf("the record is %s, newer than %s, the version this build reads", got, want))
	case n < version:
		return fmt.Errorf("%w: the schema %s is older than %s, and this build reads no earlier version", ErrInvalid, got, want)
	}
	return nil
}

// versionDigits is the n of a schema field, without a leading zero.
var versionDigits = regexp.MustCompile(`^([1-9][0-9]{0,8})$`)

// versionOf reads the n of a schema field "<name>/v<n>", n without a
// leading zero.
func versionOf(field, name string) (int, bool) {
	rest, ok := strings.CutPrefix(field, name+"/v")
	if !ok {
		return 0, false
	}
	sub := versionDigits.FindStringSubmatch(rest)
	if sub == nil {
		return 0, false
	}
	n, err := strconv.Atoi(sub[1])
	return n, err == nil
}

// clip shortens text that goes into an error.
func clip(s string) string {
	if len(s) > 64 {
		return s[:64] + "..."
	}
	return s
}

// encodeRecord writes a record as two-space indented JSON, the members
// of a map sorted and a final newline: the form every state file has,
// so a record that did not change is the same bytes.
func encodeRecord(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := jsonv2.MarshalWrite(&b, v, jsonv2.Deterministic(true),
		jsontext.Multiline(true), jsontext.WithIndent("  ")); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// DecodeToolchain reads toolchain.json (REQ-0228).
func DecodeToolchain(data []byte) (*Toolchain, error) {
	var t Toolchain
	if err := decodeRecord(data, toolchainFormat, ToolchainVersion, &t); err != nil {
		return nil, err
	}
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &t, nil
}

// EncodeToolchain writes toolchain.json, after the checks a reader
// makes. Every timestamp of the record is set to UTC at whole seconds
// first (REQ-0162), in the record the caller passed.
func EncodeToolchain(t *Toolchain) ([]byte, error) {
	t.AcknowledgedAt = stamp(t.AcknowledgedAt)
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("state: a toolchain record that would not read: %w", err)
	}
	return encodeRecord(t)
}

// DecodeProject reads projects/<name>.json (REQ-0229).
func DecodeProject(data []byte) (*Project, error) {
	var p Project
	if err := decodeRecord(data, projectFormat, ProjectVersion, &p); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &p, nil
}

// EncodeProject writes projects/<name>.json, after the checks a reader
// makes. Every timestamp of the record is set to UTC at whole seconds
// first (REQ-0162), in the record the caller passed.
func EncodeProject(p *Project) ([]byte, error) {
	p.stampTimes()
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("state: refused to write a project record that would not read, by the check a reader makes: %w", err)
	}
	return encodeRecord(p)
}

// DecodeDescriptor reads descriptors/<digest>.json (REQ-0240).
func DecodeDescriptor(data []byte) (*Descriptor, error) {
	var d Descriptor
	if err := decodeRecord(data, descriptorFormat, DescriptorVersion, &d); err != nil {
		return nil, err
	}
	if err := d.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &d, nil
}

// EncodeDescriptor writes descriptors/<digest>.json, after the checks a
// reader makes.
func EncodeDescriptor(d *Descriptor) ([]byte, error) {
	if err := d.validate(); err != nil {
		return nil, fmt.Errorf("state: a descriptor record that would not read: %w", err)
	}
	return encodeRecord(d)
}

// NewProject returns the record of a project that has none: the schema
// field and nothing else.
func NewProject() *Project { return &Project{Schema: ProjectField} }

// ErrInvalidName is the error of a project name that is not one (03 3.1,
// REQ-0175): it would be a path in the state directory.
var ErrInvalidName = errors.New("state: not a project name")

// projectName is the pattern of the rule project-name of internal/spec,
// the one home of the rule (REQ-0172).
var projectName = sync.OnceValue(func() *regexp.Regexp {
	rules := spec.Rules()
	i := slices.IndexFunc(rules, func(r spec.Rule) bool { return r.Slug == "project-name" })
	if i < 0 {
		panic("state: internal/spec has no rule project-name")
	}
	return regexp.MustCompile(rules[i].Pattern)
})

// checkName refuses a name that is not a project name.
func checkName(name string) error {
	if !projectName().MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, clip(name))
	}
	return nil
}
