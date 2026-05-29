package utils

import (
	"strings"
	"testing"
)

func TestOptsFlagStringNilMap(t *testing.T) {
	o := &OptsFlag{}
	if got := o.String(); got != "" {
		t.Fatalf("String() with nil map = %q, want \"\"", got)
	}
}

func TestOptsFlagSetWithValue(t *testing.T) {
	o := NewOptsFlag(map[string]string{})
	if err := o.Set("foo=bar"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if o.opts["foo"] != "bar" {
		t.Fatalf("opts[foo] = %q, want bar", o.opts["foo"])
	}
}

func TestOptsFlagSetWithoutEqualsDefaultsToTrue(t *testing.T) {
	o := NewOptsFlag(map[string]string{})
	if err := o.Set("debug"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if o.opts["debug"] != "true" {
		t.Fatalf("opts[debug] = %q, want true", o.opts["debug"])
	}
}

func TestOptsFlagSetWithEmptyValue(t *testing.T) {
	o := NewOptsFlag(map[string]string{})
	if err := o.Set("k="); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if v, ok := o.opts["k"]; !ok || v != "" {
		t.Fatalf("opts[k] = %q (ok=%v), want empty string present", v, ok)
	}
}

func TestOptsFlagSetOverwrites(t *testing.T) {
	o := NewOptsFlag(map[string]string{"k": "old"})
	if err := o.Set("k=new"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if o.opts["k"] != "new" {
		t.Fatalf("opts[k] = %q, want new", o.opts["k"])
	}
}

func TestOptsFlagStringFormat(t *testing.T) {
	o := NewOptsFlag(map[string]string{"a": "1", "b": "2"})
	got := o.String()
	// Map iteration order is non-deterministic; verify both items are present
	// in "k=v" form and separated by a single space.
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "b=2") {
		t.Fatalf("String() = %q, missing entries", got)
	}
	parts := strings.Split(got, " ")
	if len(parts) != 2 {
		t.Fatalf("expected 2 space-separated parts, got %d in %q", len(parts), got)
	}
}

func TestOptsFlagStringEmpty(t *testing.T) {
	o := NewOptsFlag(map[string]string{})
	if got := o.String(); got != "" {
		t.Fatalf("String() with empty map = %q, want \"\"", got)
	}
}
