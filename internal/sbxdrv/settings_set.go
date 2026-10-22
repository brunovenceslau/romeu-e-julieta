// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("settings-set", func(Fixture) []Call { return []Call{mustCall(SettingsSet("env.rememberHostCommands", "true"))} })

// SettingsSet is sbx settings set <key> <value>: it sets a user override
// of an sbx setting (timeout class sbx-change), captured. romeu
// recommends one, env.rememberHostCommands true (01 1.4), and probe A2
// sets it. "Values are parsed according to the setting's type"
// (https://docs.docker.com/reference/cli/sbx/settings/set/).
func SettingsSet(key, value string) (Call, error) {
	return (&argvBuilder{}).lit("settings", "set").slot(settingKey, key).slot(settingValue, value).call("settings-set", procgroup.SbxChange, Captured)
}
