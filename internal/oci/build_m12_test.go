// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeTheM12Types reads each type the kit schema of v3.0.0-m.12
// adds into the set the gate shows (I28): dockerd@1, host-port@1 and
// docker-auth@1, each value as written, a kit arg in place of a value
// kept as the reference.
func TestNormalizeTheM12Types(t *testing.T) {
	tests := []struct {
		name   string
		typ    string
		config string
		check  func(t *testing.T, c Capabilities)
	}{
		{"dockerd with no config", typeDockerd, "", func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, &Dockerd{}, c.Dockerd, "absent autoStart stays absent: the runtime's default is true")
		}},
		{"dockerd that leaves the start to the kit", typeDockerd, `{autoStart: false}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, &Dockerd{AutoStart: "false"}, c.Dockerd)
		}},
		{"dockerd with autoStart from a kit arg", typeDockerd, `{autoStart: "${{ kit.args.start }}"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, &Dockerd{AutoStart: "${{ kit.args.start }}"}, c.Dockerd)
		}},
		{"host-port with every field", typeHostPort, `{host: 5432, container: 15432, transport: udp, family: dual}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []HostPort{{Host: "5432", Container: "15432", Transport: "udp", Family: "dual"}}, c.HostPorts)
		}},
		{"host-port with the host alone", typeHostPort, `{host: 5432}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []HostPort{{Host: "5432"}}, c.HostPorts)
		}},
		{"host-port with container 0, the host port", typeHostPort, `{host: 5432, container: 0}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []HostPort{{Host: "5432", Container: "0"}}, c.HostPorts)
		}},
		{"host-port with kit args", typeHostPort, `{host: "${{ kit.args.h }}", transport: "${{ kit.args.t }}"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []HostPort{{Host: "${{ kit.args.h }}", Transport: "${{ kit.args.t }}"}}, c.HostPorts)
		}},
		{"docker-auth sorts its registries", typeDockerAuth, `{registries: [ghcr.io, docker.io]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []DockerAuth{{Registries: []string{"docker.io", "ghcr.io"}}}, c.DockerAuth)
		}},
		{"docker-auth with a kit arg", typeDockerAuth, `{registries: ["${{ kit.args.r }}"]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []DockerAuth{{Registries: []string{"${{ kit.args.r }}"}}}, c.DockerAuth)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, err := ParseDescriptor(capability(tt.typ, tt.config))
			require.NoError(t, err)
			tt.check(t, k.Capabilities)
		})
	}

	optional, err := ParseDescriptor(withCaps("  - type: " + typeDockerd + "\n    optional: true\n"))
	require.NoError(t, err)
	assert.Equal(t, &Dockerd{Optional: true}, optional.Capabilities.Dockerd, "the optional flag is part of the set")

	ab, err := ParseDescriptor(withCaps("  - type: " + typeHostPort + "\n    config: {host: 1}\n  - type: " + typeHostPort + "\n    config: {host: 2}\n"))
	require.NoError(t, err)
	ba, err := ParseDescriptor(withCaps("  - type: " + typeHostPort + "\n    config: {host: 2}\n  - type: " + typeHostPort + "\n    config: {host: 1}\n"))
	require.NoError(t, err)
	assert.Equal(t, ab.Capabilities, ba.Capabilities, "the order of host-port entries is not part of the set")

	twice, err := ParseDescriptor(withCaps("  - type: " + typeDockerAuth + "\n    config: {registries: [docker.io]}\n  - type: " + typeDockerAuth + "\n    config: {registries: [docker.io]}\n"))
	require.NoError(t, err)
	assert.Len(t, twice.Capabilities.DockerAuth, 1, "identical docker-auth entries collapse")
}

// TestRefuseTheM12Types refuses what the kit schema of v3.0.0-m.12 and
// the capability pages do not admit in the types it adds.
func TestRefuseTheM12Types(t *testing.T) {
	for _, tc := range []struct{ name, doc, want string }{
		{"a second dockerd", string(withCaps("  - type: " + typeDockerd + "\n  - type: " + typeDockerd + "\n")), "singleton"},
		{"dockerd with an unknown member", string(capability(typeDockerd, `{autoStart: true, flags: [x]}`)), "flags"},
		{"dockerd with text for autoStart", string(capability(typeDockerd, `{autoStart: "yes"}`)), "boolean or one whole kit arg"},
		{"dockerd with a number for autoStart", string(capability(typeDockerd, `{autoStart: 1}`)), "boolean or one whole kit arg"},
		{"dockerd with a kit arg inside text", string(capability(typeDockerd, `{autoStart: "x${{ kit.args.a }}"}`)), "boolean or one whole kit arg"},
		{"host-port with no host", string(capability(typeHostPort, `{container: 1}`)), "no host port"},
		{"host-port with host 0", string(capability(typeHostPort, `{host: 0}`)), "from 1 to 65535"},
		{"host-port above 65535", string(capability(typeHostPort, `{host: 65536}`)), "from 1 to 65535"},
		{"host-port with a container above 65535", string(capability(typeHostPort, `{host: 1, container: 70000}`)), "from 0 to 65535"},
		{"host-port with a quoted number", string(capability(typeHostPort, `{host: "5432"}`)), "whole kit arg"},
		{"host-port with a fraction", string(capability(typeHostPort, `{host: 54.5}`)), "whole number"},
		{"host-port with sctp", string(capability(typeHostPort, `{host: 1, transport: sctp}`)), "tcp or udp"},
		{"host-port with a bad family", string(capability(typeHostPort, `{host: 1, family: ipv5}`)), "ipv4, ipv6 or dual"},
		{"host-port with an address", string(capability(typeHostPort, `{host: 1, address: 0.0.0.0}`)), "address"},
		{"docker-auth with no registries", string(capability(typeDockerAuth, `{registries: []}`)), "no registries"},
		{"docker-auth with no config", string(capability(typeDockerAuth, "")), "no registries"},
		{"docker-auth with a scheme", string(capability(typeDockerAuth, `{registries: ["https://docker.io"]}`)), "registry DNS name"},
		{"docker-auth with a port", string(capability(typeDockerAuth, `{registries: ["registry.example.com:5000"]}`)), "registry DNS name"},
		{"docker-auth with an IPv4 address", string(capability(typeDockerAuth, `{registries: [10.0.0.1]}`)), "registry DNS name"},
		{"docker-auth with uppercase", string(capability(typeDockerAuth, `{registries: [Docker.io]}`)), "registry DNS name"},
		{"docker-auth with a single label", string(capability(typeDockerAuth, `{registries: [localhost]}`)), "registry DNS name"},
		{"docker-auth with a wildcard", string(capability(typeDockerAuth, `{registries: ["*.example.com"]}`)), "registry DNS name"},
		{"docker-auth with a leading hyphen", string(capability(typeDockerAuth, `{registries: [-a.example.com]}`)), "registry DNS name"},
		{"docker-auth with a registry twice", string(capability(typeDockerAuth, `{registries: [docker.io, docker.io]}`)), "listed twice"},
		{"docker-auth with a null registry", string(capability(typeDockerAuth, `{registries: [null]}`)), "a null value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDescriptor([]byte(tc.doc))
			require.ErrorContains(t, err, tc.want)
		})
	}
}
