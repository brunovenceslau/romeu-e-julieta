// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

// Package prose applies the word lists of the documentation standard
// (ADR 0001, rule 4): words and phrases this repository does not use.
// The lists are data, in tools/ci/prose.yaml, and each rule carries
// samples of its own; a list whose samples disagree with it does not
// load, so an edit that empties a rule is seen at once.
package prose

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Path is where the lists live, relative to the repository root.
const Path = "tools/ci/prose.yaml"

// Hit is one match: the 1-based line where it starts and the id of the
// rule it breaks.
type Hit struct {
	Line int
	Rule string
}

// Rules is a loaded set of word lists that passed its self-test.
type Rules struct {
	rules []rule
}

// rule is one list. A phrase is a run of words; a pair is two phrases
// that match when the second follows the first in one paragraph.
type rule struct {
	id      string
	phrases [][]string
	pairs   [][2][]string
}

// The two types below are the file format.
type fileRules struct {
	Rules []fileRule `yaml:"rules"`
}

type fileRule struct {
	ID      string     `yaml:"id"`
	Phrases []string   `yaml:"phrases"`
	Pairs   [][]string `yaml:"pairs"`
	Fails   []string   `yaml:"fails"`
	Passes  []string   `yaml:"passes"`
}

// Parse decodes the lists and runs their self-test: each sample under
// fails breaks its own rule, each sample under passes breaks no rule,
// and each phrase and pair is reached by a fails sample.
func Parse(data []byte) (*Rules, error) {
	var f fileRules
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode prose lists: %w", err)
	}
	if len(f.Rules) == 0 {
		return nil, errors.New("prose lists: no rule")
	}
	r := &Rules{}
	for i, fr := range f.Rules {
		ru, err := newRule(fr)
		if err != nil {
			return nil, fmt.Errorf("prose rule %d: %w", i+1, err)
		}
		if slices.Contains(r.IDs(), ru.id) {
			return nil, fmt.Errorf("prose rule %d: the id %q is used twice", i+1, ru.id)
		}
		r.rules = append(r.rules, ru)
	}
	for i, fr := range f.Rules {
		if err := r.selfTest(r.rules[i], fr); err != nil {
			return nil, fmt.Errorf("prose rule %q: %w", fr.ID, err)
		}
	}
	return r, nil
}

func newRule(fr fileRule) (rule, error) {
	if fr.ID == "" {
		return rule{}, errors.New("the rule has no id")
	}
	ru := rule{id: fr.ID}
	for _, p := range fr.Phrases {
		w := phraseWords(p)
		if len(w) == 0 {
			return rule{}, fmt.Errorf("the phrase %q has no word", p)
		}
		ru.phrases = append(ru.phrases, w)
	}
	for _, p := range fr.Pairs {
		if len(p) != 2 {
			return rule{}, fmt.Errorf("the pair %q does not have two parts", p)
		}
		first, second := phraseWords(p[0]), phraseWords(p[1])
		if len(first) == 0 || len(second) == 0 {
			return rule{}, fmt.Errorf("a part of the pair %q has no word", p)
		}
		ru.pairs = append(ru.pairs, [2][]string{first, second})
	}
	if len(ru.phrases) == 0 && len(ru.pairs) == 0 {
		return rule{}, errors.New("the rule lists nothing")
	}
	return ru, nil
}

func (r *Rules) selfTest(ru rule, fr fileRule) error {
	if len(fr.Fails) == 0 {
		return errors.New("the rule has no fails sample")
	}
	if len(fr.Passes) == 0 {
		return errors.New("the rule has no passes sample")
	}
	only := &Rules{rules: []rule{ru}}
	for _, s := range fr.Fails {
		if len(only.Find([]byte(s))) == 0 {
			return fmt.Errorf("the fails sample %q does not break the rule", s)
		}
	}
	for _, s := range fr.Passes {
		if hits := r.Find([]byte(s)); len(hits) != 0 {
			return fmt.Errorf("the passes sample %q breaks the rule %q", s, hits[0].Rule)
		}
	}
	// Each listed item is tried alone against the fails samples, so an
	// item that can never match is not carried along unnoticed.
	for _, p := range ru.phrases {
		alone := &Rules{rules: []rule{{id: ru.id, phrases: [][]string{p}}}}
		if !alone.findsAny(fr.Fails) {
			return fmt.Errorf("the phrase %q has no fails sample", strings.Join(p, " "))
		}
	}
	for _, p := range ru.pairs {
		alone := &Rules{rules: []rule{{id: ru.id, pairs: [][2][]string{p}}}}
		if !alone.findsAny(fr.Fails) {
			return fmt.Errorf("the pair %q has no fails sample", strings.Join(p[0], " "))
		}
	}
	return nil
}

func (r *Rules) findsAny(samples []string) bool {
	for _, s := range samples {
		if len(r.Find([]byte(s))) > 0 {
			return true
		}
	}
	return false
}

// IDs returns the rule ids in file order.
func (r *Rules) IDs() []string {
	ids := make([]string, 0, len(r.rules))
	for _, ru := range r.rules {
		ids = append(ids, ru.id)
	}
	return ids
}

// Find returns the hits in content, in the order of their lines. A
// listed word or phrase matches as whole words, with ASCII case
// ignored, outside code spans and fenced blocks, so a page can quote
// what it bans. Words are matched inside one paragraph, because prose
// is wrapped and a phrase may run over a line end.
func (r *Rules) Find(content []byte) []Hit {
	var hits []Hit
	for _, para := range paragraphs(content) {
		words := paragraphWords(para)
		for i := range words {
			for _, ru := range r.rules {
				if ru.matchesAt(words, i) {
					hits = append(hits, Hit{Line: words[i].line, Rule: ru.id})
				}
			}
		}
	}
	return hits
}

func (ru rule) matchesAt(words []word, i int) bool {
	for _, p := range ru.phrases {
		if hasPhraseAt(words, i, p) {
			return true
		}
	}
	for _, p := range ru.pairs {
		if !hasPhraseAt(words, i, p[0]) {
			continue
		}
		for j := i + len(p[0]); j < len(words); j++ {
			if hasPhraseAt(words, j, p[1]) {
				return true
			}
		}
	}
	return false
}

func hasPhraseAt(words []word, i int, phrase []string) bool {
	if i+len(phrase) > len(words) {
		return false
	}
	for k, w := range phrase {
		if words[i+k].text != w {
			return false
		}
	}
	return true
}

// paragraph is a run of lines with no blank line and no fenced block
// inside it; line is the number of its first line.
type paragraph struct {
	line int
	text []byte
}

// paragraphs splits content at blank lines and drops fenced blocks.
func paragraphs(content []byte) []paragraph {
	var out []paragraph
	var cur *paragraph
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	var fence []byte // the opening fence while inside a block
	for n := 1; len(content) > 0; n++ {
		line, rest, _ := bytes.Cut(content, []byte{'\n'})
		content = rest
		mark := fenceMark(line)
		switch {
		case fence != nil:
			if mark != nil && mark[0] == fence[0] && len(mark) >= len(fence) && closesFence(line) {
				fence = nil
			}
		case mark != nil:
			flush()
			fence = mark
		case len(bytes.TrimSpace(line)) == 0:
			flush()
		default:
			if cur == nil {
				cur = &paragraph{line: n}
			}
			cur.text = append(append(cur.text, line...), '\n')
		}
	}
	flush()
	return out
}

// fenceMark returns the run of backticks or tildes that opens or closes
// a fenced block on this line, or nil. A fence is three or more of
// them, after at most three spaces.
func fenceMark(line []byte) []byte {
	trimmed := bytes.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) == 0 {
		return nil
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return nil
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return nil
	}
	return trimmed[:n]
}

// closesFence reports whether the line holds a fence and nothing after
// it: an opening fence may name a language, a closing one may not.
func closesFence(line []byte) bool {
	rest := bytes.TrimLeft(bytes.TrimLeft(line, " "), "`~")
	return len(bytes.TrimSpace(rest)) == 0
}

// word is one lowercased word and the line it stands on.
type word struct {
	text string
	line int
}

// paragraphWords returns the words of a paragraph outside its code
// spans. A code span opens with a run of backticks and closes with the
// next run of the same length; a run with no partner is plain text.
func paragraphWords(p paragraph) []word {
	var words []word
	line := p.line
	text := p.text
	start := -1 // start of the word being read, or -1
	end := func(i int) {
		if start >= 0 {
			words = append(words, word{text: strings.ToLower(string(text[start:i])), line: line})
			start = -1
		}
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '`':
			end(i)
			run := backtickRun(text[i:])
			if closing := closingRun(text[i+run:], run); closing >= 0 {
				span := text[i : i+run+closing+run]
				line += bytes.Count(span, []byte{'\n'})
				i += len(span) - 1
			} else {
				i += run - 1
			}
		case isWordByte(c):
			if start < 0 {
				start = i
			}
		default:
			end(i)
			if c == '\n' {
				line++
			}
		}
	}
	end(len(text))
	return words
}

func backtickRun(b []byte) int {
	n := 0
	for n < len(b) && b[n] == '`' {
		n++
	}
	return n
}

// closingRun returns the offset in b of the next backtick run of
// exactly length n, or -1.
func closingRun(b []byte, n int) int {
	for i := 0; i < len(b); i++ {
		if b[i] != '`' {
			continue
		}
		run := backtickRun(b[i:])
		if run == n {
			return i
		}
		i += run - 1
	}
	return -1
}

// phraseWords splits a listed phrase the way text is split, so a
// hyphenated phrase in the list matches the same words in a page.
func phraseWords(s string) []string {
	var out []string
	for _, w := range paragraphWords(paragraph{text: []byte(strings.ReplaceAll(s, "`", " "))}) {
		out = append(out, w.text)
	}
	return out
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
