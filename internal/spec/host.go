// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

// HostSettings are the host settings, settings.yaml, of the format
// host-settings.v1 (03 3.4). romeu init writes root, config and tools;
// the operator fills the rest (REQ-0204).
type HostSettings struct {
	// Schema is "host-settings/v1" (REQ-0161).
	Schema string `json:"schema" yaml:"schema"`
	// Root is the absolute real path of the root of the host tree.
	Root string `json:"root" yaml:"root"`
	// Config names the config repo.
	Config ConfigRepo `json:"config" yaml:"config"`
	// Tools are the absolute paths of the host tools romeu runs.
	Tools Tools `json:"tools" yaml:"tools"`
	// GitHosts are the hosts a repo URL may name.
	GitHosts []GitHost `json:"gitHosts,omitzero" yaml:"gitHosts,omitempty"`
	// WorkloadRepositories is the allowlist of the repositories, as
	// <registry>/<repository>, a spec's workload may name.
	WorkloadRepositories []string `json:"workloadRepositories,omitzero" yaml:"workloadRepositories,omitempty"`
	// Signing holds the signing agent of the git-ssh-sign kit.
	Signing *Signing `json:"signing,omitzero" yaml:"signing,omitempty"`
	// Secrets maps <secret name>@<project> to the command that fetches
	// the secret's value.
	Secrets map[string]Secret `json:"secrets,omitzero" yaml:"secrets,omitempty"`
}

// ConfigRepo names the config repo, whose own spec is one of its
// projects.
type ConfigRepo struct {
	// Project is the config repo's project name.
	Project string `json:"project" yaml:"project"`
	// URL is the config repo's origin.
	URL string `json:"url" yaml:"url"`
	// Ref is the branch romeu reads specs from.
	Ref string `json:"ref" yaml:"ref"`
}

// Tools are the host tools romeu runs, resolved by romeu init and
// checked again by doctor.
type Tools struct {
	// Sbx is the absolute path of sbx.
	Sbx string `json:"sbx" yaml:"sbx"`
	// Git is the absolute path of git.
	Git string `json:"git" yaml:"git"`
	// GitCredentialHelper is the git-credential-<name> helper on
	// /usr/bin:/bin, by name, or an absolute path; hardened git uses
	// this helper and no other.
	GitCredentialHelper string `json:"gitCredentialHelper,omitzero" yaml:"gitCredentialHelper,omitempty"`
}

// GitHost is a host a repo URL may name.
type GitHost struct {
	// Host is the host name.
	Host string `json:"host" yaml:"host"`
	// CAFile is the absolute path of a CA bundle for a private host.
	CAFile string `json:"caFile,omitzero" yaml:"caFile,omitempty"`
}

// Signing holds the signing agent of the git-ssh-sign kit (06 6.4).
type Signing struct {
	// AgentSocket is the absolute path of the socket of the dedicated
	// agent that holds exactly one signing-only key.
	AgentSocket string `json:"agentSocket" yaml:"agentSocket"`
}

// Secret is the command that fetches one secret's value. It runs with
// exactly HOME and PATH=/usr/bin:/bin (REQ-0208); it fetches the value
// and never carries it (REQ-0205).
type Secret struct {
	// Service is the sbx service name.
	Service string `json:"service" yaml:"service"`
	// Argv is the command, argv[0] an absolute path; there is no shell.
	Argv []string `json:"argv" yaml:"argv"`
}
