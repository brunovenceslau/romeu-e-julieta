// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude_test

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

// agentSpecific returns one line for each file of the kit directory
// fsys that names Claude: in a descriptor (a .yaml file) any string
// value, comments left out, and in any other file its content. A
// descriptor that does not parse, and a file that cannot be read, are a
// line too, so the scan never passes on what it did not read.
func agentSpecific(fsys fs.FS) []string {
	var found []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		text := string(b)
		if path.Ext(p) == ".yaml" {
			var doc yaml.Node
			if err := yaml.Unmarshal(b, &doc); err != nil {
				found = append(found, fmt.Sprintf("%s: does not parse: %v", p, err))
				return nil
			}
			text = strings.Join(values(&doc), "\n")
		}
		if strings.Contains(strings.ToLower(text), "claude") {
			found = append(found, p+": names claude")
		}
		return nil
	})
	if err != nil {
		found = append(found, err.Error())
	}
	return found
}

// values returns every scalar of the YAML tree n, keys and values,
// without its comments.
func values(n *yaml.Node) []string {
	if n.Kind == yaml.ScalarNode {
		return []string{n.Value}
	}
	var out []string
	for _, c := range n.Content {
		out = append(out, values(c)...)
	}
	return out
}
