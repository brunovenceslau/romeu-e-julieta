// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// noRepo is the directory the verbs that read or write the sandbox's
// global and system config run in: they act on no repository, as Clone
// resolves its URL.
const noRepo = "/"

// SetGlobalHooksPath points the sandbox's global core.hooksPath at path,
// an absolute path, the directory of julieta's hook dispatcher (04 4.3,
// REQ-0397), with git config --global, in no repository. It first reads
// the global key and returns the value it found, "" when the key was
// not set. It writes only when the key is not set: it leaves the key
// alone when it already holds path, so a second run writes nothing
// (REQ-0396), and when it holds another value, which the caller
// reports. The read follows the includes of the global config, as git
// does when it runs a hook, so a value set in an included file is seen
// and kept, and reads the value as a path (--type=path), so a leading ~
// is expanded to the HOME of the mode, as git expands it. Every other key of the global config stays as it was, git
// config itself rewriting only the line of the key.
//
// The global config is the sandbox's own (05 5.1, REQ-0463), and the
// host reads none, so host mode refuses it (I2). julieta's own git
// calls keep -c core.hooksPath=/dev/null, so the key never makes them
// run the dispatcher. path must be valid UTF-8 and hold no character
// unsafeRune refuses, so that it reads back through the checks of
// HooksPath.
func (r *Runner) SetGlobalHooksPath(ctx context.Context, path string) (string, error) {
	if r.mode != Sandbox {
		return "", fmt.Errorf("%w: the global core.hooksPath in %s mode", ErrRefused, r.mode)
	}
	if err := checkHooksPathValue(path); err != nil {
		return "", err
	}
	found, set, err := r.readHooksPath(ctx, noRepo, "--global", "--includes", "--type=path")
	if err != nil {
		return "", err
	}
	if set {
		return found, nil
	}
	return "", r.run(ctx, noRepo, nil, "config", "--global", "core.hooksPath", path)
}

// SystemHooksPath returns the core.hooksPath of the sandbox's system
// config, and whether it sets one, with git config --system, in no
// repository: the hooks directory the sandbox kit sets for itself,
// which a global value hides from git, so julieta setup records it for
// the dispatcher's system stage (04 4.3, REQ-0408). The read follows
// the includes of the system config, as git does when it runs a hook,
// and has the checks of HooksPath. The value is read as a path
// (--type=path): a leading ~ is expanded to the HOME of the mode, and a
// relative value is returned as it is, for the caller to report. Host
// mode reads no system config (05 5.1), so it refuses the read.
func (r *Runner) SystemHooksPath(ctx context.Context) (string, bool, error) {
	if r.mode != Sandbox {
		return "", false, fmt.Errorf("%w: the system core.hooksPath in %s mode", ErrRefused, r.mode)
	}
	return r.readHooksPath(ctx, noRepo, "--system", "--includes", "--type=path")
}

// checkHooksPathValue returns ErrRefused for a hooks path gitsafe does
// not write: one that is not absolute, not valid UTF-8, or that holds a
// character unsafeRune refuses (a NUL and a newline among them).
func checkHooksPathValue(path string) error {
	switch {
	case !filepath.IsAbs(path):
		return fmt.Errorf("%w: hooks path %q is not absolute", ErrRefused, path)
	case !utf8.ValidString(path):
		return fmt.Errorf("%w: hooks path %q is not valid UTF-8", ErrRefused, path)
	}
	if i := strings.IndexFunc(path, unsafeRune); i >= 0 {
		c, _ := utf8.DecodeRuneInString(path[i:])
		return fmt.Errorf("%w: hooks path %q holds the character %U", ErrRefused, path, c)
	}
	return nil
}
