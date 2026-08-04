package security

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://portal.example.com/api/test", strings.NewReader("payload"))
	if err != nil { t.Fatal(err) }
	if err := SignRequest(req, "secret", time.Unix(1_700_000_000, 0)); err != nil { t.Fatal(err) }
	if req.Header.Get("X-ITPortal-Timestamp") != "1700000000" { t.Fatal("timestamp header was not set") }
	if req.Header.Get("X-ITPortal-Nonce") == "" || req.Header.Get("X-ITPortal-Signature") == "" { t.Fatal("signature headers were not set") }
}
