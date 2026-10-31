// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package version

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hostpaths"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Versions is the versions object of romeu version --json (04 4.1,
// REQ-0286): sbx, git and macOS, and per project the workload digest
// and the kit pins of its live render, so a report can paste one
// document. A member that was not read is left out, and the read that
// failed is listed under errors, by its error id; the command still
// exits 0 (04 4.2), since the version of romeu itself was printed.
type Versions struct {
	// Sbx is the version sbx version reports.
	Sbx string `json:"sbx,omitzero"`
	// Git is the version of the host git, as gitsafe reads it.
	Git string `json:"git,omitzero"`
	// MacOS is the product version of macOS; absent off macOS, where
	// romeu runs for the tests alone.
	MacOS string `json:"macos,omitzero"`
	// Projects are the projects with a record in host state, in name
	// order.
	Projects []ProjectVersions `json:"projects"`
	// Errors are the reads of the machine that failed: the host
	// settings, sbx, git, macOS, the state directory.
	Errors []ReadError `json:"errors"`
}

// ProjectVersions are the versions of one project.
type ProjectVersions struct {
	// Name is the project's name.
	Name string `json:"name"`
	// Workload is the digest of the workload image of the project's gate
	// 2 approval, sha256:<hex>; absent before the first approval.
	Workload string `json:"workload,omitzero"`
	// Kits are the kit pins of the live render, by the kit dirs of its
	// render.json, in path order; absent when they were not read: before
	// the first promotion, while a promotion is interrupted (its record
	// is the candidate's, as status says), or when the root is not known.
	Kits []KitPin `json:"kits,omitzero"`
	// Errors are the reads of the project that failed.
	Errors []ReadError `json:"errors"`
}

// KitPin is one kit of the live render: its id and its kit-tree digest
// (03 3.12), as render.json records its dir.
type KitPin struct {
	// Kit is the kit's id, its product or personal name in the spec.
	Kit string `json:"kit"`
	// Digest is the kit-tree digest, 64 lowercase hex digits.
	Digest string `json:"digest"`
}

// ReadError is a read that failed: its error id and slug of 04 4.4,
// what was read, and the error's message.
type ReadError struct {
	// ID is the error id, such as RJ-344.
	ID string `json:"id"`
	// Slug is the error's slug.
	Slug string `json:"slug"`
	// Read names what was read, such as git version.
	Read string `json:"read"`
	// Message is the error's message, cut to its first maxMessage bytes
	// and escaped (I29).
	Message string `json:"message"`
}

// The reads the errors name.
const (
	readSettings = "host settings"
	readHome     = "HOME"
	readSbx      = "sbx version"
	readGit      = "git version"
	readMacOS    = "macOS version"
	readState    = "host state"
	readRecord   = "project record"
	readRender   = "render.json"
)

// seams are what the versions are read through: the environment, the
// version of sbx and of git by the settings and HOME, and the version of
// macOS. hostSeams are the process's; a test replaces them.
type seams struct {
	getenv func(string) string
	sbx    func(ctx context.Context, h *spec.HostSettings, home string) (string, error)
	git    func(ctx context.Context, h *spec.HostSettings, home string) (string, error)
	macOS  func() (string, error)
}

// hostSeams returns the seams of the process: sbx through sbxdrv, whose
// one call before the floor check is sbx version, and git through
// gitsafe in host mode, each by the absolute path of the settings and
// in a process group with the timeout of its class (04 4.1,
// Subprocesses).
func hostSeams() seams {
	return seams{
		getenv: os.Getenv,
		sbx: func(ctx context.Context, h *spec.HostSettings, home string) (string, error) {
			r := &sbxdrv.Runner{Sbx: h.Tools.Sbx, Home: home, Host: h}
			v, err := r.Version(ctx, sbxdrv.Opts{Step: "version"})
			if err != nil {
				return "", err
			}
			return v.String(), nil
		},
		git: func(ctx context.Context, h *spec.HostSettings, home string) (string, error) {
			g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: h.Tools.Git, Home: home})
			if err != nil {
				return "", usage("settings-invalid", err.Error())
			}
			return g.Version(ctx)
		},
		macOS: macOSVersion,
	}
}

// usage returns the error usage (RJ-349) for the slug of the rule a read
// breaks, with the hint of the help of the command.
func usage(slug, message string) error {
	e := cli.Usage.New(slug + ": " + message)
	e.Fill = map[string]string{"<binary>": "romeu", " <command>": " version"}
	return e
}

// readVersions reads the versions object through s. It takes no lock,
// writes nothing and reads host state as status does (state.OpenRead);
// every read that fails is listed and the others go on.
func readVersions(ctx context.Context, env *cli.Env, s seams) Versions {
	v := Versions{Projects: []ProjectVersions{}, Errors: []ReadError{}}
	fail := func(what string, err error) { v.Errors = append(v.Errors, readError(what, err)) }
	if m, err := s.macOS(); err != nil {
		fail(readMacOS, err)
	} else {
		v.MacOS = m
	}
	root := ""
	settings, err := hostpaths.LoadSettings(env, s.getenv, usage)
	if err != nil {
		fail(readSettings, err)
	} else {
		root = settings.Root
		readTools(ctx, &v, &settings, s, fail)
	}
	dir, err := hostpaths.StateDir(s.getenv)
	if err != nil {
		fail(readState, usage("state", err.Error()))
		return v
	}
	store, err := state.OpenRead(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return v
	case err != nil:
		fail(readState, err)
		return v
	}
	defer func() { _ = store.Close() }()
	names, err := store.ProjectNames()
	if err != nil {
		fail(readState, err)
		return v
	}
	for _, name := range names {
		v.Projects = append(v.Projects, readProject(store, root, name))
	}
	return v
}

// readTools reads the versions of sbx and git, which run with HOME as it
// is; the drivers take a clean absolute path with no control character,
// so any other HOME is a failed read of both.
func readTools(ctx context.Context, v *Versions, h *spec.HostSettings, s seams, fail func(string, error)) {
	home := s.getenv("HOME")
	if !filepath.IsAbs(home) || filepath.Clean(home) != home || strings.ContainsFunc(home, unicode.IsControl) {
		fail(readHome, usage("home", "HOME is not a clean absolute path"))
		return
	}
	if sbx, err := s.sbx(ctx, h, home); err != nil {
		fail(readSbx, err)
	} else {
		v.Sbx = sbx
	}
	if git, err := s.git(ctx, h, home); err != nil {
		fail(readGit, err)
	} else {
		v.Git = git
	}
}

// readProject reads the versions of the project name: the workload
// digest from its record's gate 2 approval, and the kit pins from the
// live render.json of <root>/<name>-env/ when root is known.
func readProject(store *state.Store, root, name string) ProjectVersions {
	p := ProjectVersions{Name: name, Errors: []ReadError{}}
	record, err := store.LoadProject(name)
	switch {
	case err == nil:
		if record.Approval != nil {
			p.Workload = record.Approval.Set.Workload.Digest
		}
	case !errors.Is(err, fs.ErrNotExist):
		p.Errors = append(p.Errors, readError(readRecord, err))
	}
	if root == "" {
		return p
	}
	envDir := filepath.Join(root, name+"-env")
	rec, found, err := render.ReadLive(envDir)
	if err == nil && found && rec.Promoting == nil {
		p.Kits, err = kitPins(rec.Files)
	}
	if errors.Is(err, render.ErrRecordInvalid) || errors.Is(err, errKitDir) {
		err = state.RecordUnreadable.Wrap(err, "render.json does not decode", cli.Detail{Key: "file", Value: filepath.Join(envDir, render.LiveRecord)})
	}
	if err != nil {
		p.Errors = append(p.Errors, readError(readRender, err))
	}
	return p
}

// errKitDir is the error of a path under the kits dir of render.json
// that is not a kit dir as render writes it.
var errKitDir = errors.New("version: not a kit dir of render.json")

// kitDir is the name render gives a kit dir (render.MaterializedKit.Dir):
// the kit's id, a name of 03 3.1, then a hyphen and the first 12 hex
// digits of its kit-tree digest.
var kitDir = regexp.MustCompile(`^([a-z0-9][a-z0-9-]{0,62})-([0-9a-f]{12})$`)

// treeDigest is a kit-tree digest as render.json writes it.
var treeDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// kitPins returns the kit pins of a files map of render.json: one per
// key under render.LiveKitsDir, in path order, each the id its dir names
// and the digest the map holds. A key there that is not a kit dir, or
// whose name does not begin its digest, is errKitDir: render writes no
// such key, so it is read as a damaged record, never guessed at.
func kitPins(files map[string]string) ([]KitPin, error) {
	pins := []KitPin{}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		dir, ok := strings.CutPrefix(p, render.LiveKitsDir+"/")
		if !ok {
			continue
		}
		m := kitDir.FindStringSubmatch(dir)
		digest := files[p]
		if m == nil || !treeDigest.MatchString(digest) || !strings.HasPrefix(digest, m[2]) {
			return nil, fmt.Errorf("%w: %+q", errKitDir, p)
		}
		pins = append(pins, KitPin{Kit: m[1], Digest: digest})
	}
	return pins, nil
}

// identified is an error that declares its row of 04 4.4 by methods, as
// gitsafe's *FloorError does, since gitsafe carries no cli.Row.
type identified interface {
	error
	ID() string
	Slug() string
}

// maxMessage is how many bytes of an error's message a ReadError keeps,
// before the escape: a message can quote what a file holds, such as a
// member name of render.json, at any length.
const maxMessage = 512

// readError returns the failed read of what by its error: the error's
// own row, or internal (RJ-348) for an error that carries none, as
// cli.Main reports such an error. The message is cut to maxMessage bytes
// and escaped (termsafe.Escape, I29): it can quote a file's text, and
// the escape also makes it valid UTF-8, since a byte that is not would
// stop the JSON encoder and lose the document.
func readError(what string, err error) ReadError {
	r := ReadError{Read: what, Message: message(err.Error())}
	if e, ok := errors.AsType[*cli.Error](err); ok && e != nil {
		r.ID, r.Slug = e.ID, e.Slug
		return r
	}
	if e, ok := errors.AsType[identified](err); ok {
		r.ID, r.Slug = e.ID(), e.Slug()
		return r
	}
	r.ID, r.Slug = cli.Internal.ID, cli.Internal.Slug
	return r
}

// message returns msg cut to maxMessage bytes, on a code point's first
// byte, and escaped, with "..." after a message that was cut.
func message(msg string) string {
	if len(msg) <= maxMessage {
		return termsafe.Escape(msg)
	}
	i := maxMessage
	for i > maxMessage-utf8.UTFMax && !utf8.RuneStart(msg[i]) {
		i--
	}
	return termsafe.Escape(msg[:i]) + "..."
}
