// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package kits is the kits check of 10 10.2 (REQ-0843, REQ-0844): the
// rules every kit of kits/<name>/ keeps, and the pin file they read. It
// judges the commit fast and all read before any step, and is the kits
// subcommand.
//
// A kit is a directory below kits/ that holds a descriptor, a .yaml file
// (the v3 descriptor is <stem>.yaml, or a lone kit.yaml, with an optional
// <stem>.dockerfile beside it; sitting M0 of 2026-10-19). A directory
// without one, such as kits/mise, is not a kit. The check holds each kit
// to these rules:
//
//   - The descriptor starts with the frontend line, "# syntax=" and the
//     one frontend of kits/pins.yaml, and no file of any kit carries a
//     second frontend line or a different frontend.
//   - An install step is the command of a lifecycle@1 install or startup
//     hook, a RUN instruction of a dockerfile (also after ONBUILD, in
//     exec form, and in the build: text of a descriptor), or an ADD of a
//     URL. A step holds five lines or fewer (REQ-1043): the lines that
//     are neither blank nor a comment, a line continued by a backslash
//     counting as each of its physical lines. A shell wrapper (sh -c and
//     the like) is read through to its script; a command that cannot be
//     read is a finding.
//   - Every command of every step is judged, not only the downloads. None
//     holds "|", "||", ";", "&&", a backtick, "$(", "<(", ">(", "<<" or a
//     background "&". This is stricter than the spec's wording, which
//     bars those only on a download line, so that a download cannot hide
//     in a chained line. A heredoc RUN is a finding.
//   - A download is a command that runs curl or wget, by any path, after
//     an assignment, sudo or env, with its output named in a flag cluster
//     (-fsSLo file, -qO file), a long option, a redirect or the URL. A
//     later command of the same step starts with sha256sum -c or shasum
//     -a 256 -c and reads a literal file operand that the step did not
//     download; standard input ("-") verifies nothing and fails. A
//     downloaded file named by a command before that checksum line fails.
//     A download written with another tool is not seen, and the review of
//     the kits surface covers it. In a dockerfile a RUN holds one command,
//     so a download there cannot be checked and is always a finding.
//   - A syntax directive on line 1 of a dockerfile names the pinned
//     frontend, and no directive stands below line 1 of any kit file.
//   - A descriptor decodes with the keys of the v3 grammar only, in one
//     YAML document. When a kit exists, kits/pins.yaml exists and pins a
//     frontend.
//   - The args miseVersion, miseSha256Amd64, miseSha256Arm64 and the
//     same three for herdr, when a kit declares one, have the default of
//     the pin file.
//
// And it holds the pin file to these: the version of mise equals the
// version input of the mise step of ci.yml (REQ-1068), and the committed
// kits/mise/SHASUMS256.txt carries a valid signature of the key
// kits/mise-minisign.pub, offline, and lists the pinned hash of each
// architecture (REQ-0844). The signature is checked in every run and not
// only on a change of the pin file, which is stricter and costs nothing
// offline.
package kits

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/minisign"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// The paths the check reads, from the root of the repository.
const (
	Dir        = "kits/"
	ciWorkflow = ".github/workflows/ci.yml"
	miseKey    = "kits/mise-minisign.pub"
	miseSums   = "kits/mise/SHASUMS256.txt"
	miseSig    = "kits/mise/SHASUMS256.txt.minisig"

	// maxStepLines is the longest an install step may be (REQ-1043).
	maxStepLines = 5
	// frontendName starts the reference of the kit frontend, and
	// syntaxPrefix opens the frontend line of a descriptor (REQ-0564).
	frontendName = "docker/sandbox-kit:"
	syntaxPrefix = "# syntax=" + frontendName
	rule         = "kits"
)

// pinArgs are the args a kit may declare to carry a pin of the pin file,
// each with how the pin file reads it.
var pinArgs = []struct {
	name string
	pin  func(p pins.Pins) (string, bool)
}{
	{"miseVersion", func(p pins.Pins) (string, bool) {
		return release(p.Mise, func(r *pins.Release) string { return r.Version })
	}},
	{"miseSha256Amd64", func(p pins.Pins) (string, bool) {
		return release(p.Mise, func(r *pins.Release) string { return r.SHA256.Amd64 })
	}},
	{"miseSha256Arm64", func(p pins.Pins) (string, bool) {
		return release(p.Mise, func(r *pins.Release) string { return r.SHA256.Arm64 })
	}},
	{"herdrVersion", func(p pins.Pins) (string, bool) {
		return release(p.Herdr, func(r *pins.Release) string { return r.Version })
	}},
	{"herdrSha256Amd64", func(p pins.Pins) (string, bool) {
		return release(p.Herdr, func(r *pins.Release) string { return r.SHA256.Amd64 })
	}},
	{"herdrSha256Arm64", func(p pins.Pins) (string, bool) {
		return release(p.Herdr, func(r *pins.Release) string { return r.SHA256.Arm64 })
	}},
}

func release(r *pins.Release, field func(*pins.Release) string) (string, bool) {
	if r == nil {
		return "", false
	}
	return field(r), true
}

// Of applies the rules to the kits and the pin file among files, the
// tree of one commit as check.HeadTree reads it.
func Of(files []check.TreeFile) []check.Finding {
	j := &judge{files: files}
	j.pinFile()
	j.kits()
	j.ciMise()
	j.miseSignature()
	return j.findings
}

// judge holds what one run of Of reads and finds.
type judge struct {
	files    []check.TreeFile
	pins     pins.Pins
	havePins bool
	// pinsMissing says that the tree has no pin file at all.
	pinsMissing bool
	findings    []check.Finding
}

func (j *judge) add(where, format string, args ...any) {
	j.findings = append(j.findings, check.Finding{Where: git.Printable(where), Rule: rule, Msg: fmt.Sprintf(format, args...)})
}

// file returns the content of the file at p.
func (j *judge) file(p string) ([]byte, bool) {
	b, ok, err := check.FileIn(j.files, p)
	if err != nil {
		j.add(p, "%v", err)
		return nil, false
	}
	return b, ok
}

// pinFile reads the pin file; absent, no kit has a pin to equal.
func (j *judge) pinFile() {
	src, ok := j.file(pins.Path)
	if !ok {
		j.pinsMissing = true
		return
	}
	p, err := pins.Parse(src)
	if err != nil {
		j.add(pins.Path, "%v", err)
		return
	}
	j.pins, j.havePins = p, true
}

// kitFile is one file of a kit.
type kitFile struct {
	path    string
	content []byte
}

// kitFiles groups the files directly below kits/<name>/ by kit, for the
// directories that hold a descriptor, and returns the kit names in the
// order of the tree.
func (j *judge) kitFiles() (names []string, byKit map[string][]kitFile) {
	byKit = map[string][]kitFile{}
	isKit := map[string]bool{}
	for _, f := range j.files {
		rest, ok := strings.CutPrefix(f.Path, Dir)
		if !ok || f.Content == nil {
			continue
		}
		name, base, ok := strings.Cut(rest, "/")
		if !ok || strings.Contains(base, "/") {
			continue
		}
		byKit[name] = append(byKit[name], kitFile{f.Path, f.Content})
		if isDescriptor(base) && !isKit[name] {
			isKit[name] = true
			names = append(names, name)
		}
	}
	for name := range byKit {
		if !isKit[name] {
			delete(byKit, name)
		}
	}
	return names, byKit
}

func isDescriptor(base string) bool { return path.Ext(base) == ".yaml" || path.Ext(base) == ".yml" }

// isDockerfile reports whether a kit file is a recipe: <stem>.dockerfile,
// or a Dockerfile or Containerfile.
func isDockerfile(base string) bool {
	return path.Ext(base) == ".dockerfile" || base == "Dockerfile" || base == "Containerfile"
}

// kits judges every kit, then the frontends together.
func (j *judge) kits() {
	names, byKit := j.kitFiles()
	if len(names) > 0 {
		switch {
		case j.pinsMissing:
			j.add(pins.Path, "missing, so no kit has a frontend pin to equal")
		case j.havePins && j.pins.Frontend == nil:
			j.add(pins.Path, "pins no frontend, so no kit has one to equal")
		}
	}
	frontends := map[string]string{} // frontend ref -> the first file that carries it
	var order []string
	for _, name := range names {
		for _, f := range byKit[name] {
			switch base := path.Base(f.path); {
			case isDescriptor(base):
				j.descriptor(f, frontends, &order)
			case isDockerfile(base):
				j.frontendLines(f.path, string(f.content), false, frontends, &order)
				j.dockerfileSteps(f.path, string(f.content))
			}
		}
	}
	if len(order) > 1 {
		parts := make([]string, len(order))
		for i, ref := range order {
			parts[i] = frontends[ref]
		}
		j.add(Dir, "a second frontend pin: %d different frontends, first seen in %s", len(order), strings.Join(parts, ", "))
	}
}

// syntaxDirective is a parser directive that names a frontend, in either
// spelling Docker accepts.
var syntaxDirective = regexp.MustCompile(`(?i)^#\s*syntax\s*=\s*(\S.*?)\s*$`)

// frontendLines judges the syntax directives of one file. The first line
// of a descriptor must be the frontend line; a directive on line 1 of a
// dockerfile or of a build: text must name the sandbox-kit frontend; and
// a directive anywhere below line 1 is a second pin.
func (j *judge) frontendLines(where, content string, descriptor bool, frontends map[string]string, order *[]string) {
	for i, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r ")
		m := syntaxDirective.FindStringSubmatch(line)
		switch {
		case m == nil:
			if i == 0 && descriptor {
				j.add(where, "line 1 is not the frontend line, %q followed by the frontend of %s (REQ-0564)", syntaxPrefix, pins.Path)
			}
		case i != 0:
			j.add(where, "line %d is a syntax directive below line 1: a second frontend pin", i+1)
		case !strings.HasPrefix(m[1], frontendName) || (descriptor && !strings.HasPrefix(line, "# syntax=")):
			j.add(where, "line 1 is a syntax directive that is not the sandbox-kit frontend of %s", pins.Path)
		default:
			ref := m[1]
			if _, seen := frontends[ref]; !seen {
				frontends[ref] = where
				*order = append(*order, ref)
			}
			if j.havePins && j.pins.Frontend != nil && ref != j.pins.Frontend.Ref {
				j.add(where, "the frontend differs from the pin of %s", pins.Path)
			}
		}
	}
}

// The keys the v3 descriptor, an arg, a lifecycle@1 config and a hook may
// hold (SPEC-v3 §4, §6, lifecycle@1 at tag v3.0.0-m.8); sbx refuses a key
// outside the first (sitting M0), and the others are held to the same.
var (
	descriptorKeys = []string{"schemaVersion", "kind", "displayName", "author", "description", "sourceUrl", "iconUrl", "version", "licenses", "provides", "requires", "integrates", "conflicts", "capabilities", "args", "build", "dockerfile", "kits"}
	argKeys        = []string{"default", "required", "description", "enum", "pattern", "env", "buildArg"}
	lifecycleKeys  = []string{"install", "startup", "files", "interactive"}
	hookKeys       = []string{"command", "user", "env", "description", "background"}
)

// keys reports each key of the mapping node n that is not in allowed.
func (j *judge) keys(where string, n *yaml.Node, allowed []string, what string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i].Value; !slices.Contains(allowed, k) {
			j.add(where, "line %d: %q is not a key of %s", n.Content[i].Line, k, what)
		}
	}
}

// descriptor judges one descriptor.
func (j *judge) descriptor(f kitFile, frontends map[string]string, order *[]string) {
	j.frontendLines(f.path, string(f.content), true, frontends, order)
	dec := yaml.NewDecoder(bytes.NewReader(f.content))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if !errors.Is(err, io.EOF) {
			j.add(f.path, "%v", err)
		}
		return
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		j.add(f.path, "more than one YAML document")
		return
	}
	root := &doc
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	j.keys(f.path, root, descriptorKeys, "a v3 descriptor")
	var d struct {
		Args         map[string]yaml.Node `yaml:"args"`
		Build        string               `yaml:"build"`
		Capabilities []yaml.Node          `yaml:"capabilities"`
	}
	if err := root.Decode(&d); err != nil {
		j.add(f.path, "%v", err)
		return
	}
	for _, name := range slices.Sorted(maps.Keys(d.Args)) {
		arg := d.Args[name]
		j.keys(f.path, &arg, argKeys, "the arg "+name)
	}
	j.checkPinArgs(f.path, d.Args)
	for _, c := range d.Capabilities {
		j.capability(f.path, c)
	}
	if d.Build != "" {
		j.frontendLines(f.path+" (build:)", d.Build, false, frontends, order)
		j.dockerfileSteps(f.path+" (build:)", d.Build)
	}
}

// checkPinArgs holds the args that carry a pin to the pin file.
func (j *judge) checkPinArgs(where string, args map[string]yaml.Node) {
	if !j.havePins {
		return
	}
	for _, a := range pinArgs {
		node, ok := args[a.name]
		if !ok {
			continue
		}
		var arg struct {
			Default *string `yaml:"default"`
		}
		if err := node.Decode(&arg); err != nil {
			j.add(where, "the arg %s: %v", a.name, err)
			continue
		}
		want, pinned := a.pin(j.pins)
		switch {
		case !pinned:
			j.add(where, "the arg %s has no pin in %s to equal", a.name, pins.Path)
		case arg.Default == nil || *arg.Default != want:
			j.add(where, "the default of the arg %s differs from the pin of %s", a.name, pins.Path)
		}
	}
}

// capability judges the hooks of a lifecycle@1 capability: the install
// hooks, and the startup hooks, which run commands too.
func (j *judge) capability(where string, node yaml.Node) {
	var c struct {
		Type   string    `yaml:"type"`
		Config yaml.Node `yaml:"config"`
	}
	if err := node.Decode(&c); err != nil {
		j.add(where, "a capability: %v", err)
		return
	}
	if !strings.HasPrefix(c.Type, "com.docker.sandbox/lifecycle@") {
		return
	}
	j.keys(where, &c.Config, lifecycleKeys, "a lifecycle@1 config")
	var cfg struct {
		Install []yaml.Node `yaml:"install"`
		Startup []yaml.Node `yaml:"startup"`
	}
	if err := c.Config.Decode(&cfg); err != nil {
		j.add(where, "the lifecycle@1 config: %v", err)
		return
	}
	for _, phase := range []struct {
		name  string
		hooks []yaml.Node
	}{{"install", cfg.Install}, {"startup", cfg.Startup}} {
		for i, hook := range phase.hooks {
			label := fmt.Sprintf("%s step %d", phase.name, i+1)
			j.keys(where, &hook, hookKeys, "a lifecycle@1 hook")
			var h struct {
				Command yaml.Node `yaml:"command"`
			}
			if err := hook.Decode(&h); err != nil {
				j.add(where, "%s: %v", label, err)
				continue
			}
			script, err := commandText(h.Command)
			if err != nil {
				j.add(where, "%s: %v", label, err)
				continue
			}
			j.step(where, label, script)
		}
	}
}

// shellFlag is the flag cluster of a shell that takes its script from
// the next argument.
var shellFlag = regexp.MustCompile(`^-[A-Za-z]*c[A-Za-z]*$`)

// commandText returns the script of a hook command: a string is the
// script; a list that starts with a shell and a flag cluster holding c
// is the script that follows (arguments after it are fine); any other
// list is one line. A command of another shape cannot be read, which is
// an error and never skipped.
func commandText(n yaml.Node) (string, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value, nil
	case yaml.SequenceNode:
		words := make([]string, len(n.Content))
		for i, w := range n.Content {
			if w.Kind != yaml.ScalarNode {
				return "", errors.New("the command list holds an item that is not a string")
			}
			words[i] = w.Value
		}
		return argvText(words), nil
	}
	return "", errors.New("the command is not a string or a list of strings, so the check cannot read it")
}

// argvText returns the script of an argv: the script after a shell's -c,
// or the words as one line.
func argvText(words []string) string {
	if len(words) >= 3 && slices.Contains([]string{"sh", "bash", "dash", "ash", "zsh"}, path.Base(words[0])) && shellFlag.MatchString(words[1]) {
		return words[2]
	}
	return strings.Join(words, " ")
}

// runFlag is a leading flag of a RUN instruction (--mount=..., --network=...).
var runFlag = regexp.MustCompile(`^--[A-Za-z-]+(=\S*)?(\s+|$)`)

// dockerfileSteps judges each RUN and ADD instruction of the dockerfile
// text src. An instruction runs over its lines that end in a backslash,
// and a comment line or a blank one inside it does not end it.
func (j *judge) dockerfileSteps(where, src string) {
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i]); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		start := i
		parts := []string{lines[i]}
		cont := strings.HasSuffix(strings.TrimRight(lines[i], " \t\r"), "\\")
		for cont && i+1 < len(lines) {
			i++
			if t := strings.TrimSpace(lines[i]); t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			parts = append(parts, lines[i])
			cont = strings.HasSuffix(strings.TrimRight(lines[i], " \t\r"), "\\")
		}
		verb, rest, _ := strings.Cut(strings.TrimSpace(parts[0]), " ")
		verb = strings.ToUpper(verb)
		if verb == "ONBUILD" {
			rest = strings.TrimSpace(rest)
			verb, rest, _ = strings.Cut(rest, " ")
			verb = strings.ToUpper(verb)
		}
		parts[0] = strings.TrimSpace(rest)
		label := fmt.Sprintf("the %s step at line %d", verb, start+1)
		switch verb {
		case "RUN":
			j.runInstruction(where, label, parts)
		case "ADD":
			j.addInstruction(where, label, strings.Join(parts, " "))
		}
	}
}

// runInstruction judges a RUN: its flags are stripped, its exec form is
// read as an argv, and a heredoc is a finding, since its script is on the
// lines after it.
func (j *judge) runInstruction(where, label string, parts []string) {
	for {
		loc := runFlag.FindStringIndex(parts[0])
		if loc == nil {
			break
		}
		parts[0] = parts[0][loc[1]:]
	}
	body := strings.Join(parts, "\n")
	if strings.HasPrefix(strings.TrimSpace(body), "[") {
		var argv []string
		if err := json.Unmarshal([]byte(strings.ReplaceAll(body, "\\\n", "")), &argv); err != nil {
			j.add(where, "%s: the exec form is not a JSON list of strings, so the check cannot read it", label)
			return
		}
		j.step(where, label, argvText(argv))
		return
	}
	if strings.Contains(body, "<<") {
		j.add(where, "%s: a heredoc, whose script the check does not read", label)
		return
	}
	j.step(where, label, body)
}

// addInstruction judges an ADD: a URL source is a download, which is
// checked by the build only with --checksum.
func (j *judge) addInstruction(where, label, text string) {
	words := strings.Fields(text)
	checksum := false
	var args []string
	for _, w := range words {
		switch {
		case strings.HasPrefix(w, "--checksum="):
			checksum = true
		case strings.HasPrefix(w, "--"):
		default:
			args = append(args, w)
		}
	}
	if len(args) > 1 {
		args = args[:len(args)-1]
	}
	for _, src := range args {
		if isURL(unquote(src)) && !checksum {
			j.add(where, "%s: ADD downloads a URL with no --checksum", label)
		}
	}
}

func isURL(w string) bool {
	return strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") || strings.HasPrefix(w, "ftp://")
}

// A logical command of a step, with the physical line it starts on.
type command struct {
	words []string
	text  string
}

// splitScript returns the physical lines of a script that are neither
// blank nor a comment, and its commands: a line continued by a backslash
// joins the line after it, and a comment inside the continuation does
// not end it.
func splitScript(script string) (physical []string, commands []command) {
	var cur []string
	for line := range strings.SplitSeq(script, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		physical = append(physical, t)
		if body, more := strings.CutSuffix(t, "\\"); more {
			cur = append(cur, strings.TrimSpace(body))
			continue
		}
		text := strings.Join(append(cur, t), " ")
		commands = append(commands, command{strings.Fields(text), text})
		cur = nil
	}
	if len(cur) > 0 {
		text := strings.Join(cur, " ")
		commands = append(commands, command{strings.Fields(text), text})
	}
	return physical, commands
}

// forbidden are the shell constructs no command of a step may hold:
// they chain, substitute or redirect into another command, so a download
// and its check could hide in one line.
var forbidden = []string{"|", ";", "&&", "`", "$(", "<(", ">(", "<<"}

// step judges the script of one install step.
func (j *judge) step(where, label, script string) {
	physical, cmds := splitScript(script)
	if len(physical) > maxStepLines {
		j.add(where, "%s has %d lines, over the %d of REQ-1043", label, len(physical), maxStepLines)
	}
	for i, c := range cmds {
		for _, bad := range forbidden {
			if strings.Contains(c.text, bad) {
				j.add(where, "%s: command %d holds %q", label, i+1, bad)
			}
		}
		if slices.ContainsFunc(c.words, func(w string) bool { return w == "&" || (strings.HasSuffix(w, "&") && !strings.Contains(w, ">")) }) {
			j.add(where, "%s: command %d holds %q", label, i+1, "&")
		}
	}

	type download struct {
		at    int
		files []string
	}
	var downloads []download
	for i, c := range cmds {
		if at := downloadAt(c.words); at >= 0 {
			downloads = append(downloads, download{i, downloaded(c.words[at:])})
		}
	}
	if len(downloads) == 0 {
		return
	}
	all := map[string]bool{} // every file the step downloads
	for _, d := range downloads {
		for _, f := range d.files {
			all[f] = true
		}
	}
	isDownloaded := func(operand string) bool {
		return slices.ContainsFunc(slices.Collect(maps.Keys(all)), func(d string) bool { return sameFile(d, operand) })
	}

	type sum struct {
		at    int
		valid bool
	}
	var sums []sum
	for i, c := range cmds {
		operand, ok := checksumOperand(c.words)
		if !ok {
			continue
		}
		s := sum{at: i}
		switch {
		case operand == "" || operand == "-" || strings.ContainsAny(operand, "$*?[]{}<>~\\\"'"):
			j.add(where, "%s: the checksum line on command %d reads its standard input or a name it computes, so it verifies no kit file", label, i+1)
		case isDownloaded(operand):
			j.add(where, "%s: the checksum line reads %s, a file the step downloaded, and not a kit file", label, git.Printable(operand))
		default:
			s.valid = true
		}
		sums = append(sums, s)
	}
	for _, d := range downloads {
		checked := -1
		for _, s := range sums {
			if s.at > d.at && s.valid {
				checked = s.at
				break
			}
		}
		if checked < 0 {
			j.add(where, "%s: the download on command %d has no valid sha256sum -c line after it (REQ-0606)", label, d.at+1)
			continue
		}
		for k := d.at + 1; k < checked; k++ {
			if downloadAt(cmds[k].words) >= 0 {
				continue
			}
			for _, f := range d.files {
				if slices.ContainsFunc(cmds[k].words, func(w string) bool { return path.Clean(unquote(w)) == path.Clean(f) }) {
					j.add(where, "%s: command %d uses %s before the checksum line on command %d", label, k+1, git.Printable(f), checked+1)
				}
			}
		}
	}
}

// exempt are the commands whose arguments may name curl or wget without
// running them.
var exempt = []string{"apt-get", "apt", "apk", "dnf", "yum", "zypper", "pacman", "brew", "pip", "pip3", "echo", "printf", "which", "type"}

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// downloadAt returns the index of the word that runs curl or wget in the
// command, by any path, after an assignment, sudo, env or command, or
// -1. A word with "://" is a URL.
func downloadAt(words []string) int {
	first := -1
	for i, w := range words {
		if first < 0 && !assignment.MatchString(w) {
			first = i
		}
	}
	if first >= 0 && slices.Contains(exempt, path.Base(unquote(words[first]))) {
		return -1
	}
	for i, w := range words {
		w = unquote(w)
		if base := path.Base(w); (base == "curl" || base == "wget") && !strings.Contains(w, "://") {
			return i
		}
	}
	return -1
}

// valueFlags are the short flags that take a value, for curl and wget;
// the output flag of each is -o and -O.
const (
	curlValueFlags = "oHuAdXewmxTbcCEFKrYzQtU"
	wgetValueFlags = "oaeiBtTwQlPUADRIXHFO"
)

// downloaded returns the files a download writes, from the words that
// start at curl or wget: the value of the output flag, whether it is
// alone, in a cluster (-fsSLo file, -qO file) or joined (-ofile); the
// target of a redirect; and, for curl -O and for wget with no output
// option, the base name of each URL.
func downloaded(words []string) []string {
	curl := path.Base(unquote(words[0])) == "curl"
	valueFlags, outFlag := wgetValueFlags, byte('O')
	if curl {
		valueFlags, outFlag = curlValueFlags, 'o'
	}
	var out, urls []string
	remoteName := false
	for i := 1; i < len(words); i++ {
		w := unquote(words[i])
		next := func() string {
			if i+1 < len(words) {
				i++
				return unquote(words[i])
			}
			return ""
		}
		switch {
		case w == "--output" || (!curl && w == "--output-document"):
			out = append(out, next())
		case strings.HasPrefix(w, "--output="):
			out = append(out, strings.TrimPrefix(w, "--output="))
		case !curl && strings.HasPrefix(w, "--output-document="):
			out = append(out, strings.TrimPrefix(w, "--output-document="))
		case curl && (w == "--remote-name" || w == "--remote-name-all"):
			remoteName = true
		case strings.HasPrefix(w, "--"):
		case w == ">" || w == ">>":
			out = append(out, next())
		case strings.HasPrefix(w, ">") && !strings.HasPrefix(w, ">&") && !strings.HasPrefix(w, ">("):
			out = append(out, strings.TrimLeft(w, ">"))
		case strings.HasPrefix(w, "-") && len(w) > 1:
			for k := 1; k < len(w); k++ {
				c := w[k]
				if curl && c == 'O' {
					remoteName = true
					continue
				}
				if strings.IndexByte(valueFlags, c) < 0 {
					continue
				}
				val := w[k+1:]
				if val == "" {
					val = next()
				}
				if c == outFlag {
					out = append(out, val)
				}
				break
			}
		case isURL(w):
			urls = append(urls, w)
		}
	}
	if (curl && remoteName) || (!curl && len(out) == 0) {
		for _, u := range urls {
			u, _, _ = strings.Cut(u, "?")
			out = append(out, path.Base(u))
		}
	}
	return out
}

// checksumOperand reports whether the command is a checksum line: it
// starts with sha256sum or shasum -a 256 and holds the check flag. Its
// operand is the first word that is not a flag, "" when it names none.
func checksumOperand(words []string) (operand string, ok bool) {
	if len(words) == 0 {
		return "", false
	}
	rest := words[1:]
	switch path.Base(unquote(words[0])) {
	case "sha256sum":
	case "shasum":
		if !hasAlgorithm256(rest) {
			return "", false
		}
	default:
		return "", false
	}
	check := false
	for i := 0; i < len(rest); i++ {
		w := unquote(rest[i])
		switch {
		case w == "--check":
			check = true
		case w == "-a" || w == "--algorithm":
			i++
		case w == "-":
			if operand == "" {
				operand = "-"
			}
		case strings.HasPrefix(w, "--"):
		case strings.HasPrefix(w, "-"):
			check = check || strings.Contains(w, "c")
		case operand == "":
			operand = w
		}
	}
	return operand, check
}

// hasAlgorithm256 reports whether the words select the algorithm 256.
func hasAlgorithm256(words []string) bool {
	for i, w := range words {
		w = unquote(w)
		if (w == "-a" || w == "--algorithm") && i+1 < len(words) && unquote(words[i+1]) == "256" {
			return true
		}
		if w == "-a256" || w == "--algorithm=256" {
			return true
		}
	}
	return false
}

// unquote strips one layer of matching quotes from a word.
func unquote(w string) string {
	if len(w) >= 2 && (w[0] == '"' || w[0] == '\'') && w[len(w)-1] == w[0] {
		return w[1 : len(w)-1]
	}
	return w
}

// sameFile reports whether two paths name one file by their cleaned
// form or by their last element.
func sameFile(a, b string) bool {
	return path.Clean(a) == path.Clean(b) || path.Base(a) == path.Base(b)
}

// ciMise holds the version of mise of the pin file to the version input
// of the mise step of ci.yml (REQ-1068).
func (j *judge) ciMise() {
	if !j.havePins || j.pins.Mise == nil {
		return
	}
	src, ok := j.file(ciWorkflow)
	if !ok {
		j.add(ciWorkflow, "missing, so the mise version of %s equals no version input", pins.Path)
		return
	}
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(src, &wf); err != nil {
		j.add(ciWorkflow, "%v", err)
		return
	}
	found := false
	for _, job := range wf.Jobs {
		for _, s := range job.Steps {
			if !strings.HasPrefix(s.Uses, "jdx/mise-action@") {
				continue
			}
			found = true
			if v := s.With["version"]; v != j.pins.Mise.Version {
				j.add(ciWorkflow, "the version input of the mise step is %q, and %s pins mise %s (REQ-1068)", v, pins.Path, j.pins.Mise.Version)
			}
		}
	}
	if !found {
		j.add(ciWorkflow, "no step uses jdx/mise-action, so the mise version of %s equals no version input", pins.Path)
	}
}

// miseSignature verifies, offline, the signature of the committed
// SHASUMS256.txt with the committed key, and the pinned hashes against it
// (REQ-0844).
func (j *judge) miseSignature() {
	if !j.havePins || j.pins.Mise == nil {
		return
	}
	want := map[string]string{
		"mise-v" + j.pins.Mise.Version + "-linux-x64":   j.pins.Mise.SHA256.Amd64,
		"mise-v" + j.pins.Mise.Version + "-linux-arm64": j.pins.Mise.SHA256.Arm64,
	}
	var inputs [3][]byte
	for i, p := range []string{miseKey, miseSums, miseSig} {
		b, ok := j.file(p)
		if !ok {
			j.add(p, "missing, so the pin of mise in %s is not verified (07 7.4)", pins.Path)
			return
		}
		inputs[i] = b
	}
	key, err := minisign.ParsePublicKey(inputs[0])
	if err != nil {
		j.add(miseKey, "%v", err)
		return
	}
	trusted, err := minisign.Verify(key, inputs[1], inputs[2])
	if err != nil {
		j.add(miseSums, "the signature does not verify: %v", err)
		return
	}
	if !slices.Contains(strings.Split(trusted, "\t"), "file:SHASUMS256.txt") {
		j.add(miseSig, "the signed trusted comment does not name file:SHASUMS256.txt")
		return
	}
	listed := map[string][]string{}
	for line := range strings.Lines(string(inputs[1])) {
		if sum, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "  "); ok {
			name = strings.TrimPrefix(name, "./")
			listed[name] = append(listed[name], sum)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(want)) {
		switch sums := listed[name]; {
		case len(sums) == 0:
			j.add(miseSums, "lists no %s, the binary that %s pins", name, pins.Path)
		case len(sums) > 1:
			j.add(miseSums, "lists %s twice", name)
		case sums[0] != want[name]:
			j.add(pins.Path, "the pinned hash of %s is not the one the signed %s lists", name, miseSums)
		}
	}
}

// kitsAt reads the tree at head and applies the rules.
func kitsAt(ctx context.Context, repo git.Repo, head string) ([]check.Finding, error) {
	files, err := check.HeadTree(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	return Of(files), nil
}

// Run is the kits subcommand.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("kits takes no argument\n%s", check.Usage())
	}
	findings, err := kitsAt(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("kits", len(findings)))
	return len(findings) == 0, nil
}

// Judge is the kits check of the commit fast and all read before any
// step (Of).
func Judge(_ context.Context, c check.Commit) ([]check.Verdict, error) {
	return []check.Verdict{{Name: "kits", Findings: Of(c.Files)}}, nil
}
