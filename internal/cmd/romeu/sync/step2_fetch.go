// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	dir, err := configDir(h)
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

// configDir returns the dir of the clone of the config repo, which
// romeu init made at <root>/<cfg>-env/<dir>, <dir> being the config
// spec's primary repo dir: the one directory there that holds a .git.
// The spec that names <dir> is in the clone, so the dir is found, not
// read. None, or more than one, is exit 2: a config project with a
// second repo is not supported by this lookup yet.
func configDir(h spec.HostSettings) (string, error) {
	env := filepath.Join(h.Root, h.Config.Project+"-env")
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
