// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package all

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// What the romeu e2e harness and the scenario packages of e2e/** use of
// internal/: host settings, to write and read the file a command wrote,
// and the command definitions, to hold them against the journeys' tables
// (REQ-0971: e2e/** reaches internal/ through this package).
type (
	// HostSettings is the type of host-settings.v1 (03 3.4).
	HostSettings = spec.HostSettings
	// ConfigRepo is the config key of HostSettings.
	ConfigRepo = spec.ConfigRepo
	// Command is the definition of one command (internal/cli).
	Command = cli.Command
)

// DecodeHostSettings reads host settings by the rules of 03 3.4.
var DecodeHostSettings = spec.DecodeHostSettings

// EncodeHostSettings writes host settings as YAML.
var EncodeHostSettings = spec.EncodeHostSettings

// Commands returns the registered definitions of a binary, sorted by
// command line.
var Commands = cli.Commands
