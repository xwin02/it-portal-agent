//go:build windows

package inventory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"golang.org/x/sys/windows/registry"
)

const cimv2 = `root\CIMV2`

func Collect(ctx context.Context) (Hardware, error) {
	if err := ctx.Err(); err != nil {
		return Hardware{}, err
	}
	var hardware Hardware

	var systems []struct {
		Name, Manufacturer, Model, Domain, UserName string
		PartOfDomain                                bool
		TotalPhysicalMemory                         uint64
	}
	_ = query(`SELECT Name,Manufacturer,Model,Domain,UserName,PartOfDomain,TotalPhysicalMemory FROM Win32_ComputerSystem`, &systems)
	if len(systems) > 0 {
		system := systems[0]
		hardware.General.ComputerName = clean(system.Name)
		hardware.General.Manufacturer = clean(system.Manufacturer)
		hardware.General.Model = clean(system.Model)
		hardware.OperatingSystem.Domain = clean(system.Domain)
		if !system.PartOfDomain {
			hardware.OperatingSystem.Workgroup = clean(system.Domain)
		}
		hardware.Users.Current = clean(system.UserName)
		hardware.RAM.TotalBytes = system.TotalPhysicalMemory
	}

	var products []struct {
		UUID, Name, Vendor, Version, IdentifyingNumber string
	}
	_ = query(`SELECT UUID,Name,Vendor,Version,IdentifyingNumber FROM Win32_ComputerSystemProduct`, &products)
	if len(products) > 0 {
		product := products[0]
		hardware.General.MachineUUID = clean(product.UUID)
		if hardware.General.ProductName == "" {
			hardware.General.ProductName = clean(product.Name)
		}
		if hardware.General.Manufacturer == "" {
			hardware.General.Manufacturer = clean(product.Vendor)
		}
	}

	var enclosure []struct{ SMBIOSAssetTag, SerialNumber string }
	_ = query(`SELECT SMBIOSAssetTag,SerialNumber FROM Win32_SystemEnclosure`, &enclosure)
	if len(enclosure) > 0 {
		hardware.General.AssetTag = clean(enclosure[0].SMBIOSAssetTag)
		if hardware.General.SerialNumber == "" {
			hardware.General.SerialNumber = clean(enclosure[0].SerialNumber)
		}
	}

	var bios []struct{ Manufacturer, SMBIOSBIOSVersion, SerialNumber, ReleaseDate string }
	_ = query(`SELECT Manufacturer,SMBIOSBIOSVersion,SerialNumber,ReleaseDate FROM Win32_BIOS`, &bios)
	if len(bios) > 0 {
		hardware.BIOS = BIOS{Manufacturer: clean(bios[0].Manufacturer), Version: clean(bios[0].SMBIOSBIOSVersion), SerialNumber: clean(bios[0].SerialNumber), ReleaseDate: clean(bios[0].ReleaseDate)}
		if hardware.General.SerialNumber == "" {
			hardware.General.SerialNumber = hardware.BIOS.SerialNumber
		}
	}
	hardware.SerialNumber = hardware.General.SerialNumber

	var boards []struct{ Manufacturer, Product, SerialNumber string }
	_ = query(`SELECT Manufacturer,Product,SerialNumber FROM Win32_BaseBoard`, &boards)
	if len(boards) > 0 {
		hardware.Motherboard = Motherboard{Manufacturer: clean(boards[0].Manufacturer), Product: clean(boards[0].Product), SerialNumber: clean(boards[0].SerialNumber)}
	}

	var operatingSystems []struct {
		Caption, Version, BuildNumber, OSArchitecture, InstallDate, LastBootUpTime, CSName string
		FreePhysicalMemory, TotalVisibleMemorySize                                         uint64
	}
	_ = query(`SELECT Caption,Version,BuildNumber,OSArchitecture,InstallDate,LastBootUpTime,CSName,FreePhysicalMemory,TotalVisibleMemorySize FROM Win32_OperatingSystem`, &operatingSystems)
	if len(operatingSystems) > 0 {
		os := operatingSystems[0]
		hardware.OperatingSystem = OperatingSystem{Edition: clean(os.Caption), Version: clean(os.Version), BuildNumber: clean(os.BuildNumber), Architecture: clean(os.OSArchitecture), InstallDate: clean(os.InstallDate), LastBootTime: clean(os.LastBootUpTime), Domain: hardware.OperatingSystem.Domain, Workgroup: hardware.OperatingSystem.Workgroup, Timezone: timezone()}
		if os.TotalVisibleMemorySize > 0 {
			hardware.RAM.AvailableBytes = os.FreePhysicalMemory * 1024
			hardware.RAM.TotalBytes = os.TotalVisibleMemorySize * 1024
			if hardware.RAM.TotalBytes > hardware.RAM.AvailableBytes {
				hardware.RAM.UsedBytes = hardware.RAM.TotalBytes - hardware.RAM.AvailableBytes
			}
			hardware.RAM.TotalGB = bytesToGB(hardware.RAM.TotalBytes)
			hardware.RAM.AvailableGB = bytesToGB(hardware.RAM.AvailableBytes)
			hardware.RAM.UsedGB = bytesToGB(hardware.RAM.UsedBytes)
			if hardware.RAM.TotalBytes > 0 {
				hardware.RAM.UsagePercent = float64(hardware.RAM.UsedBytes) / float64(hardware.RAM.TotalBytes) * 100
			}
		}
	}
	if hardware.OperatingSystem.Timezone == "" {
		hardware.OperatingSystem.Timezone = timezone()
	}
	if hardware.RAM.TotalBytes > 0 && hardware.RAM.TotalGB == 0 {
		hardware.RAM.TotalGB = bytesToGB(hardware.RAM.TotalBytes)
		hardware.RAM.AvailableGB = bytesToGB(hardware.RAM.AvailableBytes)
		hardware.RAM.UsedGB = bytesToGB(hardware.RAM.UsedBytes)
		if hardware.RAM.TotalBytes > 0 {
			hardware.RAM.UsagePercent = float64(hardware.RAM.UsedBytes) / float64(hardware.RAM.TotalBytes) * 100
		}
	}

	var processors []struct {
		Name, Manufacturer, SocketDesignation                                string
		NumberOfLogicalProcessors, NumberOfCores, ThreadCount, MaxClockSpeed uint32
	}
	_ = query(`SELECT Name,Manufacturer,SocketDesignation,NumberOfLogicalProcessors,NumberOfCores,ThreadCount,MaxClockSpeed FROM Win32_Processor`, &processors)
	for _, processor := range processors {
		hardware.CPU.Model = clean(processor.Name)
		hardware.CPU.Manufacturer = clean(processor.Manufacturer)
		hardware.CPU.Socket = clean(processor.SocketDesignation)
		hardware.CPU.LogicalCores += processor.NumberOfLogicalProcessors
		hardware.CPU.PhysicalCores += processor.NumberOfCores
		hardware.CPU.Threads += processor.ThreadCount
		if processor.MaxClockSpeed > hardware.CPU.ClockMHz {
			hardware.CPU.ClockMHz = processor.MaxClockSpeed
		}
	}

	var memory []struct {
		DeviceLocator, Manufacturer, PartNumber string
		Capacity                                uint64
		Speed                                   uint32
	}
	_ = query(`SELECT DeviceLocator,Manufacturer,PartNumber,Capacity,Speed FROM Win32_PhysicalMemory`, &memory)
	hardware.RAM.Slots = uint32(len(memory))
	for _, module := range memory {
		hardware.RAM.Modules = append(hardware.RAM.Modules, MemoryModule{Locator: clean(module.DeviceLocator), Manufacturer: clean(module.Manufacturer), PartNumber: clean(module.PartNumber), Capacity: module.Capacity, SpeedMHz: module.Speed})
	}

	var logicalDisks []struct {
		DeviceID, VolumeName, FileSystem string
		MediaType                        uint32
		Size, FreeSpace                  uint64
		DriveType                        uint32
	}
	_ = query(`SELECT DeviceID,VolumeName,FileSystem,MediaType,Size,FreeSpace,DriveType FROM Win32_LogicalDisk`, &logicalDisks)
	for _, disk := range logicalDisks {
		if disk.Size == 0 {
			continue
		}
		used := uint64(0)
		if disk.FreeSpace <= disk.Size {
			used = disk.Size - disk.FreeSpace
		}
		device := StorageDevice{DeviceID: clean(disk.DeviceID), Type: normalizeStorageType(strconv.FormatUint(uint64(disk.MediaType), 10), ""), TotalBytes: disk.Size, FreeBytes: disk.FreeSpace, UsedBytes: used, CapacityGB: bytesToGB(disk.Size), UsedGB: bytesToGB(used), FreeGB: bytesToGB(disk.FreeSpace), Filesystem: clean(disk.FileSystem), DriveType: driveTypeName(disk.DriveType)}
		hardware.Disk.Devices = append(hardware.Disk.Devices, device)
		hardware.Disk.TotalBytes += disk.Size
		hardware.Disk.FreeBytes += disk.FreeSpace
	}
	if hardware.Disk.FreeBytes <= hardware.Disk.TotalBytes {
		hardware.Disk.UsedBytes = hardware.Disk.TotalBytes - hardware.Disk.FreeBytes
	}
	var physicalDisks []struct {
		DeviceID, Model, SerialNumber, MediaType, InterfaceType, Status string
		Size                                                            uint64
	}
	_ = query(`SELECT DeviceID,Model,SerialNumber,MediaType,InterfaceType,Status,Size FROM Win32_DiskDrive`, &physicalDisks)
	smartStatus := collectSMARTStatus()
	for _, disk := range physicalDisks {
		for index := range hardware.Disk.Devices {
			if strings.HasPrefix(hardware.Disk.Devices[index].DeviceID, disk.DeviceID) {
				hardware.Disk.Devices[index].Model = clean(disk.Model)
				hardware.Disk.Devices[index].SerialNumber = clean(disk.SerialNumber)
				hardware.Disk.Devices[index].Health = clean(disk.Status)
				hardware.Disk.Devices[index].SMARTStatus = smartStatus
				hardware.Disk.Devices[index].Type = normalizeStorageType(disk.MediaType, disk.Model)
			}
		}
		if disk.Size > 0 {
			hardware.Storage = append(hardware.Storage, StorageDevice{DeviceID: clean(disk.DeviceID), Type: normalizeStorageType(disk.MediaType, disk.Model), Model: clean(disk.Model), SerialNumber: clean(disk.SerialNumber), Health: clean(disk.Status), SMARTStatus: smartStatus, TotalBytes: disk.Size, CapacityGB: bytesToGB(disk.Size)})
			if hardware.Disk.Model == "" {
				hardware.Disk.Model = clean(disk.Model)
				hardware.Disk.Type = normalizeStorageType(disk.MediaType, disk.Model)
				hardware.Disk.SerialNumber = clean(disk.SerialNumber)
				hardware.Disk.Health = clean(disk.Status)
				hardware.Disk.SMARTStatus = smartStatus
			}
		}
	}
	hardware.Disk.TotalGB = bytesToGB(hardware.Disk.TotalBytes)
	hardware.Disk.UsedGB = bytesToGB(hardware.Disk.UsedBytes)
	hardware.Disk.FreeGB = bytesToGB(hardware.Disk.FreeBytes)

	var adapters []struct {
		Description, MACAddress, DHCPServer               string
		IPAddress, DefaultIPGateway, DNSServerSearchOrder []string
		DHCPEnabled                                       bool
	}
	_ = query(`SELECT Description,MACAddress,DHCPServer,IPAddress,DefaultIPGateway,DNSServerSearchOrder,DHCPEnabled FROM Win32_NetworkAdapterConfiguration WHERE IPEnabled=TRUE`, &adapters)
	for _, adapter := range adapters {
		item := NetworkAdapter{Name: clean(adapter.Description), Description: clean(adapter.Description), IPAddress: cleanStrings(adapter.IPAddress), MACAddress: clean(adapter.MACAddress), Gateway: cleanStrings(adapter.DefaultIPGateway), DNS: cleanStrings(adapter.DNSServerSearchOrder), DHCP: adapter.DHCPEnabled, DHCPServer: clean(adapter.DHCPServer), Status: "up"}
		hardware.NIC = append(hardware.NIC, item)
	}
	var adapterDetails []struct {
		Name, NetConnectionID, MACAddress string
		Speed                             uint64
		NetConnectionStatus               uint32
	}
	_ = query(`SELECT Name,NetConnectionID,MACAddress,Speed,NetConnectionStatus FROM Win32_NetworkAdapter`, &adapterDetails)
	for _, detail := range adapterDetails {
		matched := false
		for index := range hardware.NIC {
			if (clean(detail.MACAddress) != "" && clean(detail.MACAddress) == hardware.NIC[index].MACAddress) || (hardware.NIC[index].Description != "" && clean(detail.Name) == hardware.NIC[index].Description) {
				matched = true
				hardware.NIC[index].Name = clean(detail.Name)
				hardware.NIC[index].LinkSpeed = detail.Speed
				hardware.NIC[index].Status = networkStatus(detail.NetConnectionStatus)
			}
		}
		if !matched && clean(detail.Name) != "" {
			hardware.NIC = append(hardware.NIC, NetworkAdapter{Name: clean(detail.Name), MACAddress: clean(detail.MACAddress), LinkSpeed: detail.Speed, Status: networkStatus(detail.NetConnectionStatus)})
		}
	}
	hardware.Network = hardware.NIC

	var monitors []struct {
		Name, PNPDeviceID         string
		ScreenWidth, ScreenHeight uint32
	}
	_ = query(`SELECT Name,PNPDeviceID,ScreenWidth,ScreenHeight FROM Win32_DesktopMonitor`, &monitors)
	for _, monitor := range monitors {
		hardware.Monitor = append(hardware.Monitor, Monitor{Name: clean(monitor.Name), SerialNumber: clean(monitor.PNPDeviceID), Width: monitor.ScreenWidth, Height: monitor.ScreenHeight})
	}
	var gpus []struct {
		Name, DriverVersion                                    string
		CurrentHorizontalResolution, CurrentVerticalResolution uint32
	}
	_ = query(`SELECT Name,DriverVersion,CurrentHorizontalResolution,CurrentVerticalResolution FROM Win32_VideoController`, &gpus)
	for _, gpu := range gpus {
		hardware.GPU = append(hardware.GPU, GPU{Name: clean(gpu.Name), DriverVersion: clean(gpu.DriverVersion), Width: gpu.CurrentHorizontalResolution, Height: gpu.CurrentVerticalResolution})
	}

	hardware.Security = collectSecurity()
	hardware.Users.Last = registryString(`SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI`, "LastLoggedOnUser")
	if hardware.Users.Last == "" {
		hardware.Users.Last = registryString(`SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI`, "LastLoggedOnDisplayName")
	}
	return hardware, nil
}

func query(statement string, destination interface{}) error {
	return wmi.QueryNamespace(statement, destination, cimv2)
}

func collectSecurity() Security {
	security := Security{}
	var tpms []struct {
		IsEnabled_InitialValue   bool
		IsActivated_InitialValue bool
		SpecVersion              string
	}
	if err := wmi.QueryNamespace(`SELECT IsEnabled_InitialValue,IsActivated_InitialValue,SpecVersion FROM Win32_Tpm`, &tpms, `root\CIMV2\Security\MicrosoftTpm`); err == nil {
		security.TPM.Available = len(tpms) > 0
		if len(tpms) > 0 {
			security.TPM.Enabled = tpms[0].IsEnabled_InitialValue && tpms[0].IsActivated_InitialValue
			security.TPM.Status = clean(tpms[0].SpecVersion)
		}
	}
	secureBoot, found := registryDWORD(`SYSTEM\CurrentControlSet\Control\SecureBoot\State`, "UEFISecureBootEnabled")
	security.SecureBoot = SecurityState{Available: found, Enabled: found && secureBoot == 1, Status: boolStatus(found && secureBoot == 1, found)}
	var volumes []struct{ ProtectionStatus, VolumeStatus uint32 }
	if err := wmi.QueryNamespace(`SELECT ProtectionStatus,VolumeStatus FROM Win32_EncryptableVolume`, &volumes, `root\CIMV2\Security\MicrosoftVolumeEncryption`); err == nil {
		security.BitLocker.Available = len(volumes) > 0
		for _, volume := range volumes {
			if volume.ProtectionStatus == 1 {
				security.BitLocker.Enabled = true
				break
			}
		}
		security.BitLocker.Status = boolStatus(security.BitLocker.Enabled, true)
	}
	defenderDisable, defenderFound := registryDWORD(`SOFTWARE\Microsoft\Windows Defender`, "DisableAntiSpyware")
	security.Defender = SecurityState{Available: true, Enabled: !defenderFound || defenderDisable == 0, Status: boolStatus(!defenderFound || defenderDisable == 0, true)}
	var defender []struct {
		AMServiceEnabled, AntispywareEnabled, AntivirusEnabled, RealTimeProtectionEnabled bool
	}
	if err := wmi.QueryNamespace(`SELECT AMServiceEnabled,AntispywareEnabled,AntivirusEnabled,RealTimeProtectionEnabled FROM MSFT_MpComputerStatus`, &defender, `root\Microsoft\Windows\Defender`); err == nil && len(defender) > 0 {
		enabled := defender[0].AMServiceEnabled && defender[0].AntispywareEnabled && defender[0].AntivirusEnabled && defender[0].RealTimeProtectionEnabled
		security.Defender = SecurityState{Available: true, Enabled: enabled, Status: boolStatus(enabled, true)}
	}
	firewall, firewallFound := registryDWORD(`SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\StandardProfile`, "EnableFirewall")
	security.Firewall = SecurityState{Available: firewallFound, Enabled: firewallFound && firewall != 0, Status: boolStatus(firewall != 0, firewallFound)}
	return security
}

func collectSMARTStatus() string {
	var values []struct{ PredictFailure bool }
	if err := wmi.QueryNamespace(`SELECT PredictFailure FROM MSStorageDriver_FailurePredictStatus`, &values, `root\wmi`); err != nil || len(values) == 0 {
		return "unknown"
	}
	for _, value := range values {
		if value.PredictFailure {
			return "failure"
		}
	}
	return "healthy"
}

func registryDWORD(path, name string) (uint32, bool) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return 0, false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue(name)
	return uint32(value), err == nil
}

func registryString(path, name string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	return clean(value)
}

func timezone() string {
	if value := registryString(`SYSTEM\CurrentControlSet\Control\TimeZoneInformation`, "TimeZoneKeyName"); value != "" {
		return value
	}
	return time.Now().Location().String()
}

func clean(value string) string { return strings.TrimSpace(value) }

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = clean(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func boolStatus(enabled, available bool) string {
	if !available {
		return "unknown"
	}
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func normalizeStorageType(mediaType, model string) string {
	value := strings.ToLower(mediaType + " " + model)
	switch {
	case strings.Contains(value, "nvme"):
		return "NVMe"
	case strings.Contains(value, "ssd") || strings.Contains(value, "solid"):
		return "SSD"
	case strings.Contains(value, "hdd") || strings.Contains(value, "hard"):
		return "HDD"
	default:
		return "unknown"
	}
}

func driveTypeName(value uint32) string {
	switch value {
	case 3:
		return "fixed"
	case 2:
		return "removable"
	case 4:
		return "network"
	case 5:
		return "optical"
	default:
		return "unknown"
	}
}

func networkStatus(value uint32) string {
	switch value {
	case 2:
		return "up"
	case 7:
		return "down"
	default:
		return "unknown"
	}
}
