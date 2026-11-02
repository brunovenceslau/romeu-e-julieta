// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package upgrade is the command romeu upgrade (04 4.2, REQ-1323; ADR
// 0045): it installs a published release of romeu on this host after
// the operator reads what changed and confirms, and that confirmation
// acknowledges the toolchain as gate 1 (01 1.4, REQ-0061).
//
// The running romeu picks the release (release.go), downloads the darwin
// archive of its GOARCH and checksums.txt, checks the sha256 in Go and
// the attestation through the gh of tools.gh (verify.go), extracts the
// new binary under a temporary name beside itself (binary.go) and runs
// it once as the preview subprocess, romeu upgrade --preview <running
// version> (preview.go), whose text ends with the one question. Any
// answer but y deletes the temporary file; y renames it over the
// running binary and replaces the process with it, as romeu upgrade
// --finish <running version> (finish.go), which writes toolchain.json
// and runs the steps of romeu sync --all. The two argv forms are kept
// by every later release, since the old binary chooses them; neither
// flag is listed by the help.
//
// all/import_upgrade.go links the command into romeu.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/doctor"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The process's environment, standard input and the file descriptor of
// its standard error, which the CLI writer env.Stderr writes to; the
// source of the releases; and the swap step. A test replaces them.
var (
	getenv   = os.Getenv
	stdin    = func() io.Reader { return os.Stdin }
	stderrFD = func() int { return syscall.Stderr }
	sources  = newSource
	replace  = swap
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "upgrade"
	c.New = func(env *cli.Env) *cobra.Command {
		var f flags
		cmd := &cobra.Command{
			Use:     "upgrade [--version <tag>] [--check]",
			Short:   "install a published release of romeu after showing what changed (gate 1)",
			Example: "romeu upgrade\nromeu upgrade --check\nromeu upgrade --version v1.0.0",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return f.run(c.Context(), env)
			},
		}
		cmd.Flags().StringVar(&f.version, "version", "", "install the release of this tag instead of the newest")
		cmd.Flags().BoolVar(&f.check, "check", false, "print the running and the picked versions and change nothing")
		cmd.Flags().StringVar(&f.preview, "preview", "", "the preview the running romeu runs (internal)")
		cmd.Flags().StringVar(&f.finish, "finish", "", "the step the swap runs in place of the process (internal)")
		_ = cmd.Flags().MarkHidden("preview")
		_ = cmd.Flags().MarkHidden("finish")
		return cmd
	}
	return c
}())

// flags are the command line of one run.
type flags struct {
	version, preview, finish string
	check                    bool
}

// runningForm is the form of a dev build's version, which the preview
// and the finish take as the running version beside a release tag:
// v0.0.0-dev+<commit> (REQ-0065), the commit a hex id with an optional
// -dirty, or unknown (approve.Version).
var runningForm = regexp.MustCompile(`^v0\.0\.0-dev\+(?:[0-9a-f]{7,64}(?:-dirty)?|unknown)$`)

// runningVersion reports whether v is a version a romeu runs as: a
// release tag or a dev build's version.
func runningVersion(v string) bool { return tagForm.MatchString(v) || runningForm.MatchString(v) }

// run dispatches the command line: the two internal forms, then --check
// and the install.
func (f flags) run(ctx context.Context, env *cli.Env) error {
	internal := f.preview != "" || f.finish != ""
	switch {
	case internal && (f.version != "" || f.check || (f.preview != "" && f.finish != "")):
		return usage("flags", "--preview and --finish are internal and take no other flag")
	case f.preview != "" && !runningVersion(f.preview):
		return usage("flags", "--preview "+termsafe.Escape(f.preview)+" is not a version")
	case f.finish != "" && !runningVersion(f.finish):
		return usage("flags", "--finish "+termsafe.Escape(f.finish)+" is not a version")
	case f.version != "" && !tagForm.MatchString(f.version):
		return usage("flags", "--version "+termsafe.Escape(f.version)+" is not a release tag (v<major>.<minor>.<patch>, with an optional suffix)")
	case f.preview != "":
		return preview(ctx, env, f.preview)
	case f.finish != "":
		return finish(ctx, env, f.finish)
	case f.check:
		return check(ctx, env, f.version)
	}
	return install(ctx, env, f.version)
}

// usage returns the error usage (RJ-349) for a precondition the command
// stops at, named by the slug of the rule it breaks, with exit 2 and the
// hint of the help of the command.
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " upgrade"}
	return e
}

// check is romeu upgrade --check (REQ-1325): it reads the release to
// pick and prints the running and the picked versions, and current when
// they are the same; it needs no terminal and no tools.gh, reads no
// host settings and no state, and writes nothing.
func check(ctx context.Context, env *cli.Env, tag string) error {
	running := approve.Version(env.Version)
	src, err := sources(running)
	if err != nil {
		return err
	}
	rel, err := src.pick(ctx, tag)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(env.Stdout, "running  %s\npicked   %s\n", termsafe.Escape(running), rel.Tag); err != nil {
		return err
	}
	if rel.Tag == running {
		_, err = io.WriteString(env.Stdout, "current\n")
	}
	return err
}

// install is romeu upgrade: the preconditions, the pick, the download,
// the checks, the preview, the confirmation and the swap.
func install(ctx context.Context, env *cli.Env, tag string) error {
	in := stdin()
	if f, ok := in.(*os.File); !ok || !term.IsTerminal(int(f.Fd())) || !term.IsTerminal(stderrFD()) {
		return noTerminal()
	}
	h, err := hostpaths.LoadSettings(env, getenv, usage)
	if err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage("home", "HOME is not an absolute path")
	}
	gh := h.Tools.Gh
	if _, err := doctor.GhAtFloor(ctx, gh, home); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return GhUnusable.Wrap(err, "romeu upgrade checks a release through the gh of tools.gh, at or above "+doctor.GhFloor)
	}
	exe, err := runningBinary()
	if err != nil {
		return err
	}
	settings, err := hostpaths.SettingsPath(env, getenv)
	if err != nil {
		return usage("settings", err.Error())
	}
	stateDir, err := hostpaths.StateDir(getenv)
	if err != nil {
		return usage("state", err.Error())
	}
	// The lock is held for the whole command and released before the
	// process is replaced, as run releases it before its final exec (04
	// 4.1, Locking): the new binary takes it again to write
	// toolchain.json, then sync does.
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

	running := approve.Version(env.Version)
	src, err := sources(running)
	if err != nil {
		return err
	}
	rel, err := src.pick(ctx, tag)
	if err != nil {
		return err
	}
	if rel.Tag == running {
		_, err := fmt.Fprintf(env.Stdout, "current  %s\n", rel.Tag)
		return err
	}
	archiveAsset, sumsAsset, err := rel.assets()
	if err != nil {
		return err
	}
	sums, err := src.download(ctx, sumsAsset)
	if err != nil {
		return err
	}
	archive, err := src.download(ctx, archiveAsset)
	if err != nil {
		return err
	}
	if err := checkSum(archive, archiveAsset.Name, sums); err != nil {
		return err
	}
	if err := attest(ctx, gh, home, rel.Tag, archiveAsset.Name, archive); err != nil {
		return err
	}

	temp, err := extract(archive, filepath.Dir(exe))
	if err != nil {
		return err
	}
	ok, err := confirm(ctx, in, env, temp, running, home, previewEnv(home, settings, getenv))
	if err != nil || !ok {
		removeTemp(temp)
		if err == nil {
			err = Declined.New("the upgrade to " + rel.Tag + " was declined; the running romeu and toolchain.json are unchanged")
		}
		return err
	}
	if _, err := fmt.Fprintf(env.Stdout, "installing  romeu %s at %s\n", rel.Tag, termsafe.Escape(exe)); err != nil {
		removeTemp(temp)
		return err
	}
	lk.Unlock()
	err = replace(temp, exe, []string{exe, "upgrade", "--finish", running})
	removeTemp(temp)
	return err
}

// confirm runs the preview of the new binary at temp and asks its
// question on the terminal; it reports whether the answer was y.
func confirm(ctx context.Context, in io.Reader, env *cli.Env, temp, running, home string, penv []string) (bool, error) {
	text, err := runPreview(ctx, temp, running, home, penv)
	if err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	ok, err := gate.Confirm(in, stderrFD(), env.Stderr, text)
	if errors.Is(err, gate.ErrNoTerminal) {
		return false, noTerminal()
	}
	return ok, err
}

// noTerminal is the refusal without a terminal on stdin or on stderr
// (exit 2): the confirmation is gate 1, which is TTY-only (01 1.4,
// REQ-1274), and the operator must see its text where the answer is
// typed.
func noTerminal() error {
	return usage("no-tty", "romeu upgrade needs a terminal on its standard input and its standard error; run it in one, or run romeu upgrade --check")
}
