package auth

import "testing"

func TestCSRF(t *testing.T) {
	tok := CSRFToken("secret", "sess1")
	if tok == "" {
		t.Fatal("empty token")
	}
	if !ValidCSRF("secret", "sess1", tok) {
		t.Error("valid token rejected")
	}
	if ValidCSRF("secret", "sess1", "wrong") {
		t.Error("wrong token accepted")
	}
	if ValidCSRF("secret", "other-session", tok) {
		t.Error("token accepted for a different session")
	}
}
