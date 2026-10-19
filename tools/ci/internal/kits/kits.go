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
//   - An install step is the command of a lifecycle@1 install hook, a
//     RUN instruction of a dockerfile, or the build: text of a descriptor.
//     A step holds five lines or fewer (REQ-1043): the lines that are
//     neither blank nor a comment, a line continued by a backslash
//     counting as each of its physical lines.
//   - A download is a line whose first word is curl or wget. The same
//     step has a sha256sum -c or shasum -a 256 -c line after it; the
//     download line holds no "|", ";", "&&" or "$("; and the file that
//     checksum line reads is not one the step downloaded. A download
//     written with another tool is not seen, and the review of the kits
//     surface covers it.
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
	// syntaxPrefix opens the frontend line of a descriptor (REQ-0564).
	syntaxPrefix = "# syntax=docker/sandbox-kit:"
	rule         = "kits"
)

var (
	// checksumLine finds a checksum command that checks: sha256sum or
	// shasum -a 256, and -c or --check after it, on one command.
	checksumLine = regexp.MustCompile(`(?:^|[\s|;&(])(?:sha256sum|shasum\s+-a\s+256)(?:\s+[^\s|;&]+)*?\s+(?:-c|--check)(?:\s|$)`)
	// checksumCmd locates the command word to read its operands.
	checksumCmd = regexp.MustCompile(`(?:^|[\s|;&(])(sha256sum|shasum\s+-a\s+256)(?:\s|$)`)
	// forbidden are the shell constructs a download line may not hold.
	forbidden = []string{"|", ";", "&&", "$("}
	// runInstruction is a Dockerfile RUN, and runFlag one of its flags
	// (--mount=...).
	runInstruction = regexp.MustCompile(`(?i)^RUN\s+`)
	runFlag        = regexp.MustCompile(`^--[a-z-]+(=\S+)?\s+`)
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
	findings []check.Finding
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

// kits judges every kit, then the frontends together.
func (j *judge) kits() {
	names, byKit := j.kitFiles()
	frontends := map[string]string{} // frontend ref -> the first file that carries it
	var order []string
	for _, name := range names {
		for _, f := range byKit[name] {
			switch base := path.Base(f.path); {
			case isDescriptor(base):
				j.descriptor(f, frontends, &order)
			case path.Ext(base) == ".dockerfile":
				j.dockerfile(f, frontends, &order)
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

// frontendLines judges the frontend lines of one file: the first line of
// a descriptor must be one, and a line of that form further down is a
// second pin.
func (j *judge) frontendLines(f kitFile, mustStart bool, frontends map[string]string, order *[]string) {
	for i, line := range strings.Split(string(f.content), "\n") {
		line = strings.TrimRight(line, "\r ")
		if !strings.HasPrefix(line, syntaxPrefix) {
			if i == 0 && mustStart {
				j.add(f.path, "line 1 is not the frontend line, %q followed by the frontend of %s (REQ-0564)", syntaxPrefix, pins.Path)
			}
			continue
		}
		if i != 0 {
			j.add(f.path, "line %d is a frontend line below line 1: a second frontend pin", i+1)
			continue
		}
		ref := strings.TrimPrefix(line, "# syntax=")
		if _, seen := frontends[ref]; !seen {
			frontends[ref] = f.path
			*order = append(*order, ref)
		}
		if j.havePins && j.pins.Frontend != nil && ref != j.pins.Frontend.Ref {
			j.add(f.path, "the frontend differs from the pin of %s", pins.Path)
		}
	}
}

// descriptor judges one descriptor.
func (j *judge) descriptor(f kitFile, frontends map[string]string, order *[]string) {
	j.frontendLines(f, true, frontends, order)
	var d struct {
		Args map[string]struct {
			Default *string `yaml:"default"`
		} `yaml:"args"`
		Build        string `yaml:"build"`
		Capabilities []struct {
			Type   string `yaml:"type"`
			Config struct {
				Install []struct {
					Command any `yaml:"command"`
				} `yaml:"install"`
			} `yaml:"config"`
		} `yaml:"capabilities"`
	}
	if err := yaml.NewDecoder(bytes.NewReader(f.content)).Decode(&d); err != nil && !errors.Is(err, io.EOF) {
		j.add(f.path, "%v", err)
		return
	}
	if j.havePins {
		for _, a := range pinArgs {
			arg, ok := d.Args[a.name]
			if !ok {
				continue
			}
			want, pinned := a.pin(j.pins)
			switch {
			case !pinned:
				j.add(f.path, "the arg %s has no pin in %s to equal", a.name, pins.Path)
			case arg.Default == nil || *arg.Default != want:
				j.add(f.path, "the default of the arg %s differs from the pin of %s", a.name, pins.Path)
			}
		}
	}
	for _, capability := range d.Capabilities {
		if !strings.HasPrefix(capability.Type, "com.docker.sandbox/lifecycle@") {
			continue
		}
		for i, hook := range capability.Config.Install {
			j.step(f.path, fmt.Sprintf("install step %d", i+1), commandText(hook.Command))
		}
	}
	if d.Build != "" {
		j.runSteps(f.path, d.Build)
	}
}

// dockerfile judges one dockerfile.
func (j *judge) dockerfile(f kitFile, frontends map[string]string, order *[]string) {
	j.frontendLines(f, false, frontends, order)
	j.runSteps(f.path, string(f.content))
}

// commandText returns the script of an install command: a string is the
// script, a list of sh -c and a script is that script, and any other
// list is one line.
func commandText(command any) string {
	switch v := command.(type) {
	case string:
		return v
	case []any:
		words := make([]string, len(v))
		for i, w := range v {
			words[i] = fmt.Sprint(w)
		}
		if len(words) == 3 && slices.Contains([]string{"sh", "/bin/sh", "bash"}, words[0]) && words[1] == "-c" {
			return words[2]
		}
		return strings.Join(words, " ")
	}
	return ""
}

// runSteps judges each RUN instruction of the dockerfile text src as a
// step. An instruction ends at its first line that does not end in a
// backslash.
func (j *judge) runSteps(where, src string) {
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !runInstruction.MatchString(line) {
			continue
		}
		start := i
		body := []string{runInstruction.ReplaceAllString(line, "")}
		for strings.HasSuffix(strings.TrimRight(lines[i], " \t\r"), "\\") && i+1 < len(lines) {
			i++
			body = append(body, lines[i])
		}
		body[0] = runFlag.ReplaceAllString(body[0], "")
		j.step(where, fmt.Sprintf("the RUN step at line %d", start+1), strings.Join(body, "\n"))
	}
}

// step judges the script of one install step.
func (j *judge) step(where, label, script string) {
	var physical, logical, cur []string
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
		logical = append(logical, strings.Join(append(cur, t), " "))
		cur = nil
	}
	if len(cur) > 0 {
		logical = append(logical, strings.Join(cur, " "))
	}
	if len(physical) > maxStepLines {
		j.add(where, "%s has %d lines, over the %d of REQ-1043", label, len(physical), maxStepLines)
	}
	for i, line := range logical {
		if !isDownload(line) {
			continue
		}
		for _, bad := range forbidden {
			if strings.Contains(line, bad) {
				j.add(where, "%s: the download on command %d holds %q", label, i+1, bad)
			}
		}
		var sums []string
		for _, later := range logical[i+1:] {
			if checksumLine.MatchString(later) {
				sums = append(sums, later)
			}
		}
		if len(sums) == 0 {
			j.add(where, "%s: the download on command %d has no sha256sum -c line after it (REQ-0606)", label, i+1)
			continue
		}
		got := downloaded(line)
		for _, sum := range sums {
			operand := checksumOperand(sum)
			if operand != "" && slices.ContainsFunc(got, func(d string) bool { return sameFile(d, operand) }) {
				j.add(where, "%s: the checksum line reads %s, a file the step downloaded, and not a kit file", label, git.Printable(operand))
			}
		}
	}
}

// isDownload reports whether the first word of the line is curl or wget.
func isDownload(line string) bool {
	w, _, _ := strings.Cut(line, " ")
	return w == "curl" || w == "wget"
}

// unquote strips one layer of matching quotes from a word.
func unquote(w string) string {
	if len(w) >= 2 && (w[0] == '"' || w[0] == '\'') && w[len(w)-1] == w[0] {
		return w[1 : len(w)-1]
	}
	return w
}

// downloaded returns the files a download line writes: the argument of
// -o or --output (curl) or -O or --output-document (wget), and, for curl
// -O and for wget with no output option, the base name of each URL.
func downloaded(line string) []string {
	words := strings.Fields(line)
	curl := words[0] == "curl"
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
		case curl && w == "-o", curl && w == "--output", !curl && (w == "-O" || w == "--output-document"):
			out = append(out, next())
		case strings.HasPrefix(w, "--output="):
			out = append(out, strings.TrimPrefix(w, "--output="))
		case strings.HasPrefix(w, "--output-document="):
			out = append(out, strings.TrimPrefix(w, "--output-document="))
		case curl && strings.HasPrefix(w, "-o") && len(w) > 2:
			out = append(out, w[2:])
		case !curl && strings.HasPrefix(w, "-O") && len(w) > 2:
			out = append(out, w[2:])
		case curl && (w == "--remote-name" || (strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.Contains(w, "O"))):
			remoteName = true
		case strings.HasPrefix(w, "http://"), strings.HasPrefix(w, "https://"), strings.HasPrefix(w, "ftp://"):
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

// checksumOperand returns the file the checksum command of the line
// reads, "" when it reads the standard input or names none.
func checksumOperand(line string) string {
	loc := checksumCmd.FindStringSubmatchIndex(line)
	if loc == nil {
		return ""
	}
	rest := line[loc[3]:]
	if end := strings.IndexAny(rest, "|;&"); end >= 0 {
		rest = rest[:end]
	}
	words := strings.Fields(rest)
	for i := 0; i < len(words); i++ {
		switch w := unquote(words[i]); {
		case w == "<" && i+1 < len(words):
			return unquote(words[i+1])
		case w == "-a" || w == "--algorithm":
			i++
		case w == "-" || !strings.HasPrefix(w, "-"):
			if w == "-" {
				return ""
			}
			return w
		}
	}
	return ""
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
