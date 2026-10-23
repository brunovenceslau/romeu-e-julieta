// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package version is the command romeu version (04 4.2): the version
// stamp of the binary, and with --json the sha256 of the binary and its
// GOARCH (04 4.1, Version).
package version

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// executable returns the path of the running binary; a test replaces
// it. A binary that cannot find or read itself is the error internal
// (RJ-348): no row of 04 4.4 names that state, which no healthy install
// reaches.
var executable = osExecutable

// osExecutable is the executable of the process.
var osExecutable = os.Executable

// report is the --json document of romeu version.
type report struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Date     string `json:"date"`
	Go       string `json:"go"`
	Protocol int    `json:"protocol"`
	SHA256   string `json:"sha256"`
	GOARCH   string `json:"goarch"`
}

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "romeu", "version", true
	c.NoJourney = "no step of a journey runs it; its own tests assert its output"
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "version",
			Short:   "print the version of romeu",
			Example: "romeu version\nromeu version --json",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				if err := c.Context().Err(); err != nil {
					return err
				}
				v := env.Version
				if !env.JSON {
					_, err := fmt.Fprintf(env.Stdout, "version   %s\ncommit    %s\ndate      %s\ngo        %s\nprotocol  %d\n",
						v.Version, v.Commit, v.Date, runtime.Version(), v.Protocol)
					return err
				}
				sum, err := executableSHA256()
				if err != nil {
					return err
				}
				if err := c.Context().Err(); err != nil {
					return err
				}
				doc := report{Version: v.Version, Commit: v.Commit, Date: v.Date, Go: runtime.Version(), Protocol: v.Protocol, SHA256: sum, GOARCH: runtime.GOARCH}
				b, err := json.Marshal(doc, json.Deterministic(true))
				if err != nil {
					return err
				}
				_, err = env.Stdout.Write(append(b, '\n'))
				return err
			},
		}
	}
	return c
}())

// executableSHA256 returns the sha256 of the file of the running binary,
// in hex.
func executableSHA256() (string, error) {
	path, err := executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
