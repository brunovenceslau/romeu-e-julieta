// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package status is the command romeu status (04 4.2): a projection of
// host state, sbx and git per project, read only. It takes no lock,
// runs no preflight and writes nothing (REQ-0328); a read that fails is
// printed in its project's block and the report goes on, and the
// command exits 1 when any read failed, never 3 (REQ-0329).
//
// The command file, which reads the host settings for the root and the
// sbx path, lands with the shared host paths of T169; until then run
// takes the host it reads as a value.
package status

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// lister lists the sandboxes of the host: sbxdrv.Runner.List.
type lister interface {
	List(ctx context.Context, o sbxdrv.Opts) ([]sbxdrv.Sandbox, error)
}

// host is what status reads: the state directory, open to read (nil
// when there is none), and sbx.
type host struct {
	store *state.Store
	sbx   lister
}

// The reads a block names in its errors.
const (
	readRecord = "project record"
	readSbx    = "sbx ls"
)

// run prints the report of the projects names, or of every project with
// a record when names is empty, to env.Stdout, as text or with env.JSON
// as one document, and returns status-incomplete when a read failed.
func run(ctx context.Context, env *cli.Env, h host, names []string) error {
	if h.store == nil && len(names) == 0 {
		if env.JSON {
			return write(env, Report{Projects: []Project{}})
		}
		_, err := fmt.Fprintln(env.Stdout, "no host state: run romeu init, then romeu sync")
		return err
	}
	if len(names) == 0 {
		var err error
		if names, err = h.store.ProjectNames(); err != nil {
			return StatusIncomplete.Wrap(err, "the project records cannot be listed")
		}
	}
	sandboxes, listErr := h.sbx.List(ctx, sbxdrv.Opts{Step: "status"})
	rep := Report{Projects: []Project{}}
	failed := 0
	for _, name := range names {
		p := Project{Name: name, Generations: []Generation{}, Findings: []Finding{}, Errors: []ReadError{}}
		src := &source{name: name, sandboxes: sandboxes, listed: listErr == nil}
		if h.store != nil {
			record, err := h.store.LoadProject(name)
			switch {
			case err == nil:
				src.record = record
			case !errors.Is(err, fs.ErrNotExist):
				p.Errors = append(p.Errors, readError(readRecord, err))
			}
		}
		if listErr != nil {
			p.Errors = append(p.Errors, readError(readSbx, listErr))
		}
		for _, s := range sections {
			s.fill(ctx, src, &p)
		}
		failed += len(p.Errors)
		rep.Projects = append(rep.Projects, p)
	}
	if err := write(env, rep); err != nil {
		return err
	}
	if failed > 0 {
		return StatusIncomplete.New(fmt.Sprintf("%d read(s) failed; each is printed in its project's block", failed))
	}
	return ctx.Err()
}

// readError returns the failed read of what by its error: the error's
// own row, or status-incomplete for an error that carries none.
func readError(what string, err error) ReadError {
	if e, ok := errors.AsType[*cli.Error](err); ok {
		return ReadError{ID: e.ID, Slug: e.Slug, Read: what, Message: e.Error()}
	}
	return ReadError{ID: StatusIncomplete.ID, Slug: StatusIncomplete.Slug, Read: what, Message: err.Error()}
}

// write prints rep: one JSON document with env.JSON, else one block per
// project, the name on its own line and each section's lines below it,
// then the findings and the failed reads.
func write(env *cli.Env, rep Report) error {
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	for i := range rep.Projects {
		p := &rep.Projects[i]
		if _, err := fmt.Fprintln(env.Stdout, line(p.Name)); err != nil {
			return err
		}
		for _, s := range sections {
			if err := s.text(env.Stdout, p); err != nil {
				return err
			}
		}
		for _, f := range p.Findings {
			if _, err := fmt.Fprintf(env.Stdout, "  %s: %s\n", f.Slug, line(f.Message)); err != nil {
				return err
			}
		}
		for _, e := range p.Errors {
			if _, err := fmt.Fprintf(env.Stdout, "  error %s %s (%s): %s\n", e.ID, e.Slug, e.Read, line(e.Message)); err != nil {
				return err
			}
		}
	}
	return nil
}

// line returns s with each line feed written as the two characters \n,
// so a value, such as an error that holds a subprocess's stderr, stays
// on its line and cannot start one that forges another; the CLI writer
// escapes every other code point (I29).
func line(s string) string { return strings.ReplaceAll(s, "\n", `\n`) }
