package auth

import "testing"

func TestGenerateAndHashToken(t *testing.T) {
	tok, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tok == "" || hash == "" || tok == hash {
		t.Fatalf("bad token/hash: %q %q", tok, hash)
	}
	if HashToken(tok) != hash {
		t.Error("HashToken does not match GenerateToken hash")
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"/account/security": "/account/security",
		"":                  "/account",
		"//evil.com":        "/account",
		"https://evil.com":  "/account",
		"/\\evil":           "/account",
		"relative":          "/account",
	}
	for in, want := range cases {
		if got := SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
