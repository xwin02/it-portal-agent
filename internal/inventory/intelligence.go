package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func bytesToGB(value uint64) float64 { return float64(value) / (1024 * 1024 * 1024) }

// Fingerprint hashes only hardware identity and capacity data. Volatile values
// such as free space, memory usage, IP addresses, users, and boot time are
// excluded so an unchanged machine is not uploaded every day.
func Fingerprint(hardware Hardware) (string, error) {
	normalized := normalizedHardware(hardware)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal fingerprint: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func normalizedHardware(input Hardware) Hardware {
	value := input
	value.RAM.Modules = append([]MemoryModule(nil), input.RAM.Modules...)
	value.Disk.Devices = append([]StorageDevice(nil), input.Disk.Devices...)
	value.Storage = append([]StorageDevice(nil), input.Storage...)
	value.NIC = append([]NetworkAdapter(nil), input.NIC...)
	value.Monitor = append([]Monitor(nil), input.Monitor...)
	value.GPU = append([]GPU(nil), input.GPU...)
	value.Users = Users{}
	value.OperatingSystem.LastBootTime = ""
	value.RAM.UsedBytes = 0
	value.RAM.AvailableBytes = 0
	value.RAM.UsedGB = 0
	value.RAM.AvailableGB = 0
	value.RAM.UsagePercent = 0
	value.Disk.FreeBytes = 0
	value.Disk.UsedBytes = 0
	value.Disk.TotalGB = 0
	value.Disk.UsedGB = 0
	value.Disk.FreeGB = 0
	for index := range value.Disk.Devices {
		value.Disk.Devices[index].FreeBytes = 0
		value.Disk.Devices[index].UsedBytes = 0
		value.Disk.Devices[index].UsedGB = 0
		value.Disk.Devices[index].FreeGB = 0
		value.Disk.Devices[index].CapacityGB = 0
	}
	for index := range value.Storage {
		value.Storage[index].FreeBytes = 0
		value.Storage[index].UsedBytes = 0
		value.Storage[index].UsedGB = 0
		value.Storage[index].FreeGB = 0
		value.Storage[index].CapacityGB = 0
	}
	for index := range value.NIC {
		value.NIC[index].IPAddress = nil
		value.NIC[index].Gateway = nil
		value.NIC[index].DNS = nil
		value.NIC[index].DHCPServer = ""
		value.NIC[index].Status = ""
	}
	value.Network = append([]NetworkAdapter(nil), value.NIC...)
	sort.Slice(value.RAM.Modules, func(i, j int) bool { return memoryKey(value.RAM.Modules[i]) < memoryKey(value.RAM.Modules[j]) })
	sort.Slice(value.Disk.Devices, func(i, j int) bool { return storageKey(value.Disk.Devices[i]) < storageKey(value.Disk.Devices[j]) })
	sort.Slice(value.Storage, func(i, j int) bool { return storageKey(value.Storage[i]) < storageKey(value.Storage[j]) })
	sort.Slice(value.NIC, func(i, j int) bool { return networkKey(value.NIC[i]) < networkKey(value.NIC[j]) })
	sort.Slice(value.Network, func(i, j int) bool { return networkKey(value.Network[i]) < networkKey(value.Network[j]) })
	sort.Slice(value.Monitor, func(i, j int) bool { return value.Monitor[i].Name < value.Monitor[j].Name })
	sort.Slice(value.GPU, func(i, j int) bool { return value.GPU[i].Name < value.GPU[j].Name })
	return value
}

func memoryKey(value MemoryModule) string {
	return strings.ToLower(value.Locator + "|" + value.PartNumber)
}
func storageKey(value StorageDevice) string {
	return strings.ToLower(value.DeviceID + "|" + value.SerialNumber + "|" + value.Model)
}
func networkKey(value NetworkAdapter) string {
	return strings.ToLower(value.MACAddress + "|" + value.Name)
}

// DiffHardware produces a small change timeline suitable for Portal history.
func DiffHardware(previous *Hardware, current Hardware) []HardwareChange {
	if previous == nil {
		return []HardwareChange{{Section: "hardware", Type: "initial", Detail: "Initial hardware snapshot"}}
	}
	left := normalizedHardware(*previous)
	right := normalizedHardware(current)
	sections := []struct {
		name string
		old  any
		new  any
	}{
		{"general", left.General, right.General},
		{"operatingSystem", left.OperatingSystem, right.OperatingSystem},
		{"cpu", left.CPU, right.CPU},
		{"memory", left.RAM, right.RAM},
		{"storage", left.Storage, right.Storage},
		{"network", left.NIC, right.NIC},
		{"motherboard", left.Motherboard, right.Motherboard},
		{"bios", left.BIOS, right.BIOS},
		{"security", left.Security, right.Security},
		{"display", left.Monitor, right.Monitor},
		{"gpu", left.GPU, right.GPU},
	}
	changes := make([]HardwareChange, 0)
	for _, section := range sections {
		if reflect.DeepEqual(section.old, section.new) {
			continue
		}
		detail := "Section changed"
		switch section.name {
		case "memory":
			detail = "Memory modules or capacity changed"
		case "storage":
			detail = "Disk inventory changed"
		case "bios":
			detail = "BIOS version or identity changed"
		case "operatingSystem":
			detail = "Windows version or build changed"
		case "network":
			detail = "Network adapter identity changed"
		}
		changes = append(changes, HardwareChange{Section: section.name, Type: "changed", Detail: detail})
	}
	return changes
}

func HardwareCount(hardware Hardware) int {
	count := 0
	if hardware.CPU.Model != "" {
		count++
	}
	if hardware.General.MachineUUID != "" || hardware.General.SerialNumber != "" {
		count++
	}
	count += len(hardware.RAM.Modules) + len(hardware.Disk.Devices) + len(hardware.Storage) + len(hardware.NIC) + len(hardware.Monitor) + len(hardware.GPU)
	if hardware.Motherboard.Product != "" {
		count++
	}
	if hardware.BIOS.Version != "" {
		count++
	}
	return count
}

func BuildAssetLinkHints(hardware Hardware, agentUUID, hostname string) AssetLinkHints {
	hints := AssetLinkHints{SerialNumber: hardware.General.SerialNumber, MachineUUID: hardware.General.MachineUUID, Hostname: hostname, ExistingAgentUUID: agentUUID, Priority: []string{"serialNumber", "machineUuid", "hostname", "existingAgentUuid"}, Action: "suggest"}
	return hints
}

func CalculateHealthV2(cpu, memory, disk float64, heartbeatStatus string, latency float64, security Security) HealthAssessment {
	assessment := HealthAssessment{CPU: usageScore(cpu), Memory: usageScore(memory), Disk: usageScore(disk), Heartbeat: heartbeatScore(heartbeatStatus), PortalLatency: latencyScore(latency)}
	assessment.TPM = securityScore(security.TPM)
	assessment.BitLocker = securityScore(security.BitLocker)
	assessment.Defender = securityScore(security.Defender)
	assessment.Firewall = securityScore(security.Firewall)
	assessment.Security = (assessment.TPM + assessment.BitLocker + assessment.Defender + assessment.Firewall) / 4
	assessment.Overall = assessment.CPU*0.20 + assessment.Memory*0.20 + assessment.Disk*0.20 + assessment.Heartbeat*0.15 + assessment.PortalLatency*0.10 + assessment.Security*0.15
	if assessment.Overall >= 80 {
		assessment.Status = "Healthy"
	} else if assessment.Overall >= 60 {
		assessment.Status = "Warning"
	} else {
		assessment.Status = "Critical"
	}
	if cpu >= 85 {
		assessment.Explanations = append(assessment.Explanations, "High CPU usage")
	}
	if memory >= 85 {
		assessment.Explanations = append(assessment.Explanations, "High RAM usage")
	}
	if disk >= 85 {
		assessment.Explanations = append(assessment.Explanations, "Disk almost full")
	}
	if strings.EqualFold(heartbeatStatus, "OFFLINE") {
		assessment.Explanations = append(assessment.Explanations, "Heartbeat is offline")
	}
	if latency > 500 {
		assessment.Explanations = append(assessment.Explanations, "High Portal latency")
	}
	if security.Firewall.Available && !security.Firewall.Enabled {
		assessment.Explanations = append(assessment.Explanations, "Firewall disabled")
	}
	if security.Defender.Available && !security.Defender.Enabled {
		assessment.Explanations = append(assessment.Explanations, "Windows Defender disabled")
	}
	if security.BitLocker.Available && !security.BitLocker.Enabled {
		assessment.Explanations = append(assessment.Explanations, "BitLocker disabled")
	}
	return assessment
}

func usageScore(value float64) float64 {
	if value < 0 || value > 100 {
		return 50
	}
	return 100 - value
}

func heartbeatScore(status string) float64 {
	switch strings.ToUpper(status) {
	case "ONLINE":
		return 100
	case "PENDING":
		return 50
	default:
		return 0
	}
}

func latencyScore(value float64) float64 {
	if value < 0 {
		return 50
	}
	if value <= 200 {
		return 100
	}
	if value <= 500 {
		return 70
	}
	if value <= 1000 {
		return 40
	}
	return 10
}

func securityScore(value SecurityState) float64 {
	if !value.Available {
		return 50
	}
	if value.Enabled {
		return 100
	}
	return 0
}
