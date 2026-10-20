// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	memwrite "github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// AgentMemoryWritable is the warning agent-memory-writable (04 4.1,
// Warnings): the agent's own memory dir is writable or not empty, so
// the agent may remember outside julieta memory (08 8.1, REQ-0658).
const AgentMemoryWritable = "agent-memory-writable"

// checkMemory is the step memory check and the agent memory dir check
// of setup (04 4.3, REQ-0400). In the memory dir of every repo of the
// manifest it first removes julieta's temporary files older than this
// run's start, or of a time after now, which no julieta wrote yet, and
// keeps the others (REQ-0654), then checks that the dir
// is mounted, writable and in the layout of 08 8.1 (REQ-0412): a dir
// that fails leaves the sandbox unusable, so setup exits 1. Then it
// warns agent-memory-writable when the agent's own memory dir is
// writable or not empty.
func checkMemory(_ context.Context, r *run) {
	for _, repo := range r.m.Repos {
		if err := checkMemoryDir(r, repo.Memory); err != nil {
			r.fail("memory check of %s: %s", termsafe.Escape(repo.Dir), termsafe.Escape(err.Error()))
		}
	}
	if dir, ok := agentMemoryDir(r.home, r.m.Agent); ok {
		if why := agentMemoryOpen(dir); why != "" {
			r.warn(AgentMemoryWritable, fmt.Sprintf("%s %s", termsafe.Escape(dir), termsafe.Escape(why)))
		}
	}
}

// checkMemoryDir removes the old temporary files of the memory dir dir
// and checks it, as checkMemory says.
func checkMemoryDir(r *run, dir string) error {
	if err := memstore.CheckMounted(dir); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if _, _, err := memwrite.RemoveTemps(root, r.start, r.now()); err != nil {
		return err
	}
	rep, err := memstore.Check(root)
	if err != nil {
		return err
	}
	if len(rep.Findings) > 0 {
		names := make([]string, len(rep.Findings))
		for i, f := range rep.Findings {
			names[i] = f.String()
		}
		return fmt.Errorf("%s breaks the layout of 08 8.1: %s", termsafe.Escape(dir), strings.Join(names, ", "))
	}
	return nil
}

// agentMemoryDir returns the agent's own memory dir below home for the
// agent profile the manifest names, and false when this build knows
// none. The location is a field of the agent profile, which lands with
// internal/agent and the julieta-claude kit (T079) after probe A12
// records it (11 11.1); until then no profile names a dir and the
// check does not run.
var agentMemoryDir = func(home, agent string) (string, bool) { return "", false }

// agentMemoryOpen returns why the agent's memory dir dir lets the agent
// remember outside julieta: it is writable, or it holds an entry; ""
// when it is absent, or present, read-only and empty.
func agentMemoryOpen(dir string) string {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		return "cannot be read: " + err.Error()
	}
	if !info.IsDir() {
		return "is not a directory"
	}
	if unix.Access(dir, unix.W_OK) == nil {
		return "is writable"
	}
	f, err := os.Open(dir)
	if err != nil {
		return "cannot be read: " + err.Error()
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Readdirnames(1); !errors.Is(err, io.EOF) {
		return "is not empty"
	}
	return ""
}
