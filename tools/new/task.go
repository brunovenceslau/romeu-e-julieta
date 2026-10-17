// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/slug"
)

func init() {
	register(kind{name: "task", args: "<title>", run: newTask})
}

// tasksDir is the directory of the plan's task files (02 2.1).
const tasksDir = "docs/plan/tasks"

// maxTask is the last id the three digits of a task file have room for.
const maxTask = 999

// taskName matches the filename of a task file: T, three digits, a
// dash, the slug and .md.
var taskName = regexp.MustCompile(`^T([0-9]{3})-.*\.md$`)

// newTask writes the next task file of the plan, Tnnn-<slug>.md, with
// the front matter the plan defines and the fields left to fill:
// milestone and module none, fate new, empty edge and file lists, no
// sitting, and the Implements line in its none form (REQ-1126). The
// title is the argument, checked as an ADR title is, and the filename
// is slug(title), as the tree's task files are named. The file is
// created and never overwritten, through the root of the repository,
// and a run whose re-scan finds its id taken by another file removes
// its own file and fails, as newADR does.
func newTask(e env, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("task takes the title as one argument\n%s", usage())
	}
	title := strings.TrimSpace(args[0])
	if err := checkTitle(title); err != nil {
		return err
	}
	name := slug.Slug(title)
	if name == "" {
		return fmt.Errorf("the title %q has no ASCII letter or digit, so its filename would be empty", title)
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
	byID, err := tasksByID(root)
	if err != nil {
		return err
	}
	n := nextNumber(byID)
	if n > maxTask {
		return fmt.Errorf("%s holds T%03d, the last id a task filename has room for", tasksDir, maxTask)
	}
	if e.afterScan != nil {
		e.afterScan()
	}
	id := fmt.Sprintf("T%03d", n)
	file := path.Join(tasksDir, id+"-"+name+".md")
	if err := create(root, file, taskBody(id, title)); err != nil {
		return err
	}
	after, err := tasksByID(root)
	if err == nil && len(after[n]) > 1 {
		err = fmt.Errorf("another task took %s at the same time (%s); run the command again", id, strings.Join(after[n], ", "))
	}
	if err != nil {
		if rmErr := root.Remove(file); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	say(e.stdout, "%s\n", file)
	return nil
}

// taskBody returns the content of a new task file.
func taskBody(id, title string) string {
	return "---\n" +
		"id: " + id + "\n" +
		"title: " + yamlQuote(title) + "\n" +
		"milestone: none\nmodule: none\nfate: new\n" +
		"owns: []\nneeds: []\nafter: []\nask_first: []\n" +
		"sitting: none\n---\n\n" +
		"# " + id + " - " + title + "\n\n" +
		"Goal:\n\n" +
		"- Implements: none:\n- Operator: no\n- Acceptance:\n- Verify:\n"
}

// yamlQuote returns s as a YAML double-quoted scalar. checkTitle has
// refused every control character, so a backslash and a double quote
// are the only characters to escape.
func yamlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// tasksByID maps each task id in tasksDir below root to the filenames
// that carry it. A directory with a task's name is not a task.
func tasksByID(root *os.Root) (map[int][]string, error) {
	entries, err := fs.ReadDir(root.FS(), tasksDir)
	if err != nil {
		return nil, err
	}
	byID := map[int][]string{}
	for _, entry := range entries {
		m := taskName.FindStringSubmatch(entry.Name())
		if m == nil || entry.IsDir() {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		byID[n] = append(byID[n], entry.Name())
	}
	return byID, nil
}
