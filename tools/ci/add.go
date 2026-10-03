// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// add writes one denylist entry. The maintainer runs it on the host, in
// a terminal outside any sandbox and any agent session, because an
// agent must not hold the names (10 10.2, "Forbidden names").
//
// It reads the name from the terminal with echo off and refuses to
// start without one; it takes no name as an argument and prints none.
// So the name is in no argument list, no shell history and no pipe
// that another process filled.
func add(ctx context.Context, e env, args []string) error {
	if len(args) > 0 {
		return errors.New("hygiene add takes no argument: it reads the name from the terminal")
	}
	tty, ok := e.stdin.(*os.File)
	if !ok || !term.IsTerminal(int(tty.Fd())) {
		return errors.New("hygiene add needs a terminal on standard input, on the host")
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(names.Path))
	list, err := names.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		list, err = &names.List{}, nil
	}
	if err != nil {
		return err
	}

	// The name is typed blind, so it is typed twice: a slip of the
	// keyboard would write an entry that matches nothing, and no check
	// could see it, because the name is not in the tree.
	say(e.stderr, "The name is not shown. If you interrupt, \"stty echo\" brings the echo back.\n")
	typed, err := readBlind(e, tty, "name, one line, a space between segments: ")
	if err != nil {
		return err
	}
	again, err := readBlind(e, tty, "the same name again: ")
	if err != nil {
		return err
	}
	if !bytes.Equal(typed, again) {
		return errors.New("the two lines differ; nothing was written")
	}
	entry, err := names.NewEntry(string(typed))
	if err != nil {
		return err
	}
	if !list.Add(entry) {
		say(e.stdout, "the denylist already has this entry; it holds %d\n", list.Len())
		return nil
	}
	if err := writeFile(path, list.Marshal()); err != nil {
		return err
	}
	say(e.stdout, "added an entry of %d segment(s); %s now holds %d\n", entry.Segments(), names.Path, list.Len())
	return nil
}

// readBlind prompts and reads one line from the terminal with echo off.
func readBlind(e env, tty *os.File, prompt string) ([]byte, error) {
	say(e.stderr, "%s", prompt)
	line, err := term.ReadPassword(int(tty.Fd()))
	say(e.stderr, "\n")
	if err != nil {
		return nil, fmt.Errorf("read the name: %w", err)
	}
	return line, nil
}

// writeFile replaces path through a temporary file in its directory,
// so a run that is interrupted leaves the earlier denylist whole.
func writeFile(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".denylist-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil { // no file is left behind when a step fails
			err = errors.Join(err, os.Remove(tmp.Name()))
		}
	}()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o644)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
