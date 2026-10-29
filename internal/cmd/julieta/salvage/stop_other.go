// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix && !linux

package salvage

import (
	"errors"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// stopAgents needs /proc, which julieta reads on Linux, the one system
// it is built for (12 12.1); elsewhere a profile with process names is
// an error, and none is no process to stop.
func stopAgents(names []string, _ time.Duration) (salvage.AgentStop, error) {
	if len(names) == 0 {
		return salvage.AgentStop{}, nil
	}
	return salvage.AgentStop{}, errors.New("stopping the agent's processes reads /proc, which only Linux has")
}
