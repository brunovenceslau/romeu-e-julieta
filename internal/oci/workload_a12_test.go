// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestA12WorkloadConformance parses the measured descriptor annotation
// of the pinned workload (testdata/REUSE.toml says where it was read)
// the way FetchWorkload does (I28, REQ-0513, REQ-0569): it is a workload
// whose agent-interactive-sessions@1, marked optional, the grammar now
// models, so REQ-0573 no longer fails sync closed on the default
// workload; the five volumes keep their paths and sizes (REQ-0580), and
// the agent-sessions@1, network, credential, lifecycle, skills and
// context requests are each in the set the gate shows.
func TestA12WorkloadConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/workload-capabilities.json")
	require.NoError(t, err)
	k, err := ParseAnnotation(data)
	require.NoError(t, err)
	c := k.Capabilities
	assert.Equal(t, "workload", c.Kind)
	require.NotNil(t, c.InteractiveSessions)
	assert.True(t, c.InteractiveSessions.Optional)
	assert.Equal(t, []string{"--", "{{.Prompt}}"}, c.InteractiveSessions.Prompt)
	assert.Equal(t, &[]string{"--resume"}, c.InteractiveSessions.SessionPicker)
	assert.Equal(t, &[]string{"--continue"}, c.InteractiveSessions.Continue)
	assert.Nil(t, c.InteractiveSessions.NewSession, "left out: the schema falls back to the lifecycle launch")
	require.NotNil(t, c.Sessions)
	assert.False(t, c.Sessions.Optional)
	assert.Equal(t, []Volume{
		{Kind: "volume", Path: "/home/agent/.claude/projects", Size: "2g"},
		{Kind: "volume", Path: "/home/agent/.claude/sessions", Size: "512m"},
		{Kind: "volume", Path: "/home/agent/.claude/shell-snapshots", Size: "512m"},
		{Kind: "volume", Path: "/home/agent/.claude/statsig", Size: "512m"},
		{Kind: "volume", Path: "/home/agent/.claude/todos", Size: "512m"},
	}, c.Volumes)
	require.NotNil(t, c.Network)
	assert.Len(t, c.Credentials, 1)
	assert.NotNil(t, c.Lifecycle)
	assert.Len(t, c.Skills, 1)
	assert.Len(t, c.Context, 1)
	assert.Contains(t, c.Flags, Flag{Type: typeSbx})
}
