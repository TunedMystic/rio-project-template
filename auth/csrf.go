package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// CSRFToken derives a per-session token from the app secret. No storage needed.
func CSRFToken(secret, sessionID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidCSRF compares a submitted token against the expected one in constant time.
func ValidCSRF(secret, sessionID, token string) bool {
	expected := CSRFToken(secret, sessionID)
	return hmac.Equal([]byte(expected), []byte(token))
}
