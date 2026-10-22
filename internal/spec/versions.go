// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"fmt"
	"regexp"
	"strconv"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// The versions of the formats of this package (03, opening). This file
// is on the contracts surface of .github/ask-first.yaml: a change to a
// number changes which files romeu and julieta read.
const (
	// ProjectVersion is the n of the schema field "project/v<n>" this
	// build reads and writes.
	ProjectVersion = 1
	// HostSettingsVersion is the n of "host-settings/v<n>".
	HostSettingsVersion = 1
)

// format is one format of this package: its schema name and the version
// this build reads. Both are user-authored formats, which change only by
// a new version (REQ-0166), and a reader keeps a window of the current
// and the previous version (REQ-0167).
type format struct {
	name    string
	version int
	// readsPrevious says this build has the reader of version-1 beside
	// the current one (REQ-0164); the release that bumps the version
	// sets it with that reader, and TestEveryBumpReadsThePrevious fails
	// a bump without one.
	readsPrevious bool
	// field reads the schema field "<name>/v<n>".
	field *regexp.Regexp
}

// newFormat returns the format name at version.
func newFormat(name string, version int) format {
	return format{name: name, version: version,
		field: regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `/v([1-9][0-9]{0,8})$`)}
}

// The formats of this package.
var (
	project      = newFormat("project", ProjectVersion)
	hostSettings = newFormat("host-settings", HostSettingsVersion)
	// formats are the formats this package decodes.
	formats = []format{project, hostSettings}
)

// Field returns the schema field of the version this build reads,
// "<name>/v<n>" (REQ-0161).
func (f format) Field() string { return f.name + "/v" + strconv.Itoa(f.version) }

// ProjectField is the schema field of a project spec this build writes.
var ProjectField = project.Field()

// HostSettingsField is the schema field of host settings this build
// writes.
var HostSettingsField = hostSettings.Field()

// versionOf reads the n of a schema field "<name>/v<n>" of f, n without
// a leading zero, and whether the field has that form.
func (f format) versionOf(field string) (int, bool) {
	sub := f.field.FindStringSubmatch(field)
	if sub == nil {
		return 0, false
	}
	n, err := strconv.Atoi(sub[1])
	return n, err == nil
}

// accept is the one reader of a schema field (03, opening): a version
// newer than this build's is format-newer (RJ-311) with the hint
// "install the release that wrote it" (REQ-0163); one older than the
// previous version is format-unsupported (RJ-337, REQ-0167); a field of
// another form or format wraps ErrInvalid. The current and the previous
// version are accepted, the previous one only when its reader is there
// (readsPrevious), so a bump without one fails closed (REQ-0164).
func (f format) accept(field string) error {
	n, ok := f.versionOf(field)
	switch {
	case !ok:
		return fmt.Errorf("%w: the schema field %q is not %s/v<n>", ErrInvalid, clip(field), f.name)
	case n > f.version:
		return manifest.FormatNewer.New(fmt.Sprintf("the file is %s, newer than %s, the version this build reads", field, f.Field()))
	case n < f.version-1, n == f.version-1 && !f.readsPrevious:
		return FormatUnsupported.New(fmt.Sprintf("the file is %s, and this build reads %s and the version before it", field, f.Field()))
	}
	return nil
}

// checkVersion reads the schema key of the mapping root, the top of a
// file of f, and applies accept to it.
func (f format) checkVersion(root *yaml.Node) error {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "schema" {
			return f.accept(root.Content[i+1].Value)
		}
	}
	return fmt.Errorf("%w: the file has no key schema", ErrInvalid)
}

// newerLine finds a schema line in the text of a file that does not
// parse, so that a newer version is format-newer though this build
// cannot read the rest: the value plain or quoted, a comment after it,
// and a CR before the line's end allowed.
var newerLine = regexp.MustCompile(`(?m)^schema:[ \t]*["']?([a-z-]+/v[1-9][0-9]{0,8})["']?[ \t]*(?:#[^\r\n]*)?\r?$`)

// newer returns format-newer when the text of data holds a schema line
// of f naming a version newer than this build's, and nil otherwise.
func (f format) newer(data []byte) error {
	sub := newerLine.FindSubmatch(data)
	if sub == nil {
		return nil
	}
	if n, ok := f.versionOf(string(sub[1])); ok && n > f.version {
		return f.accept(string(sub[1]))
	}
	return nil
}
