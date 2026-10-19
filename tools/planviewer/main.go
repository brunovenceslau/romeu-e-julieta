// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command planviewer renders the task files of docs/plan/tasks as one
// self-contained HTML page, which opens offline. Run it from the root
// of the repository:
//
//	go run ./tools/planviewer
//	go run ./tools/planviewer -ready
//	go run ./tools/planviewer -critical-path
//
// It writes plan-viewer.html there (a path .gitignore holds), or the
// file of -o, and prints the path. With -ready it prints the ready task
// ids, one per line, in the form of "go run ./tools/ci plan --ready",
// and writes no file; -critical-path does the same for "plan
// --critical-path". The Trends view reads the git history of the task
// files (internal/gitsafe, read-only): -history=false skips it,
// -history-limit bounds the commits it shows and -repo names the
// repository; planviewer is unix-only, as internal/gitsafe is, which it
// reads the history through. Without a repository the page says so and the other views
// still render. The reading and the page are in
// tools/planviewer/internal/view; this package parses the command line,
// hands the output streams over and sets the exit status: 0 when it
// succeeds, 1 when it fails, and 2 for a wrong command line.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/planviewer/internal/view"
)

const (
	exitOK    = 0 // the page or the ids are written
	exitError = 1 // the plan cannot be read or the file written
	exitUsage = 2 // the command line is wrong
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run parses args, calls the view and returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	o := view.Defaults()
	fs := flag.NewFlagSet("planviewer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.Out, "o", o.Out, "the `file` to write")
	fs.StringVar(&o.Template, "template", o.Template, "the page `template`")
	fs.StringVar(&o.Dir, "dir", o.Dir, "the `directory` of the task files")
	fs.StringVar(&o.Plan, "plan", o.Plan, "the `file` with the milestone table")
	fs.BoolVar(&o.History, "history", o.History, "read the git history for the Trends view")
	fs.IntVar(&o.HistoryLimit, "history-limit", o.HistoryLimit, "the newest `n` commits of the plan history the Trends view shows")
	fs.StringVar(&o.Repo, "repo", o.Repo, "the root of the git `repository` the history is read from")
	list := fs.Bool("ready", false, "print the ready task ids, one per line, and write no file")
	crit := fs.Bool("critical-path", false, "print the critical path ids, one per line, and write no file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if fs.NArg() != 0 {
		return fail(stderr, exitUsage, errors.New("planviewer takes no argument"))
	}
	if *list && *crit {
		return fail(stderr, exitUsage, errors.New("-ready and -critical-path exclude each other"))
	}
	if *list || *crit {
		pick := view.Ready
		if *crit {
			pick = view.CriticalPath
		}
		ids, err := pick(o)
		if err != nil {
			return fail(stderr, exitError, err)
		}
		for _, id := range ids {
			if _, err := fmt.Fprintln(stdout, id); err != nil {
				return fail(stderr, exitError, err)
			}
		}
		return exitOK
	}
	if o.HistoryLimit < 1 {
		return fail(stderr, exitUsage, errors.New("-history-limit must be at least 1"))
	}
	path, note, err := view.Write(context.Background(), o, time.Now())
	if err != nil {
		return fail(stderr, exitError, err)
	}
	if note != "" && o.History {
		_, _ = fmt.Fprintln(stderr, "planviewer: trends:", strconv.Quote(note))
	}
	if _, err := fmt.Fprintln(stdout, path); err != nil {
		return fail(stderr, exitError, err)
	}
	return exitOK
}

func fail(stderr io.Writer, code int, err error) int {
	_, _ = fmt.Fprintln(stderr, "planviewer:", err)
	return code
}
