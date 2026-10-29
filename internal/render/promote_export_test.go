// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

// SetAfterStep sets the hook that runs after each step of l's
// promotion, for the fault injection of package render_test.
func SetAfterStep(l *Live, f func(step int) error) { l.afterStep = f }
