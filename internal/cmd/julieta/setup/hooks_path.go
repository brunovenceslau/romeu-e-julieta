// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// systemHooksPath reads the system core.hooksPath. Tests replace it: a
// test cannot plant a system config for the git of gitsafe, which
// passes no GIT_CONFIG_SYSTEM.
var systemHooksPath = func(ctx context.Context, git *gitsafe.Runner) (string, bool, error) {
	return git.SystemHooksPath(ctx)
}

// installDispatcher installs the hook dispatcher (04 4.3, REQ-0397;
// 02 2.5, REQ-0154) in three steps. It writes the scripts below
// $HOME/.local/share/julieta/hooks, each handing its hook to julieta
// hooks run through the PATH link; a script that is already right is
// left alone. It records the system core.hooksPath in julieta state
// (hooks.RecordSystem), the hooks directory the sandbox kit sets, which
// the global key hides from git and the dispatcher runs as its system
// stage (REQ-0408). And it points the global core.hooksPath at the
// scripts, which it leaves alone when the key already names them.
//
// Each failure leaves the sandbox unusable, since git then does not run
// the dispatcher, or runs it without the system hooks: a script that
// cannot be written; a system value that cannot be read or recorded, or
// that is not an absolute path, read as git reads a path (~ expanded),
// each of which also leaves the global key unset; and a global key that
// names another directory once cleaned, which setup reports with its
// fix and does not overwrite.
func installDispatcher(ctx context.Context, r *run) {
	if _, err := hooks.Install(r.home, filepath.Join(r.home, linkPath)); err != nil {
		r.fail("the hook dispatcher: %v", err)
		return
	}
	system, _, err := systemHooksPath(ctx, r.git)
	if err != nil {
		r.fail("the system core.hooksPath: %v", err)
		return
	}
	if system != "" && !filepath.IsAbs(system) {
		r.fail("the system core.hooksPath %q is not an absolute path, which the dispatcher cannot run: setup leaves the global key unset", system)
		return
	}
	if err := hooks.RecordSystem(StateDir(r.home), system); err != nil {
		r.fail("the record of the system core.hooksPath: %v", err)
		return
	}
	dir := hooks.DispatcherDir(r.home)
	found, err := r.git.SetGlobalHooksPath(ctx, dir)
	switch {
	case err != nil:
		r.fail("the global core.hooksPath: %v", err)
	case found != "" && filepath.Clean(found) != dir:
		r.fail("the global core.hooksPath is %q, not the dispatcher %s: setup leaves it as it is, and git does not run the dispatcher; "+
			"run git config --global --unset core.hooksPath, then julieta setup", found, dir)
	}
}
