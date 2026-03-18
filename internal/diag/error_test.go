package diag

import (
	"errors"
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
