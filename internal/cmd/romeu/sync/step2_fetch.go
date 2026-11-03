// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// fetch is the hardened fetch of an origin into a clone. A unit test
// replaces it with a fetch from a local repository, which gitsafe
// refuses, since it fetches https URLs only; the e2e covers the real
// one.
var fetch = func(ctx context.Context, g *gitsafe.Runner, dir, url string, refspecs []string) error {
	return g.Fetch(ctx, dir, url, refspecs)
}

// fetchConfig is step 2 of 04 4.2 (REQ-0336): the hardened fetch of the
// config repo's origin into refs/romeu/origin/<cfg-dir>/*, then the
// spec commit: the head of the config ref, or with --from <sha> that
// commit, which must be reachable from a ref the fetch wrote (exit 2
// otherwise). It fills git, configClone and specCommit.
func fetchConfig(ctx context.Context, r *run) error {
	h := r.settings
	g, err := hostGit(h, r.home)
	if err != nil {
		return err
	}
	dir, err := configDir(r)
	if err != nil {
		return err
	}
	clone := filepath.Join(h.Root, h.Config.Project+"-env", dir)
	prefix := "refs/romeu/origin/" + dir + "/"
	if err := fetch(ctx, g, clone, h.Config.URL, []string{"+refs/heads/*:" + prefix + "*"}); err != nil {
		return err
	}
	head, ok, err := g.RefOID(ctx, clone, prefix+h.Config.Ref)
	if err != nil {
		return err
	}
	if !ok {
		return usage("config-ref-missing", "the config repo's origin has no branch "+termsafe.Escape(h.Config.Ref)+", config.ref of the host settings")
	}
	commit := head
	if r.flags.from != fromOrigin {
		if err := reachable(ctx, g, clone, r.flags.from, prefix); err != nil {
			return err
		}
		commit = r.flags.from
	}
	r.git, r.configClone, r.specCommit = g, clone, commit
	return nil
}

// hostGit returns the hardened git of the host settings h, run with
// HOME home: the recorded credential helper answers the host of
// config.url only (05 5.1, REQ-0464), and each gitHosts entry brings
// its CA bundle.
func hostGit(h spec.HostSettings, home string) (*gitsafe.Runner, error) {
	c := gitsafe.Config{Mode: gitsafe.Host, Git: h.Tools.Git, Home: home}
	for _, gh := range h.GitHosts {
		c.GitHosts = append(c.GitHosts, gitsafe.GitHost{Host: gh.Host, CAFile: gh.CAFile})
	}
	if h.Tools.GitCredentialHelper != "" {
		u, err := url.Parse(h.Config.URL)
		if err != nil {
			return nil, usage("settings-invalid", "config.url: "+termsafe.Escape(err.Error()))
		}
		c.CredentialHelper, c.CredentialHost = h.Tools.GitCredentialHelper, u.Host
	}
	g, err := gitsafe.New(c)
	if err != nil {
		return nil, usage("settings-invalid", termsafe.Escape(err.Error()))
	}
	return g, nil
}

// markerName is the marker of the config repo's clone, <cfg>-env/.romeu-config
// (02 2.3), which init writes: one line, the dir of the clone. The init
// package declares it as MarkerName; a command package imports no
// other (10 10.7), so the name is repeated here.
const markerName = ".romeu-config"

// markerLimit bounds the marker read: a dir name is at most 63 bytes
// (03 3.1), and a newline.
const markerLimit = 256

// configDir returns the dir of the clone of the config repo, which
// romeu init made at <root>/<cfg>-env/<dir>: the one the marker names
// (readMarker), or, for a config project made before the marker, the
// one directory there that holds a .git, with the warning
// config-marker-missing, whose hint names the marker to write. None,
// or more than one, is exit 2, as a marker that is malformed or names
// no clone is (the rule slug config-marker, REQ-1258).
func configDir(r *run) (string, error) {
	h := r.settings
	env := filepath.Join(h.Root, h.Config.Project+"-env")
	marker := filepath.Join(env, markerName)
	if dir, found, err := readMarker(marker); err != nil {
		return "", err
	} else if found {
		fi, err := os.Lstat(filepath.Join(env, dir))
		if err != nil || !fi.IsDir() {
			return "", usage("config-marker", termsafe.Escape(marker)+" names "+termsafe.Escape(dir)+", which is not a plain directory under "+termsafe.Escape(env))
		}
		if fi, err := os.Lstat(filepath.Join(env, dir, ".git")); err != nil || !fi.IsDir() {
			return "", usage("config-marker", termsafe.Escape(marker)+" names "+termsafe.Escape(dir)+", which holds no .git directory under "+termsafe.Escape(env))
		}
		return dir, nil
	}
	dir, err := guessConfigDir(env)
	if err != nil {
		return "", err
	}
	r.warnings = append(r.warnings, func(w *cli.Writer) {
		cli.Warn(w, "config-marker-missing", termsafe.Escape(env)+" has no "+markerName+" naming the config repo's clone, which a newer romeu init writes; sync took "+termsafe.Escape(dir)+", the one dir there with a .git; write "+termsafe.Escape(dir)+" as the only line of "+termsafe.Escape(marker)+" to stop the guess")
	})
	return dir, nil
}

// readMarker reads the marker: found is false when there is none. A
// marker that is not a plain file (opened without following a link,
// and judged by the open file, so nothing swapped in between is read),
// is over markerLimit, or whose one line fails the repo dir rule of 03
// 3.1 (REQ-0176: the pattern and the reserved names of the rule
// repo-dir, read from spec.Rules so the two cannot drift) is exit 2,
// the rule slug config-marker.
func readMarker(marker string) (dir string, found bool, err error) {
	refuse := func(why string) error {
		return usage("config-marker", termsafe.Escape(marker)+" "+why)
	}
	f, err := os.OpenFile(marker, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if errors.Is(err, syscall.ELOOP) {
		return "", true, refuse("is a symbolic link, not a plain file")
	}
	if err != nil {
		return "", false, fmt.Errorf("the config repo's marker: %w", err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return "", true, fmt.Errorf("the config repo's marker: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return "", true, refuse("is not a plain file")
	}
	data, err := io.ReadAll(io.LimitReader(f, markerLimit+1))
	if err != nil {
		return "", true, fmt.Errorf("the config repo's marker: %w", err)
	}
	if len(data) > markerLimit {
		return "", true, refuse(fmt.Sprintf("is over %d bytes; it holds one line, the dir of the config repo's clone", markerLimit))
	}
	dir, ok := strings.CutSuffix(string(data), "\n")
	if !ok || !repoDir.pattern.MatchString(dir) || slices.Contains(repoDir.reserved, dir) {
		return "", true, refuse("holds no repo dir name on one line (the rule repo-dir of 03 3.1)")
	}
	return dir, true, nil
}

// dirRule is a rule on a name: its pattern and its reserved names.
type dirRule struct {
	pattern  *regexp.Regexp
	reserved []string
}

// repoDir is the rule repo-dir of 03 3.1 as spec.Rules states it: the
// pattern and the reserved names a marker's line is held to.
var repoDir = func() dirRule {
	for _, rule := range spec.Rules() {
		if rule.Slug == "repo-dir" {
			return dirRule{regexp.MustCompile(rule.Pattern), rule.Reserved}
		}
	}
	panic("spec.Rules holds no rule repo-dir")
}()

// guessConfigDir is the lookup of before the marker: the one directory
// under env that holds a .git. The spec that names <dir> is in the
// clone, so the dir is found, not read. None, or more than one, is
// exit 2.
func guessConfigDir(env string) (string, error) {
	entries, err := os.ReadDir(env)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("the config repo's clone: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Lstat(filepath.Join(env, e.Name(), ".git")); err == nil {
			dirs = append(dirs, e.Name())
		}
	}
	switch len(dirs) {
	case 1:
		return dirs[0], nil
	case 0:
		return "", usage("config-clone-missing", "no clone of the config repo under "+termsafe.Escape(env)+"; run romeu init")
	}
	return "", usage("config-clone-ambiguous", "each of "+termsafe.Escape(strings.Join(dirs, ", "))+" under "+termsafe.Escape(env)+" could be the config repo's clone")
}

// reachable returns nil when from, the value of --from, a full object
// id, names a commit that a ref of the namespace prefix,
// refs/romeu/origin/<cfg-dir>/, reaches: a branch of the origin as
// step 2 fetched it, the config ref or any other (REQ-0336, REQ-0466),
// listed by gitsafe (T183). Otherwise it returns usage (exit 2), a
// commit the clone does not hold among them. An id that names a tree or
// a blob is refused too, so step 3 never reads projects/ from an object
// that is not a commit.
func reachable(ctx context.Context, g *gitsafe.Runner, clone, from, prefix string) error {
	refuse := func(why string) error {
		return usage("from-unreachable", "--from "+termsafe.Escape(from)+" "+why)
	}
	// flags.check took from as a full object id before step 1. With no
	// exclude, the walk lists from itself exactly when it is a
	// commit: rev-list lists commits only.
	commit, err := g.Unreached(ctx, clone, []string{from}, nil)
	if _, ok := errors.AsType[*procgroup.ExitError](err); ok {
		return refuse("names no commit of the config repo's clone")
	}
	if err != nil {
		return err
	}
	if len(commit) != 1 || commit[0] != from {
		return refuse("names no commit of the config repo's clone")
	}
	refs, _, err := g.ListRefs(ctx, clone, []string{prefix})
	if err != nil {
		return err
	}
	tips := make([]string, len(refs))
	for i, ref := range refs {
		tips[i] = ref.OID
	}
	out, err := g.Unreached(ctx, clone, []string{from}, tips)
	if err != nil {
		return err
	}
	if len(out) > 0 {
		return refuse("is not reachable from " + termsafe.Escape(prefix) + "*")
	}
	return nil
}
