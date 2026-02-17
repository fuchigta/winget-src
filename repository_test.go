package main

import (
	"testing"
)

func TestById(t *testing.T) {
	cond := ById("owner/foo")
	if !cond(PackageListEntry{Id: "owner/foo"}) {
		t.Error("expected match for 'owner/foo'")
	}
	if !cond(PackageListEntry{Id: "owner/foobar"}) {
		t.Error("expected match for 'owner/foobar' (contains 'owner/foo')")
	}
	if cond(PackageListEntry{Id: "owner/bar"}) {
		t.Error("expected no match for 'owner/bar'")
	}
}

func TestByName(t *testing.T) {
	cond := ByName("foo")
	if !cond(PackageListEntry{Name: "foo"}) {
		t.Error("expected match for 'foo'")
	}
	if !cond(PackageListEntry{Name: "foobar"}) {
		t.Error("expected match for 'foobar' (contains 'foo')")
	}
	if cond(PackageListEntry{Name: "bar"}) {
		t.Error("expected no match for 'bar'")
	}
}

func TestOr(t *testing.T) {
	cond := Or(ById("foo"), ById("bar"))
	if !cond(PackageListEntry{Id: "foo"}) {
		t.Error("expected match for 'foo'")
	}
	if !cond(PackageListEntry{Id: "bar"}) {
		t.Error("expected match for 'bar'")
	}
	if cond(PackageListEntry{Id: "baz"}) {
		t.Error("expected no match for 'baz'")
	}
}

func TestAnd(t *testing.T) {
	cond := And(ById("owner"), ByName("foo"))
	if !cond(PackageListEntry{Id: "owner/test", Name: "foo"}) {
		t.Error("expected match for Id containing 'owner' and Name containing 'foo'")
	}
	if cond(PackageListEntry{Id: "owner/test", Name: "bar"}) {
		t.Error("expected no match when Name does not contain 'foo'")
	}
	if cond(PackageListEntry{Id: "other", Name: "foo"}) {
		t.Error("expected no match when Id does not contain 'owner'")
	}
}

func TestDispatchProvider(t *testing.T) {
	tests := []struct {
		provider string
		wantErr  bool
	}{
		{"github", false},
		{"gitlab", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		_, err := dispatchProvider(PackageListEntry{Provider: tt.provider})
		if (err != nil) != tt.wantErr {
			t.Errorf("dispatchProvider(%q) error = %v, wantErr = %v", tt.provider, err, tt.wantErr)
		}
	}
}
