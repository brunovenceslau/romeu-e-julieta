// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"regexp"
	"strings"
)

// Kit is a parsed descriptor: the part of it the gate reads.
type Kit struct {
	// DisplayName is the name humans read; it is not an identity.
	DisplayName string
	// Capabilities is the normalized capability set.
	Capabilities Capabilities
}

// ParseDescriptor reads a v3 kit descriptor file by the strict subset
// grammar (REQ-0168, REQ-0569) and returns its normalized capability
// set. It is the one parser of descriptors of the product (REQ-0570),
// with ParseAnnotation, which reads the same grammar from the JSON of an
// image annotation. The grammar is the kit schema of the pinned
// frontend, tools/ci/internal/schema/third_party/sandbox-kit-spec/schema,
// as a Go type: a key it does not have, and a capability type it does
// not model, are refused whatever the origin of the document and whether
// the capability is marked optional (REQ-0068, REQ-0573). A value that
// is a ${{ kit.args.* }} or ${{ kit.env.* }} reference is kept as it is.
//
// The error wraps ErrInvalid.
//
//romeu:invariant I28 guard
func ParseDescriptor(data []byte) (Kit, error) {
	tree, err := parseTree(data)
	if err != nil {
		return Kit{}, err
	}
	return fromTree(tree, true)
}

// ParseAnnotation reads the descriptor an image carries in its manifest
// annotation, compact JSON (the kit schema's own description), by the
// grammar of ParseDescriptor: the same checks of the tree, and the same
// types. JSON is read by encoding/json/v2, so a duplicate member name and
// text that is not UTF-8 are refused, and the escapes of JSON are
// JSON's.
//
//romeu:invariant I28 guard
func ParseAnnotation(data []byte) (Kit, error) {
	tree, err := jsonTree(data)
	if err != nil {
		return Kit{}, err
	}
	return fromTree(tree, false)
}

// fromTree decodes a checked tree into the kit schema's types and
// normalizes its capabilities. localFile says the tree is a file of the
// maintainer's repository.
func fromTree(tree map[string]any, localFile bool) (Kit, error) {
	raw, err := jsonv2.Marshal(tree, jsonv2.Deterministic(true))
	if err != nil {
		return Kit{}, invalid("the descriptor cannot be read as JSON data")
	}
	var d kitDoc
	if err := jsonv2.Unmarshal(raw, &d, jsonv2.RejectUnknownMembers(true)); err != nil {
		return Kit{}, invalid("%s", shapeReason(err))
	}
	if err := d.validate(); err != nil {
		return Kit{}, err
	}
	caps, err := normalize(d, localFile)
	if err != nil {
		return Kit{}, err
	}
	return Kit{DisplayName: d.DisplayName, Capabilities: caps}, nil
}

// shapeReason says why encoding/json/v2 refused a descriptor, with the
// path of the value and no text of it.
func shapeReason(err error) string {
	what := "a value the grammar does not read"
	se, isSemantic := errors.AsType[*jsonv2.SemanticError](err)
	switch {
	case errors.Is(err, jsonv2.ErrUnknownName):
		what = "an unknown key"
	case isSemantic && se.GoType != nil:
		what = "a value of the wrong kind"
	}
	if isSemantic && se.JSONPointer != "" {
		return what + " at " + quote(string(se.JSONPointer))
	}
	return what
}

// kitDoc is the descriptor, member by member as the kit schema has them.
type kitDoc struct {
	SchemaVersion string            `json:"schemaVersion"`
	Kind          string            `json:"kind"`
	DisplayName   string            `json:"displayName"`
	Author        string            `json:"author"`
	Description   string            `json:"description"`
	SourceURL     string            `json:"sourceUrl"`
	IconURL       string            `json:"iconUrl"`
	Version       string            `json:"version"`
	Licenses      []string          `json:"licenses"`
	Provides      []string          `json:"provides"`
	Requires      []string          `json:"requires"`
	Integrates    []string          `json:"integrates"`
	Conflicts     []string          `json:"conflicts"`
	Capabilities  []itemDoc         `json:"capabilities"`
	Args          map[string]argDoc `json:"args"`
	Build         string            `json:"build"`
	Dockerfile    string            `json:"dockerfile"`
	Kits          []kitRefDoc       `json:"kits"`
}

// itemDoc is a capability, or a group of capabilities; a member of a
// group is an itemDoc that sets neither Optional nor Group.
type itemDoc struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Optional    *bool          `json:"optional"`
	Config      jsontext.Value `json:"config"`
	Description string         `json:"description"`
	Source      *sourceDoc     `json:"source"`
	Group       *groupDoc      `json:"group"`
}

// groupDoc is a group of capabilities that share one optional flag.
type groupDoc struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Optional     bool      `json:"optional"`
	Capabilities []itemDoc `json:"capabilities"`
}

// sourceDoc says where the frontend found a capability when it merged a
// kit's descriptors; it is provenance, not a request.
type sourceDoc struct {
	Kit  string `json:"kit"`
	Path string `json:"path"`
}

// argDoc is an installer-supplied value of the kit.
type argDoc struct {
	Default     *string  `json:"default"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Enum        []string `json:"enum"`
	Pattern     *string  `json:"pattern"`
	Env         string   `json:"env"`
	BuildArg    string   `json:"buildArg"`
}

// kitRefDoc is a kit a set lists.
type kitRefDoc struct {
	Ref    string            `json:"ref"`
	Digest string            `json:"digest"`
	Args   map[string]string `json:"args"`
}

var (
	argName     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	typePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?/[a-z0-9]([a-z0-9-]*[a-z0-9])?@[1-9][0-9]*$`)
	digestForm  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// validate checks the rules of the kit schema that its types do not:
// the version, the kind, the exclusions among members and the shape of
// an arg.
func (d *kitDoc) validate() error {
	switch {
	case d.SchemaVersion == "":
		return invalid("the descriptor has no schemaVersion")
	case d.SchemaVersion != "3":
		return invalid("the schemaVersion is not 3")
	case d.Kind == "":
		return invalid("the descriptor has no kind")
	case d.Kind != "workload" && d.Kind != "mixin" && d.Kind != "set":
		return invalid("the kind is not workload, mixin or set")
	case d.IconURL != "" && !strings.HasPrefix(d.IconURL, "https://"):
		return invalid("the iconUrl is not an https URL")
	case d.Kind == "set" && len(d.Kits) == 0:
		return invalid("a set lists no kits")
	case len(d.Kits) > 0 && (d.Build != "" || d.Dockerfile != ""):
		return invalid("kits is exclusive with build and dockerfile")
	case d.Build != "" && d.Dockerfile != "":
		return invalid("build is exclusive with dockerfile")
	}
	for name, a := range d.Args {
		if !argName.MatchString(name) {
			return invalid("the arg %s is not named like a variable", quote(name))
		}
		if err := a.validate(name); err != nil {
			return err
		}
	}
	for i, k := range d.Kits {
		if k.Ref == "" || strings.Contains(k.Ref, "://") || strings.HasPrefix(k.Ref, "git+") {
			return invalid("kits[%d].ref is not a registry reference", i)
		}
		if k.Digest != "" && !digestForm.MatchString(k.Digest) {
			return invalid("kits[%d].digest is not sha256:<64 hex>", i)
		}
	}
	return nil
}

// validate checks one arg: its exclusions and its patterns.
func (a argDoc) validate(name string) error {
	for _, v := range []string{a.Env, a.BuildArg} {
		if v != "" && !argName.MatchString(v) {
			return invalid("the arg %s names a variable that is not one", quote(name))
		}
	}
	switch {
	case a.Env != "" && a.BuildArg != "":
		return invalid("the arg %s sets env and buildArg", quote(name))
	case len(a.Enum) > 0 && a.Pattern != nil:
		return invalid("the arg %s sets enum and pattern", quote(name))
	case a.Default != nil && a.Required:
		return invalid("the arg %s is required and has a default", quote(name))
	}
	if a.Pattern != nil {
		if _, err := regexp.Compile(*a.Pattern); err != nil {
			return invalid("the pattern of the arg %s is not a regular expression", quote(name))
		}
	}
	return nil
}
