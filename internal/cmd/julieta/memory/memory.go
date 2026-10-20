// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package memory holds the commands julieta memory add, list, show,
// edit and check (04 4.3, REQ-0411, REQ-0412), one definition each in
// its own file, on the readers of internal/memstore and the Store of
// internal/memstore/write. An entry is filed in the memory dir of the
// repo that contains the working directory, or of --repo (08 8.3,
// REQ-0677), and the commands that write print the repo they filed
// under. julieta stamps the id and the times; no flag supplies them,
// so a supplied id or time is the error usage (RJ-349). There is no
// memory rm and no memory search: edit --status done closes an entry,
// and list --query filters.
package memory

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
)

// Schema is the value of the schema field of the --json document of
// the memory commands, the format julieta-memory.v1.
const Schema = "julieta-memory/v1"

// Report is the --json document of julieta memory add, list, show and
// edit, the format julieta-memory.v1: the repo the command read or
// filed under, the entries it printed, and whether it wrote one (04
// 4.1, Idempotency: add and edit print the entry they wrote, or the
// entry that made the write needless, with wrote unset).
type Report struct {
	// Entries are the entries the command printed, oldest first.
	Entries []Entry `json:"entries"`
	// Repo is the dir of the repo, as the manifest names it.
	Repo string `json:"repo"`
	// Schema is "julieta-memory/v1".
	Schema string `json:"schema"`
	// Wrote: the command wrote the entry it printed.
	Wrote bool `json:"wrote"`
}

// Entry is one memory entry with the path of its file.
type Entry struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Status    string    `json:"status"`
	Tags      []string  `json:"tags"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Decode reads a --json document of the memory commands strictly: an
// unknown member, a duplicate one and another schema are each an error.
func Decode(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r, json.RejectUnknownMembers(true)); err != nil {
		return Report{}, err
	}
	if r.Schema != Schema {
		return Report{}, fmt.Errorf("the schema %q is not %s", r.Schema, Schema)
	}
	return r, nil
}

// entryOf returns the entry e of the reader r as the report holds it.
func entryOf(r *memstore.Reader, e memstore.Entry) Entry {
	return Entry{Body: e.Body, CreatedAt: e.CreatedAt, ID: e.ID.String(), Path: r.Path(e.ID), Status: e.Status,
		Tags: e.Tags, Title: e.Title, UpdatedAt: e.UpdatedAt}
}

// target is the repo a memory command acts on, with $HOME.
type target struct {
	home string
	repo manifest.Repo
}

// resolve returns the repo of the manifest a memory command acts on:
// the one whose dir is repo, or, when repo is empty, the one whose path
// holds the working directory, the deepest when repos nest (08 8.3,
// REQ-0677; never a question). Its dir, path and memory dir are
// checked before they become paths (REQ-0271). A missing manifest is
// exit 2 (REQ-0425).
func resolve(repo string) (target, error) {
	home, err := setup.Home()
	if err != nil {
		return target{}, err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return target{}, err
	}
	var found *manifest.Repo
	if repo != "" {
		for i := range m.Repos {
			if m.Repos[i].Dir == repo {
				found = &m.Repos[i]
			}
		}
		if found == nil {
			return target{}, fmt.Errorf("no repo of the manifest has the dir %q", repo)
		}
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return target{}, err
		}
		for i := range m.Repos {
			if within(wd, m.Repos[i].Path) && (found == nil || len(m.Repos[i].Path) > len(found.Path)) {
				found = &m.Repos[i]
			}
		}
		if found == nil {
			return target{}, fmt.Errorf("the working directory %s is in no repo of the manifest: run the command in a repo, or name one with --repo", wd)
		}
	}
	if err := setup.CheckRepo(home, m.Project, *found); err != nil {
		return target{}, err
	}
	if !filepath.IsAbs(found.Memory) || filepath.Clean(found.Memory) != found.Memory {
		return target{}, fmt.Errorf("the memory dir %q of the repo %s is not an absolute, clean path", found.Memory, found.Dir)
	}
	return target{home: home, repo: *found}, nil
}

// within reports whether the directory wd is dir or below it, with the
// symbolic links of either resolved when they can be, so a working
// directory reached through a link still names its repo.
func within(wd, dir string) bool {
	in := func(a, b string) bool {
		rel, err := filepath.Rel(b, a)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if in(wd, dir) {
		return true
	}
	rwd, err1 := filepath.EvalSymlinks(wd)
	rdir, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && in(rwd, rdir)
}

// openStore returns the store of the memory dir of t, with its lock
// and the clock and ULID source of env.
func openStore(env *cli.Env, t target) (*write.Store, error) {
	return write.Open(write.Config{Dir: t.repo.Memory, Lock: write.LockPath(t.home, t.repo.Dir), Now: env.Now, ULIDs: env.ULIDs})
}

// output writes the report: the --json document, or text, which text
// writes.
func output(env *cli.Env, rep Report, text func(out *cli.Writer) error) error {
	if rep.Entries == nil {
		rep.Entries = []Entry{}
	}
	for i := range rep.Entries {
		if rep.Entries[i].Tags == nil {
			rep.Entries[i].Tags = []string{}
		}
	}
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	return text(env.Stdout)
}

// written writes the line of a command that writes one entry: what it
// did, the id, the path and the repo it filed the entry under (04 4.1,
// Idempotency, REQ-0280).
func written(out *cli.Writer, verb string, e Entry, repo string) error {
	_, err := fmt.Fprintf(out, "%s %s %s repo %s\n", verb, e.ID, e.Path, repo)
	return err
}

// readErr returns err of a read or a write of the memory dir as the
// command returns it: an error of the error table keeps its row
// (format-newer), and the others name the memory dir.
func readErr(err error, t target) error {
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return err
	}
	return fmt.Errorf("the memory dir %s of the repo %s: %w", t.repo.Memory, t.repo.Dir, err)
}

// repoFlag adds --repo to c.
func repoFlag(c *cobra.Command, repo *string) {
	c.Flags().StringVar(repo, "repo", "", "the repo dir to file under, instead of the repo of the working directory")
}
