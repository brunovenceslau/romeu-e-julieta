// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetupEnv holds the module sources of setup's three steps to
// REQ-0862 (10 10.2): each runs with GOPROXY=https://proxy.golang.org,
// GOSUMDB=sum.golang.org and GOENV=off, whatever the caller exports, so
// a caller's settings cannot fill the module cache unverified. The
// caller below turns the checksum database off in every way the go
// command reads.
func TestSetupEnv(t *testing.T) {
	caller := map[string]string{
		"GOPROXY":    "http://proxy.invalid,direct",
		"GOSUMDB":    "off",
		"GONOSUMDB":  "*",
		"GONOPROXY":  "*",
		"GOPRIVATE":  "*",
		"GOINSECURE": "*",
		"GOVCS":      "*:all",
		"GOFLAGS":    "-insecure -mod=mod",
		"GOENV":      "/from/the/caller",
	}
	for key, value := range caller {
		t.Setenv(key, value)
	}
	root := t.TempDir()
	steps := []struct {
		name    string
		env     []string
		goflags string
	}{
		{"mise trust", trustEnv(root), ""},
		{"mise install", installEnv(root), ""},
		{"go mod download", downloadEnv(t.TempDir()), "-mod=readonly"},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			got := map[string][]string{}
			for _, kv := range s.env {
				key, value, _ := strings.Cut(kv, "=")
				got[key] = append(got[key], value)
			}
			assert.Equal(t, []string{"https://proxy.golang.org"}, got["GOPROXY"])
			assert.Equal(t, []string{"sum.golang.org"}, got["GOSUMDB"])
			assert.Equal(t, []string{"off"}, got["GOENV"])
			for _, key := range []string{"GONOSUMDB", "GONOPROXY", "GOPRIVATE", "GOINSECURE", "GOVCS"} {
				assert.NotContains(t, got, key, "the caller's %s does not reach %s", key, s.name)
			}
			if s.goflags == "" {
				assert.NotContains(t, got, "GOFLAGS", "the caller's GOFLAGS does not reach %s", s.name)
			} else {
				assert.Equal(t, []string{s.goflags}, got["GOFLAGS"], "the caller's GOFLAGS does not reach %s", s.name)
			}
		})
	}
}
