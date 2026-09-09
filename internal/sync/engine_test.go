package sync

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestEncodePayload(t *testing.T) {
	original := []byte(`{"heartbeat":true}`)
	plain, encoding, err := encodePayload(original, "")
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "" || !bytes.Equal(plain, original) {
		t.Fatalf("identity encoding changed payload: encoding=%q", encoding)
	}
	compressed, encoding, err := encodePayload(original, "gzip")
	if err != nil {
		t.Fatal(err)
	}
	if encoding != "gzip" {
		t.Fatalf("encoding = %q, want gzip", encoding)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded payload differs: %q", decoded)
	}
}

func TestRouteForType(t *testing.T) {
	if got := routeForType(HeartbeatType); got != "/api/agent/heartbeat" {
		t.Fatalf("heartbeat route = %q", got)
	}
	if got := routeForType(InventoryType); got != "/api/agent/inventory" {
		t.Fatalf("inventory route = %q", got)
	}
	if got := routeForType("future"); got != "" {
		t.Fatalf("future route = %q", got)
	}
}
