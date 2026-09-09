package software

import "time"

// Installation is the normalized, management-safe representation of one
// installed application. It intentionally contains no arbitrary registry data.
type Installation struct {
	Name                 string     `json:"name"`
	DisplayName          string     `json:"displayName"`
	Version              string     `json:"version,omitempty"`
	Publisher            string     `json:"publisher,omitempty"`
	Architecture         string     `json:"architecture,omitempty"`
	InstallDate          *time.Time `json:"installDate,omitempty"`
	InstallLocation      string     `json:"installLocation,omitempty"`
	UninstallString      string     `json:"uninstallString,omitempty"`
	QuietUninstallString string     `json:"quietUninstallString,omitempty"`
	RegistryKey          string     `json:"registryKey,omitempty"`
	Source               string     `json:"source"`
	Scope                string     `json:"scope,omitempty"`
	Evidence             string     `json:"evidence,omitempty"`
	FirstSeen            time.Time  `json:"-"`
	LastSeen             time.Time  `json:"-"`
}

type Payload struct {
	AgentUUID   string         `json:"agentUuid"`
	CollectedAt time.Time      `json:"collectedAt"`
	Items       []Installation `json:"items"`
}

type Report struct {
	CollectDuration time.Duration
	UploadDuration  time.Duration
	PayloadSize     int
	SoftwareCount   int
	ChangedItems    int
	SkippedItems    int
	Fingerprint     string
	Result          string
}
