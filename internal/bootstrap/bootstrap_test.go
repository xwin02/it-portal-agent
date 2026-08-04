package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itportal/it-portal-agent/internal/config"
	"go.uber.org/zap"
)

func TestConfigurationRequestUsesAgentToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/agent/configuration" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("X-ITPortal-Signature") == "" {
			t.Fatal("expected signed configuration request")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"configuration":{"heartbeat_interval":"5m"},"configuration_version":"7"}`))
	}))
	defer server.Close()

	service := New(nil, config.Config{PortalURL: server.URL, TLSValidation: true}, zap.NewNop(), "0.2.0")
	response, err := service.request(context.Background(), http.MethodGet, "/api/agent/configuration", "agent-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Configuration) != `{"heartbeat_interval":"5m"}` || response.ConfigurationVersion != "7" {
		t.Fatalf("unexpected response: %#v", response)
	}
}
