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
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
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

func TestRunStepsStopsAtTheFirstError(t *testing.T) {
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
	if !errors.Is(err, want) || err.Error() != want.Error() {
		t.Fatalf("got %v, want the table error itself, unwrapped", err)
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
		register(rule{name, func(context.Context, *run) error { order = append(order, name); return nil }})
	}
	add("b")
	add("a")
	if err := checkRules(context.Background(), nil); err != nil {
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
	ok := rule{"x", func(context.Context, *run) error { return nil }}
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

func TestRunStepsReleasesTheLockAndTheStore(t *testing.T) {
	dir := t.TempDir()
	st, err := state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	lk, err := st.Lock()
	if err != nil {
		t.Fatal(err)
	}
	r := newRun(&cli.Env{}, flags{})
	r.store, r.lock = st, lk
	boom := errors.New("boom")
	if err := runSteps(context.Background(), r, []step{{n: 1, name: "step", do: func(context.Context, *run) error { return boom }}}); !errors.Is(err, boom) {
		t.Fatalf("got %v, want boom", err)
	}
	if r.lock != nil || r.store != nil {
		t.Error("runSteps left the lock or the store set")
	}
	again, err := state.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	lk2, err := again.Lock()
	if err != nil {
		t.Fatalf("the lock is still held after runSteps: %v", err)
	}
	lk2.Unlock()
}

func TestUsageNamesTheRuleAndExitsTwo(t *testing.T) {
	err := usage("some-rule", "what broke")
	if got := cli.ExitCode(err); got != 2 {
		t.Errorf("exit %d, want 2", got)
	}
	if !strings.Contains(err.Error(), "some-rule: what broke") {
		t.Errorf("got %q, want the slug and the message", err)
	}
}

func TestTheCommandParsesItsFlagsAndRunsTheSteps(t *testing.T) {
	var got flags
	saved := steps
	t.Cleanup(func() { steps = saved })
	steps = []step{{n: 1, name: "step", do: func(_ context.Context, r *run) error { got = r.flags; return nil }}}
	cmd := definition.New(&cli.Env{})
	cmd.SetArgs([]string{"--all", "--from", "abc123", "--overwrite-drift", "shop", "blog"})
	cmd.SetContext(context.Background())
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := flags{all: true, from: "abc123", overwriteDrift: true, names: []string{"shop", "blog"}}
	if got.all != want.all || got.from != want.from || got.overwriteDrift != want.overwriteDrift || strings.Join(got.names, ",") != "shop,blog" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFromDefaultsToOrigin(t *testing.T) {
	f := definition.New(&cli.Env{}).Flags().Lookup("from")
	if f == nil || f.DefValue != "origin" {
		t.Fatalf("--from defaults to %v, want origin", f)
	}
}

func TestAStepNotBuiltStopsTheRun(t *testing.T) {
	err := runSteps(context.Background(), newRun(&cli.Env{}, flags{}), steps)
	if !errors.Is(err, errNotBuilt) || !strings.Contains(err.Error(), "step 1") {
		t.Fatalf("got %v, want step 1 not built", err)
	}
}
