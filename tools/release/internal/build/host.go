// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// HostOptions are the inputs of a host build.
type HostOptions struct {
	Root  string // the repository root
	Out   string // the path the romeu binary is written to; it must not exist
	GoBin string // the pinned go, as ResolveGo resolves it
}

// Host builds one romeu for the platform this program runs on, which
// embeds the julieta linux binary of the same GOARCH, the binary step 1
// of sync and run deliver (REQ-0129, REQ-0131), and writes it to Out.
// It is the build the e2e of a command that needs julieta runs (10
// 10.2, the e2e row): the julieta build, the overlay and the check that
// romeu carries the julieta bytes are the release build's, under its
// build contract; it writes no archive, reads no VERSION, consults no
// gate and builds a tree with changes, stamping its commit dirty. The
// version stamp is left unset, so gate 1 sees a dev build (01 1.4).
// Unlike a dry run, a romeu that carries no julieta is an error, since
// embedding it is all this build is for.
func Host(ctx context.Context, o HostOptions) error {
	return host(ctx, o, runtime.GOOS, runtime.GOARCH, embedKey)
}

// host is Host for the platform goos/goarch, with the function that
// names the overlay key, which a test replaces to see a key that
// matches no file refused.
func host(ctx context.Context, o HostOptions, goos, goarch string, key func(root string) string) error {
	if !slices.Contains(Arches, goarch) {
		return fmt.Errorf("no julieta is built for %s: the release builds %s", goarch, strings.Join(Arches, " and "))
	}
	if _, err := os.Lstat(o.Out); err == nil {
		return fmt.Errorf("--out %s exists", o.Out)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("--out %s: %w", o.Out, err)
	}
	// The overlay key must be the real path of the file, as in build.
	root, err := filepath.EvalSymlinks(o.Root)
	if err != nil {
		return err
	}
	commit, when, err := commitOf(ctx, root)
	if err != nil {
		return err
	}
	dirty, err := dirtyTree(ctx, root)
	if err != nil {
		return err
	}
	if dirty {
		commit += "-dirty"
	}
	stage, err := os.MkdirTemp("", "romeu-host-")
	if err != nil {
		return err
	}
	if stage, err = filepath.EvalSymlinks(stage); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	b := builder{o: Options{Root: root, GoBin: o.GoBin}, stage: stage, key: key(root), ldflags: strings.Join([]string{
		"-buildid=",
		"-X main.commit=" + commit,
		"-X main.date=" + when.Format(time.RFC3339),
	}, " ")}
	julieta := filepath.Join(stage, "julieta_linux_"+goarch)
	if err := b.goBuild(ctx, "julieta", "linux", goarch, julieta, ""); err != nil {
		return err
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {b.key: julieta}})
	if err != nil {
		return err
	}
	overlayFile := filepath.Join(stage, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0o600); err != nil {
		return err
	}
	romeu := filepath.Join(stage, "romeu_"+goos+"_"+goarch)
	if err := b.goBuild(ctx, "romeu", goos, goarch, romeu, overlayFile); err != nil {
		return err
	}
	binary, err := os.ReadFile(romeu)
	if err != nil {
		return err
	}
	embedded, err := carries(binary, julieta)
	if err != nil {
		return err
	}
	if !embedded {
		return fmt.Errorf("the romeu built for %s/%s does not carry its julieta: no package of romeu consumes %s, or the overlay did not replace it", goos, goarch, EmbedPath)
	}
	return writeNew(o.Out, binary)
}

// writeNew writes data to the new file path, mode 0755, and refuses a
// path that exists.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
