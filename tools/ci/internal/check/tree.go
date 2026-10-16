// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// TreeFile is one entry of the tree at HEAD.
type TreeFile struct {
	Path    string
	Mode    string
	Content []byte // nil for a submodule, which has no content here
}

// FileIn returns the content of the file at path among files, the
// tree of one commit, and whether it is there, as the blobAt of hygiene does for a
// commit in the repository: a submodule at path is not a file.
func FileIn(files []TreeFile, path string) ([]byte, bool, error) {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		if f.Mode == "160000" {
			return nil, false, fmt.Errorf("read %s: not a file", path)
		}
		return f.Content, true, nil
	}
	return nil, false, nil
}

// HeadTree returns the files of the tree at head (hygiene) with their
// content, in the order git lists them.
func HeadTree(ctx context.Context, repo git.Repo, head string) ([]TreeFile, error) {
	out, err := repo.Run(ctx, nil, "ls-tree", "-r", "-z", "--full-tree", head)
	if err != nil {
		return nil, err
	}
	var files []TreeFile
	var ids bytes.Buffer
	for entry := range strings.SplitSeq(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if entry == "" {
			continue
		}
		// "<mode> <type> <id>\t<path>"
		meta, path, _ := strings.Cut(entry, "\t")
		f := strings.Fields(meta)
		if len(f) != 3 {
			return nil, fmt.Errorf("read tree: unexpected entry of %d fields", len(f))
		}
		files = append(files, TreeFile{Path: path, Mode: f[0]})
		if f[1] == "blob" {
			ids.WriteString(f[2] + "\n")
		}
	}
	blobs, err := repo.Run(ctx, ids.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	for i := range files {
		if files[i].Mode == "160000" { // a submodule: a commit, not a blob
			continue
		}
		// "<id> blob <size>\n<content>\n"
		head, rest, _ := bytes.Cut(blobs, []byte{'\n'})
		hf := strings.Fields(string(head))
		if len(hf) == 2 && hf[1] == "missing" {
			return nil, fmt.Errorf("read %s: the object store lacks its blob %s; %s", git.Printable(files[i].Path), hf[0], git.FetchHint(hf[0]))
		}
		if len(hf) != 3 {
			return nil, fmt.Errorf("read blob of tree entry %d: unexpected header", i+1)
		}
		size, err := strconv.Atoi(hf[2])
		if err != nil || size+1 > len(rest) {
			return nil, fmt.Errorf("read blob of tree entry %d: unexpected size", i+1)
		}
		files[i].Content = rest[:size:size]
		blobs = rest[size+1:]
	}
	return files, nil
}

// CommitOf returns the commit that rev names.
func CommitOf(ctx context.Context, repo git.Repo, rev string) (string, error) {
	out, err := repo.Run(ctx, nil, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	return strings.TrimSpace(string(out)), err
}
