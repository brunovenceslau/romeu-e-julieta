// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The v3 kit schema of sbx, copied from docker/sandbox-kit-spec at the
// tag kitSpecTag (third_party/sandbox-kit-spec/README.md): the tag, where
// the copy's files are in the tree, and the URL they are published at,
// which the descriptors' modeline names. The check reads the copy from
// the commit it judges, as it reads every other file.
const (
	kitSpecTag  = "v3.0.0-m.11"
	kitSpecRoot = "tools/ci/internal/schema/third_party/sandbox-kit-spec/schema"
	kitSpecURL  = "https://raw.githubusercontent.com/docker/sandbox-kit-spec/" + kitSpecTag + "/schema/"
	kitSchema   = "kit.schema.json"
	// kitName names the kit schema in a finding.
	kitName = kitSchema + " at " + kitSpecTag
)

// The two lines that open a v3 kit descriptor: the frontend pin (06 6.1)
// and, under examples/, the modeline of the official kit schema at the
// tag the check validates against.
const (
	pinsFile        = "kits/pins.yaml"
	kitSyntaxPrefix = "# syntax=docker/sandbox-kit:"
	kitModeline     = hintPrefix + kitSpecURL + kitSchema
)

// compileKit compiles the kit schema from the copy's files in the tree
// files, each added at the URL it is published at, so its relative
// references resolve to the copies and no other location is read. A
// tree without the copy has none: nil and no error.
func compileKit(files []check.TreeFile) (*jsonschema.Schema, error) {
	type result struct {
		sch *jsonschema.Schema
		err error
	}
	r, ok := within(kitDeadline, func() result {
		sch, err := compileKitFiles(files)
		return result{sch, err}
	})
	if !ok {
		return nil, fmt.Errorf("compiling the kit schema takes more than %s", kitDeadline)
	}
	return r.sch, r.err
}

// compileKitFiles is compileKit without the deadline.
func compileKitFiles(files []check.TreeFile) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{})
	c.AssertFormat()
	found := false
	for _, f := range files {
		name, ok := strings.CutPrefix(f.Path, kitSpecRoot+"/")
		if !ok || path.Ext(name) != ".json" {
			continue
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(f.Content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Path, err)
		}
		if err := c.AddResource(kitSpecURL+name, doc); err != nil {
			return nil, err
		}
		found = true
	}
	if !found {
		return nil, nil
	}
	return c.Compile(kitSpecURL + kitSchema)
}

// isKitDescriptor reports whether f is a v3 kit descriptor the check
// judges: a YAML file under examples/ whose first line is the frontend
// pin. The product kits under kits/ are not judged yet; a separate task,
// decided on 2026-10-22, judges kits/ (kits/git-ssh-sign builds on the
// pinned frontend since its ssh-agent capability names a phase).
func isKitDescriptor(f check.TreeFile) bool {
	ext := path.Ext(f.Path)
	if ext != ".yaml" && ext != ".yml" || !strings.HasPrefix(f.Path, examplesDir) {
		return false
	}
	return bytes.HasPrefix(f.Content, []byte(kitSyntaxPrefix))
}

// kitDeadline is how long compiling the kit schema, or validating one
// descriptor against it, may take. The jsonschema library takes no
// context, so the work runs in a goroutine the check stops waiting for:
// a schema or a document that makes it loop fails closed, in seconds,
// and the process, which ends after the check, drops the goroutine.
const kitDeadline = 10 * time.Second

// within runs fn and returns its result, or false when fn has not
// returned after d.
func within[T any](d time.Duration, fn func() T) (T, bool) {
	done := make(chan T, 1)
	go func() { done <- fn() }()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case v := <-done:
		return v, true
	case <-timer.C:
		var zero T
		return zero, false
	}
}

// hasKitCopy reports whether the tree holds a file of the copy of the
// kit schema.
func hasKitCopy(files []check.TreeFile) bool {
	return slices.ContainsFunc(files, func(f check.TreeFile) bool { return strings.HasPrefix(f.Path, kitSpecRoot+"/") })
}

// copyFindings holds the copy of the kit schema in the tree to the
// digests of kitSpecDigests: each file of the copy is pinned and has
// its digest, and each pinned file is there. A tree with no file of the
// copy has none to hold.
func copyFindings(files []check.TreeFile) []check.Finding {
	var findings []check.Finding
	seen := map[string]bool{}
	for _, f := range files {
		name, ok := strings.CutPrefix(f.Path, kitSpecRoot+"/")
		if !ok {
			continue
		}
		seen[name] = true
		want, pinned := kitSpecDigests[name]
		sum := sha256.Sum256(f.Content)
		switch {
		case !pinned:
			findings = append(findings, check.Finding{Where: f.Path, Rule: ruleCopy, Msg: "is no file of the copy of the kit schema at " + kitSpecTag + " that kits_digests.go pins"})
		case hex.EncodeToString(sum[:]) != want:
			findings = append(findings, check.Finding{Where: f.Path, Rule: ruleCopy, Msg: "differs from the digest pinned for it in kits_digests.go: the copy of the kit schema at " + kitSpecTag + " changes only with its pin"})
		}
	}
	for name := range kitSpecDigests {
		if !seen[name] {
			findings = append(findings, check.Finding{Where: kitSpecRoot + "/" + name, Rule: ruleCopy, Msg: "is pinned in kits_digests.go and is not in the tree"})
		}
	}
	return findings
}

// frontendRef returns the frontend ref of kits/pins.yaml (02 2.1,
// REQ-0117), the one pin the descriptors' first line and kitSpecTag
// follow, or why it cannot.
func frontendRef(files []check.TreeFile) (string, error) {
	data, ok, err := check.FileIn(files, pinsFile)
	if err != nil || !ok {
		return "", errors.New(pinsFile + " is not a file")
	}
	var pins struct {
		Frontend struct {
			Ref string `yaml:"ref"`
		} `yaml:"frontend"`
	}
	if err := yaml.Unmarshal(data, &pins); err != nil || pins.Frontend.Ref == "" {
		return "", errors.New(pinsFile + " holds no frontend ref")
	}
	return pins.Frontend.Ref, nil
}

// pinFindings holds kitSpecTag to the frontend ref of kits/pins.yaml:
// the ref's tag is kitSpecTag without its v, so the vendored schema is
// the one of the frontend the kits are built with (REQ-0117).
func pinFindings(ref string, refErr error) []check.Finding {
	if refErr != nil {
		return []check.Finding{{Where: pinsFile, Rule: rulePin, Msg: refErr.Error() + ", so the vendored kit schema at " + kitSpecTag + " follows no pin"}}
	}
	_, tag, _ := strings.Cut(strings.SplitN(ref, "@", 2)[0], ":")
	if "v"+tag != kitSpecTag {
		return []check.Finding{{Where: pinsFile, Rule: rulePin, Msg: "pins the frontend at tag \"" + tag + "\", and the vendored kit schema is " + kitSpecTag + " (kitSpecTag, tools/ci/internal/schema/kits.go); copy the schema of the pinned tag and change kitSpecTag together"}}
	}
	return nil
}

// kitFindings judges one kit descriptor under examples/: its first line
// is the frontend pin of kits/pins.yaml (ref), its second line is the
// modeline of the kit schema at kitSpecTag, and it validates against
// that schema.
func kitFindings(f check.TreeFile, sch *jsonschema.Schema, ref string, refErr error) []check.Finding {
	if sch == nil {
		return []check.Finding{{Where: f.Path, Rule: ruleUnknown, Msg: "is a v3 kit descriptor, and the tree holds no copy of the kit schema at " + kitSpecRoot}}
	}
	lines := strings.SplitN(string(f.Content), "\n", 3)
	if want := "# syntax=" + ref; refErr == nil && strings.TrimSuffix(lines[0], "\r") != want {
		return []check.Finding{{Where: f.Path, Rule: rulePin, Msg: "is a v3 kit descriptor, and its first line is not \"" + want + "\", the frontend ref of " + pinsFile + " (02 2.1)"}}
	}
	if len(lines) < 2 || strings.TrimSuffix(lines[1], "\r") != kitModeline {
		return []check.Finding{{Where: f.Path, Rule: ruleHint, Msg: "is a v3 kit descriptor, and its second line is not \"" + kitModeline + "\" (10 10.2, REQ-0836)"}}
	}
	doc, err := parse(f)
	if err != nil {
		return []check.Finding{{Where: f.Path, Rule: ruleParse, Msg: err.Error()}}
	}
	findings, ok := within(kitDeadline, func() []check.Finding { return validate(f.Path, kitName, doc, compiled{kitName: sch}) })
	if !ok {
		return []check.Finding{{Where: f.Path, Rule: ruleTimeout, Msg: "is not validated against " + kitName + " within " + kitDeadline.String() + ", and the check stops there"}}
	}
	return findings
}
