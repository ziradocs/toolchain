// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCheckInputSize(t *testing.T) {
	if err := CheckInputSize(5, 10); err != nil {
		t.Errorf("expected no error for size within limit, got: %v", err)
	}
	if err := CheckInputSize(11, 10); err == nil {
		t.Error("expected error for size exceeding limit, got nil")
	}
}

func TestResolveMaxInputBytes(t *testing.T) {
	if got := ResolveMaxInputBytes(5, "SLIDELANG_TEST_MAX_SIZE_UNSET"); got != 5<<20 {
		t.Errorf("expected flag value to win, got %d", got)
	}

	t.Setenv("SLIDELANG_TEST_MAX_SIZE", "7")
	if got := ResolveMaxInputBytes(0, "SLIDELANG_TEST_MAX_SIZE"); got != 7<<20 {
		t.Errorf("expected env var value when flag unset, got %d", got)
	}

	if got := ResolveMaxInputBytes(0, "SLIDELANG_TEST_MAX_SIZE_UNSET"); got != DefaultMaxInputBytes {
		t.Errorf("expected default when neither flag nor env set, got %d", got)
	}
}

func TestResolveMaxInputBytesClampsOversizedValues(t *testing.T) {
	// A huge --max-size value must clamp to the ceiling instead of
	// overflowing int and wrapping to a negative limit (which would reject
	// every input, including a 1-byte file).
	got := ResolveMaxInputBytes(1<<40, "SLIDELANG_TEST_MAX_SIZE_UNSET")
	if got <= 0 {
		t.Fatalf("expected a positive clamped limit, got %d", got)
	}
	if got != maxAllowedInputBytes {
		t.Errorf("expected clamp to maxAllowedInputBytes (%d), got %d", maxAllowedInputBytes, got)
	}

	t.Setenv("SLIDELANG_TEST_MAX_SIZE_HUGE", "999999999999")
	got = ResolveMaxInputBytes(0, "SLIDELANG_TEST_MAX_SIZE_HUGE")
	if got <= 0 || got != maxAllowedInputBytes {
		t.Errorf("expected env var overflow to clamp to %d, got %d", maxAllowedInputBytes, got)
	}
}

func TestRecoverGuardCatchesPanic(t *testing.T) {
	err := RecoverGuard(func() error {
		panic("simulated parser panic")
	})
	if err == nil {
		t.Fatal("expected an error from a recovered panic, got nil")
	}
	if !strings.Contains(err.Error(), "simulated parser panic") {
		t.Errorf("expected error to mention the panic value, got: %v", err)
	}
}

func TestRecoverGuardPassesThroughError(t *testing.T) {
	sentinel := errors.New("boom")
	err := RecoverGuard(func() error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error to pass through unchanged, got: %v", err)
	}
}

func TestRecoverGuardNoPanicNoError(t *testing.T) {
	err := RecoverGuard(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error when fn does not panic or fail, got: %v", err)
	}
}

func TestRunWithTimeoutExpires(t *testing.T) {
	err := RunWithTimeout(10*time.Millisecond, func() error {
		time.Sleep(time.Second)
		return nil
	})
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}

func TestRunWithTimeoutCompletesInTime(t *testing.T) {
	err := RunWithTimeout(time.Second, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error for fast completion, got: %v", err)
	}
}

func TestRunGuardedCatchesPanicUnderTimeout(t *testing.T) {
	// Exercises the actual production composition (recover innermost, timeout
	// outermost): a panic inside fn must come back as a plain error, not crash
	// the process, even though the whole call is also timeout-bounded.
	err := RunGuarded(time.Second, func() error {
		panic("simulated parser panic")
	})
	if err == nil {
		t.Fatal("expected an error from a recovered panic, got nil")
	}
	if !strings.Contains(err.Error(), "simulated parser panic") {
		t.Errorf("expected error to mention the panic value, got: %v", err)
	}
}

func TestRunGuardedTimesOut(t *testing.T) {
	err := RunGuarded(10*time.Millisecond, func() error {
		time.Sleep(time.Second)
		return nil
	})
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}

func TestRunGuardedCompletesInTime(t *testing.T) {
	err := RunGuarded(time.Second, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error for fast completion, got: %v", err)
	}
}
