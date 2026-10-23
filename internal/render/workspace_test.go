// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// hostClones are the host clones of two projects, out of order, as host
// state might list them.
var hostClones = []render.Clone{
	{Project: "shop", Dir: "shop-web"},
	{Project: "my-config", Dir: "my-config"},
	{Project: "shop", Dir: "shop-api"},
}

// reviewCheckouts are the review checkouts romeu pull made.
var reviewCheckouts = []render.Clone{
	{Project: "shop", Dir: "shop-api"},
}

// decodeFolders decodes a workspace file, requires its key set to be
// {folders} and returns the folder paths (I13).
func decodeFolders(t *testing.T, data []byte) []string {
	t.Helper()
	var doc map[string]jsontext.Value
	require.NoError(t, jsonv2.Unmarshal(data, &doc))
	require.Equal(t, []string{"folders"}, slices.Collect(maps.Keys(doc)), "a workspace file holds folders alone (I13)")
	var folders []map[string]string
	require.NoError(t, jsonv2.Unmarshal(doc["folders"], &folders))
	var paths []string
	for _, f := range folders {
		require.Equal(t, []string{"path"}, slices.Collect(maps.Keys(f)))
		paths = append(paths, f["path"])
	}
	return paths
}

// The goldens of the workspace files (10 10.1, REQ-0732, I13): each
// decodes to the key set {folders}; no memory dir is a folder of
// either, and a review checkout is a folder of review.code-workspace
// alone.
func TestWorkspaceGolden(t *testing.T) {
	dev, err := render.DevWorkspace(hostClones)
	require.NoError(t, err)
	golden(t, "workspace/"+render.DevWorkspaceFile, dev)
	review, err := render.ReviewWorkspace(reviewCheckouts)
	require.NoError(t, err)
	golden(t, "workspace/"+render.ReviewWorkspaceFile, review)
	empty, err := render.DevWorkspace(nil)
	require.NoError(t, err)
	golden(t, "workspace/empty.code-workspace", empty)

	devPaths := decodeFolders(t, dev)
	assert.Equal(t, []string{"my-config-env/my-config", "shop-env/shop-api", "shop-env/shop-web"}, devPaths)
	reviewPaths := decodeFolders(t, review)
	assert.Equal(t, []string{"shop-env/review/shop-api"}, reviewPaths)
	assert.Empty(t, decodeFolders(t, empty))
	for _, p := range append(devPaths, reviewPaths...) {
		assert.NotContains(t, strings.Split(p, "/"), "memory", "the folder %s", p)
	}
	for _, p := range devPaths {
		assert.NotContains(t, strings.Split(p, "/"), "review", "the folder %s", p)
	}
}

// A clone whose dir is a name romeu reserves, memory and review among
// them, is refused, so neither reaches a workspace file by its dir; so
// is a project name out of its rule and a clone listed twice.
func TestWorkspaceErrors(t *testing.T) {
	tests := []struct {
		name   string
		clones []render.Clone
		want   string
	}{
		{"a memory dir", []render.Clone{{Project: "shop", Dir: "memory"}}, `"memory" is a name romeu reserves`},
		{"a review dir", []render.Clone{{Project: "shop", Dir: "review"}}, `"review" is a name romeu reserves`},
		{"the romeu dir", []render.Clone{{Project: "shop", Dir: ".romeu"}}, `".romeu" is a name romeu reserves`},
		{"a dir of two elements", []render.Clone{{Project: "shop", Dir: "memory/shop-api"}}, "is not one path element"},
		{"a parent dir", []render.Clone{{Project: "shop", Dir: ".."}}, "is not one path element"},
		{"a project name with a slash", []render.Clone{{Project: "a/b", Dir: "x"}}, `"a/b" is not a project name`},
		{"an empty project name", []render.Clone{{Dir: "x"}}, `"" is not a project name`},
		{"a clone listed twice", []render.Clone{{Project: "shop", Dir: "x"}, {Project: "shop", Dir: "x"}}, `lists the dir "x" twice`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render.DevWorkspace(tt.clones)
			require.ErrorContains(t, err, tt.want)
			_, err = render.ReviewWorkspace(tt.clones)
			require.ErrorContains(t, err, tt.want)
		})
	}
}
