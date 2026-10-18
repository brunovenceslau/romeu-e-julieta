// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

import (
	"context"
	"fmt"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Generate is the "generate" subcommand, which "go generate ./..."
// runs through the directive above. It writes what every registered
// generator returns (check.Generators), the consumers of the tables of
// 12 12.3. It says which files it wrote, and none when the tree was
// current. It reads and writes through the root of the
// repository, so a symbolic link cannot send it outside.
func Generate(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generate takes no argument\n%s", check.Usage())
	}
	dir, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	outputs, err := generateOutputs(root)
	if err != nil {
		return false, err
	}
	for _, out := range outputs {
		wrote, err := check.WriteIfChanged(root, out.path, out.content)
		if err != nil {
			return false, err
		}
		if wrote {
			check.Say(e.Stdout, "wrote %s\n", git.SafeLines(out.path))
		}
	}
	return true, nil
}

// output is one generated file: its slash-separated path from the root
// and its bytes.
type output struct {
	path    string
	content []byte
}

// generateOutputs returns every file generate writes: the outputs of
// each generator of the registry, in the order they were registered
// (check.Generators). It writes nothing; "generated" calls it too, to
// learn which files must be tracked and with what bytes.
func generateOutputs(root *os.Root) ([]output, error) {
	var outs []output
	for _, g := range check.Generators() {
		got, err := g.Outputs(root)
		if err != nil {
			return nil, err
		}
		for _, o := range got {
			outs = append(outs, output{o.Path, o.Content})
		}
	}
	return outs, nil
}
