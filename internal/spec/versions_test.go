// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// idOf returns the id of the error of 04 4.4 that err is, or "".
func idOf(err error) string {
	if e, ok := errors.AsType[*cli.Error](err); ok {
		return e.ID
	}
	return ""
}

// TestAccept covers the one reader of the schema field for each format
// the package decodes (03, opening): its own version is read, a newer
// one is format-newer (RJ-311) with the hint of REQ-0163, another form
// or another format is invalid. Each format is also read at a version
// 3, so that the window of REQ-0167 shows: 2 is read, 1 is
// format-unsupported (RJ-337).
func TestAccept(t *testing.T) {
	for _, f := range []format{project, hostSettings} {
		bumped := newFormat(f.name, 3)
		bumped.readsPrevious = true
		noReader := newFormat(f.name, 3)
		cases := []struct {
			f     format
			field string
			id    string
			ok    bool
		}{
			{f, f.name + "/v1", "", true},
			{f, f.name + "/v2", "RJ-311", false},
			{f, f.name + "/v10", "RJ-311", false},
			{f, f.name + "/v0", "", false},
			{f, f.name + "/v01", "", false},
			{f, f.name + "/1", "", false},
			{f, "other/v1", "", false},
			{f, f.name + "/v1 ", "", false},
			{bumped, f.name + "/v3", "", true},
			{bumped, f.name + "/v2", "", true},
			{bumped, f.name + "/v1", "RJ-337", false},
			{bumped, f.name + "/v4", "RJ-311", false},
			{noReader, f.name + "/v2", "RJ-337", false},
		}
		for _, c := range cases {
			t.Run(c.f.Field()+" reads "+c.field, func(t *testing.T) {
				err := c.f.accept(c.field)
				if c.ok {
					assert.NoError(t, err)
					return
				}
				require.Error(t, err)
				assert.Equal(t, c.id, idOf(err))
				if c.id == "" {
					assert.ErrorIs(t, err, ErrInvalid)
				}
			})
		}
	}
	assert.Equal(t, "decide: install the release that wrote it", cli.Of(project.accept("project/v2")).Fix)
	assert.Equal(t, "decide: apply the edit in the release notes of each skipped release", FormatUnsupported.Fix)
}

// TestDecodeReadsTheVersionFirst returns format-newer for a newer file
// though it holds keys, or a grammar, that this build does not read.
func TestDecodeReadsTheVersionFirst(t *testing.T) {
	for name, data := range map[string]string{
		"a new key":      "schema: project/v2\nname: shop\nnewKey: 1\n",
		"a new grammar":  "schema: project/v2\nname: &a shop\nalias: *a\n",
		"does not parse": "schema: project/v2\nname: [\n",
		"host settings":  "schema: host-settings/v2\nroot: /x\nmore: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			var err error
			if name == "host settings" {
				_, err = DecodeHostSettings([]byte(data))
			} else {
				_, err = DecodeProject([]byte(data))
			}
			assert.Equal(t, "RJ-311", idOf(err))
		})
	}
}

// TestEveryBumpReadsThePrevious fails a format whose version passes 1
// without the reader of the version before it (REQ-0164).
func TestEveryBumpReadsThePrevious(t *testing.T) {
	for _, f := range formats {
		if f.version > 1 {
			assert.True(t, f.readsPrevious, "%s is at version %d and reads no version %d", f.name, f.version, f.version-1)
		}
	}
	assert.Len(t, formats, 2)
}
