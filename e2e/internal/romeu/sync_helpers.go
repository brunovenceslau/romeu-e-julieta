// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// GatePrompt is the question of gate 2, the last text before the
// answer (internal/gate, PromptLine).
const GatePrompt = "approve this widening? [y/N] "

// SeedDescriptor writes the entry of the descriptor cache of host state
// (03 3.8) that romeu sync reads for the workload whose reference has
// the digest workload, on the platform of romeu's own GOARCH, under the
// digest manifest of the platform manifest it stands for: a workload of
// one manifest when manifest is workload, a multi-arch one otherwise.
// The capability set is a workload's with nothing in it. The e2e reads
// no registry, so a sync finds its workload's descriptor here or stops
// at step 6.
func (e *Env) SeedDescriptor(tb testing.TB, workload, manifest string) {
	tb.Helper()
	hex, ok := strings.CutPrefix(manifest, "sha256:")
	if !ok {
		tb.Fatalf("the manifest digest %q is not a sha256 digest", manifest)
	}
	dir := filepath.Join(e.StateDir(), "descriptors")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		tb.Fatal(err)
	}
	data := `{"schema":"state-descriptor/v1","digest":"` + manifest + `","workload":"` + workload +
		`","platform":"linux/` + runtime.GOARCH + `","capabilities":{"kind":"workload"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, hex+".json"), []byte(data), 0o600); err != nil {
		tb.Fatal(err)
	}
}

// AppendSettings appends the YAML members text, keys that the host
// settings do not hold yet, to the settings of the Env, such as the
// gitHosts and the workloadRepositories a sync of the origins needs
// (03 3.4), and decodes the result by the rules of 03 3.4, so a test
// never goes on with settings romeu would refuse.
func (e *Env) AppendSettings(tb testing.TB, text string) {
	tb.Helper()
	data, err := os.ReadFile(e.Settings())
	if err != nil {
		tb.Fatal(err)
	}
	data = append(data, text...)
	if _, err := all.DecodeHostSettings(data); err != nil {
		tb.Fatalf("the settings the test wrote are invalid: %v", err)
	}
	if err := os.WriteFile(e.Settings(), data, 0o600); err != nil {
		tb.Fatal(err)
	}
}

// Node is one node of a tree as Snapshot reads it: its mode, and the
// content of a file or the target of a link.
type Node struct {
	Mode fs.FileMode
	Data string
}

// Snapshot returns every node below dir by its path relative to dir,
// links not followed: what the idempotency rule of 04 4.1 compares, the
// content and the mode of every file, and every ref of a clone, which
// is a file below its .git or a line of its packed-refs.
func Snapshot(tb testing.TB, dir string) map[string]Node {
	tb.Helper()
	out := map[string]Node{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		n := Node{Mode: info.Mode()}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			n.Data, err = os.Readlink(p)
		case info.Mode().IsRegular():
			var b []byte
			b, err = os.ReadFile(p)
			n.Data = string(b)
		}
		out[filepath.ToSlash(rel)] = n
		return err
	})
	if err != nil {
		tb.Fatal(err)
	}
	return out
}
