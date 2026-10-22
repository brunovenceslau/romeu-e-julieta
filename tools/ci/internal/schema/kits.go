// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The v3 kit schema of sbx, copied from docker/sandbox-kit-spec at the
// tag kitSpecTag (third_party/sandbox-kit-spec/README.md): the tag, where
// the copy's files are in the tree, and the URL they are published at,
// which the descriptors' modeline names. The check reads the copy from
// the commit it judges, as it reads every other file.
const (
	kitSpecTag  = "v3.0.0-m.8"
	kitSpecRoot = "tools/ci/internal/schema/third_party/sandbox-kit-spec/schema"
	kitSpecURL  = "https://raw.githubusercontent.com/docker/sandbox-kit-spec/" + kitSpecTag + "/schema/"
	kitSchema   = "kit.schema.json"
	// kitName names the kit schema in a finding.
	kitName = kitSchema + " at " + kitSpecTag
)

// The two lines that open a v3 kit descriptor: the frontend pin (06 6.1)
// and, under examples/, the modeline of the official kit schema at the
// tag the check validates against.
const (
	kitSyntaxPrefix = "# syntax=docker/sandbox-kit:"
	kitModeline     = hintPrefix + kitSpecURL + kitSchema
)

// compileKit compiles the kit schema from the copy's files in the tree
// files, each added at the URL it is published at, so its relative
// references resolve to the copies and no other location is read. A
// tree without the copy has none: nil and no error.
func compileKit(files []check.TreeFile) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{})
	c.AssertFormat()
	found := false
	for _, f := range files {
		name, ok := strings.CutPrefix(f.Path, kitSpecRoot+"/")
		if !ok || path.Ext(name) != ".json" {
			continue
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(f.Content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Path, err)
		}
		if err := c.AddResource(kitSpecURL+name, doc); err != nil {
			return nil, err
		}
		found = true
	}
	if !found {
		return nil, nil
	}
	return c.Compile(kitSpecURL + kitSchema)
}

// isKitDescriptor reports whether f is a v3 kit descriptor the check
// judges: a YAML file under examples/ whose first line is the frontend
// pin. The product kits under kits/ are
// not judged yet: kits/git-ssh-sign, composed on sbx in sitting M0,
// does not validate against the schema at kitSpecTag (its ssh-agent
// capability has no phase), which the task that owns the kit decides.
func isKitDescriptor(f check.TreeFile) bool {
	ext := path.Ext(f.Path)
	if ext != ".yaml" && ext != ".yml" || !strings.HasPrefix(f.Path, examplesDir) {
		return false
	}
	return bytes.HasPrefix(f.Content, []byte(kitSyntaxPrefix))
}

// kitFindings judges one kit descriptor under examples/: its second
// line is the modeline of the kit schema at kitSpecTag, and it validates
// against that schema.
func kitFindings(f check.TreeFile, sch *jsonschema.Schema) []check.Finding {
	if sch == nil {
		return []check.Finding{{Where: f.Path, Rule: ruleUnknown, Msg: "is a v3 kit descriptor, and the tree holds no copy of the kit schema at " + kitSpecRoot}}
	}
	lines := strings.SplitN(string(f.Content), "\n", 3)
	if len(lines) < 3 || strings.TrimSuffix(lines[1], "\r") != kitModeline {
		return []check.Finding{{Where: f.Path, Rule: ruleHint, Msg: "is a v3 kit descriptor, and its second line is not \"" + kitModeline + "\" (03 3.2)"}}
	}
	doc, err := parse(f)
	if err != nil {
		return []check.Finding{{Where: f.Path, Rule: ruleParse, Msg: err.Error()}}
	}
	return validate(f.Path, kitName, doc, compiled{kitName: sch})
}

// errNoKitSchema is the error of a kit schema that does not compile.
var errNoKitSchema = errors.New("the vendored kit schema does not compile")
