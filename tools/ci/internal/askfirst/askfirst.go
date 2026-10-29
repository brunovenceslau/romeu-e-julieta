// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package askfirst is the face of .github/ask-first.yaml in tools/ci:
// the CODEOWNERS generator of 12 12.3 and the pr subcommand of 12 12.4.
// The rules themselves live in the approval package below it, which does
// not import the framework of tools/ci, so the pr job builds them alone
// (cmd/pr); this package wraps them for that framework and registers no
// check.
package askfirst

import (
	"context"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/askfirst/approval"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// Path and CodeownersPath are the list and the file generated from it.
const (
	Path           = approval.Path
	CodeownersPath = approval.CodeownersPath
)

// List is the ask-first list (approval.List).
type List = approval.List

// Surface is one surface of the list (approval.Surface).
type Surface = approval.Surface

// Parse reads and checks the list (approval.Parse), the one reader of
// it.
func Parse(src []byte) (*List, error) { return approval.Parse(src) }

// CheckGlob holds a glob to the dialect of 12 12.4 (approval.CheckGlob).
func CheckGlob(g string) error { return approval.CheckGlob(g) }

// GlobMatch reports whether a glob matches a name (approval.GlobMatch).
func GlobMatch(glob, name string) bool { return approval.GlobMatch(glob, name) }

// GlobsMeet reports whether some name matches both globs
// (approval.GlobsMeet).
func GlobsMeet(a, b string) bool { return approval.GlobsMeet(a, b) }

// Codeowners renders .github/CODEOWNERS from the list: the generated
// header of tools/ci, then the rules (approval.List.CodeownersRules).
func Codeowners(list *List) []byte {
	header := check.HashComment(check.GeneratedHeader(Path + " (the surfaces and their globs)"))
	return append([]byte(header), list.CodeownersRules()...)
}

// CodeownersOutput is the generator of .github/CODEOWNERS (12 12.3):
// it reads the list at Path below root with Parse and returns its
// CODEOWNERS file (Codeowners).
func CodeownersOutput(root *os.Root) ([]check.Output, error) {
	src, err := root.ReadFile(Path)
	if err != nil {
		return nil, err
	}
	list, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return []check.Output{{Path: CodeownersPath, Content: Codeowners(list)}}, nil
}

// RunPR is the pr subcommand (10 10.2, 12 12.4): it judges the approval
// lines of a pull request, read from a saved payload file, or, before the
// pull request exists, from --title and --body over the local range from
// --base to HEAD (approval.ReadArgs). It reads only local objects: a
// commit the clone lacks is an error, not a fetch (REQ-0816); the fetch
// is the pr job's alone (cmd/pr).
func RunPR(ctx context.Context, e check.Env, args []string) (bool, error) {
	pr, err := approval.ReadArgs(ctx, e.Repo(), args, check.Usage())
	if err != nil {
		return false, err
	}
	findings, err := approval.Judge(ctx, e.Repo(), pr, false)
	if err != nil {
		return false, err
	}
	return approval.Print(e.Stdout, findings), nil
}
