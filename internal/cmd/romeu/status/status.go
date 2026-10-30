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
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// sbxReader is what status asks sbx: the sandboxes of the host and the
// live version, sbxdrv.Runner's List and Version.
type sbxReader interface {
	List(ctx context.Context, o sbxdrv.Opts) ([]sbxdrv.Sandbox, error)
	Version(ctx context.Context, o sbxdrv.Opts) (sbxdrv.Release, error)
}

// host is what status reads: the state directory, open to read (nil
// when there is none), the root that holds each project's <name>-env/
// ("" when it is not known), sbx, and the measure of the live toolchain
// for a given sbx version, approve.Measure with this build's version
// (nil leaves the toolchain out of the report).
type host struct {
	store   *state.Store
	root    string
	sbx     sbxReader
	measure func(sbxdrv.Release) (approve.Live, error)
}

// The reads a block names in its errors.
const (
	readRecord = "project record"
	readSbx    = "sbx ls"
)

// run prints the report of the projects names, or of every project with
// a record when names is empty, to env.Stdout, as text or with env.JSON
// as one document, and returns status-incomplete when a read failed. The
// toolchain heads the report, being the machine's and not a project's.
func run(ctx context.Context, env *cli.Env, h host, names []string) error {
	for _, name := range names {
		if err := state.CheckName(name); err != nil {
			return cli.Usage.New(fmt.Sprintf("%q is not a project name (03 3.1)", name))
		}
	}
	rep := Report{Warnings: []Finding{}, Errors: []ReadError{}, Projects: []Project{}}
	failed := 0
	if h.measure != nil {
		failed += readToolchain(ctx, h, &rep)
	}
	note := ""
	switch {
	case h.store == nil && len(names) == 0:
		note = "no host state: run romeu init, then romeu sync"
	case len(names) == 0:
		var err error
		if names, err = h.store.ProjectNames(); err != nil {
			return StatusIncomplete.Wrap(err, "the project records cannot be listed")
		}
		if len(names) == 0 {
			note = "no projects: run romeu sync"
		}
	}
	if len(names) > 0 {
		failed += readProjects(ctx, h, names, &rep)
	}
	if err := write(env, rep, note); err != nil {
		return err
	}
	if failed > 0 {
		return StatusIncomplete.New(fmt.Sprintf("%d read(s) failed; each is printed in the report", failed))
	}
	return ctx.Err()
}

// readProjects fills one block per project of names into rep and
// returns how many reads failed.
func readProjects(ctx context.Context, h host, names []string, rep *Report) int {
	// sbx is listed once; a failed listing is shown in every block, the
	// read each block depends on, and counts as one failed read.
	sandboxes, listErr := h.sbx.List(ctx, sbxdrv.Opts{Step: "status"})
	shared := 0
	if listErr != nil {
		shared = 1
	}
	failed := shared
	for _, name := range names {
		p := Project{Name: name, Generations: []Generation{}, Findings: []Finding{}, Errors: []ReadError{}}
		src := &source{name: name, recorded: RecordAbsent, sandboxes: sandboxes, listed: listErr == nil}
		if h.root != "" {
			src.envDir = filepath.Join(h.root, name+"-env")
		}
		if h.store != nil {
			record, err := h.store.LoadProject(name)
			switch {
			case err == nil:
				src.record, src.recorded = record, RecordPresent
			case !errors.Is(err, fs.ErrNotExist):
				src.recorded = RecordUnreadable
				p.Errors = append(p.Errors, readError(readRecord, err))
			}
		}
		if listErr != nil {
			p.Errors = append(p.Errors, readError(readSbx, listErr))
		}
		for _, s := range sections {
			s.fill(ctx, src, &p)
		}
		// The listing's error was put in this block before the sections
		// ran, and is counted once, above.
		failed += len(p.Errors) - shared
		rep.Projects = append(rep.Projects, p)
	}
	return failed
}

// readError returns the failed read of what by its error: the error's
// own row, or status-incomplete for an error that carries none. The
// message is made valid UTF-8, since a byte that is not would stop the
// JSON encoder and lose the whole report.
func readError(what string, err error) ReadError {
	r := ReadError{ID: StatusIncomplete.ID, Slug: StatusIncomplete.Slug, Read: what, Message: err.Error()}
	if e, ok := errors.AsType[*cli.Error](err); ok {
		r.ID, r.Slug = e.ID, e.Slug
	}
	r.Message = strings.ToValidUTF8(r.Message, "\uFFFD")
	return r
}

// write prints rep: one JSON document with env.JSON; else the toolchain
// line, the report's failed reads, the note (what to run when there is
// no project), and one block per project, the name on its own line and
// each section's lines below it, then the findings and the failed
// reads. A warning goes to stderr as every warning does (04 4.1).
func write(env *cli.Env, rep Report, note string) error {
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	for _, w := range rep.Warnings {
		cli.Warn(env.Stderr, w.Slug, line(w.Message))
	}
	if err := writeToolchain(env.Stdout, rep.Toolchain); err != nil {
		return err
	}
	for _, e := range rep.Errors {
		if _, err := fmt.Fprintf(env.Stdout, "error %s %s (%s): %s\n", e.ID, e.Slug, e.Read, line(e.Message)); err != nil {
			return err
		}
	}
	if note != "" {
		if _, err := fmt.Fprintln(env.Stdout, note); err != nil {
			return err
		}
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
