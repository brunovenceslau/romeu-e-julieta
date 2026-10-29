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
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/embedded"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The process's environment and the julieta this romeu embeds; a test
// replaces them.
var (
	getenv = os.Getenv
	// embeddedJulieta returns the julieta linux binary of romeu's
	// GOARCH that this build embeds (internal/embedded, T186), empty for
	// a source build, whose placeholder (02 2.1, REQ-0130) holds no
	// binary, so such a build stops at step 1 with julieta-not-embedded.
	embeddedJulieta = embedded.Julieta
)

// lockAndCheck is step 1 of 04 4.2 (REQ-0335): it loads and validates
// the host settings, builds the julieta binary of .romeu/bin once,
// takes the state lock, runs promotion recovery (01 1.6, REQ-0087) on
// every <name>-env/ under the root, and checks gate 1 (01 1.4), a read,
// which stops a stale toolchain with exit 3 before anything is
// promoted. Everything before the lock only reads, so a romeu that
// embeds no julieta (julieta-not-embedded, RJ-312) stops before any
// write. It fills settings, home, bin, store and lock.
//
// Recovery runs on every <name>-env/ since the targets are known only
// at step 3, and a promotion left by any command is finished before
// this one renders. The binding of the spec secrets, which REQ-0335
// places here, reads the specs and so runs at step 3 (checkSecrets).
func lockAndCheck(ctx context.Context, r *run) error {
	h, err := loadSettings(r.env)
	if err != nil {
		return err
	}
	home := getenv("HOME")
	if !filepath.IsAbs(home) {
		return usage("home", "HOME is not an absolute path")
	}
	bin, err := render.NewBin(runtime.GOARCH, embeddedJulieta())
	if errors.Is(err, render.ErrJulietaNotEmbedded) {
		return cli.JulietaNotEmbedded.New("this romeu was built without tools/release build, so it embeds no julieta to deliver")
	}
	if err != nil {
		return err
	}
	stateDir, err := state.DefaultDir(getenv)
	if err != nil {
		return usage("state", err.Error())
	}
	if r.store, err = state.Open(stateDir); err != nil {
		return err
	}
	if r.lock, err = r.store.Lock(); err != nil {
		return err
	}
	if err := recoverPromotions(r.env.Stderr, h.Root, bin); err != nil {
		return err
	}
	r.settings, r.home, r.bin = h, home, bin
	sbx := &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: &r.settings}
	return approve.CheckFunc(r.store, sbx, r.env.Version)(ctx)
}

// recoverPromotions runs promotion recovery on each <name>-env/
// directly under root, in name order, with bin as the julieta a
// finished promotion writes. A root that does not exist yet holds no
// promotion, and an entry that is not a directory, a link among them,
// is not one. A finished promotion is reported on stderr. An abandoned
// one stops the command at the first such dir with
// interrupted-promotion-abandoned (RJ-304, exit 4), which wraps
// render.ErrPromotionAbandoned and whose hint names the project. The
// dirs after it are not recovered by this run; the rerun recovers them,
// since recovery runs on every one each time.
func recoverPromotions(stderr io.Writer, root string, bin render.Bin) error {
	names, err := envDirs(root)
	if err != nil {
		return fmt.Errorf("promotion recovery: %w", err)
	}
	for _, name := range names {
		dir := name + "-env"
		live := render.Live{Root: root, EnvDir: filepath.Join(root, dir), Bin: bin}
		done, err := live.Recover()
		if errors.Is(err, render.ErrPromotionAbandoned) {
			a := InterruptedPromotionAbandoned.Wrap(err, "promotion recovery of "+termsafe.Escape(dir)+" abandoned an interrupted promotion")
			a.Fill = map[string]string{"<name>": termsafe.Escape(name)}
			return a
		}
		if err != nil {
			return fmt.Errorf("promotion recovery of %s: %w", termsafe.Escape(dir), err)
		}
		if done == render.RecoveryFinished {
			if _, err := fmt.Fprintf(stderr, "finished the interrupted promotion of %s\n", termsafe.Escape(name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// envDirs returns the name of each <name>-env/ directly under root, in
// name order: each entry whose name ends in -env, with a name before
// it, that is a directory and not a link. A root that does not exist
// yet holds none.
func envDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), "-env")
		if ok && name != "" && e.IsDir() {
			names = append(names, name)
		}
	}
	return names, nil
}

// loadSettings reads and validates the host settings (03 3.4). Settings
// that are absent, or that do not decode, are usage (RJ-349) with the
// slug settings-missing or settings-invalid; an error of the table the
// decoder raises (format-newer, secret-in-argv) passes through. It is
// the reader of romeu approve, whose copy is unexported.
func loadSettings(env *cli.Env) (spec.HostSettings, error) {
	path := env.SettingsPath
	if path == "" {
		var err error
		if path, err = spec.SettingsPath(getenv); err != nil {
			return spec.HostSettings{}, usage("settings-missing", err.Error())
		}
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return spec.HostSettings{}, usage("settings-missing", "no host settings at "+termsafe.Escape(path)+"; run romeu init first")
	}
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(err.Error()))
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, spec.MaxFileSize+1))
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(err.Error()))
	}
	h, err := spec.DecodeHostSettings(data)
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return spec.HostSettings{}, err
	}
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(path)+": "+termsafe.Escape(err.Error()))
	}
	return h, nil
}
