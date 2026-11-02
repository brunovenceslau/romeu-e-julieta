// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package changelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// fixture is a CHANGELOG.md of three tags, newest first, in the form
// git-cliff writes (12 12.6): the static header, which holds no level-2
// heading, then one section per tag, with an indented entry text and a
// level-3 Upgrading part.
const fixture = `# Changelog

One section per tag, newest first.

<!-- git-cliff: end of header -->

## v1.0.0-alpha.3

### Upgrading

- RJ-366 upgrade-declined added, exit 1.

### Features

- romeu upgrade (#3)

  ## not a heading: the entry text is indented

## v1.0.0-alpha.2

### Fixes

- a fix (#2)

## v1.0.0-alpha.1

### Upgrading

The first tag has no previous one.
`

// The sections of the fixture, as Since returns each.
var (
	alpha3 = "## v1.0.0-alpha.3\n\n### Upgrading\n\n- RJ-366 upgrade-declined added, exit 1.\n\n### Features\n\n- romeu upgrade (#3)\n\n  ## not a heading: the entry text is indented"
	alpha2 = "## v1.0.0-alpha.2\n\n### Fixes\n\n- a fix (#2)"
	alpha1 = "## v1.0.0-alpha.1\n\n### Upgrading\n\nThe first tag has no previous one."
)

// TestSince holds REQ-1348 and REQ-1350 on the fixture: the sections
// after the running version up to the new one, newest first; the new
// one's section alone when the running version is not a tag of the file
// (a dev build, REQ-0065) or is not older; nothing when the new version
// has no section.
func TestSince(t *testing.T) {
	for _, tt := range []struct {
		name, running, own, want string
	}{
		{"two versions up", "v1.0.0-alpha.1", "v1.0.0-alpha.3", alpha3 + "\n\n" + alpha2},
		{"one version up", "v1.0.0-alpha.2", "v1.0.0-alpha.3", alpha3},
		{"up to a version that is not the newest", "v1.0.0-alpha.1", "v1.0.0-alpha.2", alpha2},
		{"an older running version the file does not hold", "v0.9.0", "v1.0.0-alpha.1", alpha1},
		{"a dev build runs", "v0.0.0-dev+0123456789abcdef0123456789abcdef01234567", "v1.0.0-alpha.3", alpha3},
		{"the running version is the new one", "v1.0.0-alpha.3", "v1.0.0-alpha.3", alpha3},
		{"a downgrade", "v1.0.0-alpha.3", "v1.0.0-alpha.1", alpha1},
		{"the new version has no section", "v1.0.0-alpha.3", "v0.0.0-dev+abc", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Since(fixture, tt.running, tt.own))
		})
	}
}

// TestFileIsTheChangelog holds the embed: File is CHANGELOG.md at the
// root, its static header first.
func TestFileIsTheChangelog(t *testing.T) {
	assert.Contains(t, File(), "# Changelog\n", "the embedded file is CHANGELOG.md")
	assert.Empty(t, Since(File(), "v0.0.0-dev+abc", "v0.0.0-dev+def"), "a dev build has no section")
}
