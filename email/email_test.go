package email

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsole_LogsMessage(t *testing.T) {
	var buf bytes.Buffer
	c := Console{Log: log.New(&buf, "", 0)}
	if err := c.Send(context.Background(), "to@example.com", "Subject", "Body link https://x/y"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "to@example.com") || !strings.Contains(out, "https://x/y") {
		t.Errorf("console output missing recipient or body: %q", out)
	}
}

func TestPostmark_PostsToAPI(t *testing.T) {
	var gotToken string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Postmark-Server-Token")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ErrorCode":0}`)
	}))
	defer srv.Close()

	p := Postmark{Token: "tok", From: "from@example.com", BaseURL: srv.URL, Client: srv.Client()}
	if err := p.Send(context.Background(), "to@example.com", "Subj", "the body"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotToken != "tok" {
		t.Errorf("token header = %q", gotToken)
	}
	if gotBody["To"] != "to@example.com" || gotBody["From"] != "from@example.com" {
		t.Errorf("body = %+v", gotBody)
	}
}
