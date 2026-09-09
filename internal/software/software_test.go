package software

import "testing"

func TestNormalizeAndDeduplicate(t *testing.T) {
	items := []Installation{
		{Name: "  Contoso App ", DisplayName: "Contoso App", Publisher: "Contoso", Version: "1.0", Architecture: "x64", Source: "user"},
		{Name: "Contoso App", DisplayName: "Contoso App", Publisher: "Contoso", Version: "1.0", Architecture: "x64", Source: "REGISTRY_MACHINE_64"},
		{Name: "Zeta", DisplayName: "Zeta", Publisher: "Zeta", Version: "2.0", Architecture: "x64", Source: "REGISTRY_MACHINE_64"},
	}
	got := NormalizeAndDeduplicate(items)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique installations, got %d", len(got))
	}
	if got[0].Name != "Contoso App" || got[0].Source != "REGISTRY_MACHINE_64" {
		t.Fatalf("machine source should win and sort first: %#v", got[0])
	}
	if got[1].Name != "Zeta" {
		t.Fatalf("unexpected stable ordering: %#v", got)
	}
}

func TestFingerprintDeterministic(t *testing.T) {
	a := Installation{Name: "App", Publisher: "Vendor", Version: "1", Architecture: "x64"}
	b := Installation{Name: " app ", Publisher: " vendor ", Version: "1", Architecture: "x64"}
	if Fingerprint(a) != Fingerprint(b) {
		t.Fatal("fingerprint should normalize identity fields")
	}
	if len(Fingerprint(a)) != 64 {
		t.Fatalf("expected SHA-256 hex fingerprint")
	}
}

func TestNormalizeDropsEmptyNames(t *testing.T) {
	if got := NormalizeInstallation(Installation{Publisher: "Vendor"}); got.Name != "" {
		t.Fatalf("empty display name should be ignored: %#v", got)
	}
}
