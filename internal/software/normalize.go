package software

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func clean(value string) string { return strings.Join(strings.Fields(strings.TrimSpace(value)), " ") }

func Normalize(value string) string { return strings.ToLower(clean(value)) }

func NormalizeInstallation(item Installation) Installation {
	item.Name = clean(item.Name)
	item.DisplayName = clean(item.DisplayName)
	if item.DisplayName == "" {
		item.DisplayName = item.Name
	}
	if item.Name == "" {
		item.Name = item.DisplayName
	}
	item.Version = clean(item.Version)
	item.Publisher = clean(item.Publisher)
	item.Architecture = clean(item.Architecture)
	item.InstallLocation = clean(item.InstallLocation)
	item.UninstallString = clean(item.UninstallString)
	item.QuietUninstallString = clean(item.QuietUninstallString)
	item.RegistryKey = clean(item.RegistryKey)
	item.Source = clean(item.Source)
	item.Scope = clean(item.Scope)
	item.Evidence = clean(item.Evidence)
	return item
}

func Fingerprint(item Installation) string {
	item = NormalizeInstallation(item)
	identity := strings.Join([]string{Normalize(item.Name), Normalize(item.Publisher), Normalize(item.Version), Normalize(item.Architecture)}, "|")
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}

func NormalizeAndDeduplicate(items []Installation) []Installation {
	byFingerprint := make(map[string]Installation, len(items))
	for _, item := range items {
		item = NormalizeInstallation(item)
		if item.Name == "" {
			continue
		}
		fingerprint := Fingerprint(item)
		if existing, ok := byFingerprint[fingerprint]; ok {
			// Prefer the machine-wide source and the record with more evidence,
			// while keeping the result deterministic.
			if sourceRank(item.Source) > sourceRank(existing.Source) || len(item.InstallLocation) > len(existing.InstallLocation) {
				byFingerprint[fingerprint] = item
			}
			continue
		}
		byFingerprint[fingerprint] = item
	}
	result := make([]Installation, 0, len(byFingerprint))
	for _, item := range byFingerprint {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := Normalize(result[i].Name), Normalize(result[j].Name)
		if left == right {
			return Fingerprint(result[i]) < Fingerprint(result[j])
		}
		return left < right
	})
	return result
}

func sourceRank(source string) int {
	switch source {
	case "REGISTRY_MACHINE_64":
		return 3
	case "REGISTRY_MACHINE_32":
		return 2
	case "REGISTRY_USER":
		return 1
	default:
		return 0
	}
}
