// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// TestSyntheticEntry reads the real denylist and the plaintext of its
// synthetic entry from testdata (10 10.2, REQ-0898): the matcher finds
// the name written together and with each separator it allows, and not
// across a slash, so a change to the matcher that stops the real
// entries matching fails here, and so does removing the entry.
func TestSyntheticEntry(t *testing.T) {
	l, err := names.Load(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(names.Path)))
	require.NoError(t, err)
	src, err := os.ReadFile(filepath.Join("testdata", "synthetic-name.txt"))
	require.NoError(t, err)
	var segments []string
	for line := range strings.Lines(string(src)) {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			segments = append(segments, line)
		}
	}
	require.Len(t, segments, 2, "the made-up name has two segments")
	for _, line := range segments {
		assert.False(t, l.Match([]byte(line)), "one segment alone, as the testdata file holds it, is no hit")
	}
	for _, sep := range []string{"", " ", "-", "_", ".", "  ", "\t"} {
		joined := strings.Join(segments, sep)
		assert.True(t, l.Match([]byte("see "+joined+" here")), "%q", sep)
		assert.True(t, l.Match([]byte(strings.ToUpper(joined))), "%q in upper case", sep)
	}
	assert.False(t, l.Match([]byte(strings.Join(segments, "/"))), "a slash between the segments is no hit")

	// The hit comes from the synthetic entry alone: without the entry of
	// the same segment lengths, the real file matches nothing here.
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(names.Path)))
	require.NoError(t, err)
	var file struct {
		NeverTracked []string `yaml:"neverTracked"`
		Entries      []struct {
			Segments []struct {
				Length int    `yaml:"length"`
				SHA256 string `yaml:"sha256"`
			} `yaml:"segments"`
		} `yaml:"entries"`
	}
	require.NoError(t, yaml.Unmarshal(data, &file))
	kept := file.Entries[:0]
	for _, e := range file.Entries {
		if len(e.Segments) == 2 && e.Segments[0].Length == len(segments[0]) && e.Segments[1].Length == len(segments[1]) {
			continue
		}
		kept = append(kept, e)
	}
	require.Len(t, kept, len(file.Entries)-1, "one entry is the synthetic one")
	file.Entries = kept
	without, err := yaml.Marshal(file)
	require.NoError(t, err)
	others, err := names.Parse(without)
	require.NoError(t, err)
	assert.False(t, others.Match([]byte(strings.Join(segments, " "))), "no other entry matches the made-up name")
}
