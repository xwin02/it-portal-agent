package heartbeat

import "testing"

func TestCalculateHealth(t *testing.T) {
	tests := []struct {
		name              string
		cpu, memory, disk float64
		status            string
	}{
		{name: "healthy", cpu: 20, memory: 30, disk: 40, status: "Healthy"},
		{name: "warning", cpu: 80, memory: 40, disk: 30, status: "Warning"},
		{name: "critical", cpu: 95, memory: 40, disk: 30, status: "Critical"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CalculateHealth(test.cpu, test.memory, test.disk).Status; got != test.status {
				t.Fatalf("status = %q, want %q", got, test.status)
			}
		})
	}
}
