// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// adrIndexName is the name of the index of the decision records that
// generate writes (12 12.3). A generated file is never edited by hand:
// generated fails when the run of generate changes one, and when one is
// not tracked as a regular file with the bytes that generate gives.
const adrIndexName = "README.md"

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

// The generators this package holds until each moves next to its data:
// .github/CODEOWNERS, the one consumer of .github/ask-first.yaml
// (Codeowners), the index of the decision records (ADRIndex), and the
// inventory of the checks of the registry, docs/reference/checks.md
// (ChecksPageOutput). tools/ci/generator_<name>.go registers each.

// Codeowners reads .github/ask-first.yaml below root and returns
// .github/CODEOWNERS.
func Codeowners(root *os.Root) ([]check.Output, error) {
	src, err := root.ReadFile(askfirst.Path)
	if err != nil {
		return nil, err
	}
	list, err := askfirst.Parse(src)
	if err != nil {
		return nil, err
	}
	return []check.Output{{Path: askfirst.CodeownersPath, Content: list.Codeowners()}}, nil
}

// ADRIndex returns the index of the decision records (readADRIndex).
func ADRIndex(root *os.Root) ([]check.Output, error) {
	index, indexPath, err := readADRIndex(root)
	if err != nil {
		return nil, err
	}
	return []check.Output{{Path: indexPath, Content: index}}, nil
}

// ChecksPageOutput returns docs/reference/checks.md, the inventory of
// the checks of the registry (ChecksPage).
func ChecksPageOutput(*os.Root) ([]check.Output, error) {
	return []check.Output{{Path: ChecksPath, Content: ChecksPage(check.Registered())}}, nil
}

// readADRIndex renders the index of the records in the directory that
// .adr-dir names, and returns it with its path from the root. A record
// is a regular file: a symbolic link with a record's name is skipped.
func readADRIndex(root *os.Root) ([]byte, string, error) {
	dir, err := adrdir.Dir(root)
	if err != nil {
		return nil, "", err
	}
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(dir))
	if err != nil {
		return nil, "", err
	}
	var records []adr
	for _, entry := range entries {
		if !adrdir.Name.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		content, err := root.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, "", err
		}
		record, err := parseADR(entry.Name(), content)
		if err != nil {
			return nil, "", err
		}
		records = append(records, record)
	}
	return adrIndex(records), filepath.ToSlash(filepath.Join(dir, adrIndexName)), nil
}
