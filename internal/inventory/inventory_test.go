package inventory

import (
	"encoding/json"
	"testing"
)

func TestPayloadShape(t *testing.T) {
	payload := Payload{Hardware: Hardware{CPU: CPU{Model: "test"}, RAM: Memory{TotalBytes: 1024}, Disk: DiskInventory{TotalGB: 1}}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	hardware, ok := decoded["hardware"].(map[string]any)
	if !ok {
		t.Fatal("hardware object missing")
	}
	for _, field := range []string{"cpu", "ram", "disk", "storage", "nic", "security", "monitor", "gpu", "motherboard", "bios"} {
		if _, ok := hardware[field]; !ok {
			t.Fatalf("hardware.%s missing", field)
		}
	}
}

func TestNormalizeStorageType(t *testing.T) {
	for _, test := range []struct {
		media, model, want string
	}{
		{"", "Samsung NVMe SSD", "NVMe"},
		{"", "Solid State Drive", "SSD"},
		{"", "Hard Disk Drive", "HDD"},
		{"", "unknown", "unknown"},
	} {
		if got := normalizeStorageType(test.media, test.model); got != test.want {
			t.Fatalf("normalizeStorageType(%q, %q) = %q, want %q", test.media, test.model, got, test.want)
		}
	}
}

func TestFingerprintIgnoresVolatileValues(t *testing.T) {
	hardware := Hardware{
		General: General{MachineUUID: "machine-1", SerialNumber: "serial-1"},
		RAM:     Memory{TotalBytes: 16 * 1024 * 1024 * 1024, UsedBytes: 8, AvailableBytes: 7},
		Disk:    DiskInventory{TotalBytes: 100, FreeBytes: 50, Devices: []StorageDevice{{DeviceID: "C:", Model: "NVMe", TotalBytes: 100, FreeBytes: 50}}},
		NIC:     []NetworkAdapter{{MACAddress: "00:11:22:33:44:55", IPAddress: []string{"10.0.0.2"}}},
	}
	first, err := Fingerprint(hardware)
	if err != nil {
		t.Fatal(err)
	}
	hardware.RAM.UsedBytes = 15
	hardware.RAM.AvailableBytes = 1
	hardware.Disk.FreeBytes = 10
	hardware.Disk.Devices[0].FreeBytes = 10
	hardware.NIC[0].IPAddress = []string{"10.0.0.3"}
	second, err := Fingerprint(hardware)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("volatile values changed fingerprint: %s != %s", first, second)
	}
	hardware.Disk.Devices[0].Model = "Different SSD"
	third, err := Fingerprint(hardware)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("hardware change did not change fingerprint")
	}
}

func TestDiffAndHealth(t *testing.T) {
	previous := Hardware{RAM: Memory{Modules: []MemoryModule{{PartNumber: "old"}}}, BIOS: BIOS{Version: "1"}}
	current := Hardware{RAM: Memory{Modules: []MemoryModule{{PartNumber: "new"}}}, BIOS: BIOS{Version: "2"}}
	changes := DiffHardware(&previous, current)
	if len(changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(changes))
	}
	health := CalculateHealthV2(20, 92, 96, "ONLINE", 700, Security{Firewall: SecurityState{Available: true, Enabled: false}})
	if health.Status != "Critical" || len(health.Explanations) == 0 {
		t.Fatalf("unexpected health assessment: %+v", health)
	}
}

func TestBytesToGB(t *testing.T) {
	if got := bytesToGB(1024 * 1024 * 1024); got != 1 {
		t.Fatalf("bytesToGB = %v, want 1", got)
	}
}
