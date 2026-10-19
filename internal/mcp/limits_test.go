package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPaginateClampsLimitAndBuildsCursor(t *testing.T) {
	window, err := Paginate(250, 1000, "")
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if window.Start != 0 || window.End != MaxPageSize {
		t.Fatalf("expected window [0,%d], got [%d,%d]", MaxPageSize, window.Start, window.End)
	}
	if !window.More || window.NextCursor != "100" {
		t.Fatalf("expected more results with cursor 100, got %+v", window)
	}
}

func TestPaginateEndOfResults(t *testing.T) {
	window, err := Paginate(3, 5, "0")
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if window.End != 3 || window.More || window.NextCursor != "" {
		t.Fatalf("expected final window, got %+v", window)
	}
}

func TestPaginateRejectsInvalidCursor(t *testing.T) {
	for _, cursor := range []string{"abc", "-1", "1.5"} {
		if _, err := Paginate(10, 5, cursor); err == nil {
			t.Fatalf("expected error for cursor %q", cursor)
		}
	}
}

func TestMarshalBoundedRejectsOversize(t *testing.T) {
	_, err := MarshalBounded(map[string]string{"value": strings.Repeat("x", 128)}, 32)
	if !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("expected ErrResultTooLarge, got %v", err)
	}
}

func TestMarshalBoundedAllowsWithinLimit(t *testing.T) {
	data, err := MarshalBounded(map[string]int{"n": 1}, 1024)
	if err != nil {
		t.Fatalf("marshal bounded: %v", err)
	}
	var decoded map[string]int
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode bounded payload: %v", err)
	}
	if decoded["n"] != 1 {
		t.Fatalf("unexpected payload: %v", decoded)
	}
}
