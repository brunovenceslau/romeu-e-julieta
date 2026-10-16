// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package generated

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
// runs through the directive above. It writes the consumers of the
// tables of 12 12.3 that exist today: the two of .github/ask-first.yaml
// (the third, the mutate trigger, lands with mutate) and the index of
// the decision records. It says which files it wrote, and none when the
// tree was current. It reads and writes through the root of the
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
// each check of the registry, in its order (check.Check.Outputs). It
// writes nothing; "generated" calls it too, to learn which files must be
// tracked and with what bytes.
func generateOutputs(root *os.Root) ([]output, error) {
	var outs []output
	for _, c := range check.Registered() {
		if c.Outputs == nil {
			continue
		}
		got, err := c.Outputs(root)
		if err != nil {
			return nil, err
		}
		for _, o := range got {
			outs = append(outs, output{o.Path, o.Content})
		}
	}
	return outs, nil
}

// Outputs returns the files the generated check registers as its
// outputs: those of its sources below root (sources), then the
// inventory of the checks of the registry, docs/reference/checks.md
// (ChecksPage).
func Outputs(root *os.Root) ([]check.Output, error) {
	outs, err := sources(root)
	if err != nil {
		return nil, err
	}
	return append(outs, check.Output{Path: ChecksPath, Content: ChecksPage(check.Registered())}), nil
}

// sources reads the sources of the generated check below root and
// returns the files that follow from them: the two consumers of
// .github/ask-first.yaml and the index of the decision records.
func sources(root *os.Root) ([]check.Output, error) {
	src, err := root.ReadFile(askfirst.Path)
	if err != nil {
		return nil, err
	}
	list, err := askfirst.Parse(src)
	if err != nil {
		return nil, err
	}
	index, indexPath, err := readADRIndex(root)
	if err != nil {
		return nil, err
	}
	return []check.Output{
		{Path: askfirst.CodeownersPath, Content: list.Codeowners()},
		{Path: askfirst.PagePath, Content: list.ReferencePage()},
		{Path: indexPath, Content: index},
	}, nil
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
