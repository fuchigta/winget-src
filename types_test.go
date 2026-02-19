package main

import "testing"

func TestGetLocale_Default(t *testing.T) {
	entry := PackageListEntry{}
	if got := entry.GetLocale(); got != DefaultPackageLocale {
		t.Errorf("GetLocale() = %q, want %q", got, DefaultPackageLocale)
	}
}

func TestGetLocale_Custom(t *testing.T) {
	entry := PackageListEntry{Locale: "ja-JP"}
	if got := entry.GetLocale(); got != "ja-JP" {
		t.Errorf("GetLocale() = %q, want %q", got, "ja-JP")
	}
}

func TestGetScope_Default(t *testing.T) {
	entry := PackageListEntry{}
	if got := entry.GetScope(); got != DefaultInstallerScope {
		t.Errorf("GetScope() = %q, want %q", got, DefaultInstallerScope)
	}
}

func TestGetScope_Custom(t *testing.T) {
	entry := PackageListEntry{Scope: "machine"}
	if got := entry.GetScope(); got != "machine" {
		t.Errorf("GetScope() = %q, want %q", got, "machine")
	}
}
