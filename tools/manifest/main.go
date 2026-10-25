// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command manifest writes the JULIETA_MANIFEST value of a project by
// hand, for the bridge of plan task T111, until romeu sync writes it:
//
//	go run ./tools/manifest encode <file.json | ->
//	go run ./tools/manifest ulid
//
// encode reads a manifest/v1 document (03 3.6) in any key order and
// spacing, refuses a member the format does not name, fills runDigest
// from run when it is absent and refuses one that differs, and prints
// the env value through manifest.EncodeEnv, after manifest.DecodeEnv has
// read it back, so a value it prints is one julieta accepts. ulid prints
// a new lowercase ULID for the generation member. It is a tool of this
// repository, not a product command. The exit status is 0 when the
// value is printed, 1 when the document is refused, and 2 for a wrong
// command line.
package main

import (
	"crypto/rand"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

const (
	exitOK    = 0 // the value is printed
	exitError = 1 // the document is refused
	exitUsage = 2 // the command line is wrong
)

const usage = "usage: manifest encode <file.json | ->, or manifest ulid"

func main() { os.Exit(start(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func start(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 2 && args[0] == "encode":
		v, err := encode(args[1], stdin)
		if err == nil {
			_, err = fmt.Fprintln(stdout, v)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "manifest:", err)
			return exitError
		}
		return exitOK
	case len(args) == 1 && args[0] == "ulid":
		id, err := canon.NewULID(time.Now(), canon.NewULIDSource(rand.Reader))
		if err == nil {
			_, err = fmt.Fprintln(stdout, id)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "manifest:", err)
			return exitError
		}
		return exitOK
	}
	_, _ = fmt.Fprintln(stderr, usage)
	return exitUsage
}

// encode returns the env value of the document at path, or of stdin
// when path is "-".
func encode(path string, stdin io.Reader) (string, error) {
	r := stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer func() { _ = f.Close() }()
		r = f
	}
	// A document past the JSON cap can never encode under the env cap,
	// so it is refused here, before it is decoded.
	data, err := io.ReadAll(io.LimitReader(r, manifest.MaxJSONSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > manifest.MaxJSONSize {
		return "", fmt.Errorf("the document passes %d bytes, more than a manifest can hold", manifest.MaxJSONSize)
	}
	var m manifest.Manifest
	if err := jsonv2.Unmarshal(data, &m, jsonv2.RejectUnknownMembers(true)); err != nil {
		return "", err
	}
	// Encode writes an absent or null run as {}, so the digest is of {}.
	if m.Run == nil {
		m.Run = manifest.Run{}
	}
	sum, err := canon.Digest(canon.Run, m.Run)
	if err != nil {
		return "", fmt.Errorf("run: %w", err)
	}
	switch m.RunDigest {
	case "":
		m.RunDigest = sum.String()
	case sum.String():
	default:
		return "", fmt.Errorf("runDigest %s is not %s, the digest of kind run of the run; leave it out to have it filled", m.RunDigest, sum)
	}
	v, err := manifest.EncodeEnv(m)
	if err != nil {
		return "", err
	}
	if _, err := manifest.DecodeEnv(v); err != nil {
		return "", err
	}
	return v, nil
}
