// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// This file is on the gates surface of .github/ask-first.yaml: the gate
// and apply tags below, and the list of the fields left ungated, define
// the widening set of gate 2 (01 1.4, REQ-0067).
//
// A top-level field of Project carries gate:"widening" when a change to
// it widens what the sandbox may reach or read, so gate 2 asks for it,
// and apply:"recreate" or apply:"live" for when a change takes effect. A
// member of a top-level field has the class of that field.

// Project is a project spec, projects/<name>.yaml, of the format
// project.v1 (03 3.2).
type Project struct {
	// Schema is "project/v1" (REQ-0161).
	Schema string `json:"schema" yaml:"schema"`
	// Name is the project's name; it equals the file stem (REQ-0133).
	Name string `json:"name" yaml:"name" gate:"widening" apply:"recreate"`
	// Repos are the project's repositories; exactly one is the primary.
	Repos []Repo `json:"repos" yaml:"repos" gate:"widening" apply:"recreate"`
	// Workload is the workload image, pinned by digest.
	Workload string `json:"workload" yaml:"workload" gate:"widening" apply:"recreate"`
	// Agent is the v3 agent handle the workload declares, never a v2
	// built-in agent.
	Agent string `json:"agent" yaml:"agent" gate:"widening" apply:"recreate"`
	// Kits are the kits of the sandbox, in order: product kits come from
	// the romeu build, personal kits from the config repo's kits/
	// directory at the spec commit.
	Kits []Kit `json:"kits,omitzero" yaml:"kits,omitempty" gate:"widening" apply:"recreate"`
	// SandboxOptions are the resource limits of the sandbox.
	SandboxOptions *SandboxOptions `json:"sandboxOptions,omitzero" yaml:"sandboxOptions,omitempty" apply:"recreate"`
	// Secrets are the names of the secrets the sandbox gets; each
	// resolves through the host settings entry <name>@<project>.
	Secrets []string `json:"secrets,omitzero" yaml:"secrets,omitempty" gate:"widening" apply:"recreate"`
	// Memory names the private repository that keeps the durable copy
	// of the project's memory (08 8.1). It is ungated: the remote is a
	// destination of the host's push that the sandbox never reaches, so
	// it changes nothing the sandbox may reach or read; it applies live,
	// at the next command's push. sync refuses a spec without it
	// (CheckMemoryRemote), which the general validation does not.
	Memory *Memory `json:"memory,omitzero" yaml:"memory,omitempty" apply:"live"`
	// Egress holds the domains the spec adds to the derived egress; each
	// is always gated.
	Egress *Egress `json:"egress,omitzero" yaml:"egress,omitempty" gate:"widening" apply:"live"`
	// Ports are the published ports, such as "8080:8080/tcp".
	Ports []string `json:"ports,omitzero" yaml:"ports,omitempty" gate:"widening" apply:"recreate"`
	// Salvage holds the salvage settings.
	Salvage *Salvage `json:"salvage,omitzero" yaml:"salvage,omitempty" gate:"widening" apply:"live"`
	// Run is the run layout: the tabs and panes julieta lays out.
	Run *Run `json:"run,omitzero" yaml:"run,omitempty" apply:"live"`
}

// ungated is the committed list of the members of Project, by path, that
// carry no gate:"widening" tag (01 1.4, I7): the three top-level fields
// left ungated and every member below them. Resource limits change how
// much of the host the sandbox uses, not what it can reach or read; the
// memory remote is where the host pushes, which the sandbox never sees;
// and the run layout's pane commands run only inside the sandbox. Classify
// fails when the reflected members differ from it, so a new field, at
// any depth of an ungated field, is never left out of the gate by
// omission.
var ungated = []string{
	"sandboxOptions", "sandboxOptions.cpus", "sandboxOptions.memory",
	"memory", "memory.remote",
	"run", "run.tabs", "run.tabs[].label", "run.tabs[].root",
	"run.tabs[].root.pane", "run.tabs[].root.split", "run.tabs[].root.ratio",
	"run.tabs[].root.first", "run.tabs[].root.second",
	"run.panes", "run.panes[].id", "run.panes[].repo", "run.panes[].cwd",
	"run.panes[].env", "run.panes[].steps", "run.panes[].focus",
}

// Ungated returns the committed list of the ungated members of Project,
// a copy.
func Ungated() []string { return slices.Clone(ungated) }

// Repo is one repository of a project.
type Repo struct {
	// URL is the repository's origin, https://<host>/<owner>/<repo>.
	URL string `json:"url" yaml:"url"`
	// Primary marks the primary repository, the sandbox's workspace.
	Primary bool `json:"primary,omitzero" yaml:"primary,omitempty"`
	// Dir is the repository's directory in the project; DecodeProject
	// and Validate take the URL's base name without .git when it is left
	// out.
	Dir string `json:"dir,omitzero" yaml:"dir,omitempty"`
	// Ref is the branch egress derives from; left out, the origin's HEAD.
	Ref string `json:"ref,omitzero" yaml:"ref,omitempty"`
}

// Kit is one kit of a project: exactly one of Product and Personal.
type Kit struct {
	// Product names a product kit of the romeu build.
	Product string `json:"product,omitzero" yaml:"product,omitempty"`
	// Personal names a kit under the config repo's kits/ directory.
	Personal string `json:"personal,omitzero" yaml:"personal,omitempty"`
	// Args are the kit's arguments.
	Args map[string]string `json:"args,omitzero" yaml:"args,omitempty"`
}

// SandboxOptions are the resource limits of a sandbox; sbx reads them.
type SandboxOptions struct {
	// CPUs is the number of CPUs.
	CPUs int `json:"cpus,omitzero" yaml:"cpus,omitempty"`
	// Memory is the memory limit, such as "8g".
	Memory string `json:"memory,omitzero" yaml:"memory,omitempty"`
}

// Egress holds the domains a spec adds.
type Egress struct {
	// Extra are domains the sandbox may reach.
	Extra []string `json:"extra,omitzero" yaml:"extra,omitempty"`
	// Tools maps a mise backend:tool the catalog lacks to the domains it
	// needs.
	Tools map[string][]string `json:"tools,omitzero" yaml:"tools,omitempty"`
}

// Salvage holds the salvage settings of a project.
type Salvage struct {
	// ExcludeIgnored are the ignored paths a salvage leaves out, an
	// acknowledged loss.
	ExcludeIgnored []string `json:"excludeIgnored,omitzero" yaml:"excludeIgnored,omitempty"`
}

// The values of the gate and apply tags.
const (
	// GateWidening marks a field whose change gate 2 asks for.
	GateWidening = "widening"
	// ApplyRecreate marks a field whose change takes effect at the next
	// create of the sandbox.
	ApplyRecreate = "recreate"
	// ApplyLive marks a field whose change takes effect on the running
	// sandbox.
	ApplyLive = "live"
)

// FieldClass is the classification of one top-level field of Project.
type FieldClass struct {
	// Name is the field's JSON name.
	Name string
	// Gate is GateWidening, or "" for an ungated field.
	Gate string
	// Apply is ApplyRecreate or ApplyLive.
	Apply string
}

// Classify returns the classification of each top-level field of
// Project but schema, the version field every format has, in the order
// of the type. It fails when a field carries an unknown gate or apply
// value or no apply tag, or when the members of the fields without
// gate:"widening", at every depth, differ from the committed list (I7);
// the generators of the widening set and of the reference page call it,
// so they fail too. A member of a gated field has that field's class.
//
//romeu:invariant I7 guard
func Classify() ([]FieldClass, error) {
	return classify(reflect.TypeFor[Project](), ungated)
}

// classify is Classify for the struct type t and the list ungated.
func classify(t reflect.Type, ungated []string) ([]FieldClass, error) {
	var out []FieldClass
	var untagged []string
	for f := range t.Fields() {
		name := jsonName(f)
		if name == "schema" {
			continue
		}
		gate, apply := f.Tag.Get("gate"), f.Tag.Get("apply")
		if gate != "" && gate != GateWidening {
			return nil, fmt.Errorf("spec: the field %s has the gate tag %q; the one value is %q", name, gate, GateWidening)
		}
		if apply != ApplyRecreate && apply != ApplyLive {
			return nil, fmt.Errorf("spec: the field %s has the apply tag %q; it is %q or %q", name, apply, ApplyRecreate, ApplyLive)
		}
		if gate == "" {
			untagged = append(untagged, name)
			untagged = append(untagged, memberPaths(f.Type, name, map[reflect.Type]bool{})...)
		}
		out = append(out, FieldClass{Name: name, Gate: gate, Apply: apply})
	}
	got, want := slices.Sorted(slices.Values(untagged)), slices.Sorted(slices.Values(ungated))
	if !slices.Equal(got, want) {
		return nil, fmt.Errorf("spec: the members without a gate:%q tag are [%s], and the committed list holds [%s]; tag the field or add the member to the list (I7)",
			GateWidening, strings.Join(got, " "), strings.Join(want, " "))
	}
	return out, nil
}

// memberPaths returns the path of every member below a value of type t
// at the path at, in the syntax of Rule.Field; a struct type already on
// the path (a tree's node) is not entered again.
func memberPaths(t reflect.Type, at string, seen map[reflect.Type]bool) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return memberPaths(t.Elem(), at+"[]", seen)
	case reflect.Map:
		return memberPaths(t.Elem(), at+"{}", seen)
	case reflect.Struct:
		if seen[t] {
			return nil
		}
		seen[t] = true
		defer delete(seen, t)
		var out []string
		for f := range t.Fields() {
			p := at + "." + jsonName(f)
			out = append(out, p)
			out = append(out, memberPaths(f.Type, p, seen)...)
		}
		return out
	}
	return nil
}

// jsonName returns the JSON name of the struct field f.
func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "" {
		return f.Name
	}
	return name
}
