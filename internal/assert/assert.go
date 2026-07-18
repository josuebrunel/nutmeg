package assert

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func Eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("[ASSERT-FAILED] Eq(%T):\n  got:  %v\n  want: %v", got, got, want)
	}
}

func Neq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got == want {
		t.Fatalf("[ASSERT-FAILED] Neq(%T):\n  got:  %v\n  want: != %v", got, got, want)
	}
}

func True(t *testing.T, got bool) {
	t.Helper()
	if !got {
		t.Fatalf("[ASSERT-FAILED] True:\n  got: false\n  want: true")
	}
}

func False(t *testing.T, got bool) {
	t.Helper()
	if got {
		t.Fatalf("[ASSERT-FAILED] False:\n  got: true\n  want: false")
	}
}

func Nil(t *testing.T, got any) {
	t.Helper()
	if got != nil {
		t.Fatalf("[ASSERT-FAILED] Nil:\n  got: %v\n  want: nil", got)
	}
}

func NotNil(t *testing.T, got any) {
	t.Helper()
	if got == nil {
		t.Fatal("[ASSERT-FAILED] NotNil:\n  got: nil\n  want: non-nil")
	}
}

func NoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("[ASSERT-FAILED] NoErr:\n  got: %v\n  want: nil", err)
	}
}

func ErrIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("[ASSERT-FAILED] ErrIs:\n  got:  %v\n  want: %v", err, target)
	}
}

func Panics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("[ASSERT-FAILED] Panics:\n  expected panic, but none occurred")
		}
	}()
	fn()
}

func DeepEq[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("[ASSERT-FAILED] DeepEq:\n  got:  %+v\n  want: %+v", got, want)
	}
}

func StrContains(t *testing.T, s, substr string) {
	t.Helper()
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}
	t.Fatalf("[ASSERT-FAILED] StrContains:\n  str:    %q\n  substr: %q", s, substr)
}

func EqN(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("[ASSERT-FAILED] EqN:\n  got: %d\n  want: %d\n  %s", got, want, formatDiff(got, want))
	}
}

func formatDiff(got, want int) string {
	return fmt.Sprintf("(diff: %+d)", want-got)
}
