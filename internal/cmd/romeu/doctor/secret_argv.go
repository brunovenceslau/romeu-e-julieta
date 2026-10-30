// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The check secret-argv (04 4.2, REQ-0382): a secret entry of the host
// settings whose argv[0] is missing or not absolute, or whose argv, run
// once with exactly HOME and PATH=/usr/bin:/bin (03 3.4), exits non-zero
// or prints nothing. The output is tested for emptiness only, and never
// printed or kept (I25): the messages name the entry, its argv[0] and the
// status, never the output. T049 runs it for the case "doctor never
// shows a secret argv's output"; secret-unknown-project is T134's.
var _ = register(row{slug: "secret-argv", run: checkSecretArgv})

func checkSecretArgv(ctx context.Context, h *host) Check {
	const slug = "secret-argv"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	if len(h.settings.Secrets) > 0 {
		if err := ownSettings(h.settingsPath); err != nil {
			return errorOf(slug, err)
		}
	}
	var found []string
	for _, name := range slices.Sorted(maps.Keys(h.settings.Secrets)) {
		argv := h.settings.Secrets[name].Argv
		if len(argv) == 0 || !filepath.IsAbs(argv[0]) {
			found = append(found, name+": argv[0] is missing or not an absolute path")
			continue
		}
		if _, err := os.Stat(argv[0]); err != nil {
			found = append(found, name+": "+argv[0]+" is not there")
			continue
		}
		printed, err := h.secret(ctx, argv, h.home)
		if _, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
			return errorOf(slug, cli.SubprocessTimeout.Wrap(err, "the secret argv of "+name+" did not finish"))
		}
		if ctx.Err() != nil {
			return errorOf(slug, ctx.Err())
		}
		switch {
		case err != nil:
			found = append(found, fmt.Sprintf("%s: %v", name, err))
		case !printed:
			found = append(found, name+": "+argv[0]+" printed nothing")
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, fmt.Sprintf("%d secret argv(s) ran and printed something", len(h.settings.Secrets)))
}

// ownSettings returns why the host settings at p are not a file this
// user alone wrote, read with Lstat: not a regular file, a mode other
// than 0600, or an owner other than the effective user. A secret argv is
// a program to run, so none runs from settings another user or a link
// could have put there; the error names settings-mode, the check that
// reports the mode.
func ownSettings(p string) error {
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode() != state.FileMode || !ok || int(st.Uid) != os.Geteuid() {
		return usage("settings-mode", fmt.Sprintf("the host settings at %s are not a regular file of mode 0600 that this user owns (%v), so no secret argv runs", p, info.Mode()))
	}
	return nil
}
