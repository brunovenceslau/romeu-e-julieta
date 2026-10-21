// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package actions

// julieta's action of post-merge: the snapshot (08 8.4, REQ-0686).
var _ = register("post-merge", snapshotAction)
