// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import "strconv"

// The versions of the host state formats (03 3.8, REQ-0161): the n of
// each schema field "state-<name>/v<n>" this build reads and writes.
// Adding a member keeps the version; removing or renaming one, or
// changing its type or meaning, is a new version (REQ-0163, ADR 0041).
const (
	// ToolchainVersion is the version of state-toolchain.
	ToolchainVersion = 1
	// ProjectVersion is the version of state-project.
	ProjectVersion = 1
	// DescriptorVersion is the version of state-descriptor.
	DescriptorVersion = 1
)

// The schema names of the formats (02 2.1).
const (
	toolchainFormat  = "state-toolchain"
	projectFormat    = "state-project"
	descriptorFormat = "state-descriptor"
)

// The schema fields this build writes.
var (
	ToolchainField  = toolchainFormat + "/v" + strconv.Itoa(ToolchainVersion)
	ProjectField    = projectFormat + "/v" + strconv.Itoa(ProjectVersion)
	DescriptorField = descriptorFormat + "/v" + strconv.Itoa(DescriptorVersion)
)
