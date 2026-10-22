// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import "github.com/brunovenceslau/romeu-e-julieta/internal/spec"

// The formats of internal/spec, the project spec and host settings (03
// 3.2, 3.4), and what their generated schemas and reference pages are
// made from: the rules of rules.go and the classification of the
// project's fields. They are here so that tools/ci, which reaches
// internal/ through this package (REQ-0971), registers the schemas and
// writes the pages; julieta spec validate links internal/spec in any
// case.
type (
	// SpecProject is the type of the format project.v1.
	SpecProject = spec.Project
	// SpecHostSettings is the type of the format host-settings.v1.
	SpecHostSettings = spec.HostSettings
	// SpecRule is one validation rule.
	SpecRule = spec.Rule
	// SpecFormat names a format of internal/spec.
	SpecFormat = spec.Format
	// SpecFieldClass is the gate and apply class of a project field.
	SpecFieldClass = spec.FieldClass
	// SpecSchemaKeyword is a keyword a format's schema carries.
	SpecSchemaKeyword = spec.SchemaKeyword
)

// The formats of internal/spec.
const (
	SpecFormatProject      = spec.FormatProject
	SpecFormatHostSettings = spec.FormatHostSettings
)

// What the generators of tools/ci read from internal/spec: the rules,
// the committed list of the ungated project fields, the gate and apply
// class of each project field (which fails when the tags and the list
// disagree, I7), and the keywords of a format's schema.
var (
	SpecRules          = spec.Rules
	SpecUngated        = spec.Ungated
	SpecClassify       = spec.Classify
	SpecSchemaKeywords = spec.SchemaKeywords
)
