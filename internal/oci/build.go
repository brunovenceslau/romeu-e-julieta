// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The capability types of the kit schema of the pinned frontend
// (REQ-0516): the closed list of the grammar. A type outside it is
// refused (REQ-0573).
const (
	typeNetwork1    = "com.docker.sandbox/network-policy@1"
	typeNetwork2    = "com.docker.sandbox/network-policy@2"
	typeCredential  = "com.docker.sandbox/credential@1"
	typeSSHAgent    = "com.docker.sandbox/ssh-agent@1"
	typeSkills      = "com.docker.sandbox/agent-skills@1"
	typeSkill       = "com.docker.sandbox/agent-skill@1"
	typeVolume      = "com.docker.sandbox/volume@1"
	typeHostMount   = "com.docker.sandbox/host-mount@1"
	typePort        = "com.docker.sandbox/port@1"
	typeUSB         = "com.docker.sandbox/usb-device@1"
	typeResources   = "com.docker.sandbox/resources@1"
	typePrivileged  = "com.docker.sandbox/privileged@1"
	typeSbx         = "com.docker.sandbox/sbx@1"
	typeKitRegistry = "com.docker.sandbox/kit-registry@1"
	typeSessions    = "com.docker.sandbox/agent-sessions@1"
	typeInteractive = "com.docker.sandbox/agent-interactive-sessions@1"
	typeACP         = "com.docker.sandbox/acp@1"
	typeContext2    = "com.docker.sandbox/agent-context@2"
	typeLifecycle   = "com.docker.sandbox/lifecycle@1"
	typeContext     = "com.docker.sandbox/agent-context@1"
	typeLongRunning = "com.docker.sandbox/long-running@1"
	typeGitIdentity = "com.docker.sandbox/git-identity@1"
)

// capabilityTypes are the capability types the grammar models: the
// closed list of the kit schema of the pinned frontend (REQ-0516).
var capabilityTypes = []string{
	typeNetwork1, typeNetwork2, typeCredential, typeSSHAgent, typeSkills, typeSkill,
	typeVolume, typeHostMount, typePort, typeUSB, typeResources, typePrivileged, typeSbx,
	typeKitRegistry, typeSessions, typeLifecycle, typeContext, typeLongRunning, typeGitIdentity,
	typeInteractive, typeACP, typeContext2,
}

// builder collects the capabilities of one descriptor.
type builder struct {
	caps Capabilities
	// kind is the descriptor's kind; sbx@1 is a workload's alone.
	kind string
	// localFile says the descriptor is a file the maintainer's repository
	// holds, which may leave the phase of an ssh-agent out.
	localFile bool
	// at names the capability being added, for a message.
	at string
}

// normalize flattens the groups of the descriptor, decodes the config of
// each capability by its type and returns the set in its canonical form.
func normalize(d kitDoc, localFile bool) (Capabilities, error) {
	b := &builder{kind: d.Kind, localFile: localFile}
	b.caps.Kind = d.Kind
	b.caps.Build = d.Build
	b.caps.Dockerfile = d.Dockerfile
	b.members(d)
	for i, it := range d.Capabilities {
		at := "capabilities[" + strconv.Itoa(i) + "]"
		if it.Group == nil {
			if err := b.item(at, it, false); err != nil {
				return Capabilities{}, err
			}
			continue
		}
		g := it.Group
		if it.Type != "" || it.Name != "" || it.Optional != nil || len(it.Config) > 0 || it.Description != "" {
			return Capabilities{}, invalid("%s is a group and a capability both", at)
		}
		if len(g.Capabilities) == 0 {
			return Capabilities{}, invalid("%s.group has no capabilities", at)
		}
		for j, m := range g.Capabilities {
			mat := at + ".group.capabilities[" + strconv.Itoa(j) + "]"
			if m.Optional != nil || m.Group != nil {
				return Capabilities{}, invalid("%s sets optional or group, which a member of a group may not", mat)
			}
			if err := b.item(mat, m, g.Optional); err != nil {
				return Capabilities{}, err
			}
		}
	}
	b.caps.sort()
	return b.caps, nil
}

// members records the members of the descriptor besides its capability
// list that decide what it grants: the arg declarations, the relations
// and the kits of a set. The members that only say who made the kit or
// label a request (displayName, author, description, sourceUrl, iconUrl,
// licenses, the name, description and source of a capability or a group,
// the description of an arg) are not kept; the test of kit_test.go holds
// the list.
func (b *builder) members(d kitDoc) {
	if len(d.Args) > 0 {
		b.caps.Args = make(map[string]Arg, len(d.Args))
		for name, a := range d.Args {
			b.caps.Args[name] = Arg{Default: a.Default, Required: a.Required, Enum: sortedSet(a.Enum),
				Pattern: a.Pattern, Env: a.Env, BuildArg: a.BuildArg}
		}
	}
	r := Relations{Version: d.Version, Provides: sortedSet(d.Provides), Requires: sortedSet(d.Requires),
		Integrates: sortedSet(d.Integrates), Conflicts: sortedSet(d.Conflicts)}
	if r.Version != "" || len(r.Provides)+len(r.Requires)+len(r.Integrates)+len(r.Conflicts) > 0 {
		b.caps.Relations = &r
	}
	for _, k := range d.Kits {
		b.caps.Kits = append(b.caps.Kits, KitRef(k))
	}
}

// item adds one capability. inherited is its group's optional flag.
func (b *builder) item(at string, it itemDoc, inherited bool) error {
	if !typePattern.MatchString(it.Type) {
		return invalid("%s has a type that is not <namespace>/<name>@<version>", at)
	}
	b.at = at + " (" + it.Type + ")"
	optional := inherited || (it.Optional != nil && *it.Optional)
	if it.Type == typeSbx && b.kind == "mixin" {
		return invalid("%s: sbx@1 is a workload's alone, and this is a mixin", b.at)
	}
	switch it.Type {
	case typeNetwork1:
		return b.network(it.Config, optional, false)
	case typeNetwork2:
		return b.network(it.Config, optional, true)
	case typeCredential:
		return b.credential(it.Config, optional)
	case typeSSHAgent:
		return b.sshAgent(it.Config, optional)
	case typeSkills:
		return b.skill(it.Config, optional, true)
	case typeSkill:
		return b.skill(it.Config, optional, false)
	case typeVolume:
		return b.volume(it.Config, optional, "volume")
	case typeHostMount:
		return b.volume(it.Config, optional, "host-mount")
	case typePort:
		return b.port(it.Config, optional)
	case typeUSB:
		return b.usb(it.Config, optional)
	case typeResources:
		return b.resources(it.Config, optional)
	case typeSessions:
		return b.sessions(it.Config, optional)
	case typeLifecycle:
		return b.lifecycle(it.Config, optional)
	case typeContext:
		return b.agentContext(it.Config, optional)
	case typeContext2:
		return b.agentContext2(it.Config, optional)
	case typeInteractive:
		return b.interactiveSessions(it.Config, optional)
	case typeACP:
		return b.acp(it.Config, optional)
	case typePrivileged, typeSbx, typeKitRegistry, typeLongRunning, typeGitIdentity:
		if len(it.Config) > 0 {
			return invalid("%s takes no config", b.at)
		}
		b.caps.Flags = append(b.caps.Flags, Flag{Type: it.Type, Optional: optional})
		return nil
	}
	return invalid("%s is a capability type the grammar does not model (REQ-0573)", b.at)
}

// strict decodes a config into out, refusing an unknown member; an
// absent config reads as an empty one, so the required members are
// judged the same way whether it is absent or empty.
func strict[T any](b *builder, cfg jsontext.Value, out *T) error {
	if len(cfg) == 0 {
		cfg = jsontext.Value("{}")
	}
	if err := jsonv2.Unmarshal(cfg, out, jsonv2.RejectUnknownMembers(true)); err != nil {
		return invalid("%s: %s", b.at, shapeReason(err))
	}
	return nil
}

// need returns an error unless every value is set.
func (b *builder) need(name string, values ...string) error {
	if slices.Contains(values, "") {
		return invalid("%s has no %s", b.at, name)
	}
	return nil
}

// isRef reports whether s holds a ${{ kit.args.* }} or ${{ kit.env.* }}
// reference, which a field that is an enum or a number may carry
// in place of its value.
func isRef(s string) bool { return strings.Contains(s, "${{") }

// scalar reads a value that is a string, a number or a boolean and
// returns its text.
func (b *builder) scalar(name string, v jsontext.Value) (string, error) {
	if len(v) == 0 {
		return "", nil
	}
	switch v.Kind() {
	case '"':
		var s string
		if err := jsonv2.Unmarshal(v, &s); err != nil {
			return "", invalid("%s: %s is not text", b.at, name)
		}
		return s, nil
	case '0':
		// The text the tree was written with: a whole number is exact
		// (the tree refuses one past 2^53), a fraction is the shortest
		// text of its float64.
		return string(v), nil
	case 't', 'f':
		return string(v), nil
	}
	return "", invalid("%s: %s is not a string, a number or a boolean", b.at, name)
}

// stringList reads a value that is a string or a list of strings.
func (b *builder) stringList(name string, v jsontext.Value) ([]string, error) {
	switch {
	case len(v) == 0:
		return nil, nil
	case v.Kind() == '"':
		var s string
		if err := jsonv2.Unmarshal(v, &s); err != nil {
			return nil, invalid("%s: %s is not text", b.at, name)
		}
		return []string{s}, nil
	}
	var l []string
	if err := jsonv2.Unmarshal(v, &l); err != nil || v.Kind() != '[' {
		return nil, invalid("%s: %s is not a string or a list of strings", b.at, name)
	}
	return l, nil
}

// phases reads and checks the phase of a credential or an ssh agent: a
// string or a list of "install" and "runtime", or references.
func (b *builder) phases(v jsontext.Value) ([]string, error) {
	if len(v) == 0 {
		return nil, invalid("%s has no phase", b.at)
	}
	l, err := b.stringList("phase", v)
	if err != nil {
		return nil, err
	}
	if len(l) == 0 {
		return nil, invalid("%s has an empty phase list", b.at)
	}
	for _, p := range l {
		if p != "install" && p != "runtime" && !isRef(p) {
			return nil, invalid("%s has a phase that is not install or runtime", b.at)
		}
	}
	return sortedSet(l), nil
}

// command reads a shell string or an argv.
func (b *builder) command(v jsontext.Value) (Command, error) {
	switch {
	case len(v) == 0:
		return Command{}, invalid("%s has a hook without a command", b.at)
	case v.Kind() == '"':
		var s string
		if err := jsonv2.Unmarshal(v, &s); err != nil {
			return Command{}, invalid("%s: a command is not text", b.at)
		}
		return Command{Shell: s}, nil
	}
	var argv []string
	if err := jsonv2.Unmarshal(v, &argv); err != nil || v.Kind() != '[' || len(argv) == 0 {
		return Command{}, invalid("%s: a command is not a string or a non-empty list of strings", b.at)
	}
	return Command{Argv: argv}, nil
}

// sortedSet returns the strings sorted without repeats.
func sortedSet(l []string) []string {
	if len(l) == 0 {
		return nil
	}
	out := slices.Clone(l)
	slices.Sort(out)
	return slices.Compact(out)
}

// singleton refuses a second request of a type that may be made once.
func (b *builder) singleton(present bool) error {
	if present {
		return invalid("%s: the type is requested twice, and it is a singleton", b.at)
	}
	return nil
}

// ---- network

type netPolicyDoc struct {
	Install *netRulesDoc `json:"install"`
	Runtime *netRulesDoc `json:"runtime"`
}

type netRulesDoc struct {
	Allow []jsontext.Value `json:"allow"`
	Deny  []jsontext.Value `json:"deny"`
}

type netEntryDoc struct {
	Hosts   []string `json:"hosts"`
	Methods []string `json:"methods"`
	Paths   []string `json:"paths"`
}

// network adds a network-policy@1 (host patterns alone) or @2 (entries
// that may bound hosts by method and path). Both are one singleton: the
// two are exclusive.
func (b *builder) network(cfg jsontext.Value, optional, v2 bool) error {
	if err := b.singleton(b.caps.Network != nil); err != nil {
		return err
	}
	var doc netPolicyDoc
	if err := strict(b, cfg, &doc); err != nil {
		return err
	}
	n := &Network{Optional: optional}
	var err error
	if n.Install, err = b.rules(doc.Install, v2); err != nil {
		return err
	}
	if n.Runtime, err = b.rules(doc.Runtime, v2); err != nil {
		return err
	}
	b.caps.Network = n
	return nil
}

// rules normalizes the rules of one phase.
func (b *builder) rules(d *netRulesDoc, v2 bool) (Rules, error) {
	if d == nil {
		return Rules{}, nil
	}
	var r Rules
	var err error
	if r.Allow, err = b.entries(d.Allow, v2); err != nil {
		return Rules{}, err
	}
	if r.Deny, err = b.entries(d.Deny, v2); err != nil {
		return Rules{}, err
	}
	return r, nil
}

// entries normalizes a list of entries, each a host pattern or, in @2,
// an object.
func (b *builder) entries(in []jsontext.Value, v2 bool) ([]NetEntry, error) {
	out := make([]NetEntry, 0, len(in))
	for _, v := range in {
		switch v.Kind() {
		case '"':
			var host string
			if err := jsonv2.Unmarshal(v, &host); err != nil || host == "" {
				return nil, invalid("%s has an empty host pattern", b.at)
			}
			out = append(out, NetEntry{Hosts: []string{host}})
		case '{':
			if !v2 {
				return nil, invalid("%s: network-policy@1 has host patterns only", b.at)
			}
			var e netEntryDoc
			if err := strict(b, jsontext.Value(v), &e); err != nil {
				return nil, err
			}
			if len(e.Hosts) == 0 || slices.Contains(e.Hosts, "") {
				return nil, invalid("%s has an entry without hosts", b.at)
			}
			if len(e.Paths) > 0 && len(e.Methods) == 0 {
				return nil, invalid("%s has an entry with paths and no methods", b.at)
			}
			out = append(out, NetEntry{Hosts: sortedSet(e.Hosts), Methods: sortedSet(e.Methods), Paths: sortedSet(e.Paths)})
		default:
			return nil, invalid("%s has an entry that is not a host pattern or an object", b.at)
		}
	}
	return sortedSet2(out), nil
}

// ---- storage

type volumeDoc struct {
	Path  string         `json:"path"`
	Size  string         `json:"size"`
	Tmpfs jsontext.Value `json:"tmpfs"`
	Mode  string         `json:"mode"`
}

// volume adds a volume@1 or a host-mount@1.
func (b *builder) volume(cfg jsontext.Value, optional bool, kind string) error {
	var d volumeDoc
	if kind == "host-mount" && len(cfg) == 0 {
		return invalid("%s needs a config", b.at)
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if err := b.need("path", d.Path); err != nil {
		return err
	}
	tmpfs, err := b.scalar("tmpfs", d.Tmpfs)
	if err != nil {
		return err
	}
	b.caps.Volumes = append(b.caps.Volumes, Volume{Kind: kind, Path: d.Path, Size: d.Size, Tmpfs: tmpfs, Mode: d.Mode, Optional: optional})
	return nil
}

// ---- credentials

type credentialDoc struct {
	Service string         `json:"service"`
	Phase   jsontext.Value `json:"phase"`
	APIKey  *apiKeyDoc     `json:"apiKey"`
	OAuth   *oauthDoc      `json:"oauth"`
}

type apiKeyDoc struct {
	Name         string         `json:"name"`
	ProxyManaged jsontext.Value `json:"proxyManaged"`
	Inject       []Inject       `json:"inject"`
}

type oauthDoc struct {
	TokenEndpoint  *Endpoint       `json:"tokenEndpoint"`
	ResourceHosts  []string        `json:"resourceHosts"`
	Sentinels      *Sentinels      `json:"sentinels"`
	CredentialFile *credFileDoc    `json:"credentialFile"`
	ResponseFields *ResponseFields `json:"responseFields"`
	Passthrough    jsontext.Value  `json:"passthrough"`
}

type credFileDoc struct {
	Path      string         `json:"path"`
	Structure jsontext.Value `json:"structure"`
	Format    string         `json:"format"`
}

// credential adds a credential@1.
func (b *builder) credential(cfg jsontext.Value, optional bool) error {
	var d credentialDoc
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if err := b.need("service", d.Service); err != nil {
		return err
	}
	phases, err := b.phases(d.Phase)
	if err != nil {
		return err
	}
	if d.APIKey == nil && d.OAuth == nil {
		return invalid("%s has neither apiKey nor oauth", b.at)
	}
	c := Credential{Service: d.Service, Phases: phases, Optional: optional}
	if k := d.APIKey; k != nil {
		if k.Name == "" && len(k.Inject) == 0 {
			return invalid("%s has an apiKey with neither name nor inject", b.at)
		}
		for _, in := range k.Inject {
			if err := b.need("inject domain", in.Domain); err != nil {
				return err
			}
		}
		managed, err := b.scalar("proxyManaged", k.ProxyManaged)
		if err != nil {
			return err
		}
		c.APIKey = &APIKey{Name: k.Name, ProxyManaged: managed, Inject: sortedSet2(k.Inject)}
	}
	if o := d.OAuth; o != nil {
		out := &OAuth{TokenEndpoint: o.TokenEndpoint, ResourceHosts: sortedSet(o.ResourceHosts), Sentinels: o.Sentinels, ResponseFields: o.ResponseFields}
		if o.TokenEndpoint != nil {
			if err := b.need("token endpoint host", o.TokenEndpoint.Host); err != nil {
				return err
			}
		}
		if out.Passthrough, err = b.scalar("passthrough", o.Passthrough); err != nil {
			return err
		}
		if f := o.CredentialFile; f != nil {
			if err := b.need("credential file path", f.Path); err != nil {
				return err
			}
			cf := &CredentialFile{Path: f.Path, Format: f.Format}
			if len(f.Structure) > 0 {
				if err := jsonv2.Unmarshal(f.Structure, &cf.Structure); err != nil {
					return invalid("%s: the credential file structure is not a mapping", b.at)
				}
			}
			out.CredentialFile = cf
		}
		c.OAuth = out
	}
	b.caps.Credentials = append(b.caps.Credentials, c)
	return nil
}

// ---- ssh agent

type sshAgentDoc struct {
	Phase        jsontext.Value `json:"phase"`
	Sign         []string       `json:"sign"`
	Authenticate []string       `json:"authenticate"`
	Unrestricted jsontext.Value `json:"unrestricted"`
}

// sshAgent adds an ssh-agent@1.
func (b *builder) sshAgent(cfg jsontext.Value, optional bool) error {
	var d sshAgentDoc
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	// The kit schema of the pinned frontend requires a phase. The sitting
	// M0 descriptor of git-ssh-sign, which sbx v0.48.0 accepted, has
	// none (no document of sbx says what it reads then). A local file is
	// read as the widest phases, so the gate shows the most it can
	// mean; the annotation of an image, written by a third party, is
	// refused.
	var phases []string
	switch {
	case len(d.Phase) > 0:
		var err error
		if phases, err = b.phases(d.Phase); err != nil {
			return err
		}
	case b.localFile:
		phases = []string{"install", "runtime"}
	default:
		return invalid("%s has no phase", b.at)
	}
	unrestricted, err := b.scalar("unrestricted", d.Unrestricted)
	if err != nil {
		return err
	}
	if d.Sign != nil && len(d.Sign) == 0 || d.Authenticate != nil && len(d.Authenticate) == 0 {
		return invalid("%s has an empty sign or authenticate list", b.at)
	}
	bounded := len(d.Sign) > 0 || len(d.Authenticate) > 0
	switch unrestricted {
	case "false":
		if !bounded {
			return invalid("%s is restricted and bounds neither sign nor authenticate", b.at)
		}
	case "", "true":
		if bounded {
			return invalid("%s is unrestricted and bounds sign or authenticate", b.at)
		}
	default:
		if !isRef(unrestricted) {
			return invalid("%s has an unrestricted that is not a boolean", b.at)
		}
	}
	b.caps.SSHAgent = append(b.caps.SSHAgent, SSHAgent{Phases: phases, Unrestricted: unrestricted,
		Sign: sortedSet(d.Sign), Authenticate: sortedSet(d.Authenticate), Optional: optional})
	return nil
}

// ---- lifecycle

type lifecycleDoc struct {
	Install     []hookDoc `json:"install"`
	Startup     []hookDoc `json:"startup"`
	Files       []fileDoc `json:"files"`
	Interactive []string  `json:"interactive"`
}

type fileDoc struct {
	Path        string         `json:"path"`
	Content     string         `json:"content"`
	Mode        string         `json:"mode"`
	Overwrite   jsontext.Value `json:"overwrite"`
	Description string         `json:"description"`
}

type hookDoc struct {
	Command     jsontext.Value `json:"command"`
	User        string         `json:"user"`
	Env         []string       `json:"env"`
	Description string         `json:"description"`
	Background  jsontext.Value `json:"background"`
}

// lifecycle adds a lifecycle@1.
func (b *builder) lifecycle(cfg jsontext.Value, optional bool) error {
	if err := b.singleton(b.caps.Lifecycle != nil); err != nil {
		return err
	}
	var d lifecycleDoc
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if len(d.Install) == 0 && len(d.Startup) == 0 && len(d.Files) == 0 && len(d.Interactive) == 0 {
		return invalid("%s declares nothing", b.at)
	}
	l := &Lifecycle{Interactive: d.Interactive, Optional: optional}
	var err error
	if l.Install, err = b.hooks(d.Install); err != nil {
		return err
	}
	if l.Startup, err = b.hooks(d.Startup); err != nil {
		return err
	}
	for _, f := range d.Files {
		if err := b.need("file path", f.Path); err != nil {
			return err
		}
		overwrite, err := b.scalar("overwrite", f.Overwrite)
		if err != nil {
			return err
		}
		l.Files = append(l.Files, File{Path: f.Path, Content: f.Content, Mode: f.Mode, Overwrite: overwrite, Description: f.Description})
	}
	slices.SortStableFunc(l.Files, func(x, y File) int { return strings.Compare(x.Path, y.Path) })
	b.caps.Lifecycle = l
	return nil
}

// hooks normalizes hooks and keeps their order.
func (b *builder) hooks(in []hookDoc) ([]Hook, error) {
	var out []Hook
	for _, h := range in {
		cmd, err := b.command(h.Command)
		if err != nil {
			return nil, err
		}
		bg, err := b.scalar("background", h.Background)
		if err != nil {
			return nil, err
		}
		out = append(out, Hook{Description: h.Description, User: h.User, Env: sortedSet(h.Env), Command: cmd, Background: bg})
	}
	return out, nil
}

// ---- the rest

type portDoc struct {
	Name      string         `json:"name"`
	Container jsontext.Value `json:"container"`
	Transport string         `json:"transport"`
	Family    string         `json:"family"`
}

// port adds a port@1.
func (b *builder) port(cfg jsontext.Value, optional bool) error {
	var d portDoc
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	c, err := b.scalar("container", d.Container)
	if err != nil {
		return err
	}
	if err := b.need("container port", c); err != nil {
		return err
	}
	if d.Family != "" && !isRef(d.Family) && !slices.Contains([]string{"ipv4", "ipv6", "dual"}, d.Family) {
		return invalid("%s: family is not ipv4, ipv6 or dual", b.at)
	}
	b.caps.Ports = append(b.caps.Ports, Port{Name: d.Name, Container: c, Transport: d.Transport, Family: d.Family, Optional: optional})
	return nil
}

// usb adds a usb-device@1: a vendor and a product, or a class.
func (b *builder) usb(cfg jsontext.Value, optional bool) error {
	var d struct {
		VendorID  string `json:"vendorId"`
		ProductID string `json:"productId"`
		Class     string `json:"class"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	pair := d.VendorID != "" && d.ProductID != "" && d.Class == ""
	class := d.Class != "" && d.VendorID == "" && d.ProductID == ""
	if !pair && !class {
		return invalid("%s matches by vendorId and productId together, or by class, exactly one of the two", b.at)
	}
	b.caps.Devices = append(b.caps.Devices, Device{VendorID: d.VendorID, ProductID: d.ProductID, Class: d.Class, Optional: optional})
	return nil
}

// resources adds a resources@1.
func (b *builder) resources(cfg jsontext.Value, optional bool) error {
	if err := b.singleton(b.caps.Resources != nil); err != nil {
		return err
	}
	var d struct {
		CPU    jsontext.Value `json:"cpu"`
		Memory string         `json:"memory"`
		GPU    string         `json:"gpu"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	cpu, err := b.scalar("cpu", d.CPU)
	if err != nil {
		return err
	}
	b.caps.Resources = &Resources{CPU: cpu, Memory: d.Memory, GPU: d.GPU, Optional: optional}
	return nil
}

// sessions adds an agent-sessions@1.
func (b *builder) sessions(cfg jsontext.Value, optional bool) error {
	if err := b.singleton(b.caps.Sessions != nil); err != nil {
		return err
	}
	var d struct {
		Prompt   []string       `json:"prompt"`
		Resume   []string       `json:"resume"`
		Continue []string       `json:"continue"`
		List     jsontext.Value `json:"list"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if len(d.Prompt) == 0 && len(d.Resume) == 0 && len(d.Continue) == 0 && len(d.List) == 0 {
		return invalid("%s declares no verb", b.at)
	}
	s := &Sessions{Prompt: d.Prompt, Resume: d.Resume, Continue: d.Continue, Optional: optional}
	if len(d.List) > 0 {
		c, err := b.command(d.List)
		if err != nil {
			return err
		}
		s.List = &c
	}
	b.caps.Sessions = s
	return nil
}

// agentContext adds an agent-context@1.
func (b *builder) agentContext(cfg jsontext.Value, optional bool) error {
	var d struct {
		Filename    string `json:"filename"`
		ContentFile string `json:"contentFile"`
		Content     string `json:"content"`
		Directory   string `json:"directory"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if d == (struct {
		Filename    string `json:"filename"`
		ContentFile string `json:"contentFile"`
		Content     string `json:"content"`
		Directory   string `json:"directory"`
	}{}) {
		return invalid("%s declares nothing", b.at)
	}
	if d.Directory != "" && d.Filename == "" {
		return invalid("%s sets a directory without a filename", b.at)
	}
	b.caps.Context = append(b.caps.Context, Context{Filename: d.Filename, ContentFile: d.ContentFile, Content: d.Content, Directory: d.Directory, Optional: optional})
	return nil
}

// skill adds an agent-skills@1 (a discovery directory) or an
// agent-skill@1 (a bundle).
func (b *builder) skill(cfg jsontext.Value, optional, discovery bool) error {
	var d struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Mode string `json:"mode"`
	}
	if len(cfg) == 0 {
		return invalid("%s needs a config", b.at)
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if err := b.need("path", d.Path); err != nil {
		return err
	}
	if !discovery && d.Mode != "" {
		return invalid("%s: agent-skill@1 has no mode", b.at)
	}
	if discovery && d.Name != "" {
		return invalid("%s: agent-skills@1 has no name", b.at)
	}
	if d.Mode != "" && d.Mode != "readonly" && d.Mode != "readwrite" && !isRef(d.Mode) {
		return invalid("%s has a mode that is not readonly or readwrite", b.at)
	}
	b.caps.Skills = append(b.caps.Skills, Skill{Discovery: discovery, Path: d.Path, Name: d.Name, Mode: d.Mode, Optional: optional})
	return nil
}

// ---- canonical order

// jsonKey returns the deterministic JSON of v, the sort key of a set.
func jsonKey(v any) string {
	data, err := jsonv2.Marshal(v, jsonv2.Deterministic(true))
	if err != nil {
		// Every type that reaches here is plain data.
		panic("oci: " + err.Error())
	}
	return string(data)
}

// sortedSet2 returns the elements sorted by their JSON without repeats.
func sortedSet2[T any](in []T) []T {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	slices.SortStableFunc(out, func(a, b T) int { return strings.Compare(jsonKey(a), jsonKey(b)) })
	return slices.CompactFunc(out, func(a, b T) bool { return jsonKey(a) == jsonKey(b) })
}

// sort puts every group in its canonical order. Hooks, which run in the
// order they are written, keep it.
func (c *Capabilities) sort() {
	c.Volumes = sortedSet2(c.Volumes)
	c.Credentials = sortedSet2(c.Credentials)
	c.SSHAgent = sortedSet2(c.SSHAgent)
	c.Ports = sortedSet2(c.Ports)
	c.Devices = sortedSet2(c.Devices)
	c.Context = sortedSet2(c.Context)
	c.Skills = sortedSet2(c.Skills)
	c.Flags = sortedSet2(c.Flags)
}

// interactiveSessions adds an agent-interactive-sessions@1. A present
// empty continue, newSession or sessionPicker is the launch argv alone,
// and an absent one is a verb the workload does not offer, so the set
// keeps the two apart; prompt and resume, when present, hold the
// {{.Prompt}} or {{.SessionID}} placeholder, or a kit arg in its place.
func (b *builder) interactiveSessions(cfg jsontext.Value, optional bool) error {
	if err := b.singleton(b.caps.InteractiveSessions != nil); err != nil {
		return err
	}
	var d struct {
		Prompt        *[]string      `json:"prompt"`
		Resume        *[]string      `json:"resume"`
		Continue      *[]string      `json:"continue"`
		NewSession    *[]string      `json:"newSession"`
		SessionPicker *[]string      `json:"sessionPicker"`
		List          jsontext.Value `json:"list"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if d.Prompt == nil && d.Resume == nil && d.Continue == nil && d.NewSession == nil && d.SessionPicker == nil && len(d.List) == 0 {
		return invalid("%s declares no verb", b.at)
	}
	for _, v := range []struct {
		name, placeholder string
		argv              *[]string
	}{{"prompt", "{{.Prompt}}", d.Prompt}, {"resume", "{{.SessionID}}", d.Resume}} {
		if v.argv == nil {
			continue
		}
		if !slices.ContainsFunc(*v.argv, func(a string) bool { return strings.Contains(a, v.placeholder) || isRef(a) }) {
			return invalid("%s: %s holds no %s", b.at, v.name, v.placeholder)
		}
	}
	s := &InteractiveSessions{Continue: d.Continue, NewSession: d.NewSession, SessionPicker: d.SessionPicker, Optional: optional}
	if d.Prompt != nil {
		s.Prompt = *d.Prompt
	}
	if d.Resume != nil {
		s.Resume = *d.Resume
	}
	if len(d.List) > 0 {
		c, err := b.runnable("list", d.List)
		if err != nil {
			return err
		}
		s.List = &c
	}
	b.caps.InteractiveSessions = s
	return nil
}

// runnable reads a command that must run something: a shell string that
// is not blank, or an argv whose first element is not.
func (b *builder) runnable(name string, v jsontext.Value) (Command, error) {
	c, err := b.command(v)
	if err != nil {
		return Command{}, err
	}
	if strings.TrimSpace(c.Shell) == "" && (len(c.Argv) == 0 || strings.TrimSpace(c.Argv[0]) == "") {
		return Command{}, invalid("%s: %s runs nothing", b.at, name)
	}
	return c, nil
}

// envName is the form of an environment variable's name.
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// acp adds an acp@1: an agent and its command are required, the agent
// once per descriptor, since the type is keyed on it; the protocol
// version is a whole number above zero or a kit arg.
func (b *builder) acp(cfg jsontext.Value, optional bool) error {
	var d struct {
		Agent           string            `json:"agent"`
		Command         jsontext.Value    `json:"command"`
		Env             map[string]string `json:"env"`
		ProtocolVersion jsontext.Value    `json:"protocolVersion"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if err := b.need("agent", d.Agent); err != nil {
		return err
	}
	if slices.ContainsFunc(b.caps.ACP, func(a ACP) bool { return a.Agent == d.Agent }) {
		return invalid("%s: the agent %s is declared twice", b.at, quote(d.Agent))
	}
	c, err := b.runnable("command", d.Command)
	if err != nil {
		return err
	}
	for k := range d.Env {
		if !envName.MatchString(k) {
			return invalid("%s: the env name %s is not of the form [A-Za-z_][A-Za-z0-9_]*", b.at, quote(k))
		}
	}
	v, err := b.scalar("protocolVersion", d.ProtocolVersion)
	if err != nil {
		return err
	}
	if v != "" && !isRef(v) {
		if n, err := strconv.Atoi(v); err != nil || n < 1 {
			return invalid("%s: protocolVersion is not a whole number above zero", b.at)
		}
	}
	b.caps.ACP = append(b.caps.ACP, ACP{Agent: d.Agent, Command: c, Env: d.Env, ProtocolVersion: v, Optional: optional})
	return nil
}

// profileDir is the form of an agent-context@2 directory: an absolute
// path with no empty, "." or ".." element.
var profileDir = regexp.MustCompile(`^(/|(/([^./][^/]*|\.[^./][^/]*|\.\.[^/]+))+)$`)

// agentContext2 adds an agent-context@2: inline content, a filename, or
// both; a directory only with a filename, and then an absolute clean
// path, with a filename that is one name (or a kit arg in either place).
func (b *builder) agentContext2(cfg jsontext.Value, optional bool) error {
	var d struct {
		Filename  string `json:"filename"`
		Content   string `json:"content"`
		Directory string `json:"directory"`
	}
	if err := strict(b, cfg, &d); err != nil {
		return err
	}
	if d.Filename == "" && d.Content == "" {
		return invalid("%s declares neither a filename nor content", b.at)
	}
	if d.Directory != "" {
		if d.Filename == "" {
			return invalid("%s sets a directory without a filename", b.at)
		}
		if !isRef(d.Directory) && (!profileDir.MatchString(d.Directory) || strings.Contains(d.Directory, "\\")) {
			return invalid("%s: the directory is not an absolute path with no ., .. or empty element", b.at)
		}
		if !isRef(d.Filename) && (strings.ContainsAny(d.Filename, "/\\") || d.Filename == "." || d.Filename == "..") {
			return invalid("%s: the filename is not one name", b.at)
		}
	}
	b.caps.Context = append(b.caps.Context, Context{Filename: d.Filename, Content: d.Content, Directory: d.Directory, Version: 2, Optional: optional})
	return nil
}
