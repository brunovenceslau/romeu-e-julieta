// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// MaxRecordSize is the most bytes of render.json DecodeRecord takes:
// 16 MiB, far past any record romeu writes, so a reader's memory stays
// bounded.
const MaxRecordSize = 16 << 20

// ErrRecordInvalid is the error of a document that is not a render.json
// this build reads.
var ErrRecordInvalid = errors.New("render: not a render.json this build reads")

// hexDigest is the form of a digest in Files: 64 lowercase hex digits.
var hexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// commitID is the form of a git object id: SHA-1 or SHA-256, lowercase.
var commitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// EncodeRecord returns render.json for r (03 3.7), written by the JSON
// marshaller alone (REQ-0018, I11): two-space indent, the members in
// the order of Record, map keys sorted, and a final newline. It sets
// Schema, and refuses a record whose paths or digests are not of their
// form, so no file names a path outside <name>-env/.
//
//romeu:invariant I11 guard
func EncodeRecord(r Record) ([]byte, error) {
	r.Schema = RecordSchema
	if err := r.check(); err != nil {
		return nil, err
	}
	b, err := jsonv2.Marshal(r, jsonv2.Deterministic(true), jsontext.WithIndent("  "))
	if err != nil {
		return nil, fmt.Errorf("render: encode render.json: %w", err)
	}
	return append(b, '\n'), nil
}

// DecodeRecord reads render.json strictly: romeu alone writes it and
// reads it back, so an unknown member, a duplicate member, another
// schema, trailing data or a value out of its form is ErrRecordInvalid.
func DecodeRecord(data []byte) (Record, error) {
	if len(data) > MaxRecordSize {
		return Record{}, fmt.Errorf("%w: %d bytes pass the cap of %d", ErrRecordInvalid, len(data), MaxRecordSize)
	}
	var r Record
	if err := jsonv2.Unmarshal(data, &r, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Record{}, fmt.Errorf("%w: %w", ErrRecordInvalid, err)
	}
	if r.Schema != RecordSchema {
		return Record{}, fmt.Errorf("%w: the schema is not %s", ErrRecordInvalid, RecordSchema)
	}
	if err := r.check(); err != nil {
		return Record{}, fmt.Errorf("%w: %w", ErrRecordInvalid, err)
	}
	return r, nil
}

// WriteRecord writes render.json for r into the candidate c, once the
// gate has its digests.
func WriteRecord(c Candidate, r Record) error {
	data, err := EncodeRecord(r)
	if err != nil {
		return err
	}
	return writeFile(c, CandidateRecord, data)
}

// check holds the fields of r that name a path or a commit to their
// form.
func (r Record) check() error {
	if r.SpecSource.Kind != SpecSourceOrigin {
		return fmt.Errorf("render: the spec source kind %q is not %q", r.SpecSource.Kind, SpecSourceOrigin)
	}
	if !commitID.MatchString(r.SpecSource.Commit) {
		return fmt.Errorf("render: the spec source commit %q is not an object id", r.SpecSource.Commit)
	}
	for _, repo := range r.Repos {
		if err := oneElement(repo.Dir); err != nil {
			return fmt.Errorf("render: the repo %q: %w", repo.URL, err)
		}
		if !commitID.MatchString(repo.EgressCommit) {
			return fmt.Errorf("render: the repo %q: the egress commit %q is not an object id", repo.Dir, repo.EgressCommit)
		}
		if repo.LockSha256 != "" && !hexDigest.MatchString(repo.LockSha256) {
			return fmt.Errorf("render: the repo %q: the mise.lock digest is not 64 lowercase hex digits", repo.Dir)
		}
	}
	if err := checkFiles("files", r.Files); err != nil {
		return err
	}
	if r.Promoting != nil {
		if err := checkFiles("previousFiles", r.Promoting.PreviousFiles); err != nil {
			return err
		}
	}
	return nil
}

// checkFiles holds a files map to its form: each key a clean relative
// slash path inside <name>-env/ with no control character, each value
// a digest.
func checkFiles(name string, files map[string]string) error {
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if p == "" || p == "." || path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, `\`) || strings.ContainsFunc(p, unicode.IsControl) {
			return fmt.Errorf("render: %s: %q is not a clean relative path", name, p)
		}
		if !hexDigest.MatchString(files[p]) {
			return fmt.Errorf("render: %s: the digest of %q is not 64 lowercase hex digits", name, p)
		}
	}
	return nil
}
