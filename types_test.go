package main

import (
	"strings"
	"testing"
)

func validEntry() PackageListEntry {
	return PackageListEntry{
		Id:            "owner/repo",
		Provider:      "github",
		Name:          "Test App",
		Publisher:     "Test Publisher",
		InstallerType: InstallerTypeZipPortable,
	}
}

func TestValidateEntry_Valid(t *testing.T) {
	if err := validateEntry(validEntry()); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateEntry_MissingFields(t *testing.T) {
	fields := []struct {
		name  string
		clear func(*PackageListEntry)
	}{
		{"id", func(e *PackageListEntry) { e.Id = "" }},
		{"provider", func(e *PackageListEntry) { e.Provider = "" }},
		{"name", func(e *PackageListEntry) { e.Name = "" }},
		{"publisher", func(e *PackageListEntry) { e.Publisher = "" }},
		{"installer_type", func(e *PackageListEntry) { e.InstallerType = "" }},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			entry := validEntry()
			f.clear(&entry)
			err := validateEntry(entry)
			if err == nil {
				t.Errorf("expected error when %q is empty, got nil", f.name)
			} else if !strings.Contains(err.Error(), f.name) {
				t.Errorf("expected error to mention field %q, got: %v", f.name, err)
			}
		})
	}
}

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
	if got := entry.GetScope(); got != "" {
		t.Errorf("GetScope() = %q, want %q", got, "")
	}
}

func TestGetScope_MsiDefault(t *testing.T) {
	entry := PackageListEntry{InstallerType: InstallerTypeMsi}
	if got := entry.GetScope(); got != "machine" {
		t.Errorf("GetScope() = %q, want %q", got, "machine")
	}
}

func TestGetScope_Custom(t *testing.T) {
	entry := PackageListEntry{Scope: "machine"}
	if got := entry.GetScope(); got != "machine" {
		t.Errorf("GetScope() = %q, want %q", got, "machine")
	}
}
