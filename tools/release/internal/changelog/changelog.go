// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package changelog is "tools/release changelog --version <tag>"
// (12 12.6, REQ-1344): the section of a tag, rendered by git-cliff from
// cliff.toml and the merge commits since the previous tag, prepended to
// CHANGELOG.md, with the Upgrading part computed here (REQ-1175) and
// passed through --with-tag-message. git-cliff runs with --offline and
// --no-exec (REQ-1340), by the path "mise which" resolves (REQ-1053).
//
// The command refuses a tag that is not of the form of REQ-0899 and
// REQ-1337, one that exists on the local repository or on origin, a
// file that already holds the section, and a render with no entry (no
// merged pull request since the previous tag). With Stdout it prints
// the section and writes nothing. Two runs on one commit give one file:
// the second refuses, since the section exists.
package changelog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/release/internal/build"
)

// The files of the repository the command reads and writes.
const (
	// File is the changelog, at the root of the repository (REQ-1341).
	File = "CHANGELOG.md"
	// Config is git-cliff's configuration, at the root (REQ-1341).
	Config = "cliff.toml"
	// versionFile is the committed VERSION file (REQ-1249).
	versionFile = "VERSION"
	// Marker ends the static header of File; git-cliff --prepend keeps
	// what is before it and writes the new section after it
	// (header_marker of Config).
	Marker = "<!-- git-cliff: end of header -->"
)

// prereleaseSuffix is the suffix of a prerelease tag before v1.0.0
// (REQ-1337): alpha.<n> or rc.<n>, <n> counting from 1.
var prereleaseSuffix = regexp.MustCompile(`^-(alpha|rc)\.[1-9][0-9]*$`)

// Options are the inputs of Run.
type Options struct {
	// Root is the root of the repository, where Config, File and
	// VERSION are and where git runs.
	Root string
	// Version is the tag whose section is written.
	Version string
	// Cliff is the path of git-cliff (tools/ci/pinned).
	Cliff string
	// Stdout, when set, prints the section to Out and writes nothing.
	Stdout bool
	// Out receives the section with Stdout.
	Out io.Writer
}

// Run writes the section of o.Version to File, or prints it with
// o.Stdout, after the checks the package comment lists.
func Run(ctx context.Context, o Options) error {
	if err := checkVersion(o.Root, o.Version); err != nil {
		return err
	}
	if err := checkAbsent(ctx, o.Root, o.Version); err != nil {
		return err
	}
	file, err := os.ReadFile(filepath.Join(o.Root, File))
	if err != nil {
		return fmt.Errorf("%s: %w; the task that added %s commits the file with its header", File, err, Config)
	}
	if !bytes.Contains(file, []byte(Marker)) {
		return fmt.Errorf("%s holds no header marker %q, after which git-cliff writes the section", File, Marker)
	}
	if _, ok := Section(file, o.Version); ok {
		return fmt.Errorf("%s already holds the section of %s; prepare regenerates it from main", File, o.Version)
	}
	prev, err := previousTag(ctx, o.Root)
	if err != nil {
		return err
	}
	upgrading, err := Upgrading(ctx, o.Root, prev)
	if err != nil {
		return err
	}
	section, err := render(ctx, o, upgrading, []string{"--strip", "all"})
	if err != nil {
		return err
	}
	if !hasEntry(section) {
		since := "the first commit"
		if prev != "" {
			since = prev
		}
		return fmt.Errorf("no merged pull request since %s: the section of %s would hold no entry", since, o.Version)
	}
	if o.Stdout {
		if o.Out == nil {
			o.Out = io.Discard
		}
		_, err := o.Out.Write(append(bytes.TrimRight(section, "\n"), '\n'))
		return err
	}
	if _, err := render(ctx, o, upgrading, []string{"--prepend", File}); err != nil {
		return err
	}
	path := filepath.Join(o.Root, File)
	written, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, normalize(written), 0o644)
}

// checkVersion refuses a version that is not a release tag of VERSION
// (build.CheckVersion: the form of REQ-0899, and <major>.<minor>.<patch>
// the one VERSION names), or a prerelease whose suffix is not alpha.<n>
// or rc.<n> (REQ-1337).
func checkVersion(root, version string) error {
	file, err := os.ReadFile(filepath.Join(root, versionFile))
	if err != nil {
		return err
	}
	prerelease, err := build.CheckVersion(version, file)
	if err != nil {
		return err
	}
	if prerelease && !prereleaseSuffix.MatchString(version[strings.IndexByte(version, '-'):]) {
		return fmt.Errorf("version %q is a prerelease whose suffix is not alpha.<n> or rc.<n> (REQ-1337)", version)
	}
	return nil
}

// checkAbsent refuses a tag that exists on the local repository or on
// origin, which must be a remote of the repository.
func checkAbsent(ctx context.Context, root, version string) error {
	local, err := git(ctx, root, "tag", "--list", "--", version)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(local)) != 0 {
		return fmt.Errorf("the tag %s exists on this repository", version)
	}
	if _, err := git(ctx, root, "remote", "get-url", "origin"); err != nil {
		return fmt.Errorf("no remote origin, where the tag is checked: %w", err)
	}
	remote, err := git(ctx, root, "ls-remote", "--tags", "origin", "refs/tags/"+version)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(remote)) != 0 {
		return fmt.Errorf("the tag %s exists on origin", version)
	}
	return nil
}

// previousTag returns the nearest tag of the form v<digit>... that HEAD
// reaches (git describe), the one git-cliff ends the section at, or ""
// when there is none.
func previousTag(ctx context.Context, root string) (string, error) {
	out, err := git(ctx, root, "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*")
	if err == nil {
		return string(bytes.TrimSpace(out)), nil
	}
	if gitExit(err) == 128 && strings.Contains(err.Error(), "No names found") {
		return "", nil
	}
	return "", err
}

// render runs git-cliff for the unreleased section of o.Version with
// the Upgrading part as the tag message and the given output flags,
// and returns its standard output. A failure carries git-cliff's
// standard error, escaped.
func render(ctx context.Context, o Options, upgrading string, flags []string) ([]byte, error) {
	argv := append([]string{o.Cliff, "--offline", "--no-exec",
		"--config", filepath.Join(o.Root, Config), "--repository", o.Root, "--workdir", o.Root,
		"--tag", o.Version, "--unreleased", "--with-tag-message", upgrading}, flags...)
	return output(ctx, procgroup.ToolStep, o.Root, argv)
}

// hasEntry reports whether the rendered section holds an entry: a
// list item after the first level-3 heading that follows the Upgrading
// part, whose own bullets are not entries.
func hasEntry(section []byte) bool {
	rest := section
	if i := bytes.Index(rest, []byte("\n### Upgrading\n")); i >= 0 {
		rest = rest[i+len("\n### Upgrading\n"):]
	}
	i := bytes.Index(rest, []byte("\n### "))
	if i < 0 {
		return false
	}
	return bytes.Contains(rest[i:], []byte("\n- "))
}

// heading is a level-2 heading at the start of a line, which only a
// section begins (REQ-1350): the Why text of an entry is indented and
// the Upgrading part uses level 3.
var heading = regexp.MustCompile(`(?m)^## (\S+)[ \t]*$`)

// Section returns the section of version in file (REQ-1350): from the
// level-2 heading whose text is the tag to the next level-2 heading or
// the end of the file, trailing blank lines removed, and whether the
// file holds it.
func Section(file []byte, version string) ([]byte, bool) {
	all := heading.FindAllSubmatchIndex(file, -1)
	for i, m := range all {
		if string(file[m[2]:m[3]]) != version {
			continue
		}
		end := len(file)
		if i+1 < len(all) {
			end = all[i+1][0]
		}
		return bytes.TrimRight(file[m[0]:end], "\n"), true
	}
	return nil, false
}

// normalize rewrites the seams git-cliff --prepend leaves in file and
// nothing inside a section: one blank line after the header marker,
// one between sections, and one newline at the end. git-cliff writes
// the header of its configuration, the section and two newlines before
// what followed the marker, so the blank line after the marker and the
// one before the next section vary with the file it read (measured with
// git-cliff 2.14.2).
func normalize(file []byte) []byte {
	i := bytes.Index(file, []byte(Marker))
	if i < 0 {
		return file
	}
	rest := file[i+len(Marker):]
	idx := heading.FindAllIndex(rest, -1)
	if len(idx) == 0 {
		return append(bytes.TrimRight(file, "\n"), '\n')
	}
	out := append([]byte{}, file[:i+len(Marker)]...)
	for n, m := range idx {
		end := len(rest)
		if n+1 < len(idx) {
			end = idx[n+1][0]
		}
		out = append(out, '\n', '\n')
		out = append(out, bytes.TrimRight(rest[m[0]:end], "\n")...)
	}
	return append(out, '\n')
}
