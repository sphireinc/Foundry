package diag

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorFormattingAndUnwrap(t *testing.T) {
	base := errors.New("boom")
	err := &Error{Kind: KindBuild, Op: "load", Err: base}

	if err.Error() != "load: boom" {
		t.Fatalf("unexpected error string: %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Fatal("expected wrapped error to unwrap")
	}
}

func TestWrapKindOfPresentAndExitCode(t *testing.T) {
	base := errors.New("boom")
	err := Wrap(KindUsage, "parse args", base)

	if KindOf(err) != KindUsage {
		t.Fatalf("expected usage kind, got %s", KindOf(err))
	}
	if Present(err) != "usage error: parse args: boom" {
		t.Fatalf("unexpected presentation: %q", Present(err))
	}
	if ExitCode(err) != 2 {
		t.Fatalf("expected usage exit code 2, got %d", ExitCode(err))
	}

	unknown := New(KindUnknown, "x")
	rewrapped := Wrap(KindPlugin, "hook", unknown)
	if KindOf(rewrapped) != KindPlugin {
		t.Fatalf("expected plugin kind, got %s", KindOf(rewrapped))
	}
}

func TestWrapNilAndFallbacks(t *testing.T) {
	if Wrap(KindBuild, "x", nil) != nil {
		t.Fatal("expected wrapping nil to return nil")
	}
	if opOrFallback("primary", "fallback") != "primary" {
		t.Fatal("expected primary op")
	}
	if opOrFallback("", "fallback") != "fallback" {
		t.Fatal("expected fallback op")
	}
}

func TestAdditionalErrorBranches(t *testing.T) {
	if (*Error)(nil).Error() != "" {
		t.Fatal("expected nil error string")
	}
	if (*Error)(nil).Unwrap() != nil {
		t.Fatal("expected nil unwrap")
	}
	if (&Error{Kind: KindUnknown}).Error() != "unknown" {
		t.Fatal("expected kind-only error string")
	}
	if (&Error{Op: "op"}).Error() != "op" {
		t.Fatal("expected op-only error string")
	}
	if (&Error{Err: errors.New("boom")}).Error() != "boom" {
		t.Fatal("expected err-only error string")
	}

	cases := []Kind{KindConfig, KindPlugin, KindIO, KindDependency, KindBuild, KindServe, KindRender, KindInternal}
	for _, kind := range cases {
		msg := Present(New(kind, "problem"))
		if !strings.Contains(msg, "problem") {
			t.Fatalf("expected presentation for %s, got %q", kind, msg)
		}
	}
	if Present(nil) != "" {
		t.Fatal("expected empty presentation for nil")
	}
	if ExitCode(New(KindBuild, "x")) != 1 {
		t.Fatal("expected non-usage exit code 1")
	}
}
