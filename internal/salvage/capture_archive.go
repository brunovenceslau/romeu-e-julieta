// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// The bundle files of a salvage dir: the repo's, and one per nested
// repository, numbered from 1 in the order the capture found them.
const (
	repoBundle   = "repo.bundle"
	nestedBundle = "nested-"
)

// bundle runs step 3 for the repository dir (REQ-0704): the refs a
// snapshot bundles (Pairs), the capture and stash refs of this salvage
// among them, minus what bases reach, built in the work dir
// (gitsafe.BundleOf) and copied into the salvage dir. A repository whose
// refs bases all reach needs no bundle. What Pairs left out is
// unreadable.
func (s *salvager) bundle(ctx context.Context, run *repoRun, dir, nested string, bases []string) error {
	what := "the bundle of the repo"
	if nested != "" {
		what = "the bundle of " + nested
	}
	plan, err := Pairs(ctx, s.c.Git, dir, s.c.Generation, bases)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		run.skip(what, failReason(err), err)
		return nil
	}
	for _, sk := range plan.Skipped {
		run.skip(sk.Item, ReasonUnreadable, sk.Err)
	}
	if len(plan.Pairs) == 0 {
		return nil
	}
	oids := make([]string, 0, len(plan.Pairs))
	for _, p := range plan.Pairs {
		oids = append(oids, p.OID)
	}
	beyond, err := s.c.Git.Beyond(ctx, dir, oids, bases)
	if err != nil {
		run.skip(what, failReason(err), err)
		return nil
	}
	if !beyond {
		return nil
	}
	u, err := canon.NewULID(s.now, s.c.ULIDs)
	if err != nil {
		return err
	}
	built := filepath.Join(s.c.Work, "bundle-"+u.String()+".bundle")
	defer func() { _ = os.Remove(built) }()
	if err := s.c.Git.BundleOf(ctx, dir, built, plan.Pairs, bases); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		run.skip(what, failReason(err), err)
		return nil
	}
	name := repoBundle
	if nested != "" {
		run.nested++
		name = nestedBundle + strconv.Itoa(run.nested) + ".bundle"
	}
	in, err := os.Open(built)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	f, err := writePayload(run.root, path.Join(run.dir, name), func(w io.Writer) error {
		_, err := io.Copy(w, in)
		return err
	})
	if err != nil {
		return err
	}
	run.m.Bundles = append(run.m.Bundles, Bundle{File: name, Nested: nested, SHA256: f.SHA256, Size: f.Size})
	for _, p := range plan.Pairs {
		run.m.Refs = append(run.m.Refs, Ref{Nested: nested, Ref: p.Name, SHA: p.OID})
	}
	return nil
}

// archiveIgnored runs step 4 for the repo of run (REQ-0705): each
// ignored entry the reads of step 2 found goes whole into
// ignored.tar.gz, as <label>/<path>, while the salvage cap leaves room
// for it, and is over-cap otherwise. It reports whether it stored one.
func (s *salvager) archiveIgnored(ctx context.Context, run *repoRun) (bool, error) {
	type pick struct {
		root  string
		label string
		files []ignoredFile
	}
	var picks []pick
	for _, a := range run.ignored {
		if a.entry.size > s.budget {
			run.skip(a.label+"/"+a.entry.path, ReasonOverCap, nil)
			continue
		}
		s.budget -= a.entry.size
		picks = append(picks, pick{root: a.worktree, label: a.label, files: a.entry.files})
	}
	if len(picks) == 0 {
		return false, nil
	}
	f, err := writePayload(run.root, path.Join(run.dir, IgnoredFile), func(w io.Writer) error {
		return writeTarGz(w, func(tw *tar.Writer) error {
			for _, p := range picks {
				if err := ctx.Err(); err != nil {
					return err
				}
				root, err := os.OpenRoot(p.root)
				if err != nil {
					return err
				}
				for _, file := range p.files {
					if err := addFile(tw, root, file.path, p.label+"/"+file.path, run); err != nil {
						return errors.Join(err, root.Close())
					}
				}
				if err := root.Close(); err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	run.m.Files = append(run.m.Files, File{Path: IgnoredFile, SHA256: f.SHA256, Size: f.Size})
	return true, nil
}

// archiveAgent runs step 5 into the salvage dir of the primary repo
// (REQ-0706, REQ-0707): a session open with no process matched is
// agent-not-matched; each state path of the profile below $HOME goes
// into agent/<profile>.tar.gz, file by file while the salvage cap
// leaves room, the transcripts only with --include-transcripts
// (transcript-excluded otherwise); a missing required path is
// agent-path-missing, and a missing optional one is no item. It reports
// whether it stored a file.
func (s *salvager) archiveAgent(ctx context.Context, run *repoRun) (bool, error) {
	p := s.c.Profile
	if p == nil {
		return false, nil
	}
	if s.c.AgentNotMatched {
		run.skip(p.Name, ReasonAgentNotMatched, errors.New("no process of the agent matched while a session is open"))
	}
	home, err := os.OpenRoot(s.c.Home)
	if err != nil {
		return false, err
	}
	defer func() { _ = home.Close() }()
	var files []ignoredFile
	for _, ap := range p.Paths {
		top, err := home.Lstat(ap.Rel)
		if errors.Is(err, fs.ErrNotExist) {
			if ap.Required {
				run.skip(ap.Rel, ReasonAgentPathMissing, nil)
			}
			continue
		}
		if err != nil {
			run.skip(ap.Rel, ReasonUnreadable, err)
			continue
		}
		visit := func(rel string, info fs.FileInfo) {
			link := info.Mode()&fs.ModeSymlink != 0
			switch {
			case !info.Mode().IsRegular() && !link:
			case p.Transcript != nil && p.Transcript(rel) && !s.c.IncludeTranscripts:
				run.skip(rel, ReasonTranscriptExcluded, nil)
			case info.Size() > s.budget:
				run.skip(rel, ReasonOverCap, nil)
			default:
				s.budget -= info.Size()
				files = append(files, ignoredFile{path: rel, size: info.Size(), modTime: info.ModTime(), link: link})
			}
		}
		if !top.IsDir() {
			visit(ap.Rel, top)
			continue
		}
		err = fs.WalkDir(home.FS(), ap.Rel, func(rel string, d fs.DirEntry, err error) error {
			if err != nil {
				run.skip(rel, ReasonUnreadable, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				run.skip(rel, ReasonUnreadable, err)
				return nil
			}
			visit(rel, info)
			return nil
		})
		if err != nil {
			return false, err
		}
	}
	if len(files) == 0 {
		return false, nil
	}
	if err := mkdirIn(run.root, path.Join(run.dir, AgentDir)); err != nil {
		return false, err
	}
	name := path.Join(AgentDir, p.Name+".tar.gz")
	f, err := writePayload(run.root, path.Join(run.dir, name), func(w io.Writer) error {
		return writeTarGz(w, func(tw *tar.Writer) error {
			for _, file := range files {
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := addFile(tw, home, file.path, file.path, run); err != nil {
					return err
				}
			}
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	run.m.Files = append(run.m.Files, File{Path: name, SHA256: f.SHA256, Size: f.Size})
	return true, nil
}

// writeTarGz writes a gzip-compressed tar archive to w, its entries
// added by add.
func writeTarGz(w io.Writer, add func(*tar.Writer) error) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := add(tw); err != nil {
		return err
	}
	return errors.Join(tw.Close(), gz.Close())
}

// archiveMode is the mode of every file of an archive: the payloads
// are secret-bearing (REQ-0710), so even an extracted copy is the
// owner's alone.
const archiveMode = 0o600

// addFile adds the file p of root to tw as name: a symbolic link as a
// link, not followed; a regular file with the size it has when opened,
// opened without following a link. A file that is gone or no longer
// regular is unreadable, and one that shrank while it was read is
// padded with zeros and unreadable, so the archive stays whole.
func addFile(tw *tar.Writer, root *os.Root, p, name string, run *repoRun) error {
	info, err := root.Lstat(p)
	if err != nil {
		run.skip(name, ReasonUnreadable, err)
		return nil
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := root.Readlink(p)
		if err != nil {
			run.skip(name, ReasonUnreadable, err)
			return nil
		}
		return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeSymlink, Name: name, Linkname: target, Mode: archiveMode, ModTime: info.ModTime()})
	}
	f, err := root.OpenFile(p, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		run.skip(name, ReasonUnreadable, err)
		return nil
	}
	defer func() { _ = f.Close() }()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		run.skip(name, ReasonUnreadable, fmt.Errorf("not a regular file when opened: %w", err))
		return nil
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: info.Size(), Mode: archiveMode, ModTime: info.ModTime()}); err != nil {
		return err
	}
	n, err := io.Copy(tw, io.LimitReader(f, info.Size()))
	if err != nil || n < info.Size() {
		if _, padErr := io.CopyN(tw, zeros{}, info.Size()-n); padErr != nil {
			return errors.Join(err, padErr)
		}
		if err == nil {
			err = errors.New("the file shrank while it was archived")
		}
		run.skip(name, ReasonUnreadable, err)
	}
	return nil
}

// zeros reads zero bytes forever.
type zeros struct{}

// Read fills p with zeros.
func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
