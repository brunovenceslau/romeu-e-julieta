// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

// Capabilities is the normalized capability set of one descriptor
// (REQ-0068, REQ-0240): what the descriptor asks of the host, in the
// groups the gate shows and the personal-kit rule judges. Two
// descriptors that ask for the same things have equal sets whatever the
// order of their capabilities, the way they spell a phase or a command,
// or whether they group a capability. A scalar the descriptor writes as
// a number or a boolean is kept as its text, and a value that is a
// ${{ kit.args.* }} reference is kept as written: the set records the
// request, not an expansion of it.
//
// Every capability type of the grammar has a place here, so a type
// sbx adds later has none and is refused (REQ-0573). A request marked
// optional keeps Optional: the gate shows it all the same.
type Capabilities struct {
	// Network is the egress policy, by phase.
	Network *Network `json:"network,omitempty"`
	// Volumes are the volume and host-mount requests, by path.
	Volumes []Volume `json:"volumes,omitempty"`
	// Credentials are the credential requests, by service.
	Credentials []Credential `json:"credentials,omitempty"`
	// SSHAgent is the use of an ssh agent, by phase.
	SSHAgent []SSHAgent `json:"sshAgent,omitempty"`
	// Lifecycle is what the engine runs for the kit: install and startup
	// hooks, files and the interactive argv.
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
	// Ports are the container ports to publish.
	Ports []Port `json:"ports,omitempty"`
	// Devices are the USB passthrough requests.
	Devices []Device `json:"devices,omitempty"`
	// Resources are the CPU, memory and GPU limits.
	Resources *Resources `json:"resources,omitempty"`
	// Context is the instruction content the agent reads.
	Context []Context `json:"context,omitempty"`
	// Skills are the agent-skill and agent-skills requests.
	Skills []Skill `json:"skills,omitempty"`
	// Sessions is the argv shapes that drive the agent's sessions.
	Sessions *Sessions `json:"sessions,omitempty"`
	// Flags are the requests that take no config: privileged execution,
	// the sandbox-agent platform, a long-running workload, the git
	// identity and the kit registry.
	Flags []Flag `json:"flags,omitempty"`
}

// Network is the phase-scoped egress policy (network-policy@1 and @2,
// which are one set: @1 is @2 with host patterns only).
type Network struct {
	// Install is the policy while the install hooks run.
	Install Rules `json:"install,omitzero"`
	// Runtime is the policy the agent runs under.
	Runtime Rules `json:"runtime,omitzero"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Rules are the entries a phase permits and the entries it refuses; a
// deny wins.
type Rules struct {
	// Allow are the entries permitted.
	Allow []NetEntry `json:"allow,omitempty"`
	// Deny are the entries refused.
	Deny []NetEntry `json:"deny,omitempty"`
}

// NetEntry is one entry of a rule: host patterns, and optionally the
// HTTP methods and path globs that bound them.
type NetEntry struct {
	// Hosts are the host patterns, sorted.
	Hosts []string `json:"hosts"`
	// Methods bound the entry to these HTTP methods, sorted.
	Methods []string `json:"methods,omitempty"`
	// Paths bound the entry to these path globs, sorted.
	Paths []string `json:"paths,omitempty"`
}

// Volume is a volume or a host-mount request.
type Volume struct {
	// Kind is "volume" or "host-mount".
	Kind string `json:"kind"`
	// Path is the in-container path.
	Path string `json:"path"`
	// Size is the volume's size, as written.
	Size string `json:"size,omitempty"`
	// Tmpfs says the volume is a tmpfs, as written.
	Tmpfs string `json:"tmpfs,omitempty"`
	// Mode is the file mode, as written.
	Mode string `json:"mode,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Credential is one credential need: the service, the phases and how the
// proxy presents it, never where the secret lives.
type Credential struct {
	// Service is the identifier in the host credential store.
	Service string `json:"service"`
	// Phases are "install" and "runtime", sorted.
	Phases []string `json:"phases"`
	// APIKey is the api-key presentation.
	APIKey *APIKey `json:"apiKey,omitempty"`
	// OAuth is the oauth presentation.
	OAuth *OAuth `json:"oauth,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// APIKey is the api-key form of a credential.
type APIKey struct {
	// Name is the in-container variable.
	Name string `json:"name,omitempty"`
	// ProxyManaged keeps the real value on the host, as written.
	ProxyManaged string `json:"proxyManaged,omitempty"`
	// Inject are the outbound domains the proxy rewrites.
	Inject []Inject `json:"inject,omitempty"`
}

// Inject is one rewrite of an api key on an outbound domain.
type Inject struct {
	// Domain is the outbound domain.
	Domain string `json:"domain"`
	// Header is the header to set.
	Header string `json:"header,omitempty"`
	// Format renders the credential into the header value.
	Format string `json:"format,omitempty"`
	// Scheme is a non-header presentation such as "basic".
	Scheme string `json:"scheme,omitempty"`
	// Username is the basic-auth user name.
	Username string `json:"username,omitempty"`
}

// OAuth is the oauth form of a credential.
type OAuth struct {
	// TokenEndpoint is where tokens are issued.
	TokenEndpoint *Endpoint `json:"tokenEndpoint,omitempty"`
	// ResourceHosts are the hosts the token is presented to, sorted.
	ResourceHosts []string `json:"resourceHosts,omitempty"`
	// Sentinels are the placeholder values the container sees.
	Sentinels *Sentinels `json:"sentinels,omitempty"`
	// CredentialFile is the file the container reads the token from.
	CredentialFile *CredentialFile `json:"credentialFile,omitempty"`
	// ResponseFields name the fields of the token response.
	ResponseFields *ResponseFields `json:"responseFields,omitempty"`
	// Passthrough returns the real token to the container, as written.
	Passthrough string `json:"passthrough,omitempty"`
}

// Endpoint is a host and an optional path.
type Endpoint struct {
	// Host is the token endpoint's host.
	Host string `json:"host"`
	// Path is its path.
	Path string `json:"path,omitempty"`
}

// Sentinels are the placeholders for the tokens.
type Sentinels struct {
	// AccessToken is the access token's placeholder.
	AccessToken string `json:"accessToken,omitempty"`
	// RefreshToken is the refresh token's placeholder.
	RefreshToken string `json:"refreshToken,omitempty"`
}

// CredentialFile is the credential file of an oauth need.
type CredentialFile struct {
	// Path is the file's path.
	Path string `json:"path"`
	// Structure is the nested map written to the file. A whole number
	// of it is read as a float64, which holds it exactly to 2^53.
	Structure map[string]any `json:"structure,omitempty"`
	// Format is "json" or "toml", as written.
	Format string `json:"format,omitempty"`
}

// ResponseFields name the fields of a token response.
type ResponseFields struct {
	// AccessToken names the access token's field.
	AccessToken string `json:"accessToken,omitempty"`
	// RefreshToken names the refresh token's field.
	RefreshToken string `json:"refreshToken,omitempty"`
	// ExpiresIn names the expiry's field.
	ExpiresIn string `json:"expiresIn,omitempty"`
}

// SSHAgent is the use of an ssh agent in a phase.
type SSHAgent struct {
	// Phases are "install" and "runtime", sorted. The sitting M0
	// descriptor of git-ssh-sign names none.
	Phases []string `json:"phases,omitempty"`
	// Unrestricted is as written; absent means true.
	Unrestricted string `json:"unrestricted,omitempty"`
	// Sign are the namespaces the agent may sign in, sorted.
	Sign []string `json:"sign,omitempty"`
	// Authenticate are the servers the agent may log in to, sorted.
	Authenticate []string `json:"authenticate,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Lifecycle is what the engine runs on the kit's behalf. The hooks keep
// the order they were written in, which is the order they run in.
type Lifecycle struct {
	// Install run once at create.
	Install []Hook `json:"install,omitempty"`
	// Startup run at every boot.
	Startup []Hook `json:"startup,omitempty"`
	// Files are written at start.
	Files []File `json:"files,omitempty"`
	// Interactive is the argv tail of a TTY session.
	Interactive []string `json:"interactive,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Hook is one install or startup hook.
type Hook struct {
	// Description is shown wherever the hook is listed.
	Description string `json:"description,omitempty"`
	// User is the user the hook runs as.
	User string `json:"user,omitempty"`
	// Env are the only variables the hook sees, sorted.
	Env []string `json:"env,omitempty"`
	// Command is what runs.
	Command Command `json:"command"`
	// Background says a startup hook runs in the background, as written.
	Background string `json:"background,omitempty"`
}

// Command is a shell string (run by sh -c) or an argv; exactly one is
// set.
type Command struct {
	// Shell is the string form.
	Shell string `json:"shell,omitempty"`
	// Argv is the argv form.
	Argv []string `json:"argv,omitempty"`
}

// File is a file written at start.
type File struct {
	// Path is where it is written.
	Path string `json:"path"`
	// Content is what is written.
	Content string `json:"content"`
	// Mode is the file mode, as written.
	Mode string `json:"mode,omitempty"`
	// Overwrite is as written; absent means true.
	Overwrite string `json:"overwrite,omitempty"`
	// Description is shown wherever the file is listed.
	Description string `json:"description,omitempty"`
}

// Port is a container port to publish.
type Port struct {
	// Name is a label.
	Name string `json:"name,omitempty"`
	// Container is the container port, as written.
	Container string `json:"container"`
	// Transport is "tcp" or "udp"; absent means tcp.
	Transport string `json:"transport,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Device is a USB passthrough request.
type Device struct {
	// VendorID is set with ProductID.
	VendorID string `json:"vendorId,omitempty"`
	// ProductID is set with VendorID.
	ProductID string `json:"productId,omitempty"`
	// Class is set alone.
	Class string `json:"class,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Resources are the limits a kit puts on itself.
type Resources struct {
	// CPU is as written.
	CPU string `json:"cpu,omitempty"`
	// Memory is as written.
	Memory string `json:"memory,omitempty"`
	// GPU is as written.
	GPU string `json:"gpu,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Context is written instruction content the agent reads.
type Context struct {
	// Filename selects the agent profile.
	Filename string `json:"filename,omitempty"`
	// ContentFile is a context-relative path of the content.
	ContentFile string `json:"contentFile,omitempty"`
	// Content is the content inline.
	Content string `json:"content,omitempty"`
	// Directory is the in-sandbox profile directory.
	Directory string `json:"directory,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Skill is an agent-skill bundle or an agent-skills discovery directory.
type Skill struct {
	// Discovery is true for agent-skills@1 (a directory the agent reads
	// skills from) and false for agent-skill@1 (one bundle).
	Discovery bool `json:"discovery,omitzero"`
	// Path is the path of the bundle or the directory.
	Path string `json:"path"`
	// Name is the bundle's directory name at the destination.
	Name string `json:"name,omitempty"`
	// Mode is "readonly" or "readwrite" for a discovery directory.
	Mode string `json:"mode,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Sessions are the argv shapes that drive the agent's sessions.
type Sessions struct {
	// Prompt runs one prompt.
	Prompt []string `json:"prompt,omitempty"`
	// Resume reopens a named session.
	Resume []string `json:"resume,omitempty"`
	// Continue reopens the latest session.
	Continue []string `json:"continue,omitempty"`
	// List enumerates the resumable sessions.
	List *Command `json:"list,omitempty"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}

// Flag is a request that takes no config; Type is the capability type.
type Flag struct {
	// Type is the capability type, such as com.docker.sandbox/privileged@1.
	Type string `json:"type"`
	// Optional is the request's optional flag.
	Optional bool `json:"optional,omitzero"`
}
