// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

func TestStepsAreTheTenOf0442InOrder(t *testing.T) {
	if len(steps) != 10 {
		t.Fatalf("got %d steps, want the 10 of 04 4.2", len(steps))
	}
	for i, s := range steps {
		if s.n != i+1 {
			t.Errorf("steps[%d] is step %d, want step %d", i, s.n, i+1)
		}
		if s.name == "" || s.do == nil {
			t.Errorf("step %d lacks a name or a function", s.n)
		}
	}
}

func TestRunStepsStopsAtTheFirstErrorAndReleases(t *testing.T) {
	var ran []int
	mark := func(n int, err error) step {
		return step{n, "step", func(context.Context, *run) error { ran = append(ran, n); return err }}
	}
	boom := errors.New("boom")
	r := newRun(&cli.Env{}, flags{})
	err := runSteps(context.Background(), r, []step{mark(1, nil), mark(2, boom), mark(3, nil)})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "step 2") {
		t.Fatalf("got %v, want boom wrapped with step 2", err)
	}
	if len(ran) != 2 {
		t.Errorf("ran steps %v, want 1 and 2 only", ran)
	}
}

func TestRunStepsReturnsAnErrorOfTheTableAsIs(t *testing.T) {
	want := cli.Usage.New("rule: message")
	err := runSteps(context.Background(), newRun(&cli.Env{}, flags{}), []step{
		{1, "step", func(context.Context, *run) error { return want }},
	})
	if err != want {
		t.Fatalf("got %v, want the table error itself", err)
	}
}

func TestRunStepsStopsOnACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runSteps(ctx, newRun(&cli.Env{}, flags{}), []step{
		{1, "step", func(context.Context, *run) error { t.Error("a step ran"); return nil }},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestCheckRulesRunsInNameOrder(t *testing.T) {
	saved := rules
	t.Cleanup(func() { rules = saved })
	rules = nil
	var order []string
	add := func(name string) {
		register(rule{name, func(*run, *target) error { order = append(order, name); return nil }})
	}
	add("b")
	add("a")
	if err := checkRules(nil, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "a,b" {
		t.Errorf("ran %v, want a then b", order)
	}
}

func TestRegisterPanics(t *testing.T) {
	saved := rules
	t.Cleanup(func() { rules = saved })
	rules = nil
	ok := rule{"x", func(*run, *target) error { return nil }}
	register(ok)
	for _, ru := range []rule{ok, {"", ok.check}, {"y", nil}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("register(%q) did not panic", ru.name)
				}
			}()
			register(ru)
		}()
	}
}
