// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	changelog "github.com/brunovenceslau/romeu-e-julieta"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Question is the one confirmation of romeu upgrade, the last text of
// the preview: it replaces the question of gate 1 at the end of what
// approve --toolchain prints, since the answer installs the binary and
// acknowledges the toolchain at once (REQ-1348).
const Question = "install this romeu and acknowledge this toolchain? [y/N] "

// changelogFile returns the embedded CHANGELOG.md; a test replaces it.
var changelogFile = changelog.File

// The limits of what the running romeu keeps of the preview: its
// standard output, the text it shows the operator, and the head of its
// standard error, which the details of a failure hold.
const (
	previewLimit = 1 << 20
	stderrLimit  = 4096
)

// previewEnv returns the environment of the preview subprocess: the
// scrubbed environment of REQ-0296 (PATH=/usr/bin:/bin, HOME, LANG=C),
// and the two paths the running romeu read its host settings and its
// state from, so the new binary reads the same ones (T196,
// provisional): ROMEU_SETTINGS, the path of the settings file, and
// XDG_STATE_HOME when it is an absolute path.
func previewEnv(home, settings string, getenv func(string) string) []string {
	env := []string{"HOME=" + home, scrubbedPath, "LANG=C", hostpaths.SettingsEnv + "=" + settings}
	if x := getenv("XDG_STATE_HOME"); filepath.IsAbs(x) {
		env = append(env, "XDG_STATE_HOME="+x)
	}
	return env
}

// runPreview runs the new binary at path once as the preview
// subprocess (REQ-1348), romeu upgrade --preview <running>, with env,
// stdin closed and under the class Preview, and returns the text it
// printed, which ends with Question. A preview that fails, prints over
// previewLimit or prints no Question is verify-failed, with the head of
// its standard error in the details. The call site is admitted by the
// table of I16 (REQ-1321).
func runPreview(ctx context.Context, path, running, dir string, env []string) (string, error) {
	out := &capped{left: previewLimit + 1}
	errOut := &capped{left: stderrLimit}
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Preview,
		Argv:   []string{path, "upgrade", "--preview", running},
		Dir:    dir,
		Env:    env,
		Stdout: out,
		Stderr: errOut,
	})
	details := []cli.Detail{{Key: "preview stderr", Value: termsafe.Escape(errOut.b.String())}}
	switch _, timeout := errors.AsType[*procgroup.TimeoutError](err); {
	case timeout:
		return "", cli.SubprocessTimeout.Wrap(err, "the preview of the new romeu did not finish")
	case errors.Is(err, context.Canceled):
		return "", err
	case err != nil:
		return "", VerifyFailed.Wrap(err, "the preview of the new romeu failed", details...)
	case out.b.Len() > previewLimit:
		return "", VerifyFailed.New("the preview of the new romeu passed its cap", details...)
	case !strings.HasSuffix(out.b.String(), Question):
		return "", VerifyFailed.New("the preview of the new romeu asked no confirmation", details...)
	}
	return out.b.String(), nil
}

// preview is romeu upgrade --preview <running>, run by the running
// romeu as the new binary's one subprocess before the confirmation
// (REQ-1348): it prints the changelog sections of every version after
// running up to its own, or its own alone, then what approve
// --toolchain prints, with Question in place of gate 1's. It reads the
// host settings, sbx version and the state directory as they are, takes
// no lock and writes nothing.
func preview(ctx context.Context, env *cli.Env, running string) error {
	own := approve.Version(env.Version)
	if text := changelog.Since(changelogFile(), running, own); text != "" {
		if _, err := io.WriteString(env.Stdout, escape(text)+"\n\n"); err != nil {
			return err
		}
	}
	h, err := hostpaths.LoadSettings(env, getenv, usage)
	if err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage("home", "HOME is not an absolute path")
	}
	sbx := &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: &h}
	release, err := sbx.RequireFloor(ctx, sbxdrv.Opts{Step: "upgrade --preview"})
	if err != nil {
		return err
	}
	live, err := approve.Measure(release, env.Version)
	if err != nil {
		return err
	}
	var st *state.Store
	stateDir, err := hostpaths.StateDir(getenv)
	if err != nil {
		return usage("state", err.Error())
	}
	st, err = state.OpenRead(stateDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		st = nil
	case err != nil:
		return err
	default:
		defer func() { _ = st.Close() }()
	}
	var rec *state.Toolchain
	if st != nil {
		if rec, err = approve.Load(st); err != nil {
			return err
		}
	}
	text, err := approve.TextOf(st, rec, live)
	if err != nil {
		return err
	}
	text = strings.TrimSuffix(text, approve.Question) + Question
	_, err = io.WriteString(env.Stdout, text)
	return err
}

// escape escapes each line of s (termsafe, I29) and keeps the line
// feeds that end them.
func escape(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = termsafe.Escape(l)
	}
	return strings.Join(lines, "\n")
}
