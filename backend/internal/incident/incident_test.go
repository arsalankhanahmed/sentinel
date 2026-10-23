package incident

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestLinkTokenEntropyAndHash(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok, hash := NewLinkToken()
		if len(tok) != 43 { // 32 bytes, base64url without padding
			t.Fatalf("token length %d", len(tok))
		}
		if strings.ContainsAny(tok, "+/=") {
			t.Fatalf("token must be URL-safe: %s", tok)
		}
		want := sha256.Sum256([]byte(tok))
		if !bytes.Equal(hash, want[:]) || !bytes.Equal(HashToken(tok), hash) {
			t.Fatal("hash mismatch")
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestPlaybookByKind(t *testing.T) {
	cases := map[string]string{
		"business_exception": "auto-resolves when the rule next returns 0 rows",
		"runtime_exception":  "group by error message",
		"db_outage":          "SELECT 1 succeeds again",
		"endpoint_outage":    "App Pool",
	}
	for kind, want := range cases {
		if got := Playbook(kind, ""); !strings.Contains(got, want) {
			t.Errorf("%s playbook missing %q:\n%s", kind, want, got)
		}
	}
}

func TestPlaybookIsContextAware(t *testing.T) {
	if !strings.HasPrefix(Playbook("db_outage", "context deadline exceeded (timeout)"), "Probe TIMED OUT") {
		t.Error("timeout hint should be first step")
	}
	if !strings.HasPrefix(Playbook("db_outage", "Login failed for user 'x'"), "Authentication failure") {
		t.Error("auth hint should be first step")
	}
	if !strings.HasPrefix(Playbook("endpoint_outage", "unexpected HTTP 503"), "HTTP 503") {
		t.Error("503 hint should be first step")
	}
}
