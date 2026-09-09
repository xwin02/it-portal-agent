// Package inventory contains the Windows hardware inventory contract and delivery engine.
package inventory

import "time"

// Payload is the raw snapshot sent to the Portal. The Portal stores this
// payload as an immutable snapshot and updates the related asset metadata.
type Payload struct {
	Timestamp    time.Time        `json:"timestamp"`
	AgentUUID    string           `json:"agentUuid"`
	AgentVersion string           `json:"agentVersion"`
	Hostname     string           `json:"hostname"`
	Fingerprint  string           `json:"fingerprint"`
	ChangedItems int              `json:"changedItems"`
	Changes      []HardwareChange `json:"changes,omitempty"`
	AssetLink    AssetLinkHints   `json:"assetLink"`
	Health       HealthAssessment `json:"health"`
	Hardware     Hardware         `json:"hardware"`
}

type Hardware struct {
	General         General          `json:"general"`
	OperatingSystem OperatingSystem  `json:"operatingSystem"`
	CPU             CPU              `json:"cpu"`
	RAM             Memory           `json:"ram"`
	Disk            DiskInventory    `json:"disk"`
	Storage         []StorageDevice  `json:"storage"`
	NIC             []NetworkAdapter `json:"nic"`
	Network         []NetworkAdapter `json:"network"`
	Motherboard     Motherboard      `json:"motherboard"`
	BIOS            BIOS             `json:"bios"`
	Security        Security         `json:"security"`
	Monitor         []Monitor        `json:"monitor"`
	GPU             []GPU            `json:"gpu"`
	Users           Users            `json:"users"`
	SerialNumber    string           `json:"serialNumber"`
}

type General struct {
	ComputerName string `json:"computerName"`
	MachineUUID  string `json:"machineUuid"`
	SerialNumber string `json:"serialNumber"`
	AssetTag     string `json:"assetTag"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	ProductName  string `json:"productName"`
}

type OperatingSystem struct {
	Edition      string `json:"edition"`
	Version      string `json:"version"`
	BuildNumber  string `json:"buildNumber"`
	Architecture string `json:"architecture"`
	InstallDate  string `json:"installDate"`
	LastBootTime string `json:"lastBootTime"`
	Domain       string `json:"domain"`
	Workgroup    string `json:"workgroup"`
	Timezone     string `json:"timezone"`
}

type CPU struct {
	Model         string `json:"model"`
	Manufacturer  string `json:"manufacturer"`
	Socket        string `json:"socket"`
	LogicalCores  uint32 `json:"logicalCores"`
	PhysicalCores uint32 `json:"physicalCores"`
	Threads       uint32 `json:"threads"`
	ClockMHz      uint32 `json:"clockMHz"`
}

type Memory struct {
	TotalBytes     uint64         `json:"totalBytes"`
	UsedBytes      uint64         `json:"usedBytes"`
	AvailableBytes uint64         `json:"availableBytes"`
	TotalGB        float64        `json:"totalGb"`
	UsedGB         float64        `json:"usedGb"`
	AvailableGB    float64        `json:"availableGb"`
	UsagePercent   float64        `json:"usagePercent"`
	Slots          uint32         `json:"slots"`
	Modules        []MemoryModule `json:"modules"`
}

type MemoryModule struct {
	Locator      string `json:"locator"`
	Manufacturer string `json:"manufacturer"`
	PartNumber   string `json:"partNumber"`
	Capacity     uint64 `json:"capacityBytes"`
	SpeedMHz     uint32 `json:"speedMHz"`
}

type DiskInventory struct {
	TotalBytes   uint64          `json:"totalBytes"`
	UsedBytes    uint64          `json:"usedBytes"`
	FreeBytes    uint64          `json:"freeBytes"`
	TotalGB      float64         `json:"totalGb"`
	UsedGB       float64         `json:"usedGb"`
	FreeGB       float64         `json:"freeGb"`
	Model        string          `json:"model"`
	Type         string          `json:"type"`
	SerialNumber string          `json:"serialNumber"`
	Health       string          `json:"health"`
	SMARTStatus  string          `json:"smartStatus"`
	Devices      []StorageDevice `json:"devices"`
}

type StorageDevice struct {
	DeviceID     string  `json:"deviceId"`
	Type         string  `json:"type"`
	Model        string  `json:"model"`
	SerialNumber string  `json:"serialNumber"`
	Health       string  `json:"health"`
	SMARTStatus  string  `json:"smartStatus"`
	TotalBytes   uint64  `json:"totalBytes"`
	FreeBytes    uint64  `json:"freeBytes"`
	UsedBytes    uint64  `json:"usedBytes"`
	CapacityGB   float64 `json:"capacityGb"`
	UsedGB       float64 `json:"usedGb"`
	FreeGB       float64 `json:"freeGb"`
	Filesystem   string  `json:"filesystem"`
	DriveType    string  `json:"driveType"`
}

type NetworkAdapter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IPAddress   []string `json:"ipAddress"`
	MACAddress  string   `json:"macAddress"`
	Gateway     []string `json:"gateway"`
	DNS         []string `json:"dns"`
	DHCP        bool     `json:"dhcp"`
	DHCPServer  string   `json:"dhcpServer"`
	LinkSpeed   uint64   `json:"linkSpeed"`
	Status      string   `json:"status"`
}

type Motherboard struct {
	Manufacturer string `json:"manufacturer"`
	Product      string `json:"product"`
	SerialNumber string `json:"serialNumber"`
}

type BIOS struct {
	Manufacturer string `json:"manufacturer"`
	Version      string `json:"version"`
	SerialNumber string `json:"serialNumber"`
	ReleaseDate  string `json:"releaseDate"`
}

type Security struct {
	TPM        SecurityState `json:"tpm"`
	SecureBoot SecurityState `json:"secureBoot"`
	BitLocker  SecurityState `json:"bitLocker"`
	Defender   SecurityState `json:"windowsDefender"`
	Firewall   SecurityState `json:"firewall"`
}

type SecurityState struct {
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
}

type Monitor struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serialNumber"`
	Width        uint32 `json:"width"`
	Height       uint32 `json:"height"`
}

type GPU struct {
	Name          string `json:"name"`
	DriverVersion string `json:"driverVersion"`
	Width         uint32 `json:"width"`
	Height        uint32 `json:"height"`
}

type Users struct {
	Current string `json:"current"`
	Last    string `json:"last"`
}

type Report struct {
	CollectDuration time.Duration
	UploadDuration  time.Duration
	Duration        time.Duration
	PayloadSize     int
	HardwareCount   int
	ChangedItems    int
	SkippedItems    int
	Fingerprint     string
	Queued          bool
	Result          string
}

type AssetLinkHints struct {
	SerialNumber      string   `json:"serialNumber,omitempty"`
	MachineUUID       string   `json:"machineUuid,omitempty"`
	Hostname          string   `json:"hostname,omitempty"`
	ExistingAgentUUID string   `json:"existingAgentUuid,omitempty"`
	Priority          []string `json:"priority"`
	Action            string   `json:"action"`
}

type HardwareChange struct {
	Section string `json:"section"`
	Type    string `json:"type"`
	Detail  string `json:"detail"`
}

type InventoryHistoryEvent struct {
	Timestamp   time.Time        `json:"timestamp"`
	Fingerprint string           `json:"fingerprint"`
	Changes     []HardwareChange `json:"changes"`
}

type HealthAssessment struct {
	CPU           float64  `json:"cpu"`
	Memory        float64  `json:"memory"`
	Disk          float64  `json:"disk"`
	Heartbeat     float64  `json:"heartbeat"`
	PortalLatency float64  `json:"portalLatency"`
	Security      float64  `json:"security"`
	TPM           float64  `json:"tpm"`
	BitLocker     float64  `json:"bitLocker"`
	Defender      float64  `json:"defender"`
	Firewall      float64  `json:"firewall"`
	Overall       float64  `json:"overall"`
	Status        string   `json:"status"`
	Explanations  []string `json:"explanations"`
}
