package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServer_SetsTimeouts(t *testing.T) {
	srv := newHTTPServer(":3000", http.NewServeMux())
	if srv.Addr != ":3000" {
		t.Errorf("Addr = %q, want :3000", srv.Addr)
	}
	if srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 {
		t.Errorf("timeouts not set: read=%v write=%v idle=%v",
			srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
}

func TestServe_GracefulShutdownOnContextCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln) }()

	url := "http://" + ln.Addr().String() + "/"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET while serving: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Cancelling the context triggers a graceful shutdown; serve returns nil.
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after context cancel")
	}

	// The server no longer accepts connections.
	if _, err := http.Get(url); err == nil {
		t.Error("expected connection error after shutdown")
	}
}
