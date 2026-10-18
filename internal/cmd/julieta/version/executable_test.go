// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package version

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// TestVersionUnreadableBinary prints the error internal (RJ-348), exit
// 1, when the binary cannot find itself; the text form does not look
// and still prints. On linux the hash reads /proc/self/exe, which a
// test cannot make unreadable.
func TestVersionUnreadableBinary(t *testing.T) {
	t.Cleanup(func() { executable = osExecutable })
	for name, exe := range map[string]func() (string, error){
		"not found": func() (string, error) { return "", errors.New("no executable") },
	} {
		executable = exe
		var stdout, stderr bytes.Buffer
		env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr)}
		assert.Equal(t, 1, cli.Main(t.Context(), []string{"version", "--json"}, env), name)
		assert.Empty(t, stdout.String(), name)
		assert.True(t, strings.HasPrefix(stderr.String(), `{"id":"RJ-348","slug":"internal","exit":1,`), "%s: %s", name, stderr.String())
		stdout.Reset()
		assert.Equal(t, 0, cli.Main(t.Context(), []string{"version"}, env), name)
		assert.Contains(t, stdout.String(), "protocol  0\n", name)
	}
}
