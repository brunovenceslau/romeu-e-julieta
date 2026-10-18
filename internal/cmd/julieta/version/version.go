// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package version is the command julieta version (04 4.3): the version
// stamp of the binary, and with --json the sha256 of the binary, its
// GOARCH and whether its own directory is writable, which the
// compatibility check of romeu reads (04 4.1, Version; 06 6.3).
package version

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// executable returns the path of the running binary; a test replaces
// it. A binary that cannot find or read itself is the error internal
// (RJ-348): no row of 04 4.4 names that state, which no healthy install
// reaches.
var executable = osExecutable

// osExecutable is the executable of the process.
var osExecutable = os.Executable

// report is the --json document of julieta version.
type report struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Date        string `json:"date"`
	Go          string `json:"go"`
	Protocol    int    `json:"protocol"`
	SHA256      string `json:"sha256"`
	GOARCH      string `json:"goarch"`
	BinWritable bool   `json:"binWritable"`
}

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name, c.JSON = "julieta", "version", true
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "version",
			Short:   "print the version of julieta",
			Example: "julieta version\njulieta version --json",
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
				path, err := executable()
				if err != nil {
					return err
				}
				sum, err := fileSHA256(runningBinary(path))
				if err != nil {
					return err
				}
				if err := c.Context().Err(); err != nil {
					return err
				}
				doc := report{
					Version: v.Version, Commit: v.Commit, Date: v.Date, Go: runtime.Version(), Protocol: v.Protocol,
					SHA256: sum, GOARCH: runtime.GOARCH, BinWritable: writable(filepath.Dir(path)) || writable(path),
				}
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

// runningBinary returns the file to hash for the binary at path: on
// linux /proc/self/exe, which opens the file the process runs even when
// path has since been replaced; elsewhere path itself.
func runningBinary(path string) string {
	if runtime.GOOS == "linux" {
		return "/proc/self/exe"
	}
	return path
}

// fileSHA256 returns the sha256 of the file at path, in hex.
func fileSHA256(path string) (string, error) {
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

// writable reports whether this process may write path, a directory or
// a file, as access(2) answers for its real user and group: a read-only
// mount answers no whatever the mode bits say, and root, whom the mode
// bits do not stop, answers yes elsewhere, which fails safe. binWritable
// asks it of julieta's directory and of julieta itself, since a file
// its user may write can be rewritten in place in a directory it may
// not (06 6.3). It writes nothing.
func writable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}
