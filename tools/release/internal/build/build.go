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
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
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
	Root       string    // the repository root
	Version    string    // the tag, v<major>.<minor>.<patch> with an optional suffix
	Out        string    // where the archives go: absent or empty; the empty string means a new temporary directory
	DryRun     bool      // build without the fuzz gate and without network
	AllowDirty bool      // a dry run may build a tree with changes; it is stamped dirty
	GoBin      string    // the pinned go, as "mise which go" resolves it
	Gate       *FuzzGate // required unless DryRun
}

// Result is what a build wrote.
type Result struct {
	Archives  []string // paths, in the order of Arches
	Checksums string   // path of checksums.txt
	// Embedded reports that a package of romeu consumes EmbedPath, so the
	// julieta binary is in the romeu binary. False means the build
	// carried no julieta: nothing in the tree reads the file yet.
	Embedded bool
	// Dirty reports that the tree had changes (AllowDirty) and the commit
	// stamp says so.
	Dirty bool
}

// Build runs the build. Everything is built in memory and in a staging
// directory first; the archives and checksums.txt reach Out in one
// rename, so a failure leaves no partial set.
func Build(ctx context.Context, o Options) (Result, error) {
	// The go command resolves the directory it runs in to its real path,
	// so the overlay key, which must equal that path, is built from the
	// resolved root: a root or a TMPDIR behind a symlink (macOS keeps
	// /var/folders under /private/var) would otherwise match no file.
	root, err := filepath.EvalSymlinks(o.Root)
	if err != nil {
		return Result{}, err
	}
	o.Root = root
	version, err := os.ReadFile(filepath.Join(o.Root, "VERSION"))
	if err != nil {
		return Result{}, err
	}
	if _, err := CheckVersion(o.Version, version); err != nil {
		return Result{}, err
	}
	if !o.DryRun && o.Gate == nil {
		return Result{}, errors.New("a release build needs the fuzz gate")
	}
	if err := checkOut(o.Out); err != nil {
		return Result{}, err
	}
	commit, when, err := commitOf(ctx, o.Root)
	if err != nil {
		return Result{}, err
	}
	dirty, err := dirtyTree(ctx, o.Root)
	if err != nil {
		return Result{}, err
	}
	res := Result{Dirty: dirty}
	if dirty {
		if !o.DryRun || !o.AllowDirty {
			return Result{}, errors.New("the tree has changes (git status --porcelain --untracked-files=all); a dry run builds one only with --allow-dirty")
		}
		commit += "-dirty"
	}
	if !o.DryRun {
		if err := tagNamesHead(ctx, o.Root, o.Version); err != nil {
			return Result{}, err
		}
		if err := o.Gate.Check(ctx, o.Root); err != nil {
			return Result{}, err
		}
	}
	copying, err := os.ReadFile(filepath.Join(o.Root, "COPYING"))
	if err != nil {
		return Result{}, err
	}
	stage, err := os.MkdirTemp("", "romeu-stage-")
	if err != nil {
		return Result{}, err
	}
	if stage, err = filepath.EvalSymlinks(stage); err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	b := builder{o: o, stage: stage, ldflags: strings.Join([]string{
		"-buildid=",
		"-X main.version=" + o.Version,
		"-X main.commit=" + commit,
		"-X main.date=" + when.Format(time.RFC3339),
	}, " ")}
	archives := map[string][]byte{}
	var names []string
	res.Embedded = true
	for _, arch := range Arches {
		name := fmt.Sprintf("romeu_%s_darwin_%s.tar.gz", o.Version, arch)
		data, embedded, err := b.archive(ctx, arch, copying, when)
		if err != nil {
			return Result{}, err
		}
		res.Embedded = res.Embedded && embedded
		archives[name] = data
		names = append(names, name)
	}
	if !o.DryRun && !res.Embedded {
		return Result{}, fmt.Errorf("no package of romeu consumes %s, so the build embeds no julieta; a release refuses", EmbedPath)
	}
	out, err := publish(o.Out, archives)
	if err != nil {
		return Result{}, err
	}
	for _, name := range names {
		res.Archives = append(res.Archives, filepath.Join(out, name))
	}
	res.Checksums = filepath.Join(out, ChecksumsName)
	return res, nil
}

// checkOut refuses an Out that holds anything. It must be absent or an
// empty directory; empty means none was named.
func checkOut(out string) error {
	if out == "" {
		return nil
	}
	entries, err := os.ReadDir(out)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("--out %s: %w", out, err)
	case len(entries) > 0:
		return fmt.Errorf("--out %s is not empty", out)
	}
	return nil
}

// publish writes the archives and checksums.txt, each with O_EXCL, into
// a new directory beside out and renames it to out. With no out the
// directory stays where it was made, under the temporary directory.
func publish(out string, archives map[string][]byte) (string, error) {
	parent := os.TempDir()
	if out != "" {
		abs, err := filepath.Abs(out)
		if err != nil {
			return "", err
		}
		out, parent = abs, filepath.Dir(abs)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
	}
	tmp, err := os.MkdirTemp(parent, ".romeu-release-")
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	files := map[string][]byte{ChecksumsName: Checksums(archives)}
	maps.Copy(files, archives)
	for name, data := range files {
		f, err := os.OpenFile(filepath.Join(tmp, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fail(err)
		}
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			return fail(err)
		}
		if err := f.Close(); err != nil {
			return fail(err)
		}
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return fail(err)
	}
	if out == "" {
		return tmp, nil
	}
	if err := os.Remove(out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fail(err)
	}
	if err := os.Rename(tmp, out); err != nil {
		return fail(err)
	}
	return out, nil
}

// dirtyTree reports whether the tree has any change, an untracked file
// included.
func dirtyTree(ctx context.Context, root string) (bool, error) {
	out, err := git(ctx, root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// tagNamesHead refuses unless the tag exists and names the commit HEAD
// is.
func tagNamesHead(ctx context.Context, root, tag string) error {
	named, err := git(ctx, root, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{commit}")
	if err != nil {
		return fmt.Errorf("the tag %s does not exist in this checkout", tag)
	}
	head, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(named), bytes.TrimSpace(head)) {
		return fmt.Errorf("the tag %s names %s, not HEAD %s", tag, bytes.TrimSpace(named), bytes.TrimSpace(head))
	}
	return nil
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
func (b builder) archive(ctx context.Context, arch string, copying []byte, when time.Time) ([]byte, bool, error) {
	julieta := filepath.Join(b.stage, "julieta_linux_"+arch)
	if err := b.goBuild(ctx, "julieta", "linux", arch, julieta, ""); err != nil {
		return nil, false, err
	}
	overlay, err := json.Marshal(map[string]map[string]string{
		"Replace": {filepath.Join(b.o.Root, filepath.FromSlash(EmbedPath)): julieta},
	})
	if err != nil {
		return nil, false, err
	}
	overlayFile := filepath.Join(b.stage, "overlay_"+arch+".json")
	if err := os.WriteFile(overlayFile, overlay, 0o600); err != nil {
		return nil, false, err
	}
	romeu := filepath.Join(b.stage, "romeu_darwin_"+arch)
	if err := b.goBuild(ctx, "romeu", "darwin", arch, romeu, overlayFile); err != nil {
		return nil, false, err
	}
	embedded, err := b.embeds(ctx, arch, overlayFile)
	if err != nil {
		return nil, false, err
	}
	binary, err := os.ReadFile(romeu)
	if err != nil {
		return nil, false, err
	}
	notices, err := Notices(ctx, b.o.Root, b.o.GoBin, []Target{{"romeu", "darwin", arch}, {"julieta", "linux", arch}})
	if err != nil {
		return nil, false, err
	}
	data, err := Archive([]File{
		{Name: "romeu", Mode: 0o755, Data: binary},
		{Name: "COPYING", Mode: 0o644, Data: copying},
		{Name: NoticesName, Mode: 0o644, Data: notices},
	}, when)
	return data, embedded, err
}

// embeds reports whether a package romeu links has EmbedPath among its
// embedded files under the overlay, which is what makes the go command
// put the julieta bytes in the binary.
func (b builder) embeds(ctx context.Context, arch, overlay string) (bool, error) {
	env := append(goEnv(b.o.GoBin), "GOOS=darwin", "GOARCH="+arch, "CGO_ENABLED=0")
	out, err := output(ctx, procgroup.ToolStep, b.o.Root, env,
		[]string{b.o.GoBin, "list", "-deps", "-overlay", overlay, "-json=Dir,EmbedFiles", "./cmd/romeu"})
	if err != nil {
		return false, err
	}
	dir, file := path.Split(EmbedPath)
	dir = strings.TrimSuffix(dir, "/")
	dec := jsontext.NewDecoder(bytes.NewReader(out))
	for {
		var p struct {
			Dir        string
			EmbedFiles []string
		}
		if err := json.UnmarshalDecode(dec, &p); errors.Is(err, io.EOF) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		if strings.HasSuffix(filepath.ToSlash(p.Dir), "/"+dir) && slices.Contains(p.EmbedFiles, file) {
			return true, nil
		}
	}
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
