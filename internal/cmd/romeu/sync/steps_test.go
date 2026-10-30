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
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		args []string
		want flags
	}{
		{[]string{"--from", sha, "--overwrite-drift", "shop", "blog"}, flags{from: sha, overwriteDrift: true, names: []string{"shop", "blog"}}},
		{[]string{"--all"}, flags{all: true, from: "origin"}},
	} {
		var got flags
		saved := steps
		t.Cleanup(func() { steps = saved })
		steps = []step{{n: 1, name: "step", do: func(_ context.Context, r *run) error { got = r.flags; return nil }}}
		cmd := definition.New(&cli.Env{})
		cmd.SetArgs(tc.args)
		cmd.SetContext(context.Background())
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if got.all != tc.want.all || got.from != tc.want.from || got.overwriteDrift != tc.want.overwriteDrift ||
			strings.Join(got.names, ",") != strings.Join(tc.want.names, ",") {
			t.Errorf("%v: got %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

func TestFromDefaultsToOrigin(t *testing.T) {
	f := definition.New(&cli.Env{}).Flags().Lookup("from")
	if f == nil || f.DefValue != "origin" {
		t.Fatalf("--from defaults to %v, want origin", f)
	}
}

// TestRunStepsPrintsTheWarningsLast: the warnings of the run close its
// output, after every step, on a failed step too, so a warning found
// before it is not lost (04 4.2).
func TestRunStepsPrintsTheWarningsLast(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var out strings.Builder
		w := cli.NewWriter(&out)
		r := newRun(&cli.Env{Stdout: w, Stderr: w}, flags{})
		boom := errors.New("boom")
		err := runSteps(context.Background(), r, []step{
			{1, "warns", func(_ context.Context, r *run) error {
				r.warnings = append(r.warnings, func(w *cli.Writer) { cli.Warn(w, "handoff-legacy", "a dir") })
				return nil
			}},
			{2, "reports", func(_ context.Context, r *run) error {
				_, _ = r.env.Stdout.Write([]byte("report\n"))
				if fail {
					return boom
				}
				return nil
			}},
		})
		if fail != errors.Is(err, boom) {
			t.Fatalf("fail %v: got %v", fail, err)
		}
		if got := out.String(); got != "report\nwarning handoff-legacy: a dir\n" {
			t.Errorf("fail %v: got %q, want the report, then the warning", fail, got)
		}
	}
}
