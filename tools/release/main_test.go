// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no_command", nil},
		{"another_command", []string{"publish"}},
		{"no_version", []string{"build", "--dry-run"}},
		{"extra_argument", []string{"build", "--version", "v1.0.0", "extra"}},
		{"unknown_flag", []string{"build", "--version", "v1.0.0", "--now"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.Equal(t, exitUsage, run(t.Context(), tt.args, &stdout, &stderr))
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), "usage:")
		})
	}
}
