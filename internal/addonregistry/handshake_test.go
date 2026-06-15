package addonregistry

import "testing"

func TestEntrySupportsVersion(t *testing.T) {
	cases := []struct {
		name    string
		entry   Entry
		version string
		want    VersionSupport
	}{
		{"within min only", Entry{MinGOWDK: "0.3.0"}, "0.5.0", VersionSupported},
		{"equals min", Entry{MinGOWDK: "0.3.0"}, "0.3.0", VersionSupported},
		{"below min", Entry{MinGOWDK: "0.3.0"}, "0.2.9", VersionUnsupported},
		{"within range", Entry{MinGOWDK: "0.3.0", MaxGOWDK: "0.9.0"}, "0.5.0", VersionSupported},
		{"above max", Entry{MinGOWDK: "0.3.0", MaxGOWDK: "0.4.0"}, "0.5.0", VersionUnsupported},
		{"equals max", Entry{MaxGOWDK: "0.5.0"}, "0.5.0", VersionSupported},
		{"no bounds", Entry{}, "0.5.0", VersionSupported},
		{"leading v and prerelease", Entry{MinGOWDK: "0.3.0"}, "v0.5.0-rc1", VersionSupported},
		{"unparseable version", Entry{MinGOWDK: "0.3.0"}, "latest", VersionUnknown},
		{"unparseable bound", Entry{MinGOWDK: "next"}, "0.5.0", VersionUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.SupportsVersion(tc.version); got != tc.want {
				t.Fatalf("SupportsVersion(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestRegistryUnsupportedFor(t *testing.T) {
	registry := Registry{Addons: []Entry{
		{Name: "ok", MinGOWDK: "0.3.0"},
		{Name: "too-new", MinGOWDK: "0.9.0"},
		{Name: "too-old", MaxGOWDK: "0.4.0"},
		{Name: "unknown-bound", MinGOWDK: "next"},
	}}
	unsupported := registry.UnsupportedFor("0.5.0")
	if len(unsupported) != 2 {
		t.Fatalf("expected 2 unsupported entries, got %d: %+v", len(unsupported), unsupported)
	}
	names := map[string]bool{}
	for _, entry := range unsupported {
		names[entry.Name] = true
	}
	if !names["too-new"] || !names["too-old"] {
		t.Fatalf("expected too-new and too-old, got %v", names)
	}
	if names["unknown-bound"] {
		t.Fatal("entries with unparseable bounds must not be reported as unsupported")
	}
}

func TestBundledRegistrySupportsCurrentLine(t *testing.T) {
	registry, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	// Every bundled built-in declares minGOWDK 0.3.0, so the current 0.x line
	// must not report any of them as version-incompatible.
	if unsupported := registry.UnsupportedFor("0.5.0"); len(unsupported) != 0 {
		t.Fatalf("bundled registry should support 0.5.0, got unsupported: %+v", unsupported)
	}
}
