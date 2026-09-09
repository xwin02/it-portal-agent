//go:build windows

package software

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

type registrySource struct {
	root                registry.Key
	path, source, scope string
	access              uint32
}

func collectWindows(ctx context.Context) ([]Installation, error) {
	sources := []registrySource{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "REGISTRY_MACHINE_64", "MACHINE", registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "REGISTRY_MACHINE_32", "MACHINE", registry.WOW64_32KEY},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`, "REGISTRY_USER", "USER", 0},
	}
	items := make([]Installation, 0)
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		found, err := readRegistrySource(source)
		if err != nil {
			continue
		} // A missing hive/view is normal on some systems.
		items = append(items, found...)
	}
	return NormalizeAndDeduplicate(items), nil
}

func readRegistrySource(source registrySource) ([]Installation, error) {
	key, err := registry.OpenKey(source.root, source.path, registry.READ|source.access)
	if err != nil {
		return nil, err
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}
	items := make([]Installation, 0, len(names))
	for _, name := range names {
		item, ok := readRegistryItem(key, name, source)
		if ok {
			items = append(items, item)
		}
	}
	return items, nil
}

func readRegistryItem(parent registry.Key, name string, source registrySource) (Installation, bool) {
	key, err := registry.OpenKey(parent, name, registry.READ)
	if err != nil {
		return Installation{}, false
	}
	defer key.Close()
	displayName, _, err := key.GetStringValue("DisplayName")
	if err != nil || strings.TrimSpace(displayName) == "" {
		return Installation{}, false
	}
	version, _ := stringValue(key, "DisplayVersion")
	publisher, _ := stringValue(key, "Publisher")
	installLocation, _ := stringValue(key, "InstallLocation")
	uninstallString, _ := stringValue(key, "UninstallString")
	quietUninstallString, _ := stringValue(key, "QuietUninstallString")
	architecture, _ := stringValue(key, "Architecture")
	if architecture == "" {
		if source.source == "REGISTRY_MACHINE_64" {
			architecture = "x64"
		} else if source.source == "REGISTRY_MACHINE_32" {
			architecture = "x86"
		}
	}
	parsedInstallDate := parseInstallDate(mustStringValue(key, "InstallDate"))
	var installDate *time.Time
	if !parsedInstallDate.IsZero() {
		installDate = &parsedInstallDate
	}
	return Installation{Name: displayName, DisplayName: displayName, Version: version, Publisher: publisher, Architecture: architecture, InstallDate: installDate, InstallLocation: installLocation, UninstallString: uninstallString, QuietUninstallString: quietUninstallString, RegistryKey: fmt.Sprintf(`%s\%s`, source.path, name), Source: source.source, Scope: source.scope, Evidence: "Windows Registry"}, true
}

func stringValue(key registry.Key, name string) (string, error) {
	value, _, err := key.GetStringValue(name)
	return strings.TrimSpace(value), err
}
func mustStringValue(key registry.Key, name string) string {
	value, _ := stringValue(key, name)
	return value
}

func parseInstallDate(value string) time.Time {
	if len(value) != 8 {
		return time.Time{}
	}
	parsed, err := time.ParseInLocation("20060102", value, time.Local)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
