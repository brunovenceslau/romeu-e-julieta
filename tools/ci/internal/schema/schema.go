// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package schema is the schema check of 10 10.2 (REQ-0836) and the
// Schema level of 10 10.1: the schemas under schemas/ equal the ones
// tools/schemagen generates (REQ-0733), the files under examples/ and
// the testdata files that name a format validate against its schema
// (REQ-0734), and each YAML file under examples/ carries the $schema
// hint of the release VERSION names (REQ-1249, 03 3.2), or, for a kit
// descriptor, the modeline of the official kit schema (REQ-0836). It
// also holds the generator that writes schemas/*.json ("go generate
// ./...").
//
// The files it judges are those of the commit's tree, each read once:
//
//   - every registered format's schemas/<name>.json, and every other
//     file under schemas/, which no format generates;
//   - every YAML file under examples/, which validates against the
//     schema its hint names, and every JSON file there, which validates
//     against the format its "schema" field names;
//   - every v3 kit descriptor (a YAML file whose first line is the
//     frontend pin) under examples/, which carries the modeline of the
//     kit schema of sbx at kitSpecTag on its second line, in place of a
//     hint of ours, and validates against that schema, copied here
//     (kits.go);
//   - every YAML or JSON file below a testdata directory whose "schema"
//     field has the form "<name>/v<n>" (REQ-1250): a format no
//     registered type generates is a finding; the file validates,
//     unless its name starts with "invalid", in which case it must not,
//     so a fixture that a decoder refuses stays one the schema refuses
//     too. A testdata file that does not parse, or has no such field,
//     is the fixture of something else and is not judged.
//
// Validation asserts "format" (date-time and the other formats of JSON
// Schema 2020-12), rather than reading it as an annotation only.
package schema

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The paths the check reads, from the root of the repository.
const (
	schemasDir  = "schemas/"
	examplesDir = "examples/"
	versionFile = "VERSION"
	goModFile   = "go.mod"
)

// The rules of the findings, one name per rule, each "schema <rule>".
const (
	ruleDrift   = "schema drift"
	ruleMissing = "schema missing"
	ruleOrphan  = "schema orphan"
	ruleInvalid = "schema invalid"
	ruleValid   = "schema valid"
	ruleParse   = "schema parse"
	ruleUnknown = "schema unknown"
	ruleHint    = "schema hint"
	ruleVersion = "schema version"
	rulePin     = "schema pin"
	ruleCopy    = "schema copy"
)

// hintPrefix is the start of the hint line of 03 3.2, which the
// yaml-language-server reads; the URL follows it.
const hintPrefix = "# yaml-language-server: $schema="

// validVersion is the content of VERSION: the next release and a
// newline (REQ-1249).
var validVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\n$`)

// modulePath reads the module line of go.mod.
var modulePath = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

// Comment returns the generated-file header of the document of s, as
// one string for its "$comment" (12 12.3; JSON has no comment syntax).
func Comment(s schemagen.Schema) string {
	return strings.Join(check.GeneratedHeader("tools/schemagen (the Go type "+s.Source()+")"), "\n")
}

// Outputs returns schemas/<name>.json for each of schemas: the files
// "go generate ./..." writes and the check holds the tree to.
func Outputs(schemas []schemagen.Schema) ([]check.Output, error) {
	var outs []check.Output
	for _, s := range schemas {
		doc, err := schemagen.Generate(s, Comment(s))
		if err != nil {
			return nil, err
		}
		outs = append(outs, check.Output{Path: s.Path(), Content: doc})
	}
	return outs, nil
}

// compiled is the set of compiled schemas, by schema name.
type compiled map[string]*jsonschema.Schema

// compile compiles each generated document. No schema reaches another
// location: the loader refuses every URL that is not one of them.
func compile(outs []check.Output) (compiled, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{})
	c.AssertFormat()
	urls := map[string]string{}
	for _, o := range outs {
		name := strings.TrimSuffix(strings.TrimPrefix(o.Path, schemasDir), ".json")
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(o.Content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", o.Path, err)
		}
		urls[name] = "https://romeu-e-julieta.invalid/" + o.Path
		if err := c.AddResource(urls[name], doc); err != nil {
			return nil, fmt.Errorf("%s: %w", o.Path, err)
		}
	}
	out := compiled{}
	for name, url := range urls {
		sch, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", url, err)
		}
		out[name] = sch
	}
	return out, nil
}

// errNoKitSchema is the error of a kit schema that does not compile.
var errNoKitSchema = errors.New("the vendored kit schema does not compile")

// Of returns the findings of the check over files, the tree of one
// commit, for the registered formats schemas, sorted by place. A
// format the generator cannot express is an error.
func Of(files []check.TreeFile, schemas []schemagen.Schema) ([]check.Finding, error) {
	outs, err := Outputs(schemas)
	if err != nil {
		return nil, err
	}
	sch, err := compile(outs)
	if err != nil {
		return nil, err
	}
	kit, err := compileKit(files)
	findings := driftFindings(files, outs)
	// A copy that holds a file other than the pinned ones, or lacks one,
	// is reported as such, and the descriptors are not judged against it.
	ref, refErr := frontendRef(files)
	broken := false
	if hasKitCopy(files) {
		findings = append(findings, pinFindings(ref, refErr)...)
		copyF := copyFindings(files)
		findings = append(findings, copyF...)
		broken = err != nil && len(copyF) > 0
	}
	if err != nil && !broken {
		return nil, fmt.Errorf("%w: %w", errNoKitSchema, err)
	}
	hint, hintErr := hintBase(files)
	for _, f := range files {
		if f.Mode == "160000" {
			continue
		}
		switch {
		case isKitDescriptor(f) && broken:
			continue
		case isKitDescriptor(f):
			findings = append(findings, kitFindings(f, kit, ref, refErr)...)
		case strings.HasPrefix(f.Path, examplesDir):
			findings = append(findings, exampleFindings(f, sch, hint, hintErr)...)
		case underTestdata(f.Path):
			findings = append(findings, testdataFindings(f, sch)...)
		}
	}
	slices.SortStableFunc(findings, func(a, b check.Finding) int { return strings.Compare(a.Where, b.Where) })
	return findings, nil
}

// driftFindings compares each generated schema with the committed one,
// and names each file under schemas/ that no format generates.
func driftFindings(files []check.TreeFile, outs []check.Output) []check.Finding {
	var findings []check.Finding
	for _, o := range outs {
		got, ok, err := check.FileIn(files, o.Path)
		switch {
		case err != nil:
			findings = append(findings, check.Finding{Where: o.Path, Rule: ruleMissing, Msg: "is not a file"})
		case !ok:
			findings = append(findings, check.Finding{Where: o.Path, Rule: ruleMissing, Msg: "is not committed; run go generate ./..."})
		case !bytes.Equal(got, o.Content):
			findings = append(findings, check.Finding{Where: o.Path, Rule: ruleDrift, Msg: "differs from the schema tools/schemagen generates; run go generate ./..."})
		}
	}
	for _, f := range files {
		if strings.HasPrefix(f.Path, schemasDir) && !slices.ContainsFunc(outs, func(o check.Output) bool { return o.Path == f.Path }) {
			findings = append(findings, check.Finding{Where: f.Path, Rule: ruleOrphan, Msg: "no format registered with tools/schemagen generates it"})
		}
	}
	return findings
}

// hintBase returns the start of the hint URL of the release VERSION
// names, up to the schema file: the raw URL of the tag "v<version>" of
// the repository that go.mod names (03 3.2), or why it cannot.
func hintBase(files []check.TreeFile) (string, error) {
	version, ok, err := check.FileIn(files, versionFile)
	if err != nil || !ok {
		return "", errors.New("VERSION is not committed, so no release names the hint (REQ-1249)")
	}
	if !validVersion.Match(version) {
		return "", errors.New("VERSION holds no <major>.<minor>.<patch> and a newline (REQ-1249)")
	}
	mod, _, _ := check.FileIn(files, goModFile)
	repo, ok := "", false
	if m := modulePath.FindSubmatch(mod); m != nil {
		repo, ok = strings.CutPrefix(string(m[1]), "github.com/")
	}
	if !ok {
		return "", errors.New("go.mod names no github.com module, so no repository names the hint")
	}
	return "https://raw.githubusercontent.com/" + repo + "/v" + strings.TrimSuffix(string(version), "\n") + "/" + schemasDir, nil
}

// exampleFindings judges one file under examples/: a YAML file carries
// the hint and validates against the schema it names; a JSON file
// validates against the format its schema field names.
func exampleFindings(f check.TreeFile, sch compiled, hint string, hintErr error) []check.Finding {
	switch path.Ext(f.Path) {
	case ".yaml", ".yml":
		if hintErr != nil {
			return []check.Finding{{Where: f.Path, Rule: ruleVersion, Msg: hintErr.Error()}}
		}
		url, found := hintOf(f.Content)
		name, ok := strings.CutPrefix(url, hint)
		name, isJSON := strings.CutSuffix(name, ".json")
		if !found || !ok || !isJSON || strings.Contains(name, "/") {
			return []check.Finding{{Where: f.Path, Rule: ruleHint, Msg: "carries no $schema hint of the release VERSION names: its first comment lines hold \"" + hintPrefix + hint + "<name>.json\" (03 3.2)"}}
		}
		doc, err := parse(f)
		if err != nil {
			return []check.Finding{{Where: f.Path, Rule: ruleParse, Msg: err.Error()}}
		}
		return validate(f.Path, name, doc, sch)
	case ".json":
		doc, err := parse(f)
		if err != nil {
			return []check.Finding{{Where: f.Path, Rule: ruleParse, Msg: err.Error()}}
		}
		name, ok := formatOf(doc)
		if !ok {
			return []check.Finding{{Where: f.Path, Rule: ruleUnknown, Msg: "has no schema field of the form <name>/v<n> (REQ-0161)"}}
		}
		return validate(f.Path, name, doc, sch)
	}
	return nil
}

// testdataFindings judges one YAML or JSON file below a testdata
// directory that names a registered format; see the package comment.
func testdataFindings(f check.TreeFile, sch compiled) []check.Finding {
	switch path.Ext(f.Path) {
	case ".yaml", ".yml", ".json":
	default:
		return nil
	}
	doc, err := parse(f)
	if err != nil {
		return nil
	}
	name, ok := formatOf(doc)
	if !ok {
		return nil
	}
	if sch[name] == nil {
		return []check.Finding{{Where: f.Path, Rule: ruleUnknown, Msg: name + " is no format registered with tools/schemagen"}}
	}
	if strings.HasPrefix(path.Base(f.Path), "invalid") {
		if sch[name].Validate(doc) == nil {
			return []check.Finding{{Where: f.Path, Rule: ruleValid, Msg: "validates against " + name + ", and a fixture named invalid must not"}}
		}
		return nil
	}
	return validate(f.Path, name, doc, sch)
}

// underTestdata reports whether p lies below a directory named testdata.
func underTestdata(p string) bool {
	return slices.Contains(strings.Split(path.Dir(p), "/"), "testdata")
}

// hintOf returns the URL of the hint among the comment lines that open
// a YAML file, and whether there is one.
func hintOf(content []byte) (string, bool) {
	for line := range strings.Lines(string(content)) {
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "#") {
			break
		}
		if url, ok := strings.CutPrefix(line, hintPrefix); ok {
			return url, true
		}
	}
	return "", false
}

// parse decodes a YAML or JSON file into the values a JSON decoder
// gives: objects as map[string]any, arrays as []any. A duplicate key is
// an error in both, as in the strict decoders of 03 (opening).
func parse(f check.TreeFile) (any, error) {
	var doc any
	if path.Ext(f.Path) == ".json" {
		if err := jsonv2.Unmarshal(f.Content, &doc); err != nil {
			return nil, errors.New("does not decode as one JSON value, or repeats a member name")
		}
		return doc, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(f.Content))
	var next any
	if err := dec.Decode(&doc); err != nil || !errors.Is(dec.Decode(&next), io.EOF) {
		return nil, errors.New("does not decode as one YAML document, or repeats a key")
	}
	return jsonValue(doc)
}

// jsonValue returns v with every mapping a map[string]any, or an error
// for a mapping key that is not a string, which no JSON object holds.
func jsonValue(v any) (any, error) {
	switch v := v.(type) {
	case map[string]any:
		for k, e := range v {
			j, err := jsonValue(e)
			if err != nil {
				return nil, err
			}
			v[k] = j
		}
		return v, nil
	case map[any]any:
		return nil, errors.New("has a mapping key that is not a string")
	case []any:
		for i, e := range v {
			j, err := jsonValue(e)
			if err != nil {
				return nil, err
			}
			v[i] = j
		}
		return v, nil
	default:
		return v, nil
	}
}

// formatOf returns the schema name the "schema" field of doc names,
// "<name>.v<n>" for "<name>/v<n>" (REQ-0161), and whether it names one.
func formatOf(doc any) (string, bool) {
	obj, _ := doc.(map[string]any)
	field, _ := obj["schema"].(string)
	name, version, ok := strings.Cut(field, "/")
	if !ok || name == "" || strings.Contains(version, "/") || !strings.HasPrefix(version, "v") {
		return "", false
	}
	return name + "." + version, true
}

// validate validates doc against the schema of name, and returns a
// finding that names where it fails and which keyword, never the
// value: CI logs are public (check.Finding).
func validate(where, name string, doc any, sch compiled) []check.Finding {
	s, ok := sch[name]
	if !ok {
		return []check.Finding{{Where: where, Rule: ruleUnknown, Msg: name + " is no format registered with tools/schemagen"}}
	}
	err := s.Validate(doc)
	if err == nil {
		return nil
	}
	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return []check.Finding{{Where: where, Rule: ruleInvalid, Msg: "does not validate against " + name}}
	}
	var places []string
	for _, leaf := range leaves(verr) {
		places = append(places, "/"+strings.Join(leaf.InstanceLocation, "/")+" ("+strings.Join(leaf.ErrorKind.KeywordPath(), "/")+")")
	}
	slices.Sort(places)
	return []check.Finding{{Where: where, Rule: ruleInvalid, Msg: "does not validate against " + name + " at " + strconv.Quote(strings.Join(slices.Compact(places), ", "))}}
}

// leaves returns the errors of e that have no causes of their own.
func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

// Judge is the schema check of the commit all reads before any step,
// for the formats registered with tools/schemagen.
func Judge(_ context.Context, c check.Commit) ([]check.Verdict, error) {
	findings, err := Of(c.Files, schemagen.Registered())
	return []check.Verdict{{Name: "schema", Findings: findings, Err: err}}, nil
}

// Run applies the check to the tree at HEAD, for the formats registered
// with tools/schemagen.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	return run(ctx, e, args, schemagen.Registered())
}

// run applies the check to the tree at HEAD for schemas.
func run(ctx context.Context, e check.Env, args []string, schemas []schemagen.Schema) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("schema takes no argument\n%s", check.Usage())
	}
	files, err := check.HeadTree(ctx, e.Repo(), "HEAD")
	if err != nil {
		return false, err
	}
	findings, err := Of(files, schemas)
	if err != nil {
		return false, err
	}
	check.Say(e.Stdout, "%s%s\n", check.Lines(findings), check.Summary("schema", len(findings)))
	return len(findings) == 0, nil
}
