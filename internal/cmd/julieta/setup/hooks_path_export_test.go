// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// SetSystemHooksPath makes setup read the system core.hooksPath with f
// until the returned function restores the read of gitsafe.
func SetSystemHooksPath(f func(context.Context, *gitsafe.Runner) (string, bool, error)) (restore func()) {
	old := systemHooksPath
	systemHooksPath = f
	return func() { systemHooksPath = old }
}
