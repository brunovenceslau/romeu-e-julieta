// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// configEnv names the file the fake reads its configuration from. The
// scrubbed environment of romeu (04 4.1) carries no such variable, so
// when it is unset the fake reads the file beside the path it was
// started by: <path>.json. A test copies or links the fake to a path of
// its own and writes the file beside it.
const configEnv = "FAKESBX_CONFIG"

// config tells the fake which session to replay and where it keeps its
// state between calls.
type config struct {
	// Session is the absolute path of the session file.
	Session string `json:"session"`
	// State is the absolute path of the file that holds the progress of
	// the replay; the fake creates it, and a lock file beside it.
	State string `json:"state"`
	// GitURL is the git URL of the sandbox daemon the test serves, which
	// the stanza of a create names (<git-url>); it may be empty for a
	// session with no such effect.
	GitURL string `json:"gitUrl"`
	// Replace are pairs applied in order to the stdout and the stderr of
	// an answer: the recording holds fixed example values, the test its
	// own paths and names.
	Replace []replacement `json:"replace"`
	// AllowEnv are the names of the environment contributions the test
	// declares (04 4.1): the child environment may hold these besides
	// PATH, HOME, LANG and TERM.
	AllowEnv []string `json:"allowEnv"`
	// Sentinels are values that must appear in no variable of the child
	// environment: a variable whose value holds one is refused.
	Sentinels []string `json:"sentinels"`
	// Forward is the command that runs, with the words after "--" of an
	// env exec added, in place of the recorded answer (10 10.3, Replay
	// only; the hybrid level forwards into the julieta container).
	Forward []string `json:"forward"`
}

// replacement is one pair of Replace.
type replacement struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// loadConfig reads the configuration for the fake started as argv0.
func loadConfig(argv0 string, environ []string) (config, error) {
	path := argv0 + ".json"
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, configEnv+"="); ok && v != "" {
			path = v
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, fmt.Errorf("the configuration: %w (set %s, or put the file beside the fake)", err, configEnv)
	}
	var c config
	if err := jsonv2.Unmarshal(data, &c); err != nil {
		return config{}, fmt.Errorf("%s: %w", path, err)
	}
	for name, p := range map[string]string{"session": c.Session, "state": c.State} {
		if !filepath.IsAbs(p) {
			return config{}, errors.New(path + ": " + name + " is an absolute path")
		}
	}
	for _, r := range c.Replace {
		if r.From == "" {
			return config{}, errors.New(path + ": a replacement has an empty from")
		}
	}
	if slices.Contains(c.Sentinels, "") {
		return config{}, errors.New(path + ": a sentinel is empty")
	}
	if slices.Contains(c.Forward, "") {
		return config{}, errors.New(path + ": the forward command has an empty word")
	}
	return c, nil
}
