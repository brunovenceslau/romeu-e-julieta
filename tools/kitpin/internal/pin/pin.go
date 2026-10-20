// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package pin holds the commands of tools/kitpin, which writes
// kits/pins.yaml, the one pin file (02 2.1); the main package parses
// the command line and calls Frontend, Workload, Herdr or Mise.
//
//	go run ./tools/kitpin frontend
//	go run ./tools/kitpin workload <registry>/<repository>@sha256:<64 hex>
//	go run ./tools/kitpin herdr <version>
//	go run ./tools/kitpin mise <version>
//
// Each rewrites its own entry and keeps the others; the file is
// written whole, with its header comment (package pins).
//
// frontend reads the tags of docker/sandbox-kit from Docker Hub's
// registry with an anonymous pull token, takes the highest milestone
// tag 3.0.0-m.<N>, and pins it with the sha256 of the image index the
// registry serves for it, computed from the bytes; a
// Docker-Content-Digest header that differs is an error, and so is an
// index that does not list both linux/amd64 and linux/arm64 (06 6.1).
//
// workload pins a reference given whole, which must be
// <registry>/<repository>@sha256:<64 hex> (03 3.1), and reads no
// registry; that the image covers linux/amd64 and linux/arm64 is
// checked by "julieta pin workload" (06 6.2). It keeps the entry's
// home, which is written by hand with its source.
//
// herdr reads the release v<version> of github.com/herdrdev/herdr from
// the GitHub API, refuses it unless it is marked immutable, and takes
// the sha256 of herdr-linux-x86_64 and herdr-linux-aarch64 from each
// asset's digest field, without downloading a binary; a digest field
// that is missing or not sha256:<64 hex> is refused by name (06 6.4).
//
// mise downloads SHASUMS256.txt and SHASUMS256.txt.minisig of the
// release v<version> of github.com/jdx/mise, verifies the signature
// with the public key committed at kits/mise-minisign.pub (package
// minisign) and that its trusted comment names SHASUMS256.txt, and
// only then writes both files to kits/mise/ and the version and the
// sha256 of mise-v<version>-linux-x64 and mise-v<version>-linux-arm64
// to the pin file (07 7.4). The key is the trust root, confirmed by the
// maintainer out of band: a missing or unreadable key is an error, and
// no flag or environment variable skips the verification. A version
// lower than the pinned one is refused.
//
// Every request is https, to the hosts named for its source, and a
// redirect elsewhere, or to another scheme, is refused (Source).
//
// A pin that changes needs the probes that read it run again (12 12.1,
// REQ-1069); the header of the pin file says so.
package pin

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
	"github.com/google/renameio/v2"
)

// Source is where one kind of pin is read: the base URL of its
// requests, and the hosts a request or a redirect may reach, each
// "host" or "host:port". A request goes over https only.
type Source struct {
	Base  string
	Hosts []string
}

// Env holds what a run reads from outside: the directory it starts in,
// git's environment, the HTTP client and the sources, which the tests
// point at a local server. Nothing in it comes from the environment or
// the command line.
type Env struct {
	Dir      string
	GitEnv   []string // nil means this process's
	Client   *http.Client
	Mise     Source // the release downloads of jdx/mise
	GitHub   Source // the GitHub API, for herdr
	Auth     Source // Docker Hub's token endpoint
	Registry Source // Docker Hub's registry
}

// Default is the Env of a real run from the current directory. The
// hosts are those measured on 2026-10-18: a mise release download
// redirects to release-assets.githubusercontent.com; the GitHub API,
// the token endpoint and the registry's tag list and manifests answer
// without a redirect, so no blob host is named.
func Default() Env {
	return Env{
		Dir:    ".",
		Client: &http.Client{Timeout: 60 * time.Second},
		Mise: Source{Base: "https://github.com/jdx/mise/releases/download",
			Hosts: []string{"github.com", "release-assets.githubusercontent.com"}},
		GitHub:   Source{Base: "https://api.github.com", Hosts: []string{"api.github.com"}},
		Auth:     Source{Base: "https://auth.docker.io/token", Hosts: []string{"auth.docker.io"}},
		Registry: Source{Base: "https://registry-1.docker.io", Hosts: []string{"registry-1.docker.io"}},
	}
}

// pinFunc changes the pins, and returns the line that says what it
// pinned.
type pinFunc func(ctx context.Context, e Env, w *writer, p *pins.Pins) (string, error)

// Frontend pins the highest milestone tag of the kit frontend.
func Frontend(ctx context.Context, e Env) (string, error) {
	return pin(ctx, e, pinFrontend)
}

// Workload pins the workload reference ref.
func Workload(ctx context.Context, e Env, ref string) (string, error) {
	return pin(ctx, e, func(_ context.Context, _ Env, _ *writer, p *pins.Pins) (string, error) {
		return pinWorkload(p, ref)
	})
}

// Herdr pins herdr at version.
func Herdr(ctx context.Context, e Env, version string) (string, error) {
	return pin(ctx, e, func(ctx context.Context, e Env, _ *writer, p *pins.Pins) (string, error) {
		return pinHerdr(ctx, e, p, version)
	})
}

// Mise pins mise at version, after the signature check.
func Mise(ctx context.Context, e Env, version string) (string, error) {
	return pin(ctx, e, func(ctx context.Context, e Env, w *writer, p *pins.Pins) (string, error) {
		return pinMise(ctx, e, w, p, version)
	})
}

// pin reads the pin file, runs f on it, encodes it, and only then
// writes the files f staged and, last, the pin file. It returns f's
// line.
func pin(ctx context.Context, e Env, f pinFunc) (string, error) {
	top, err := git.Repo{Dir: e.Dir, Env: e.GitEnv}.Run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	w, err := openWriter(strings.TrimSuffix(string(top), "\n"))
	if err != nil {
		return "", err
	}
	defer w.close()
	p, err := w.readPins()
	if err != nil {
		return "", err
	}
	done, err := f(ctx, e, w, &p)
	if err != nil {
		return "", err
	}
	src, err := pins.Encode(p)
	if err != nil {
		return "", err
	}
	for _, s := range w.staged {
		if err := w.write(s.path, s.data); err != nil {
			return "", err
		}
	}
	if err := w.write(pins.Path, src); err != nil {
		return "", err
	}
	return done, nil
}

// writer reads and writes files below the top of the repository,
// through an os.Root, so no path leaves it.
type writer struct {
	root   *os.Root
	staged []stagedFile // written by pin, after the pin file is encoded
}

type stagedFile struct {
	path string
	data []byte
}

func openWriter(top string) (*writer, error) {
	r, err := os.OpenRoot(top)
	if err != nil {
		return nil, err
	}
	return &writer{root: r}, nil
}

func (w *writer) close() { _ = w.root.Close() }

// stage records a file that pin writes once the command succeeded and
// the pin file is encoded.
func (w *writer) stage(path string, data []byte) {
	w.staged = append(w.staged, stagedFile{path, data})
}

// readPins reads the pin file; a missing one is an empty one.
func (w *writer) readPins() (pins.Pins, error) {
	src, err := w.root.ReadFile(pins.Path)
	if errors.Is(err, os.ErrNotExist) {
		return pins.Pins{}, nil
	}
	if err != nil {
		return pins.Pins{}, err
	}
	return pins.Parse(src)
}

// write replaces path with data through renameio (ADR 0038): a new
// temporary file beside it, under a name no other run takes (O_EXCL),
// synced to disk, then renamed over path, so a reader never sees half a file; the directory is
// synced after the rename.
func (w *writer) write(path string, data []byte) error {
	dir := "."
	if i := strings.LastIndex(path, "/"); i > 0 {
		dir = path[:i]
		if err := w.root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// renameio makes its temporary file at the top of the root it is
	// given, so it gets the root of path's directory.
	sub, err := w.root.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Close() }()
	t, err := renameio.NewPendingFile(path[strings.LastIndex(path, "/")+1:],
		renameio.WithRoot(sub), renameio.WithStaticPermissions(0o644))
	if err != nil {
		return err
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return err
	}
	if err := t.CloseAtomicallyReplace(); err != nil {
		return err
	}
	d, err := w.root.Open(dir)
	if err != nil {
		return err
	}
	err = d.Sync()
	return errors.Join(err, d.Close())
}
