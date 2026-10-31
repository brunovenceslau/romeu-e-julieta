// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// TestDoctorRemovalPending: a removing generation with its sandbox
// present fails removal-pending, whose message names romeu rm
// (REQ-0390); with the sandbox gone it passes.
func TestDoctorRemovalPending(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		sandboxes []sbxdrv.Sandbox
		want      string
	}{
		{"the sandbox present", []sbxdrv.Sandbox{shopSandbox}, resultFail},
		{"the sandbox gone", nil, resultPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeProject(t, "shop", project(t, state.GenerationRemoving))
			f.sbx.sandboxes = tc.sandboxes

			rep, out, _ := f.output(t)

			c := check(t, rep, "removal-pending")
			assert.Equal(t, tc.want, c.Result, out)
			if tc.want == resultFail {
				assert.Contains(t, c.Message, "romeu rm shop")
			}
		})
	}
}
