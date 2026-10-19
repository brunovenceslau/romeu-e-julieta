// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// NoticesName is the third-party notices file of every archive
// (REQ-0904).
const NoticesName = "THIRD_PARTY_NOTICES.txt"

// licenseFile is the name of a file in a module's root that holds its
// license or its notices.
var licenseFile = regexp.MustCompile(`(?i)^(licen[sc]e|copying|notice|unlicense)([._-][^/]*)?$`)

// module is a module some binary links, with the binaries that do.
type module struct {
	Path, Version, Dir string
	Binaries           []string
}

// listed is the part of "go list -json" the notices read.
type listed struct {
	Standard bool
	Module   *struct {
		Path, Version, Dir string
		Main               bool
		Replace            *struct{ Path, Version, Dir string }
	}
}

// Target is the go command's view of one binary: the command under
// cmd/ and the platform it is built for.
type Target struct {
	Binary, GOOS, GOARCH string
}

// modulesOf returns the modules other than the main module that the
// binary of t links, from "go list -deps" for its platform with cgo
// off, in the build's environment.
func modulesOf(ctx context.Context, root, goBin string, t Target) ([]module, error) {
	env := append(goEnv(goBin), "GOOS="+t.GOOS, "GOARCH="+t.GOARCH, "CGO_ENABLED=0")
	out, err := output(ctx, procgroup.ToolStep, root, env,
		[]string{goBin, "list", "-deps", "-json=Standard,Module", "./cmd/" + t.Binary})
	if err != nil {
		return nil, fmt.Errorf("the module graph of %s: %w", t.Binary, err)
	}
	var mods []module
	dec := jsontext.NewDecoder(bytes.NewReader(out))
	for {
		var p listed
		if err := json.UnmarshalDecode(dec, &p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("the module graph of %s: %w", t.Binary, err)
		}
		if p.Standard || p.Module == nil || p.Module.Main {
			continue
		}
		m := module{Path: p.Module.Path, Version: p.Module.Version, Dir: p.Module.Dir, Binaries: []string{t.Binary}}
		if r := p.Module.Replace; r != nil {
			m.Dir = r.Dir
		}
		if m.Dir == "" {
			return nil, fmt.Errorf("module %s of %s is not in the module cache", m.Path, t.Binary)
		}
		if !slices.ContainsFunc(mods, func(o module) bool { return o.Path == m.Path }) {
			mods = append(mods, m)
		}
	}
	return mods, nil
}

// Notices writes the third-party notices of one archive: every module
// that the binaries of targets link, with the license files of its root,
// and the license of the Go distribution that built them. A module with
// no license file is an error, since the archive could not carry its
// notice.
func Notices(ctx context.Context, root, goBin string, targets []Target) ([]byte, error) {
	var all []module
	for _, t := range targets {
		mods, err := modulesOf(ctx, root, goBin, t)
		if err != nil {
			return nil, err
		}
		for _, m := range mods {
			if at := slices.IndexFunc(all, func(o module) bool { return o.Path == m.Path && o.Version == m.Version }); at >= 0 {
				all[at].Binaries = append(all[at].Binaries, m.Binaries...)
				continue
			}
			all = append(all, m)
		}
	}
	slices.SortFunc(all, func(a, b module) int {
		return strings.Compare(a.Path+"@"+a.Version, b.Path+"@"+b.Version)
	})

	var b bytes.Buffer
	b.WriteString("Third-party notices\n\n")
	b.WriteString("The binaries of this archive link the modules below and the Go\n")
	b.WriteString("distribution that built them. Each section is a module, its version,\n")
	b.WriteString("the binaries that link it, and the license files of its root.\n")

	goroot, err := output(ctx, procgroup.ToolProbe, root, goEnv(goBin), []string{goBin, "env", "GOROOT"})
	if err != nil {
		return nil, err
	}
	std := module{Path: "go (the standard library and runtime)", Dir: strings.TrimSpace(string(goroot)), Binaries: binaries(targets)}
	for _, m := range append([]module{std}, all...) {
		if err := section(&b, m); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

func binaries(targets []Target) []string {
	var names []string
	for _, t := range targets {
		if !slices.Contains(names, t.Binary) {
			names = append(names, t.Binary)
		}
	}
	return names
}

// section writes the license files of m's root.
func section(b *bytes.Buffer, m module) error {
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		return fmt.Errorf("module %s: %w", m.Path, err)
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && licenseFile.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("module %s %s has no license file in %s", m.Path, m.Version, m.Dir)
	}
	slices.Sort(names)
	bins := slices.Clone(m.Binaries)
	slices.Sort(bins)
	bins = slices.Compact(bins)
	title := m.Path
	if m.Version != "" {
		title += " " + m.Version
	}
	fmt.Fprintf(b, "\n================================================================\n%s\nlinked by: %s\n", title, strings.Join(bins, ", "))
	for _, name := range names {
		text, err := os.ReadFile(filepath.Join(m.Dir, name))
		if err != nil {
			return fmt.Errorf("module %s: %w", m.Path, err)
		}
		fmt.Fprintf(b, "\n---- %s ----\n\n%s", name, text)
		if !bytes.HasSuffix(text, []byte("\n")) {
			b.WriteByte('\n')
		}
	}
	return nil
}
