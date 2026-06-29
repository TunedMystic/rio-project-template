package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// shutdownGrace is how long in-flight requests get to finish on shutdown.
const shutdownGrace = 10 * time.Second

// newHTTPServer builds an http.Server with production timeouts (mirroring rio's
// defaults) for the given address and handler.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		IdleTimeout:    time.Minute,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 524288,
	}
}

// serve runs srv on ln until ctx is cancelled, then shuts down gracefully,
// letting in-flight requests finish within shutdownGrace. It returns nil on a
// clean shutdown.
func serve(ctx context.Context, srv *http.Server, ln net.Listener) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
