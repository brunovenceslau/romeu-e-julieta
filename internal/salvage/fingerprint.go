// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// Fingerprint is the fingerprint object of julieta salvage (04 4.3,
// REQ-0417): the sandbox-side values of the fingerprint of 03 3.8, as
// the reads of steps 2 and 4 of 08 8.5 saw them; romeu adds the daemon
// heads it fetches itself and digests the whole (canon.Fingerprint).
// julieta salvage --fingerprint prints it alone, and the --json
// document of a full salvage carries it. The fields are in the order of
// their JSON names.
type Fingerprint struct {
	// Repos are the repos of the manifest, in its order.
	Repos []FingerprintRepo `json:"repos"`
}

// FingerprintRepo is the fingerprint of one repo.
type FingerprintRepo struct {
	// Dir is the repo's dir in the project.
	Dir string `json:"dir"`
	// Worktrees are its worktrees, by label (Worktree.Path), then those
	// of each nested repository below the worktree that holds it.
	Worktrees []FingerprintWorktree `json:"worktrees"`
}

// FingerprintWorktree is the fingerprint of one worktree: its HEAD and
// two digests, each a plain sha256 in lowercase hex.
type FingerprintWorktree struct {
	// Head is the commit its HEAD names; empty on an unborn branch or
	// when the worktree could not be read.
	Head string `json:"head"`
	// Ignored digests its ignored files outside salvage.excludeIgnored,
	// each by path, size and modification time.
	Ignored string `json:"ignored"`
	// Path is its label, as Worktree.Path.
	Path string `json:"path"`
	// Status digests its HEAD, its branch, and each tracked change and
	// untracked file with the digest of its content; empty when the
	// worktree could not be read.
	Status string `json:"status"`
}

// worktreeRead is one read of a worktree, step 2's and the
// fingerprint's: what git lists and the digest of it.
type worktreeRead struct {
	changes gitsafe.Changes
	status  string
}

// same reports whether two reads of a worktree saw the same HEAD and
// the same status (08 8.5 step 2, REQ-0702).
func (r worktreeRead) same(o worktreeRead) bool {
	return r.changes.OID == o.changes.OID && r.changes.Branch == o.changes.Branch && r.status == o.status
}

// readWorktree reads the worktree whose root is dir: its changes, and
// the digest of its status, which holds HEAD, the branch, the operation
// in progress, and each tracked change and untracked file with the
// digest of its content (fileDigest).
func readWorktree(ctx context.Context, git *gitsafe.Runner, dir string) (worktreeRead, error) {
	c, err := git.Changes(ctx, dir)
	if err != nil {
		return worktreeRead{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return worktreeRead{}, err
	}
	defer func() { _ = root.Close() }()
	h := sha256.New()
	field(h, "head", c.OID, c.Branch, c.Operation)
	for _, list := range []struct {
		kind  string
		paths []string
	}{{"tracked", c.Tracked}, {"added", c.Added}, {"untracked", c.Untracked}} {
		for _, p := range list.paths {
			d, err := fileDigest(root, p)
			if err != nil {
				return worktreeRead{}, err
			}
			field(h, list.kind, p, d)
		}
	}
	return worktreeRead{changes: c, status: hex.EncodeToString(h.Sum(nil))}, nil
}

// field writes one record of a digest: its values, each followed by a
// NUL byte, which no path holds, and a final line feed.
func field(h hash.Hash, values ...string) {
	for _, v := range values {
		_, _ = io.WriteString(h, v)
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write([]byte{'\n'})
}

// fileDigest returns what the digest of a status holds of the path p of
// the worktree root: "absent", "dir" for a directory (a nested
// repository), "link:" and the sha256 of the target of a symbolic link,
// which is not followed, "file:" and the sha256 of a regular file's
// content, and "other:" and the mode for anything else.
func fileDigest(root *os.Root, p string) (string, error) {
	p = strings.TrimSuffix(p, "/")
	info, err := root.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "absent", nil
	case err != nil:
		return "", err
	case info.IsDir():
		return "dir", nil
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := root.Readlink(p)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(target))
		return "link:" + hex.EncodeToString(sum[:]), nil
	case !info.Mode().IsRegular():
		return "other:" + info.Mode().String(), nil
	}
	f, err := root.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "file:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ignoredFile is one file below an ignored path of a worktree.
type ignoredFile struct {
	path    string // the path in the worktree
	size    int64  // the bytes the salvage cap counts
	modTime time.Time
	link    bool // a symbolic link, kept as one
}

// ignoredEntry is one ignored path git listed, with the files below
// it, the ones salvage.excludeIgnored names left out.
type ignoredEntry struct {
	path  string
	files []ignoredFile
	size  int64
}

// ignoredRead is the read of step 4 of a worktree: its ignored entries,
// the nested repositories among them, the patterns of
// salvage.excludeIgnored that left something out, and the digest of the
// fingerprint.
type ignoredRead struct {
	entries  []ignoredEntry
	nested   []string
	excluded []string
	digest   string
}

// readIgnored lists the ignored files of the worktree whose root is
// dir (gitsafe.Ignored), each by path, size and modification time,
// without following a symbolic link: a directory git lists whole is
// walked. A path salvage.excludeIgnored names, or one below it, is left
// out (excluded-by-spec), and an ignored directory that holds .git is a
// nested repository, which the salvage captures as one.
func readIgnored(ctx context.Context, git *gitsafe.Runner, dir string, exclude []string) (ignoredRead, error) {
	paths, err := git.Ignored(ctx, dir)
	if err != nil {
		return ignoredRead{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return ignoredRead{}, err
	}
	defer func() { _ = root.Close() }()
	var r ignoredRead
	h := sha256.New()
	excludedBy := func(p string) (string, bool) {
		for _, e := range exclude {
			e = strings.TrimSuffix(e, "/")
			if p == e || strings.HasPrefix(p, e+"/") {
				return e, true
			}
		}
		return "", false
	}
	note := func(e string) {
		if !slices.Contains(r.excluded, e) {
			r.excluded = append(r.excluded, e)
		}
	}
	for _, p := range paths {
		clean := strings.TrimSuffix(p, "/")
		if e, ok := excludedBy(clean); ok {
			note(e)
			continue
		}
		if strings.HasSuffix(p, "/") && isRepo(root, clean) {
			r.nested = append(r.nested, clean)
			continue
		}
		entry := ignoredEntry{path: clean}
		visit := func(p string, info fs.FileInfo) {
			f := ignoredFile{path: p, size: info.Size(), modTime: info.ModTime(), link: info.Mode()&fs.ModeSymlink != 0}
			if !info.Mode().IsRegular() && !f.link {
				return // a FIFO, a socket or a device holds no content to keep
			}
			field(h, p, strconv.FormatInt(f.size, 10), f.modTime.UTC().Format(time.RFC3339Nano))
			entry.files = append(entry.files, f)
			entry.size += f.size
		}
		// Lstat first: fs.WalkDir stats its root, which would follow a
		// symbolic link.
		top, err := root.Lstat(clean)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue // removed since git listed it
		case err != nil:
			return ignoredRead{}, fmt.Errorf("the ignored path %s: %w", clean, err)
		case !top.IsDir():
			visit(clean, top)
		default:
			err = fs.WalkDir(root.FS(), clean, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if e, ok := excludedBy(p); ok {
					note(e)
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
				if d.IsDir() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				visit(p, info)
				return nil
			})
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return ignoredRead{}, fmt.Errorf("the ignored path %s: %w", clean, err)
			}
		}
		r.entries = append(r.entries, entry)
	}
	r.digest = hex.EncodeToString(h.Sum(nil))
	return r, nil
}

// isRepo reports whether the directory p of root holds .git, a
// directory or the file of a linked repository.
func isRepo(root *os.Root, p string) bool {
	_, err := root.Lstat(path.Join(p, ".git"))
	return err == nil
}

// worktreeSet is a repo's worktrees in the order of their labels: the
// main one first, then the linked ones by absolute path, so a second
// read labels them alike; a bare entry and one whose directory is gone
// are left out.
func worktreeSet(ctx context.Context, git *gitsafe.Runner, dir string) ([]gitsafe.Worktree, error) {
	all, err := git.Worktrees(ctx, dir)
	if err != nil {
		return nil, err
	}
	var main []gitsafe.Worktree
	var linked []gitsafe.Worktree
	for i, w := range all {
		switch {
		case w.Bare || w.Prunable:
		case i == 0:
			main = append(main, w)
		default:
			linked = append(linked, w)
		}
	}
	slices.SortFunc(linked, func(a, b gitsafe.Worktree) int { return strings.Compare(a.Path, b.Path) })
	return append(main, linked...), nil
}

// label returns the label of the worktree at index i of a worktreeSet.
func label(i int) string { return "worktree-" + strconv.Itoa(i) }

// ReadFingerprint reads the fingerprint of the repos (julieta salvage
// --fingerprint, REQ-0709): each worktree's HEAD and status and its
// ignored files, as step 2 and step 4 of a salvage read them, and the
// same of each nested repository found in a worktree. It writes
// nothing. A worktree git cannot read has an empty head and status, as
// a salvage records it.
func ReadFingerprint(ctx context.Context, git *gitsafe.Runner, repos []Repo, exclude []string) (Fingerprint, error) {
	fp := Fingerprint{Repos: []FingerprintRepo{}}
	for _, r := range repos {
		fr := FingerprintRepo{Dir: r.Dir, Worktrees: []FingerprintWorktree{}}
		if err := fingerprintRepo(ctx, git, r.Path, "", exclude, 0, &fr); err != nil {
			return Fingerprint{}, fmt.Errorf("the repo %s: %w", r.Dir, err)
		}
		fp.Repos = append(fp.Repos, fr)
	}
	return fp, nil
}

// fingerprintRepo adds the worktrees of the repository in dir to fr:
// all of them for a repo of the manifest, the one at dir for a nested
// repository, whose label is under.
func fingerprintRepo(ctx context.Context, git *gitsafe.Runner, dir, under string, exclude []string, depth int, fr *FingerprintRepo) error {
	type unit struct{ path, label string }
	var units []unit
	if under == "" {
		set, err := worktreeSet(ctx, git, dir)
		if err != nil {
			return err
		}
		for i, w := range set {
			units = append(units, unit{w.Path, label(i)})
		}
	} else {
		units = []unit{{dir, under}}
	}
	for _, u := range units {
		fw := FingerprintWorktree{Path: u.label}
		read, err := readWorktree(ctx, git, u.path)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var nested []string
		if err == nil {
			fw.Head, fw.Status = read.changes.OID, read.status
			nested = nestedOf(u.path, read.changes)
		}
		ign, err := readIgnored(ctx, git, u.path, exclude)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err == nil {
			fw.Ignored = ign.digest
			nested = append(nested, ign.nested...)
		}
		if links, err := git.Gitlinks(ctx, u.path); err == nil {
			nested = append(nested, existingRepos(u.path, links)...)
		}
		fr.Worktrees = append(fr.Worktrees, fw)
		if depth >= MaxNestedDepth {
			continue
		}
		for _, n := range dedupe(nested) {
			if err := fingerprintRepo(ctx, git, path.Join(u.path, n), u.label+"/"+n, exclude, depth+1, fr); err != nil {
				return err
			}
		}
	}
	return nil
}

// MaxNestedDepth is how deep salvage looks for nested repositories: a
// repository nested in a nested repository is captured, and so on, to
// this depth (decided while building T061).
const MaxNestedDepth = 4

// nestedOf returns the nested repositories among the untracked paths
// of c in the worktree dir: the directories git did not descend into
// that hold .git.
func nestedOf(dir string, c gitsafe.Changes) []string {
	var dirs []string
	for _, p := range c.Untracked {
		if before, ok := strings.CutSuffix(p, "/"); ok {
			dirs = append(dirs, before)
		}
	}
	return existingRepos(dir, dirs)
}

// existingRepos returns those of the paths of the worktree dir that are
// directories holding .git.
func existingRepos(dir string, paths []string) []string {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil
	}
	defer func() { _ = root.Close() }()
	var repos []string
	for _, p := range paths {
		if isRepo(root, p) {
			repos = append(repos, p)
		}
	}
	return repos
}

// dedupe returns the paths sorted, each once.
func dedupe(paths []string) []string {
	s := slices.Clone(paths)
	slices.Sort(s)
	return slices.Compact(s)
}
