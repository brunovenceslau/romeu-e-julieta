// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
)

// SbxCalls returns the argv of every call romeu made to the fake sbx, in
// order, without the program name: the [X] assertion of a scenario
// (REQ-0729) reads them.
func (e *Env) SbxCalls(tb testing.TB) [][]string {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(e.Bin), "sbx-state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil // the fake was never called
	}
	if err != nil {
		tb.Fatal(err)
	}
	var st struct {
		Calls [][]string `json:"calls"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		tb.Fatal(err)
	}
	return st.Calls
}

// World returns the Env as the world of a scenario (scenarios.World): a
// command line of the journey runs the romeu binary in the Env, and the
// records are the Env's settings, state and fake sbx.
func (e *Env) World(tb testing.TB) scenarios.World {
	tb.Helper()
	return world{tb: tb, e: e}
}

// world adapts an Env to scenarios.World.
type world struct {
	tb testing.TB
	e  *Env
}

func (w world) Run(line string) (scenarios.Outcome, error) {
	words := strings.Fields(line)
	if len(words) < 2 || words[0] != "romeu" {
		return scenarios.Outcome{}, fmt.Errorf("the CI suite runs romeu lines here, not %q", line)
	}
	res := w.e.Run(w.tb, words[1:])
	return scenarios.Outcome{Stdout: res.Stdout, Stderr: res.Stderr, Code: res.Code}, nil
}

func (w world) Root() string { return w.e.Root }

func (w world) Settings() ([]byte, error) { return os.ReadFile(w.e.Settings()) }

func (w world) State(rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(w.e.StateDir(), filepath.FromSlash(rel)))
}

func (w world) SbxCalls() ([][]string, error) { return w.e.SbxCalls(w.tb), nil }
