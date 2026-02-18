package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
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
	if !cond(PackageListEntry{Id: "Owner/Foo"}) {
		t.Error("expected case-insensitive match for 'Owner/Foo'")
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
	if !cond(PackageListEntry{Name: "Foo"}) {
		t.Error("expected case-insensitive match for 'Foo'")
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
		_, err := dispatchProvider(PackageListEntry{Provider: tt.provider}, http.DefaultClient)
		if (err != nil) != tt.wantErr {
			t.Errorf("dispatchProvider(%q) error = %v, wantErr = %v", tt.provider, err, tt.wantErr)
		}
	}
}

func TestDispatchProvider_ErrorContainsProviderName(t *testing.T) {
	_, err := dispatchProvider(PackageListEntry{Provider: "myunknownprovider"}, http.DefaultClient)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "myunknownprovider") {
		t.Errorf("expected error message to contain provider name, got: %s", err.Error())
	}
}

func TestNewWingetSrcRepository_InvalidYAML(t *testing.T) {
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.WriteString("invalid: yaml: :")
	f.Close()

	ctx := context.Background()
	_, err = NewWingetSrcRepository(ctx, f.Name(), 5*time.Minute, 10*time.Minute, 30*time.Second, false, 0)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNewWingetSrcRepository_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	// 空のYAMLリスト（[]）を書き込む
	f.WriteString("[]")
	f.Close()

	ctx := context.Background()
	_, err = NewWingetSrcRepository(ctx, f.Name(), 5*time.Minute, 10*time.Minute, 30*time.Second, false, 0)
	if err != nil {
		t.Fatalf("unexpected error for empty yaml list: %v", err)
	}
}

func TestNewWingetSrcRepository_NonExistentFile(t *testing.T) {
	ctx := context.Background()
	_, err := NewWingetSrcRepository(ctx, "/nonexistent/path/packages.yaml", 5*time.Minute, 10*time.Minute, 30*time.Second, false, 0)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func BenchmarkQueryManifest_Parallel(b *testing.B) {
	// Build a repository with 100 pre-cached packages
	const numPackages = 100
	entries := make([]PackageListEntry, numPackages)
	packageMap := make(map[string]PackageListEntry, numPackages)
	for i := range entries {
		id := fmt.Sprintf("owner/pkg%d", i)
		entries[i] = PackageListEntry{Id: id, Name: fmt.Sprintf("pkg%d", i)}
		packageMap[strings.ToLower(id)] = entries[i]
	}

	cache := NewCache[[]Version](5*time.Minute, 0)
	for _, e := range entries {
		cache.Set(e.Id, []Version{{Version: "1.0.0"}})
	}

	repo := WingetSrcRepositoryImpl{
		packageList:         entries,
		packageMap:          packageMap,
		versionCache:        cache,
		httpClient:          http.DefaultClient,
		gracefulDegradation: false,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := repo.QueryManifest(context.Background(), ByName("pkg"))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCache_Get_Hit(b *testing.B) {
	c := NewCache[[]Version](5*time.Minute, 0)
	c.Set("key", []Version{{Version: "1.0.0"}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("key")
	}
}

func BenchmarkCache_Set(b *testing.B) {
	c := NewCache[[]Version](5*time.Minute, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("key", []Version{{Version: "1.0.0"}})
	}
}
