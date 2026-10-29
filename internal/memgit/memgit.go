// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package memgit keeps the durable copy of a project's memory (08 8.1,
// REQ-1303, REQ-1304; ADR 0043): the private git repository memory.remote
// names, whose git dir is <name>-env/memory.git and whose work tree is
// <name>-env/memory/. romeu runs Sync as the last step of sync, run,
// pull, salvage, rm, recreate and retire: it fetches the remote,
// fast-forwards to it, commits entries/ and stamps/ of every memory dir
// and nothing else, and pushes main with gitsafe's one push (I18). A
// remote that refuses, or a history that cannot fast-forward, is a
// warning, never a refusal: the command's work is done, and the next
// command pushes again. Every git call runs through gitsafe in host
// mode, with the identity and the key passed per call (I2).
package memgit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The warnings of 04 4.1 this package returns, by slug.
const (
	// SlugLocal: the spec keeps memory on this machine only
	// (memory.remote: none).
	SlugLocal = "memory-local"
	// SlugUnsigned: the commits are unsigned, since host settings hold
	// no signing socket or the project names no signing key.
	SlugUnsigned = "memory-unsigned"
	// SlugDiverged: the remote's main and the local one cannot be
	// fast-forwarded one to the other; nothing was committed or pushed.
	SlugDiverged = "memory-diverged"
	// SlugPushFailed: the fetch or the push of the remote failed.
	SlugPushFailed = "memory-push-failed"
)

// Warning is one warning for the command's output: a slug of 04 4.1
// and its message.
type Warning struct {
	Slug, Message string
}

// Project is what Sync needs of a project: its spec and host settings,
// resolved by the caller.
type Project struct {
	// Name is the project's name, the author of each commit.
	Name string
	// EnvDir is the project's tree, <root>/<name>-env.
	EnvDir string
	// Remote is memory.remote: a URL gitsafe accepts, or none.
	Remote string
	// SigningKey is the project's signing key, an OpenSSH public key
	// line, or empty.
	SigningKey string
	// AgentSocket is signing.agentSocket of host settings, or empty.
	AgentSocket string
}

// none is the value of memory.remote that opts out (spec.MemoryNone).
const none = "none"

// The paths below EnvDir.
const (
	gitDirName   = "memory.git"
	workTreeName = "memory"
)

// remoteRefs is where the fetch writes the remote's branches: a host
// fetch destination of gitsafe (05 5.1, I5), under the reserved dir
// name memory, which no repo dir can take (REQ-0176); remoteMain is the
// remote's main there, and localMain the memory repository's own.
const (
	remoteRefs = "refs/romeu/origin/memory/"
	remoteMain = remoteRefs + "main"
	localMain  = "refs/heads/main"
)

// exclude is the ignore rule Sync writes into the git dir's
// info/exclude, never into the memory dir: every path of the work tree
// but the files of entries/ and stamps/ of each memory dir is left out,
// dotfiles and directories below them too, julieta's temporary files
// among the dotfiles (08 8.1, REQ-0654). snapshot/ and salvage/ are
// secret-bearing (08 8.5) and never leave the machine. Staging does not
// rest on it, since a .gitignore the sandbox writes overrides it:
// collect names the files, and gitsafe.StageExactly stages those alone.
// It keeps what git reads of the work tree, the fast-forward's check of
// untracked files among it, to the same set.
const exclude = `# Written by romeu (08 8.1): only entries/ and stamps/ of each memory dir.
/*
!/*/
/*/*
!/*/entries/
!/*/stamps/
/*/entries/*/
/*/stamps/*/
/*/entries/.*
/*/stamps/.*
`

// Notices returns the warnings sync, run and doctor print every time
// for p (REQ-1304, REQ-1305): memory-local when the spec opts out, and
// memory-unsigned when the commits would be unsigned.
func Notices(p Project) []Warning {
	if p.Remote == none {
		return []Warning{{SlugLocal, "the memory of " + p.Name + " is kept on this machine only (memory.remote: none)"}}
	}
	if p.SigningKey == "" || p.AgentSocket == "" {
		return []Warning{{SlugUnsigned, "the memory commits of " + p.Name + " are unsigned: host settings name no signing.agentSocket, or the project no signing key"}}
	}
	return nil
}

// Separate returns the memory repository of p.
func (p Project) Separate() gitsafe.Separate {
	return gitsafe.Separate{GitDir: filepath.Join(p.EnvDir, gitDirName), WorkTree: filepath.Join(p.EnvDir, workTreeName)}
}

// Sync commits and pushes the memory of p with g, a host-mode runner,
// and returns the warnings for the command's output. A project whose
// spec opts out does nothing. An error is a failure of the machine's
// own repository (the git dir, the excludes, a commit), which the
// caller reports; what the remote does is a warning.
//
// The steps, in order: the git dir is made when absent, and the memory
// dir too; the excludes are written; the remote is fetched; a remote
// that moved on is fast-forwarded to, after its tree passes gitsafe's
// Fsck and WalkTree and holds nothing but entries/ and stamps/ files,
// and an empty memory dir takes the remote's history so (the clone into
// place of REQ-1305); a history that cannot fast-forward is
// memory-diverged and stops Sync; the files of entries/ and stamps/
// are collected without following a link and staged exactly (a link or
// a directory there is an error) and, when the index changed,
// committed; main is pushed when it is not the
// remote's.
func Sync(ctx context.Context, g *gitsafe.Runner, p Project) ([]Warning, error) {
	if p.Remote == none || p.Remote == "" {
		return nil, nil
	}
	s := p.Separate()
	if err := os.Mkdir(s.WorkTree, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("memgit: the memory dir: %w", err)
	}
	switch _, err := os.Lstat(s.GitDir); {
	case errors.Is(err, fs.ErrNotExist):
		if err := g.InitSeparate(ctx, s); err != nil {
			return nil, fmt.Errorf("memgit: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("memgit: the git dir: %w", err)
	}
	if err := writeExclude(s.GitDir); err != nil {
		return nil, err
	}
	// Every branch of the memory remote is mirrored under remoteRefs,
	// and only remoteMain is read. A whole-component * is the one
	// pattern gitsafe takes under refs/romeu/origin/ (T183): a partial
	// one such as main* could take HEAD from refs/heads/HEA. The plain
	// refs/heads/main fails on an empty remote (exit 128), and a pattern
	// matches nothing there. Decided on 2026-10-29, scope this
	// repository.
	if err := g.Fetch(ctx, s.GitDir, p.Remote, []string{"+refs/heads/*:" + remoteRefs + "*"}); err != nil {
		return []Warning{{SlugPushFailed, "the fetch of " + p.Remote + " failed, so nothing was committed or pushed: " + err.Error()}}, nil
	}
	remote, hasRemote, err := g.RefOID(ctx, s.GitDir, remoteMain)
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	local, hasLocal, err := g.RefOID(ctx, s.GitDir, localMain)
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	if hasRemote && remote != local {
		w, err := catchUp(ctx, g, s, p, remote, local, hasLocal)
		if w != nil || err != nil {
			return w, err
		}
	}
	files, closeDirs, err := collect(s.WorkTree)
	if err != nil {
		return nil, err
	}
	err = g.StageExactly(ctx, s, files)
	closeDirs()
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	changed, err := g.StagedChanges(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	if changed {
		id := gitsafe.Identity{Name: p.Name}
		if p.SigningKey != "" && p.AgentSocket != "" {
			id.SigningKey, id.AgentSocket = p.SigningKey, p.AgentSocket
		}
		if err := g.Commit(ctx, s, id, "romeu: the memory of "+p.Name); err != nil {
			return nil, fmt.Errorf("memgit: %w", err)
		}
	}
	head, hasHead, err := g.RefOID(ctx, s.GitDir, localMain)
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	if !hasHead || (hasRemote && head == remote) {
		return nil, nil // nothing to push (REQ-0277)
	}
	if err := g.Push(ctx, s.GitDir, p.Remote); err != nil {
		return []Warning{{SlugPushFailed, "the push to " + p.Remote + " failed; the next command pushes again: " + err.Error()}}, nil
	}
	return nil, nil
}

// catchUp brings the local main to remote when it is behind, or unborn,
// and leaves it when it is ahead. It returns memory-diverged when
// neither holds, when the remote's tree holds what the memory dir may
// not, or when the fast-forward fails; Sync then stops, having
// committed and pushed nothing and rewritten nothing.
func catchUp(ctx context.Context, g *gitsafe.Runner, s gitsafe.Separate, p Project, remote, local string, hasLocal bool) ([]Warning, error) {
	if hasLocal {
		ahead, err := g.IsAncestor(ctx, s.GitDir, remote, local)
		if err != nil {
			return nil, fmt.Errorf("memgit: %w", err)
		}
		if ahead {
			return nil, nil
		}
		behind, err := g.IsAncestor(ctx, s.GitDir, local, remote)
		if err != nil {
			return nil, fmt.Errorf("memgit: %w", err)
		}
		if !behind {
			return diverged(p, "the local main and the remote's have diverged"), nil
		}
	}
	if err := g.Fsck(ctx, s.GitDir, remote); err != nil {
		return diverged(p, "the remote's main fails git fsck: "+err.Error()), nil
	}
	files, err := g.WalkTree(ctx, s.GitDir, remote)
	if err != nil {
		return diverged(p, "the remote's main fails the tree walk: "+err.Error()), nil
	}
	for _, f := range files {
		if !committed(f.Path) {
			return diverged(p, "the remote's main holds "+f.Path+", which is not a file of entries/ or stamps/ of a memory dir"), nil
		}
	}
	if err := g.FastForwardSeparate(ctx, s, remote); err != nil {
		return diverged(p, "the memory dir cannot be fast-forwarded to the remote's main: "+err.Error()), nil
	}
	return nil, nil
}

// diverged is the warning memory-diverged for p.
func diverged(p Project, why string) []Warning {
	return []Warning{{SlugDiverged, why + "; nothing of " + p.Name + " was committed or pushed, and the operator merges by hand"}}
}

// committed reports whether path, a slash path of a tree, is one the
// memory repository holds: <dir>/entries/<file> or <dir>/stamps/<file>,
// no dotfile.
func committed(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) == 3 && !strings.HasPrefix(parts[0], ".") && (parts[1] == "entries" || parts[1] == "stamps") && !strings.HasPrefix(parts[2], ".")
}

// writeExclude writes the excludes into gitDir/info/exclude when the
// file does not hold them yet, so a run that changes nothing writes
// nothing.
func writeExclude(gitDir string) error {
	info := filepath.Join(gitDir, "info")
	if err := os.Mkdir(info, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("memgit: the excludes: %w", err)
	}
	path := filepath.Join(info, "exclude")
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, []byte(exclude)) {
		return nil
	}
	if err := os.WriteFile(path, []byte(exclude), 0o600); err != nil {
		return fmt.Errorf("memgit: the excludes: %w", err)
	}
	return nil
}

// collect returns the files Sync stages: each regular file, not a
// dotfile, directly in entries/ or stamps/ of a memory dir, a directory
// of the work tree whose name is not a dotfile. Each level is opened
// with O_NOFOLLOW relative to the one above (openat), so no symbolic
// link the sandbox plants, before or during the walk, leads the read
// outside the memory dir (08 8.1, REQ-1304). A symbolic link, a
// directory or another node where an entry or a stamp belongs is the
// layout allowlist's refusal (REQ-0645), and an error: nothing is
// staged. The returned function closes the directories the files are
// opened from, once they are staged.
func collect(workTree string) ([]gitsafe.StagedFile, func(), error) {
	var open []int
	closeAll := func() {
		for _, fd := range open {
			_ = unix.Close(fd)
		}
	}
	fail := func(err error) ([]gitsafe.StagedFile, func(), error) {
		closeAll()
		return nil, func() {}, err
	}
	rootFD, err := openDir(unix.AT_FDCWD, workTree)
	if err != nil {
		return fail(fmt.Errorf("memgit: the memory dir: %w", err))
	}
	open = append(open, rootFD)
	dirs, err := readNames(rootFD, memstore.MaxEntries)
	if err != nil {
		return fail(err)
	}
	var files []gitsafe.StagedFile
	var total int64
	for _, dir := range dirs {
		dirFD, err := openDir(rootFD, dir)
		switch {
		case notADir(err):
			continue // a file or a link at the top: the excludes leave it out, and so does this
		case err != nil:
			return fail(fmt.Errorf("memgit: the memory dir %s: %w", dir, err))
		}
		subs, err := openSubs(dir, dirFD)
		_ = unix.Close(dirFD)
		open = append(open, subs...)
		if err != nil {
			return fail(err)
		}
		for i, sub := range []string{"entries", "stamps"} {
			subFD := subs[i]
			if subFD < 0 {
				continue
			}
			names, err := readNames(subFD, memstore.MaxEntries)
			if err != nil {
				return fail(fmt.Errorf("memgit: %s/%s: %w", dir, sub, err))
			}
			for _, name := range names {
				p := dir + "/" + sub + "/" + name
				size, err := regularSize(subFD, name)
				if err != nil {
					return fail(fmt.Errorf("memgit: %s %w, which the layout allowlist of 08 8.1 refuses; nothing was committed or pushed", p, err))
				}
				if size > memstore.MaxFileSize {
					return fail(fmt.Errorf("memgit: %s holds %d bytes, over the cap of %d of 08 8.6 (REQ-0648); nothing was committed or pushed", p, size, memstore.MaxFileSize))
				}
				if total += size; total > maxTotal {
					return fail(fmt.Errorf("memgit: the memory dirs hold more than %d bytes in entries/ and stamps/; nothing was committed or pushed", maxTotal))
				}
				fd := subFD
				files = append(files, gitsafe.StagedFile{Path: p, Size: size, Open: func() (io.ReadCloser, error) {
					return openRegular(fd, name)
				}})
			}
		}
	}
	return files, closeAll, nil
}

// maxTotal bounds the bytes of all the files Sync stages, so a sandbox
// that writes many files at the cap of one cannot fill the host's disk
// with the memory repository: 08 8.6 caps a file and the entries of one
// memory dir, not the sum. It is far above a working store.
var maxTotal int64 = 1 << 30

// notADir reports whether err is that of openDir on a name that is a
// file, a link or nothing: the reasons to skip a name at the top of the
// work tree. Any other error (EACCES, EMFILE) fails the walk, so a
// directory that could not be read never stages as deleted.
func notADir(err error) bool {
	return errors.Is(err, unix.ENOTDIR) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOENT)
}

// openSubs opens entries/ and stamps/ of the memory dir dir, open at
// dirFD, and returns their descriptors, -1 for one that does not exist.
// One that is not a directory is the layout allowlist's refusal.
func openSubs(dir string, dirFD int) ([]int, error) {
	fds := []int{-1, -1}
	for i, sub := range []string{"entries", "stamps"} {
		fd, err := openDir(dirFD, sub)
		switch {
		case errors.Is(err, unix.ENOENT):
			continue
		case notADir(err):
			return slices.DeleteFunc(fds, func(fd int) bool { return fd < 0 }),
				fmt.Errorf("memgit: %s/%s is not a directory, which the layout allowlist of 08 8.1 refuses; nothing was committed or pushed", dir, sub)
		case err != nil:
			return slices.DeleteFunc(fds, func(fd int) bool { return fd < 0 }), fmt.Errorf("memgit: %s/%s: %w", dir, sub, err)
		}
		fds[i] = fd
	}
	return fds, nil
}

// openDir opens the directory name below the directory dirFD, following
// no link.
func openDir(dirFD int, name string) (int, error) {
	return unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
}

// readNames returns the names in the directory fd that are not
// dotfiles, sorted, leaving fd open, and fails on more than limit of
// them, read in batches so a directory of millions of names is not held.
func readNames(fd, limit int) ([]string, error) {
	dup, err := unix.Dup(fd)
	if err != nil {
		return nil, fmt.Errorf("memgit: %w", err)
	}
	f := os.NewFile(uintptr(dup), "dir")
	defer func() { _ = f.Close() }()
	var names []string
	for {
		batch, err := f.Readdirnames(256)
		for _, n := range batch {
			if !strings.HasPrefix(n, ".") {
				names = append(names, n)
			}
		}
		if len(names) > limit {
			return nil, fmt.Errorf("more than %d names, over the cap of 08 8.6 (REQ-0648); nothing was committed or pushed", limit)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("memgit: %w", err)
		}
	}
	slices.Sort(names)
	return names, nil
}

// regularSize returns the size of the regular file name below dirFD, or
// why it is none.
func regularSize(dirFD int, name string) (int64, error) {
	var st unix.Stat_t
	if err := unix.Fstatat(dirFD, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return 0, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return 0, errors.New("is not a regular file")
	}
	return st.Size, nil
}

// openRegular opens the regular file name below dirFD, following no
// link, blocking on no FIFO and taking no terminal, and refuses
// anything else.
func openRegular(dirFD int, name string) (io.ReadCloser, error) {
	fd, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	return f, nil
}
