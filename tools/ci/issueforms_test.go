// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// issueForm is an issue form, with every key the two forms use, so a
// misspelled key fails the strict decoding.
type issueForm struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Body        []struct {
		Type       string `yaml:"type"`
		ID         string `yaml:"id"`
		Attributes struct {
			Label       string `yaml:"label"`
			Description string `yaml:"description"`
			Value       string `yaml:"value"`
			Render      string `yaml:"render"`
		} `yaml:"attributes"`
		Validations struct {
			Required bool `yaml:"required"`
		} `yaml:"validations"`
	} `yaml:"body"`
}

// TestIssueForms parses the two issue forms of 12 12.7 (REQ-1180;
// REQ-0127): a bug, which asks for the romeu and sbx versions, the
// command, and its output or the doctor report, and a catalog gap, which
// asks for the tool and its domains, each required. Every key is one an
// issue form has, every field has an id GitHub accepts and a label, the
// ids of a form differ, render is set on a textarea only, and the
// directory holds no other file.
func TestIssueForms(t *testing.T) {
	dir := filepath.Join(citest.ModuleRoot(t), ".github", "ISSUE_TEMPLATE")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"bug.yml", "catalog-gap.yml"}, names)

	want := map[string][]string{
		"bug.yml":         {"romeu-version", "sbx-version", "command", "output"},
		"catalog-gap.yml": {"tool", "domains"},
	}
	for file, required := range want {
		src, err := os.ReadFile(filepath.Join(dir, file))
		require.NoError(t, err)
		var form issueForm
		dec := yaml.NewDecoder(bytes.NewReader(src))
		dec.KnownFields(true)
		require.NoError(t, dec.Decode(&form), "%s: every key is one an issue form has", file)
		assert.NotEmpty(t, form.Name, file)
		assert.NotEmpty(t, form.Description, file)
		var ids, got []string
		for _, f := range form.Body {
			require.Contains(t, []string{"markdown", "input", "textarea"}, f.Type, "%s: a field type of an issue form", file)
			if f.Type == "markdown" {
				assert.NotEmpty(t, f.Attributes.Value, file)
				continue
			}
			assert.NotEmpty(t, f.Attributes.Label, "%s: %s", file, f.ID)
			assert.Regexp(t, `^[A-Za-z0-9_-]+$`, f.ID, "%s: an id GitHub accepts", file)
			assert.True(t, f.Attributes.Render == "" || f.Type == "textarea", "%s: %s: render belongs to a textarea", file, f.ID)
			assert.NotContains(t, ids, f.ID, "%s: ids differ", file)
			ids = append(ids, f.ID)
			if f.Validations.Required {
				got = append(got, f.ID)
			}
		}
		assert.Equal(t, required, got, "%s: the required fields of 12 12.7", file)
	}
}
