// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package build is "tools/release build" (10 10.2, Release and
// bootstrap, step 1; 02 2.1, Embedding). It cross-compiles the julieta
// linux binaries, embeds them into romeu, builds romeu for darwin and
// writes one archive per romeu platform, each with COPYING and the
// third-party notices of the module graphs of its binaries, and
// checksums.txt.
//
// Unless it is a dry run the build first asks the FuzzGate. A dry run
// reads no network: the go commands run with GOPROXY=off and the gate
// is not consulted.
//
// The build contract (REQ-0905): -trimpath, CGO_ENABLED=0, an empty
// build id, no VCS stamp, the commit time as the build date, archives
// with fixed times, owners and order, and an environment built from
// nothing (goEnv).
package build

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// EmbedPath is the file, from the root of the repository, that the
// romeu build finds the julieta linux binary of its own GOARCH at. The
// build does not write it into the tree: it maps it to the binary with a
// go build overlay, so a source build without this tool finds no file
// there and embeds a placeholder (REQ-0130).
const EmbedPath = "internal/embedded/julieta"

// Arches are the architectures of the romeu archives, and of the julieta
// binary each one embeds.
var Arches = []string{"amd64", "arm64"}

// Options are the inputs of a build.
type Options struct {
	Root    string    // the repository root
	Version string    // the tag, v<major>.<minor>.<patch> with an optional suffix
	Out     string    // where the archives go; empty means a new temporary directory
	DryRun  bool      // build without the fuzz gate and without network
	GoBin   string    // the pinned go, as "mise which go" resolves it
	Gate    *FuzzGate // required unless DryRun
}

// Result is what a build wrote.
type Result struct {
	Archives  []string // paths, in the order of Arches
	Checksums string   // path of checksums.txt
}

// Build runs the build.
func Build(ctx context.Context, o Options) (Result, error) {
	version, err := os.ReadFile(filepath.Join(o.Root, "VERSION"))
	if err != nil {
		return Result{}, err
	}
	if _, err := CheckVersion(o.Version, version); err != nil {
		return Result{}, err
	}
	commit, when, err := commitOf(ctx, o.Root)
	if err != nil {
		return Result{}, err
	}
	if !o.DryRun {
		if o.Gate == nil {
			return Result{}, errors.New("a release build needs the fuzz gate")
		}
		if err := o.Gate.Check(ctx, o.Root); err != nil {
			return Result{}, err
		}
	}
	copying, err := os.ReadFile(filepath.Join(o.Root, "COPYING"))
	if err != nil {
		return Result{}, err
	}
	out := o.Out
	if out == "" {
		if out, err = os.MkdirTemp("", "romeu-release-"); err != nil {
			return Result{}, err
		}
	} else if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	stage, err := os.MkdirTemp("", "romeu-stage-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	b := builder{o: o, stage: stage, ldflags: strings.Join([]string{
		"-buildid=",
		"-X main.version=" + o.Version,
		"-X main.commit=" + commit,
		"-X main.date=" + when.Format(time.RFC3339),
	}, " ")}
	var res Result
	archives := map[string][]byte{}
	for _, arch := range Arches {
		name := fmt.Sprintf("romeu_%s_darwin_%s.tar.gz", o.Version, arch)
		data, err := b.archive(ctx, arch, copying, when)
		if err != nil {
			return Result{}, err
		}
		path := filepath.Join(out, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return Result{}, err
		}
		archives[name] = data
		res.Archives = append(res.Archives, path)
	}
	res.Checksums = filepath.Join(out, ChecksumsName)
	if err := os.WriteFile(res.Checksums, Checksums(archives), 0o644); err != nil {
		return Result{}, err
	}
	return res, nil
}

// commitOf returns the commit HEAD names and its commit time, which is
// the build date.
func commitOf(ctx context.Context, root string) (string, time.Time, error) {
	out, err := git(ctx, root, "log", "-1", "--format=%H%n%ct", "HEAD")
	if err != nil {
		return "", time.Time{}, err
	}
	commit, secs, ok := strings.Cut(strings.TrimSpace(string(out)), "\n")
	n, perr := strconv.ParseInt(secs, 10, 64)
	if !ok || perr != nil {
		return "", time.Time{}, fmt.Errorf("git log answered %q", out)
	}
	return commit, time.Unix(n, 0).UTC(), nil
}

type builder struct {
	o       Options
	stage   string
	ldflags string
}

// archive builds the julieta binary for linux/arch, embeds it in the
// romeu build for darwin/arch and returns the archive of that romeu.
func (b builder) archive(ctx context.Context, arch string, copying []byte, when time.Time) ([]byte, error) {
	julieta := filepath.Join(b.stage, "julieta_linux_"+arch)
	if err := b.goBuild(ctx, "julieta", "linux", arch, julieta, ""); err != nil {
		return nil, err
	}
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(b.o.Root, filepath.FromSlash(EmbedPath)): julieta},
	})
	if err != nil {
		return nil, err
	}
	overlayFile := filepath.Join(b.stage, "overlay_"+arch+".json")
	if err := os.WriteFile(overlayFile, overlay, 0o600); err != nil {
		return nil, err
	}
	romeu := filepath.Join(b.stage, "romeu_darwin_"+arch)
	if err := b.goBuild(ctx, "romeu", "darwin", arch, romeu, overlayFile); err != nil {
		return nil, err
	}
	binary, err := os.ReadFile(romeu)
	if err != nil {
		return nil, err
	}
	notices, err := Notices(ctx, b.o.Root, b.o.GoBin, []Target{{"romeu", "darwin", arch}, {"julieta", "linux", arch}})
	if err != nil {
		return nil, err
	}
	return Archive([]File{
		{Name: "romeu", Mode: 0o755, Data: binary},
		{Name: "COPYING", Mode: 0o644, Data: copying},
		{Name: NoticesName, Mode: 0o644, Data: notices},
	}, when)
}

// goBuild builds ./cmd/<binary> for the platform to dst under the build
// contract. An empty overlay means none.
func (b builder) goBuild(ctx context.Context, binary, goos, goarch, dst, overlay string) error {
	argv := []string{b.o.GoBin, "build", "-trimpath", "-buildvcs=false", "-ldflags", b.ldflags}
	if overlay != "" {
		argv = append(argv, "-overlay", overlay)
	}
	argv = append(argv, "-o", dst, "./cmd/"+binary)
	env := append(goEnv(b.o.GoBin), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	if _, err := output(ctx, procgroup.ToolStep, b.o.Root, env, argv); err != nil {
		return fmt.Errorf("build %s for %s/%s: %w", binary, goos, goarch, err)
	}
	return nil
}
