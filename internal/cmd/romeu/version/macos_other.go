// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix && !darwin

package version

// macOSVersion returns "" off macOS: romeu's host is darwin, and its
// linux build exists for the tests alone (02 2.1, REQ-0113), so the
// versions object leaves macos out there, and no read failed.
func macOSVersion() (string, error) { return "", nil }
