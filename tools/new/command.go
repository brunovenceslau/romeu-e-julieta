// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"cmp"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

func init() {
	register(kind{name: "command", args: "<binary> <command> [<subcommand>]", run: newCommand})
}

// commandWord is the shape of a command or subcommand name that a
// scaffold accepts: one lowercase word, which is also the name of the
// command's Go package.
var commandWord = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// goFileSuffixes are the words that, as the last element of a Go file
// name before .go, make the go command treat the file specially: test,
// and each GOOS and GOARCH it knows (go/build), which constrain the
// build. A scaffold's file is named after a command, so a command of
// one of these names would build on some platforms only, or never.
var goFileSuffixes = strings.Fields(`test
	aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos
	386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le
	ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm`)

// checkFileWord refuses a word that cannot name a Go package and its
// files: a Go keyword, the package all of the assembled command set,
// or a word that makes a file name special (goFileSuffixes).
func checkFileWord(w string) error {
	switch {
	case token.IsKeyword(w):
		return fmt.Errorf("the name %q is a Go keyword, so it cannot name a package", w)
	case w == "all":
		return fmt.Errorf("the name %q is the package of the assembled command set", w)
	case slices.Contains(goFileSuffixes, w):
		return fmt.Errorf("the name %q makes the go command treat a file named after it as a test or for one platform", w)
	}
	return nil
}

// newCommand writes the scaffold of one command of a binary (12 12.3,
// REQ-1124): its definition in the package
// internal/cmd/<binary>/<command>/, the five side-effect fields left
// unset, so Register names each one to fill at init, a function that
// returns its *cobra.Command with RunE (ADR 0015), its help test, and,
// for a new package, the blank-import file
// internal/cmd/<binary>/all/import_<command>.go that links it in. A
// subcommand is one more definition in the package of its command. It
// creates files and never overwrites one, so it edits no shared file.
func newCommand(e env, args []string) error {
	if len(args) != 2 && len(args) != 3 {
		return fmt.Errorf("command takes a binary, a command and, for a subcommand, its name\n%s", usage())
	}
	binary, name, sub := args[0], args[1], ""
	if len(args) == 3 {
		sub = args[2]
	}
	if binary != "romeu" && binary != "julieta" {
		return fmt.Errorf("the binary %q is neither romeu nor julieta", binary)
	}
	words := []string{name}
	if sub != "" {
		words = append(words, sub)
	}
	for _, w := range words {
		if !commandWord.MatchString(w) {
			return fmt.Errorf("the name %q is not one lowercase word of letters and digits that starts with a letter", w)
		}
		if slices.Contains(cli.Builtins, w) {
			return fmt.Errorf("the name %q is a command every tree already has", w)
		}
		if err := checkFileWord(w); err != nil {
			return err
		}
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

	dir := path.Join("internal/cmd", binary, name)
	_, err = fs.Stat(root.FS(), dir)
	newPackage := errors.Is(err, fs.ErrNotExist)
	if err != nil && !newPackage {
		return err
	}
	if !newPackage && sub == "" {
		return fmt.Errorf("%s exists, so %s %s has a package already", dir, binary, name)
	}
	file := cmp.Or(sub, name)
	files := []struct{ path, content string }{
		{path.Join(dir, file+".go"), commandSource(binary, name, sub, newPackage)},
		{path.Join(dir, file+"_test.go"), commandTestSource(binary, name, sub)},
	}
	importFile := path.Join("internal/cmd", binary, "all", "import_"+name+".go")
	if newPackage {
		files = append(files, struct{ path, content string }{importFile, importSource(binary, name)})
	}
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var written []string
	for _, f := range files {
		if err := create(root, f.path, f.content); err != nil {
			for _, w := range written {
				err = errors.Join(err, root.Remove(w))
			}
			if newPackage {
				err = errors.Join(err, root.Remove(dir))
			}
			return err
		}
		written = append(written, f.path)
	}
	for _, w := range written {
		say(e.stdout, "%s\n", w)
	}
	return nil
}

// specSection is the section of the specification that lists the
// commands of the binary.
func specSection(binary string) string {
	if binary == "romeu" {
		return "04 4.2"
	}
	return "04 4.3"
}

// camel returns the words of s, each with its first letter upper case,
// joined: "memory add" gives "MemoryAdd".
func camel(s string) string {
	var b strings.Builder
	for w := range strings.FieldsSeq(s) {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// header is the license header every Go file of the repository opens
// with.
const header = "// SPDX-FileCopyrightText: 2026 Bruno Venceslau\n// SPDX-License-Identifier: GPL-3.0-only\n\n"

// commandSource returns the definition of the command: its package
// comment when it starts the package, and Register with the five
// side-effect fields unset.
func commandSource(binary, name, sub string, newPackage bool) string {
	line := strings.TrimSpace(name + " " + sub)
	use := cmp.Or(sub, name)
	var b strings.Builder
	b.WriteString(header)
	if newPackage {
		fmt.Fprintf(&b, "// Package %s holds the command %s %s (%s).\n", name, binary, name, specSection(binary))
	}
	fmt.Fprintf(&b, `package %[1]s

import (
	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The definition of %[2]s %[3]s. Fill the five side-effect fields of
// Define (12 12.3; 04 4.1, Idempotency and Locking): Register refuses an
// unset one at init and names it.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightUnset, cli.LockUnset, cli.Appending{}, cli.SnapshotUnset, cli.IdempotencyUnset)
	c.Binary, c.Name, c.Sub = %[2]q, %[4]q, %[5]q
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     %[6]q,
			Short:   "fill: what the command does, in one line",
			Example: %[7]q,
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, args []string) error {
				return nil
			},
		}
	}
	return c
}())
`, name, binary, line, name, sub, use, binary+" "+line)
	return b.String()
}

// commandTestSource returns the help test of the command: its help
// exits 0 and opens with its usage line.
func commandTestSource(binary, name, sub string) string {
	line := strings.TrimSpace(name + " " + sub)
	args := fmt.Sprintf("%q", name)
	if sub != "" {
		args += fmt.Sprintf(", %q", sub)
	}
	return header + fmt.Sprintf(`package %[1]s_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/%[2]s/%[1]s"
)

// Test%[3]sHelp is the help test of %[2]s %[4]s (12 12.3): its help
// exits 0 and opens with its usage line.
func Test%[3]sHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: %[2]q, Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr)}
	assert.Equal(t, 0, cli.Main(t.Context(), []string{%[5]s, "--help"}, env))
	assert.Empty(t, stderr.String())
	assert.True(t, strings.HasPrefix(stdout.String(), "Usage: %[2]s %[4]s [flags]\n"), stdout.String())
}
`, name, binary, camel(line), line, args)
}

// importSource returns the one-line blank-import file that links the
// command's package into the binary (02 2.1).
func importSource(binary, name string) string {
	return header + fmt.Sprintf("package all\n\nimport _ \"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/%s/%s\"\n", binary, name)
}
