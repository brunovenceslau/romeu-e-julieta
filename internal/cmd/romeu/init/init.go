// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package initcmd is the command romeu init (04 4.2, REQ-0309): it
// writes the host settings (root, config and tools; 03 3.4) and clones
// the config repo into <root>/<cfg>-env/<dir>. The directory is named
// init so that the command's path matches its name; the package name
// cannot be init.
package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The environment and lookups of the command; a test replaces them.
var (
	getenv     = os.Getenv
	lookPath   = procgroup.LookPath
	defaultRef = "main"

	// cloneRepo is the hardened clone of the config repo, with the
	// credential helper for its host only when helper is not empty; a
	// unit test replaces it with a fixture, since the e2e covers the
	// real one.
	cloneRepo = func(ctx context.Context, g *gitsafe.Runner, url, dest, helper string) error {
		return g.CloneWithCredentialHelper(ctx, url, dest, helper)
	}
)

// dirMode is the mode of the directories init creates: the root and
// <cfg>-env/ (02 2.3, REQ-0141).
const dirMode = 0o700

// configFileLimit bounds the config project's spec read from the clone.
const configFileLimit = spec.MaxFileSize

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "init"
	c.New = func(env *cli.Env) *cobra.Command {
		var o options
		cmd := &cobra.Command{
			Use:   "init --config-url <url>",
			Short: "write the host settings and clone the config repo",
			Example: "romeu init --config-url https://github.com/<owner>/<config-repo>\n" +
				"romeu init --config-url https://github.com/<owner>/<config-repo> --root /abs/dev\n" +
				"romeu init --config-url https://github.com/<owner>/<config-repo> --git-credential-helper osxkeychain",
			Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return run(c, env, o)
			},
		}
		f := cmd.Flags()
		f.StringVar(&o.configURL, "config-url", "", "the https URL of the config repo (required)")
		f.StringVar(&o.project, "config-project", "", "the config project's name (default: the repo's name)")
		f.StringVar(&o.root, "root", "", "the absolute root of the host tree (default: $HOME/Developer)")
		f.StringVar(&o.ref, "config-ref", defaultRef, "the branch that holds projects/<name>.yaml")
		f.StringVar(&o.helper, "git-credential-helper", "", "the git credential helper of a private config repo, for its host only: a git-credential-<name> helper's name, or the absolute path of an executable only you or root can write; recorded as tools.gitCredentialHelper. Git may send it erase when the origin refuses a credential, which deletes what a storing helper (osxkeychain, store) holds")
		_ = cmd.MarkFlagRequired("config-url")
		return cmd
	}
	return c
}())

// options are the flags of the command.
type options struct {
	configURL, project, root, ref, helper string
}

// usage returns the error usage (RJ-349) for an input init refuses,
// with the hint of the help of the command.
func usage(cmd *cobra.Command, message string) error {
	e := cli.Usage.New(message)
	binary, command, _ := strings.Cut(cmd.CommandPath(), " ")
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}

// refuse is a precondition init stops at, named by the slug of the rule
// it breaks, with exit 2 (REQ-1258: a rule with no row of 04 4.4 has no
// id of its own).
func refuse(cmd *cobra.Command, slug, message string) error {
	return usage(cmd, slug+": "+message)
}

// run is the command.
func run(cmd *cobra.Command, env *cli.Env, o options) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage(cmd, "HOME is not an absolute path")
	}
	settings := env.SettingsPath
	if settings == "" {
		var err error
		if settings, err = spec.SettingsPath(getenv); err != nil {
			return usage(cmd, err.Error())
		}
	}
	h, err := build(cmd, o, home)
	if err != nil {
		return err
	}
	if err := absent(cmd, settings); err != nil {
		return err
	}

	// The root check comes before anything is written, the state
	// directory included (REQ-0309, I20).
	rs, err := realPath(settings)
	if err != nil {
		return usage(cmd, "the settings path: "+err.Error())
	}
	stateDir, err := state.DefaultDir(getenv)
	if err != nil {
		return usage(cmd, err.Error())
	}
	rstate, err := realPath(stateDir)
	if err != nil {
		return usage(cmd, "the state path: "+err.Error())
	}
	rhome, err := realPath(home)
	if err != nil {
		return usage(cmd, "HOME: "+err.Error())
	}
	if why := rootProblem(h.Root, rhome, rs, rstate); why != "" {
		return refuse(cmd, "root-unsafe", why+"; pass another root with --root")
	}

	g, err := tools(cmd, &h, home)
	if err != nil {
		return err
	}
	if err := g.CheckFloor(ctx); err != nil {
		return floorError(err)
	}

	st, err := state.Open(stateDir)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	lk, err := st.Lock()
	if err != nil {
		return err
	}
	defer lk.Unlock()
	if err := absent(cmd, settings); err != nil { // another init may have won
		return err
	}

	rt, envDir, made, err := openTree(cmd, h.Root, h.Config.Project+"-env")
	if err != nil {
		return err
	}
	defer func() { _ = rt.Close() }()
	// The settings are encoded and their directory and temporary file
	// made before the clone, so that a settings directory init cannot
	// write fails here, with nothing cloned.
	data, err := spec.EncodeHostSettings(h)
	if err != nil {
		return err
	}
	pending, err := prepareSettings(cmd, settings)
	if err != nil {
		return err
	}
	defer func() { _ = pending.Cleanup() }()
	dest, err := cloneConfig(ctx, cmd, g, h, rt, envDir)
	if err != nil {
		// Nothing of this run is left: the env dir and the root, when
		// this run made them and they are empty (a decision of
		// 2026-10-23).
		made.undo(rt, filepath.Base(envDir), h.Root)
		return err
	}
	if err := commitSettings(cmd, pending, settings, data); err != nil {
		// dest is this run's clone: absent proved <envDir>/<dir> did not
		// exist, so nothing of the operator's goes with it.
		err = errors.Join(err, os.RemoveAll(dest))
		made.undo(rt, filepath.Base(envDir), h.Root)
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "settings  %s\nroot      %s\nconfig    %s\n", termsafe.Escape(settings), termsafe.Escape(h.Root), termsafe.Escape(dest))
	return err
}

// prepareSettings makes the directory of settings, mode 0700, and the
// temporary file the settings are written to, mode 0600. A directory or
// a file init cannot make is settings-unwritable: a precondition (exit
// 2, as usage, since 04 4.4 has no row for it) and not an internal error.
func prepareSettings(cmd *cobra.Command, settings string) (*renameio.PendingFile, error) {
	if err := os.MkdirAll(filepath.Dir(settings), dirMode); err != nil {
		return nil, refuse(cmd, "settings-unwritable", err.Error())
	}
	pf, err := renameio.NewPendingFile(settings, renameio.WithStaticPermissions(0o600))
	if err != nil {
		return nil, refuse(cmd, "settings-unwritable", err.Error())
	}
	return pf, nil
}

// commitSettings writes data to the pending file and links it to its
// place, which fails if a file appeared there meanwhile (settings-exist),
// where a rename would silently replace it. The temporary file is
// removed by the caller's Cleanup.
func commitSettings(cmd *cobra.Command, pf *renameio.PendingFile, settings string, data []byte) error {
	if _, err := pf.Write(data); err != nil {
		return refuse(cmd, "settings-unwritable", err.Error())
	}
	if err := pf.Sync(); err != nil {
		return refuse(cmd, "settings-unwritable", err.Error())
	}
	if err := os.Link(pf.Name(), settings); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return refuse(cmd, "settings-exist", "the host settings appeared at "+termsafe.Escape(settings)+" meanwhile; init never overwrites them")
		}
		return refuse(cmd, "settings-unwritable", err.Error())
	}
	return nil
}

// absent refuses settings that exist: init never overwrites them.
func absent(cmd *cobra.Command, settings string) error {
	_, err := os.Lstat(settings)
	switch {
	case err == nil:
		return refuse(cmd, "settings-exist", "the host settings already exist at "+termsafe.Escape(settings)+"; init never overwrites them")
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("init: the settings: %w", err)
	}
	return nil
}

// build returns the settings the flags describe, with the root made
// real; tools fills the rest and validates it by the rules of 03 3.4.
func build(cmd *cobra.Command, o options, home string) (spec.HostSettings, error) {
	var zero spec.HostSettings
	if err := gitsafe.CheckURL(o.configURL); err != nil {
		return zero, usage(cmd, err.Error())
	}
	u, err := url.Parse(o.configURL)
	if err != nil {
		return zero, usage(cmd, "--config-url: "+err.Error())
	}
	if o.project == "" {
		o.project = strings.TrimSuffix(path.Base(u.Path), ".git")
	}
	if o.root == "" {
		o.root = filepath.Join(home, "Developer")
	}
	o.root = filepath.Clean(o.root) // a trailing slash is no fault
	if !filepath.IsAbs(o.root) {
		return zero, refuse(cmd, "root-unsafe", "the root is not an absolute, clean path")
	}
	root, err := realPath(o.root)
	if err != nil {
		return zero, usage(cmd, "--root: "+err.Error())
	}
	h := spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   root,
		Config: spec.ConfigRepo{Project: o.project, URL: o.configURL, Ref: o.ref},
	}
	// The helper is validated with the rest of the settings (REQ-1267),
	// before anything is written; gitsafe's cap bounds it first, so a long
	// value is never echoed back.
	if len(o.helper) > gitsafe.MaxCredentialHelper {
		return zero, usage(cmd, fmt.Sprintf("tools.gitCredentialHelper: the value has %d bytes, over the cap of %d", len(o.helper), gitsafe.MaxCredentialHelper))
	}
	h.Tools.GitCredentialHelper = o.helper
	return h, nil
}

// validate checks h by the rules of 03 3.4 after the tools are set.
func validate(cmd *cobra.Command, h spec.HostSettings) error {
	err := h.Validate()
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*spec.InvalidError](err); !ok && !errors.Is(err, spec.ErrInvalid) {
		return err
	}
	return usage(cmd, "the settings init would write are invalid: "+termsafe.Escape(err.Error()))
}

// tools resolves the absolute paths of sbx and git into h, validates h
// and returns the hardened git runner for the config clone.
func tools(cmd *cobra.Command, h *spec.HostSettings, home string) (*gitsafe.Runner, error) {
	for _, t := range []struct {
		name string
		to   *string
	}{{"sbx", &h.Tools.Sbx}, {"git", &h.Tools.Git}} {
		p, err := lookPath(t.name)
		if err != nil {
			return nil, usage(cmd, t.name+" is not on PATH: install it, then run init again")
		}
		if p, err = filepath.Abs(p); err != nil {
			return nil, usage(cmd, t.name+": "+err.Error())
		}
		*t.to = p
	}
	if err := validate(cmd, *h); err != nil {
		return nil, err
	}
	// A helper at a path that someone else could write is refused here,
	// before anything is written, as every command that runs it does.
	if h.Tools.GitCredentialHelper != "" {
		if err := gitsafe.CheckCredentialHelperFile(h.Tools.GitCredentialHelper); err != nil {
			return nil, usage(cmd, "tools.gitCredentialHelper: "+termsafe.Escape(err.Error()))
		}
	}
	g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: h.Tools.Git, Home: home})
	if err != nil {
		return nil, usage(cmd, err.Error())
	}
	return g, nil
}

// cloneConfig clones the config repo under envDir, reads the config
// project's spec at the ref, and moves the clone to <envDir>/<dir>, dir
// being the config repo's own entry under repos (03 3.2). It returns
// the path of the clone. The move goes through rt, the handle of the
// root, so a link planted under it cannot redirect it. A failure of its
// own leaves nothing behind; once it returns the path, the clone is in
// place and removing it, if a later step fails, is the caller's.
func cloneConfig(ctx context.Context, cmd *cobra.Command, g *gitsafe.Runner, h spec.HostSettings, rt *os.Root, envDir string) (string, error) {
	stage, err := os.MkdirTemp(envDir, ".init-")
	if err != nil {
		return "", fmt.Errorf("init: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()
	clone := filepath.Join(stage, "clone")
	if err := cloneRepo(ctx, g, h.Config.URL, clone, h.Tools.GitCredentialHelper); err != nil {
		return "", cloneError(cmd, err, h.Config.URL, false)
	}
	dir, err := configDir(ctx, cmd, g, h, clone)
	if err != nil {
		// The clone is the remote's content: what gitsafe refuses in it
		// (a default branch, a ref, a file) is the remote's, not the
		// operator's input.
		return "", cloneError(cmd, err, h.Config.URL, true)
	}
	if !filepath.IsLocal(dir) || strings.ContainsRune(dir, filepath.Separator) {
		return "", usage(cmd, "the config repo's dir is not a plain name")
	}
	env := filepath.Base(envDir)
	dest := filepath.Join(envDir, dir)
	if _, err := rt.Lstat(filepath.Join(env, dir)); err == nil {
		return "", refuse(cmd, "clone-exists", dest+" exists already; move it away and run init again")
	}
	if err := rt.Rename(filepath.Join(env, filepath.Base(stage), "clone"), filepath.Join(env, dir)); err != nil {
		return "", fmt.Errorf("init: %w", err)
	}
	return dest, nil
}

// openTree makes the root and <root>/<envName>, and returns a handle of
// the root and the path of the env dir. A root or an env dir that is a
// symbolic link, not a directory, not owned by the user or writable by
// its group or others is root-unsafe: it is checked after it is made,
// not only before (rootProblem), so a directory or a link created in
// between is caught. A directory init made is 0700; one that was there
// is chmod'ed through the handle only when the checks passed, and never
// followed out of the root.
func openTree(cmd *cobra.Command, root, envName string) (*os.Root, string, made, error) {
	var m made
	// The levels above the root are made as they are needed and stay;
	// the root's last element is this run's when its Mkdir succeeds.
	if err := os.MkdirAll(filepath.Dir(root), dirMode); err != nil {
		return nil, "", m, fmt.Errorf("init: the root: %w", err)
	}
	if err := os.Mkdir(root, dirMode); err == nil {
		m.root = true
	} else if !errors.Is(err, fs.ErrExist) {
		return nil, "", m, fmt.Errorf("init: the root: %w", err)
	}
	if why := dirProblem(os.Lstat, root); why != "" {
		return nil, "", m, refuse(cmd, "root-unsafe", "the root "+why)
	}
	rt, err := os.OpenRoot(root)
	if err != nil {
		return nil, "", m, fmt.Errorf("init: the root: %w", err)
	}
	if err := rt.Mkdir(envName, dirMode); err == nil {
		m.env = true
	} else if !errors.Is(err, fs.ErrExist) {
		_ = rt.Close()
		return nil, "", m, fmt.Errorf("init: %w", err)
	}
	if why := dirProblem(rt.Lstat, envName); why != "" {
		_ = rt.Close()
		return nil, "", m, refuse(cmd, "root-unsafe", envName+" "+why)
	}
	if !m.env {
		if err := rt.Chmod(envName, dirMode); err != nil {
			_ = rt.Close()
			return nil, "", m, fmt.Errorf("init: %w", err)
		}
	}
	return rt, filepath.Join(root, envName), m, nil
}

// made says which directories of the tree this run of init created, by
// a Mkdir that succeeded: the last element of the root, and the env dir.
type made struct {
	root, env bool
}

// undo removes the env dir and then the root, each only when this run
// made it, it is still a directory (Lstat, so a link is never followed
// or removed) and it is empty: os.Remove refuses a directory that is
// not, so nothing that appeared in it meanwhile goes with it. The env
// dir goes through rt. Its failures are not the command's: the error
// that led here is.
//
// A race remains between the Lstat and the Remove: a process of the
// user's could replace the directory with another empty directory or a
// link in between. Remove takes an empty directory, which holds nothing
// to lose, or the link itself, never its target.
func (m made) undo(rt *os.Root, envName, root string) {
	if m.env {
		if fi, err := rt.Lstat(envName); err == nil && fi.IsDir() {
			_ = rt.Remove(envName)
		}
	}
	if m.root {
		if fi, err := os.Lstat(root); err == nil && fi.IsDir() {
			_ = os.Remove(root)
		}
	}
}

// configDir reads projects/<cfg>.yaml at the config ref of the clone
// and returns the dir of the repo whose URL is config.url, the primary.
// A ref with no such file, or a spec that does not name the config repo
// as its primary, is config-project-missing (RJ-346). A ref other than the
// remote's default branch is config-ref-not-default.
func configDir(ctx context.Context, cmd *cobra.Command, g *gitsafe.Runner, h spec.HostSettings, clone string) (string, error) {
	missing := func(why string) error {
		return spec.ConfigProjectMissing.New(why, cli.Detail{Key: "config.url", Value: termsafe.Escape(h.Config.URL)})
	}
	// In v1 only the default branch of the remote is a config ref: the
	// clone checks that branch out, and the settings name the ref the
	// spec is read from (a decision of 2026-10-23).
	def, err := g.DefaultBranch(ctx, clone)
	if err != nil {
		return "", err
	}
	if def == "" {
		return "", refuse(cmd, "config-ref-not-default", "the config repo has no default branch: its HEAD names no branch")
	}
	if def != h.Config.Ref {
		return "", refuse(cmd, "config-ref-not-default", "--config-ref "+termsafe.Escape(h.Config.Ref)+" is not the default branch of the config repo ("+termsafe.Escape(def)+"); init takes the default branch only")
	}
	oid, ok, err := g.RefOID(ctx, clone, "refs/remotes/origin/"+h.Config.Ref)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", missing("the config repo has no branch " + termsafe.Escape(h.Config.Ref))
	}
	name := "projects/" + h.Config.Project + ".yaml"
	data, ok, err := g.CommitFile(ctx, clone, oid, name, configFileLimit)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", missing("branch " + termsafe.Escape(h.Config.Ref) + " holds no " + name)
	}
	p, err := spec.DecodeProject(data)
	if err != nil {
		return "", usage(cmd, name+" does not decode: "+termsafe.Escape(err.Error()))
	}
	if p.Name != h.Config.Project {
		return "", missing(name + " names the project " + termsafe.Escape(p.Name))
	}
	for _, r := range p.Repos {
		if r.Primary && spec.NormalizeURL(r.URL) == spec.NormalizeURL(h.Config.URL) {
			return r.Dir, nil
		}
	}
	return "", missing(name + " has no primary repo at config.url")
}

// cloneError sorts an error of the clone of the config repo, or of the
// reads of the clone when remote is true:
//
//   - an error of the table, a timeout, a cancellation passes through;
//   - a git that exited non-zero, and what gitsafe refuses in what the
//     remote sent (gitsafe.ErrRemoteContent, or any refusal when remote
//     is true), is config-clone-failed (RJ-356), with git's output or
//     the reason, which gitsafe escaped and cut to its first 4 KiB, in
//     the details;
//   - a refusal of the operator's input before git ran (the URL, the
//     helper: gitsafe.ErrRefused, gitsafe.ErrConfig) is usage (RJ-349),
//     an input init cannot take, never internal.
//
// Git itself prints no credential a helper gave it, and the URL holds
// none (CheckURL refuses user information); what a helper writes to its
// own standard error is git's standard error too, and is not filtered.
func cloneError(cmd *cobra.Command, err error, configURL string, remote bool) error {
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return err
	}
	refused := errors.Is(err, gitsafe.ErrRefused) || errors.Is(err, gitsafe.ErrConfig)
	_, exited := errors.AsType[*procgroup.ExitError](err)
	switch {
	case errors.Is(err, gitsafe.ErrRemoteContent) || (remote && refused) || (!remote && exited):
		return ConfigCloneFailed.New("git could not clone the config repo",
			cli.Detail{Key: "config.url", Value: termsafe.Escape(configURL)},
			cli.Detail{Key: "git", Value: termsafe.Escape(err.Error())})
	case refused:
		return usage(cmd, termsafe.Escape(err.Error()))
	}
	return err
}

// floorError returns the error git-floor (RJ-344) as the row of the
// table, for the *gitsafe.FloorError that gitsafe returns, which carries
// no cli.Row; any other error passes through. gitsafe declares the id,
// the slug, the exit code and the hint, and this takes them from it.
func floorError(err error) error {
	fe, ok := errors.AsType[*gitsafe.FloorError](err)
	if !ok {
		return err
	}
	row := cli.Row{ID: fe.ID(), Slug: fe.Slug(), Exit: fe.ExitCode(), Fix: fe.Fix()}
	return row.Wrap(fe, fe.Error(), cli.Detail{Key: "git", Value: fe.Details()})
}
