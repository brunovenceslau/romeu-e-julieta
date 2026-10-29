// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

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
	"strconv"
	"strings"
)

// Separate is a repository whose git dir and work tree are apart: the
// durable copy of a project's memory, whose git dir <name>-env/memory.git
// sits outside every mounted directory and whose work tree is
// <name>-env/memory/ (08 8.1, REQ-1303). git --separate-git-dir would
// write a .git file into the work tree, which the layout allowlist of 08
// 8.1 refuses in a memory dir and which would show the sandbox a host
// path; so the git dir is a bare repository, and each verb names both
// on git's command line (--git-dir, --work-tree), with core.bare=false,
// and writes neither into a config.
type Separate struct {
	// GitDir is the absolute path of the git dir.
	GitDir string
	// WorkTree is the absolute path of the work tree.
	WorkTree string
}

// separateSettings are the extra settings of every verb of a Separate:
// the bare git dir is read with a work tree.
var separateSettings = []setting{{"core.bare", "false"}}

// global returns git's options that name the git dir and the work tree.
func (s Separate) global() []string {
	return []string{"--git-dir=" + s.GitDir, "--work-tree=" + s.WorkTree}
}

// check returns ErrRefused, wrapped with the reason, for a Separate whose
// paths are not absolute and clean, hold a control character, or lie
// one inside the other, or whose work tree is not a directory (a
// symbolic link included).
func (s Separate) check() error {
	for _, p := range []string{s.GitDir, s.WorkTree} {
		switch {
		case !filepath.IsAbs(p) || filepath.Clean(p) != p || p == "/":
			return fmt.Errorf("%w: %q is not an absolute, clean path below /", ErrRefused, p)
		case strings.ContainsFunc(p, isControl):
			return fmt.Errorf("%w: %q holds a control character", ErrRefused, p)
		}
	}
	if within(s.GitDir, s.WorkTree) || within(s.WorkTree, s.GitDir) {
		return fmt.Errorf("%w: the git dir %q and the work tree %q lie one inside the other", ErrRefused, s.GitDir, s.WorkTree)
	}
	fi, err := os.Lstat(s.WorkTree)
	switch {
	case err != nil:
		return fmt.Errorf("gitsafe: the work tree %q: %w", s.WorkTree, err)
	case !fi.IsDir():
		return fmt.Errorf("%w: the work tree %q is not a directory", ErrRefused, s.WorkTree)
	}
	return nil
}

// within reports whether p is dir or below it; both are clean.
func within(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// separate checks s and that its git dir is a repository, and refuses
// sandbox mode: julieta never reaches the memory repository.
func (r *Runner) separate(s Separate) error {
	if r.mode != Host {
		return fmt.Errorf("%w: the memory repository in %s mode", ErrRefused, r.mode)
	}
	if err := s.check(); err != nil {
		return err
	}
	return checkRepo(s.GitDir)
}

// runSeparate runs a verb of s: git in the work tree, with the git dir
// and the work tree named on its command line and the filter drivers of
// the git dir's config emptied (filterOverrides), so neither git add
// nor a checkout runs one.
func (r *Runner) runSeparate(ctx context.Context, s Separate, extra []setting, env []string, args ...string) error {
	if slices.Contains(refusedSubcommands, args[0]) {
		return fmt.Errorf("%w: git subcommand %q", ErrRefused, args[0])
	}
	noFilters, err := r.filterOverrides(ctx, s.GitDir)
	if err != nil {
		return err
	}
	return r.exec(ctx, s.WorkTree, s.global(), slices.Concat(separateSettings, extra), slices.Concat(noFilters, env), nil, nil, args...)
}

// InitSeparate makes the git dir of s, which must not exist yet, a new
// bare repository whose HEAD names refs/heads/main, with no template,
// so no hook or file of the machine's templates enters it, and with the
// work tree, which must exist, left as it is. Host mode only.
func (r *Runner) InitSeparate(ctx context.Context, s Separate) error {
	if r.mode != Host {
		return fmt.Errorf("%w: the memory repository in %s mode", ErrRefused, r.mode)
	}
	if err := s.check(); err != nil {
		return err
	}
	switch _, err := os.Lstat(s.GitDir); {
	case err == nil:
		return fmt.Errorf("%w: the git dir %q exists", ErrRefused, s.GitDir)
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("gitsafe: the git dir %q: %w", s.GitDir, err)
	}
	return r.runInput(ctx, filepath.Dir(s.GitDir), nil, nil, nil, nil,
		"init", "-q", "--bare", "--template=", "--initial-branch=main", "--", s.GitDir)
}

// StagedFile is one file StageExactly stages: its slash path from the
// root of the work tree, its size, and the function that opens it. The
// caller opens it so that no link is followed (O_NOFOLLOW at each
// level), and the reader yields exactly Size bytes.
type StagedFile struct {
	Path string
	Size int64
	Open func() (io.ReadCloser, error)
}

// maxStaged bounds the files of one StageExactly, above the caps of 08
// 8.6 for every memory dir of a project.
const maxStaged = 200_000

// StageExactly makes the index of s hold files and nothing else, each
// a regular file of mode 100644. git reads no path of the work tree:
// the blobs are written from the readers the caller opened, with one
// git fast-import, which writes no object the repository already
// holds, and the index is then rebuilt with git read-tree --empty and
// git update-index -z --index-info, whose NUL-terminated paths git does
// not unquote. So no .gitignore, symbolic link or
// nested repository of the work tree, which the sandbox writes, can
// change what is staged (08 8.1, REQ-1304).
func (r *Runner) StageExactly(ctx context.Context, s Separate, files []StagedFile) error {
	if err := r.separate(s); err != nil {
		return err
	}
	if len(files) > maxStaged {
		return fmt.Errorf("%w: %d files to stage, over %d", ErrRefused, len(files), maxStaged)
	}
	// Two paths one under case folding or Unicode normalization are one
	// file on APFS, and WalkTree refuses such a tree on every other
	// machine, so they are refused here, before git runs.
	seen := map[string]string{}
	for _, f := range files {
		if err := checkLiteralPath(f.Path); err != nil {
			return err
		}
		key, err := checkTreePath(f.Path)
		if err != nil {
			return err
		}
		if f.Size < 0 || f.Open == nil {
			return fmt.Errorf("%w: the staged file %q", ErrRefused, f.Path)
		}
		if other, ok := seen[key]; ok {
			return fmt.Errorf("%w: the staged files %q and %q are one under case folding or normalization", ErrRefused, other, f.Path)
		}
		seen[key] = f.Path
	}
	oids, err := r.writeBlobs(ctx, s, files)
	if err != nil {
		return err
	}
	if err := r.runSeparate(ctx, s, nil, nil, "read-tree", "--empty"); err != nil {
		return err
	}
	var info bytes.Buffer
	for i, f := range files {
		fmt.Fprintf(&info, "%s %s\t%s\x00", modeFile, oids[i], f.Path)
	}
	noFilters, err := r.filterOverrides(ctx, s.GitDir)
	if err != nil {
		return err
	}
	return r.exec(ctx, s.WorkTree, s.global(), separateSettings, noFilters, &info, nil, "update-index", "-z", "--index-info")
}

// writeBlobs writes the blob of each file with git fast-import, which
// reads them from its standard input and answers each get-mark with the
// blob's id, and returns the ids in the order of files.
func (r *Runner) writeBlobs(ctx context.Context, s Separate, files []StagedFile) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	pr, pw := io.Pipe()
	streamed := make(chan error, 1)
	go func() {
		err := streamBlobs(pw, files)
		_ = pw.CloseWithError(err)
		streamed <- err
	}()
	out := limitedBuffer{limit: len(files) * (maxOIDLine + 1)}
	err := r.exec(ctx, s.WorkTree, s.global(), separateSettings, nil, pr, &out, "fast-import", "--quiet", "--done")
	_ = pr.Close() // a git that stopped reading ends the stream
	if serr := <-streamed; serr != nil && (err == nil || !errors.Is(serr, io.ErrClosedPipe)) {
		return nil, serr // the cause: git's error follows from it
	}
	if err != nil {
		return nil, err
	}
	oids := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if out.truncated || len(oids) != len(files) {
		return nil, fmt.Errorf("%w: git fast-import answered %d ids for %d files", ErrRefused, len(oids), len(files))
	}
	for _, oid := range oids {
		if err := checkObjectID(oid); err != nil {
			return nil, err
		}
	}
	return oids, nil
}

// maxOIDLine is the longest object id, SHA-256's.
const maxOIDLine = 64

// streamBlobs writes the fast-import stream of files to w: per file a
// blob with a mark and its exact bytes, then a get-mark, and done.
func streamBlobs(w io.Writer, files []StagedFile) error {
	for i, f := range files {
		mark := strconv.Itoa(i + 1)
		if _, err := fmt.Fprintf(w, "blob\nmark :%s\ndata %d\n", mark, f.Size); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("gitsafe: %s: %w", f.Path, err)
		}
		_, err = io.CopyN(w, rc, f.Size)
		var extra [1]byte
		more, _ := io.ReadFull(rc, extra[:])
		_ = rc.Close()
		switch {
		case errors.Is(err, io.EOF), more > 0:
			return fmt.Errorf("%w: %s changed size while it was staged", ErrRefused, f.Path)
		case err != nil:
			return fmt.Errorf("gitsafe: %s: %w", f.Path, err)
		}
		if _, err := fmt.Fprintf(w, "\nget-mark :%s\n", mark); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "done\n")
	return err
}

// StagedChanges reports whether the index of s differs from HEAD, or
// from the empty tree on an unborn branch: git diff --cached --quiet.
func (r *Runner) StagedChanges(ctx context.Context, s Separate) (bool, error) {
	if err := r.separate(s); err != nil {
		return false, err
	}
	err := r.runSeparate(ctx, s, nil, nil, "diff", "--cached", "--quiet", "--no-ext-diff", "--no-textconv")
	switch {
	case err == nil:
		return false, nil
	case exitedWith(err, 1):
		return true, nil
	}
	return false, err
}

// Identity is who a commit of the memory repository is by, and how it
// is signed (08 8.1, REQ-1304).
type Identity struct {
	// Name is the author's and the committer's name: the project's.
	Name string
	// SigningKey is the OpenSSH public key line the commit is signed
	// with, and AgentSocket the socket of the agent that holds it. Both
	// set sign the commit, with SSH_AUTH_SOCK set for that git alone;
	// both empty leave it unsigned. One without the other is refused.
	SigningKey, AgentSocket string
}

// Commit commits the index of s on refs/heads/main, with message as
// the whole message, by id: author and committer id.Name, with an empty
// email, signed when id names a key. It reads no path of the work tree:
// git write-tree writes the index's tree, git commit-tree the commit on
// the branch's tip, and git update-ref moves the branch from that tip,
// so a branch another process moved meanwhile is not overwritten. No
// hook runs, and the identity and the key are passed per call, written
// into no config (I2).
func (r *Runner) Commit(ctx context.Context, s Separate, id Identity, message string) error {
	if err := r.separate(s); err != nil {
		return err
	}
	if id.Name == "" || strings.ContainsFunc(id.Name, isControl) || strings.ContainsAny(id.Name, "<>") {
		return fmt.Errorf("%w: the author name %q", ErrRefused, id.Name)
	}
	if strings.ContainsFunc(message, func(r rune) bool { return isControl(r) && r != '\n' }) || message == "" {
		return fmt.Errorf("%w: a commit message with a control character, or none", ErrRefused)
	}
	env := []string{
		"GIT_AUTHOR_NAME=" + id.Name, "GIT_AUTHOR_EMAIL=",
		"GIT_COMMITTER_NAME=" + id.Name, "GIT_COMMITTER_EMAIL=",
	}
	var extra []setting
	args := []string{"commit-tree"}
	switch {
	case id.SigningKey != "" && id.AgentSocket != "":
		if strings.ContainsFunc(id.SigningKey, isControl) || !strings.HasPrefix(id.SigningKey, "ssh-") {
			return fmt.Errorf("%w: the signing key is not an OpenSSH public key line", ErrRefused)
		}
		if !filepath.IsAbs(id.AgentSocket) || strings.ContainsFunc(id.AgentSocket, isControl) {
			return fmt.Errorf("%w: the agent socket %q is not an absolute path", ErrRefused, id.AgentSocket)
		}
		extra = []setting{
			{"gpg.format", "ssh"},
			{"gpg.ssh.program", "ssh-keygen"},
			{"user.signingKey", "key::" + id.SigningKey},
		}
		args = append(args, "-S")
		env = append(env, "SSH_AUTH_SOCK="+id.AgentSocket)
	case id.SigningKey != "" || id.AgentSocket != "":
		return fmt.Errorf("%w: a signing key without an agent socket, or a socket without a key", ErrRefused)
	}
	tree, err := r.separateLine(ctx, s, nil, nil, "write-tree")
	if err != nil {
		return err
	}
	parent, hasParent, err := r.RefOID(ctx, s.GitDir, branchMain)
	if err != nil {
		return err
	}
	if hasParent {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", message, tree)
	commit, err := r.separateLine(ctx, s, extra, env, args...)
	if err != nil {
		return err
	}
	// update-ref <ref> <new> <old>: an empty old value is "must not exist".
	return r.runSeparate(ctx, s, nil, nil, "update-ref", branchMain, commit, parent)
}

// branchMain is the branch of the memory repository.
const branchMain = "refs/heads/main"

// separateLine runs a verb of s that answers one object id, and returns
// it.
func (r *Runner) separateLine(ctx context.Context, s Separate, extra []setting, env []string, args ...string) (string, error) {
	out := limitedBuffer{limit: maxOIDLine + 1}
	if err := r.exec(ctx, s.WorkTree, s.global(), slices.Concat(separateSettings, extra), env, nil, &out, args...); err != nil {
		return "", err
	}
	oid := strings.TrimSuffix(out.String(), "\n")
	if out.truncated {
		return "", fmt.Errorf("%w: git %s answered more than an object id", ErrRefused, args[0])
	}
	if err := checkObjectID(oid); err != nil {
		return "", err
	}
	return oid, nil
}

// FastForwardSeparate moves the branch of s forward to oid, checking
// the change out into the work tree, and fails when that is not a
// fast-forward, when an untracked or ignored file stands where the
// move would write one (--no-overwrite-ignore), or when a changed file
// of the work tree would be overwritten. An unborn branch moves to oid.
// It verifies no signature, so merge.verifySignatures never starts a
// program, and runs no filter (runSeparate). It is the one move of a
// branch in host mode, in the memory repository only, as 08 8.1 asks
// of a remote that moved on (REQ-1304).
func (r *Runner) FastForwardSeparate(ctx context.Context, s Separate, oid string) error {
	if err := r.separate(s); err != nil {
		return err
	}
	if err := checkObjectID(oid); err != nil {
		return err
	}
	return r.runSeparate(ctx, s, nil, nil, "merge", "-q", "--ff-only", "--no-stat", "--no-overwrite-ignore", "--no-verify-signatures", oid)
}

// IsAncestor reports whether the commit a is an ancestor of the commit
// b, or b itself, in the repository in dir: git merge-base
// --is-ancestor. Both are full object ids.
func (r *Runner) IsAncestor(ctx context.Context, dir, a, b string) (bool, error) {
	if err := checkRepo(dir); err != nil {
		return false, err
	}
	for _, oid := range []string{a, b} {
		if err := checkObjectID(oid); err != nil {
			return false, err
		}
	}
	err := r.runInput(ctx, dir, historySettings, historyEnv, nil, nil, "merge-base", "--is-ancestor", a, b)
	switch {
	case err == nil:
		return true, nil
	case exitedWith(err, 1):
		return false, nil
	}
	return false, err
}
