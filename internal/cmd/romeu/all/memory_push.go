// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package all

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memgit"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The last step of the commands 08 8.1 names (REQ-1304, REQ-1305): the
// memory commit and push of internal/memgit, wired once, here, around
// each command by its name (cli.Wrap), so no command's own package
// changes. The step runs only after the command's RunE returned no
// error, so a command that failed before its work was done pushes
// nothing; it takes romeu.lock again for the push; and its warnings,
// printed after the command's own output, close it.

// memoryRole is what the step does after one command.
type memoryRole struct {
	// refuse: a spec without memory.remote is the error
	// memory-remote-missing (sync).
	refuse bool
	// notices: memory-local and memory-unsigned are warned every time
	// (sync, run, doctor).
	notices bool
	// push: the commit and the push (the seven commands of 08 8.1).
	push bool
	// targets returns the projects the command acted on.
	targets func(h *memoryHost, cmd *cobra.Command, args []string) ([]memoryTarget, error)
}

// memoryRoles are the commands of 08 8.1 and doctor, by command line.
var memoryRoles = map[string]memoryRole{
	"sync":     {refuse: true, notices: true, push: true, targets: syncTargets},
	"run":      {notices: true, push: true, targets: namedTargets},
	"doctor":   {notices: true, targets: liveTargets},
	"pull":     {push: true, targets: pullTargets},
	"salvage":  {push: true, targets: namedTargets},
	"rm":       {push: true, targets: namedTargets},
	"recreate": {push: true, targets: namedTargets},
	"retire":   {push: true, targets: retiredTargets},
}

// The steps, one per command line. A command not linked yet takes its
// step when its import_<command>.go links it in.
var (
	_ = cli.Wrap("romeu", "sync", memoryStep(memoryRoles["sync"]))
	_ = cli.Wrap("romeu", "run", memoryStep(memoryRoles["run"]))
	_ = cli.Wrap("romeu", "doctor", memoryStep(memoryRoles["doctor"]))
	_ = cli.Wrap("romeu", "pull", memoryStep(memoryRoles["pull"]))
	_ = cli.Wrap("romeu", "salvage", memoryStep(memoryRoles["salvage"]))
	_ = cli.Wrap("romeu", "rm", memoryStep(memoryRoles["rm"]))
	_ = cli.Wrap("romeu", "recreate", memoryStep(memoryRoles["recreate"]))
	_ = cli.Wrap("romeu", "retire", memoryStep(memoryRoles["retire"]))
)

// getenv reads the process environment; a test replaces it.
var getenv = os.Getenv

// memoryStep returns the step around a command of role.
func memoryStep(role memoryRole) cli.Around {
	return func(env *cli.Env, cmd *cobra.Command, args []string, run func() error) error {
		if err := run(); err != nil {
			return err
		}
		return afterWork(cmd.Context(), env, role, cmd, args)
	}
}

// afterWork is the step once the command's work is done: for each
// project the command acted on, the refusal of sync, the notices and
// the commit and push, each project's warnings printed before the next.
// An error of one project does not stop the others; the first error
// of the table among them is returned.
func afterWork(ctx context.Context, env *cli.Env, role memoryRole, cmd *cobra.Command, args []string) error {
	// A role that only warns (doctor) leaves what it cannot read to the
	// command's own checks, and fails nothing.
	quiet := !role.push && !role.refuse
	h, err := loadMemoryHost(env)
	if err != nil {
		if quiet {
			return nil
		}
		return err
	}
	targets, err := role.targets(h, cmd, args)
	if quiet && err != nil {
		return nil
	}
	if err != nil || len(targets) == 0 {
		return err
	}
	if role.push {
		release, err := lockState()
		if err != nil {
			return err
		}
		defer release()
	}
	var errs []error
	for _, t := range targets {
		p, sp, err := h.project(ctx, t)
		if err != nil {
			if !quiet {
				errs = append(errs, err)
			}
			continue
		}
		if role.refuse {
			if err := spec.CheckMemoryRemote(sp); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		if sp.Memory == nil || sp.Memory.Remote == "" {
			continue // sync refuses it; the other commands leave it
		}
		var warnings []memgit.Warning
		if role.notices {
			warnings = append(warnings, memgit.Notices(p)...)
		}
		if role.push {
			w, err := memgit.Sync(ctx, h.git, p)
			warnings = append(warnings, w...)
			if err != nil {
				errs = append(errs, err)
			}
		}
		for _, w := range warnings {
			warnMemory(env.Stderr, w)
		}
	}
	for _, err := range errs {
		if _, ok := errors.AsType[*cli.Error](err); ok {
			return err
		}
	}
	return errors.Join(errs...)
}

// warnMemory prints w by its slug, each a literal of 04 4.1, Warnings.
func warnMemory(w *cli.Writer, m memgit.Warning) {
	switch m.Slug {
	case memgit.SlugLocal:
		cli.Warn(w, "memory-local", m.Message)
	case memgit.SlugUnsigned:
		cli.Warn(w, "memory-unsigned", m.Message)
	case memgit.SlugDiverged:
		cli.Warn(w, "memory-diverged", m.Message)
	case memgit.SlugPushFailed:
		cli.Warn(w, "memory-push-failed", m.Message)
	}
}

// lockState takes romeu.lock for the push and returns the function that
// releases it. The command released the lock it took when its RunE
// returned, so another romeu that took it in between makes this the
// error locked (RJ-101), and nothing is pushed.
func lockState() (func(), error) {
	dir, err := state.DefaultDir(getenv)
	if err != nil {
		return nil, err
	}
	st, err := state.Open(dir)
	if err != nil {
		return nil, err
	}
	lk, err := st.Lock()
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	return func() {
		lk.Unlock()
		_ = st.Close()
	}, nil
}

// memoryHost is what the step reads of the host: the settings and a
// host-mode runner of git.
type memoryHost struct {
	settings spec.HostSettings
	git      *gitsafe.Runner
}

// maxSettings bounds the host settings file the step reads.
const maxSettings = 1 << 20

// loadMemoryHost reads the host settings at env.SettingsPath, or at the
// path 03 3.4 names, and makes the runner of git they configure.
func loadMemoryHost(env *cli.Env) (*memoryHost, error) {
	settings := env.SettingsPath
	if settings == "" {
		var err error
		if settings, err = spec.SettingsPath(getenv); err != nil {
			return nil, err
		}
	}
	data, err := readCapped(settings, maxSettings)
	if err != nil {
		return nil, fmt.Errorf("the host settings: %w", err)
	}
	h, err := spec.DecodeHostSettings(data)
	if err != nil {
		return nil, err
	}
	home := getenv("HOME")
	c := gitsafe.Config{Mode: gitsafe.Host, Git: h.Tools.Git, Home: home}
	if h.Tools.GitCredentialHelper != "" {
		u, err := url.Parse(h.Config.URL)
		if err != nil {
			return nil, fmt.Errorf("config.url: %w", err)
		}
		c.CredentialHelper, c.CredentialHost = h.Tools.GitCredentialHelper, u.Host
	}
	for _, gh := range h.GitHosts {
		c.GitHosts = append(c.GitHosts, gitsafe.GitHost{Host: gh.Host, CAFile: gh.CAFile})
	}
	g, err := gitsafe.New(c)
	if err != nil {
		return nil, err
	}
	return &memoryHost{settings: h, git: g}, nil
}

// readCapped reads the regular file at p, of at most limit bytes.
func readCapped(p string, limit int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	switch {
	case err != nil:
		return nil, err
	case !fi.Mode().IsRegular():
		return nil, fmt.Errorf("%s is not a regular file", p)
	case fi.Size() > limit:
		return nil, fmt.Errorf("%s is more than %d bytes", p, limit)
	}
	return io.ReadAll(io.LimitReader(f, limit))
}

// memoryTarget is a project a command acted on: its name and its tree.
type memoryTarget struct {
	name, envDir string
}

// envDirOf returns the tree of the project name, or an error for a name
// that is no single path element.
func (h *memoryHost) envDirOf(name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("%q is not a project name", name)
	}
	return filepath.Join(h.settings.Root, name+"-env"), nil
}

// namedTargets are the projects named by the arguments: run, salvage,
// rm and recreate take one.
func namedTargets(h *memoryHost, _ *cobra.Command, args []string) ([]memoryTarget, error) {
	var out []memoryTarget
	for _, name := range args {
		dir, err := h.envDirOf(name)
		if err != nil {
			return nil, err
		}
		out = append(out, memoryTarget{name, dir})
	}
	return out, nil
}

// syncTargets are the projects named by the arguments, or every live
// project with --all or with none named.
func syncTargets(h *memoryHost, cmd *cobra.Command, args []string) ([]memoryTarget, error) {
	if all, _ := cmd.Flags().GetBool("all"); all || len(args) == 0 {
		return liveTargets(h, cmd, nil)
	}
	return namedTargets(h, cmd, args)
}

// liveTargets are the projects of the root with a live render: each
// <name>-env/ that holds .romeu/render.json.
func liveTargets(h *memoryHost, _ *cobra.Command, _ []string) ([]memoryTarget, error) {
	entries, err := os.ReadDir(h.settings.Root)
	if err != nil {
		return nil, err
	}
	var out []memoryTarget
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), "-env")
		if !ok || !e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(h.settings.Root, e.Name())
		if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(render.LiveRecord))); err == nil {
			out = append(out, memoryTarget{name, dir})
		}
	}
	return out, nil
}

// pullTargets is the project whose tree holds the directory of -C, or
// the working directory: romeu pull names no project (04 4.2).
func pullTargets(h *memoryHost, cmd *cobra.Command, _ []string) ([]memoryTarget, error) {
	dir, _ := cmd.Flags().GetString("C")
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return nil, err
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(h.settings.Root)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("%s is not below the root %s", dir, h.settings.Root)
	}
	first, _, _ := strings.Cut(rel, string(filepath.Separator))
	name, ok := strings.CutSuffix(first, "-env")
	if !ok {
		return nil, fmt.Errorf("%s is in no project's tree", dir)
	}
	return namedTargets(h, cmd, []string{name})
}

// retiredTargets is the project retire moved to the attic: the newest
// .attic/<name>/<UTC-ts>/ (02 2.3), whose memory and git dir moved with
// it.
func retiredTargets(h *memoryHost, cmd *cobra.Command, args []string) ([]memoryTarget, error) {
	named, err := namedTargets(h, cmd, args)
	if err != nil || len(named) != 1 {
		return named, err
	}
	attic := filepath.Join(h.settings.Root, ".attic", named[0].name)
	entries, err := os.ReadDir(attic)
	if err != nil {
		return nil, err
	}
	var stamps []string
	for _, e := range entries {
		if e.IsDir() {
			stamps = append(stamps, e.Name())
		}
	}
	if len(stamps) == 0 {
		return nil, fmt.Errorf("the attic %s holds no retired tree", attic)
	}
	return []memoryTarget{{named[0].name, filepath.Join(attic, slices.Max(stamps))}}, nil
}

// maxSpec bounds a project spec read from the config repo.
const maxSpec = 1 << 20

// signingKit is the product kit whose signingKey argument is the
// project's signing key (06 6.4, I34).
const signingKit = "git-ssh-sign"

// project reads what memgit needs of t: the spec at the commit of its
// live render, read from the config repo's clone, and the signing
// socket of host settings.
func (h *memoryHost) project(ctx context.Context, t memoryTarget) (memgit.Project, *spec.Project, error) {
	rec, err := readRecord(t.envDir)
	if err != nil {
		return memgit.Project{}, nil, err
	}
	clone, err := h.configClone()
	if err != nil {
		return memgit.Project{}, nil, err
	}
	data, ok, err := h.git.CommitFile(ctx, clone, rec.SpecSource.Commit, path.Join("projects", t.name+".yaml"), maxSpec)
	switch {
	case err != nil:
		return memgit.Project{}, nil, err
	case !ok:
		return memgit.Project{}, nil, fmt.Errorf("the config repo holds no projects/%s.yaml at %s", t.name, rec.SpecSource.Commit)
	}
	sp, err := spec.DecodeProject(data)
	if err != nil {
		return memgit.Project{}, nil, err
	}
	p := memgit.Project{Name: sp.Name, EnvDir: t.envDir}
	if sp.Memory != nil {
		p.Remote = sp.Memory.Remote
	}
	for _, k := range sp.Kits {
		if k.Product == signingKit {
			p.SigningKey = k.Args["signingKey"]
		}
	}
	if h.settings.Signing != nil {
		p.AgentSocket = h.settings.Signing.AgentSocket
	}
	return p, &sp, nil
}

// readRecord reads the live render.json of the tree envDir.
func readRecord(envDir string) (render.Record, error) {
	data, err := readCapped(filepath.Join(envDir, filepath.FromSlash(render.LiveRecord)), render.MaxRecordSize)
	if err != nil {
		return render.Record{}, err
	}
	return render.DecodeRecord(data)
}

// configClone returns the config repo's clone, <root>/<cfg>-env/<dir>:
// dir is the config repo's entry in the config project's live render,
// or, before its first sync, the base name of config.url, the one romeu
// init gives it when the config spec names no dir.
func (h *memoryHost) configClone() (string, error) {
	env := filepath.Join(h.settings.Root, h.settings.Config.Project+"-env")
	want := spec.NormalizeURL(h.settings.Config.URL)
	switch rec, err := readRecord(env); {
	case err == nil:
		for _, r := range rec.Repos {
			if spec.NormalizeURL(r.URL) == want {
				return filepath.Join(env, r.Dir), nil
			}
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	u, err := url.Parse(h.settings.Config.URL)
	if err != nil {
		return "", fmt.Errorf("config.url: %w", err)
	}
	return filepath.Join(env, strings.TrimSuffix(path.Base(u.Path), ".git")), nil
}

// RunMemoryStep runs the step of the command line line around work, a
// probe in place of the command's own RunE, as Tree runs it around the
// command, and returns the exit status cli.Report gives the outcome.
// The probe takes the flags the targets read (--all, -C) and args as
// its arguments. It is for e2e/** and the tests (REQ-0971): the
// commands of 08 8.1 are not all linked in yet, and a test-only command
// in the binary would be one more command.
func RunMemoryStep(ctx context.Context, settingsPath string, stdout, stderr io.Writer, line string, args []string, work func() error) int {
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(stdout), Stderr: cli.NewWriter(stderr), SettingsPath: settingsPath}
	role, ok := memoryRoles[line]
	if !ok {
		return cli.Report(env, cli.Internal.New("no memory step around "+line))
	}
	cmd := &cobra.Command{Use: line, SilenceErrors: true, SilenceUsage: true}
	cmd.Flags().Bool("all", false, "every live project")
	cmd.Flags().StringP("C", "C", "", "a directory of the project")
	if err := cmd.Flags().Parse(args); err != nil {
		return cli.Report(env, cli.Usage.New(err.Error()))
	}
	cmd.SetContext(ctx)
	return cli.Report(env, memoryStep(role)(env, cmd, cmd.Flags().Args(), work))
}
