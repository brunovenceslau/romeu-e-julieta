// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

func init() {
	register(kind{name: "kit", args: "<name>", run: newKit})
}

// kitsDir is the directory of the kits (06 6.1).
const kitsDir = "kits"

// kitName is the name of a kit: lower-case letters, digits and dashes,
// starting with a letter, so it is a directory name and a kit name at
// once.
var kitName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// newKit writes kits/<name>/<name>.yaml, the descriptor of a mixin kit
// on the pinned frontend (REQ-1123): the descriptor file is <stem>.yaml,
// with an optional <stem>.dockerfile beside it (sitting M0 of
// 2026-10-19), and its first line is the frontend line of kits/pins.yaml
// (REQ-0564). The SPDX header follows on lines 2 and 3, since the first
// line is the frontend's. The directory is created and never reused, so
// a kit that exists is an error, and the files are written through the
// root of the repository.
func newKit(e env, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("kit takes the name as one argument\n%s", usage())
	}
	name := args[0]
	if !kitName.MatchString(name) {
		return fmt.Errorf("the kit name %q is not lower-case letters, digits and dashes, starting with a letter", name)
	}
	top, err := repoRoot(e)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(top)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	src, err := root.ReadFile(pins.Path)
	if err != nil {
		return fmt.Errorf("the frontend pin is read from %s: %w", pins.Path, err)
	}
	p, err := pins.Parse(src)
	if err != nil {
		return err
	}
	if p.Frontend == nil {
		return fmt.Errorf("%s pins no frontend; run go run ./tools/kitpin frontend first", pins.Path)
	}
	dir := path.Join(kitsDir, name)
	if err := root.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s exists already", dir)
		}
		return err
	}
	file := path.Join(dir, name+".yaml")
	body := fmt.Sprintf(`# syntax=%s
# SPDX-FileCopyrightText: 2026 Bruno Venceslau
# SPDX-License-Identifier: GPL-3.0-only
schemaVersion: "3"
kind: mixin
displayName: %s
description: TODO
`, p.Frontend.Ref, name)
	if err := create(root, file, body); err != nil {
		return errors.Join(err, root.Remove(dir))
	}
	say(e.stdout, "%s\n", file)
	return nil
}
