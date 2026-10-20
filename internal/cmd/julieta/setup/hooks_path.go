// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// installDispatcher writes the scripts of the hook dispatcher below
// $HOME/.local/share/julieta/hooks (04 4.3, REQ-0397; 02 2.5,
// REQ-0154), each handing its hook to julieta hooks run through the
// PATH link. A script that is already right is left alone. A script
// that cannot be written leaves the sandbox unusable.
//
// Deferred: pointing the global core.hooksPath at that directory, the
// other half of REQ-0397, needs a verb of internal/gitsafe that writes
// the sandbox's global git config (git config --global), since git
// runs only through gitsafe (05 5.1) and no verb does. The step that
// sets it joins this one when the verb lands.
func installDispatcher(_ context.Context, r *run) {
	if _, err := hooks.Install(r.home, filepath.Join(r.home, linkPath)); err != nil {
		r.fail("the hook dispatcher: %v", err)
	}
}
