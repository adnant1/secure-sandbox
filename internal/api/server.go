package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sandbox-runtime/internal/manager"
	"time"
)

// Server represents the sandbox daemon (sandboxd).
//
// It is responsible for exposing the Manager (control plane)
// over a Unix domain socket using an HTTP API.
type Server struct {
	mgr        *manager.Manager
	httpServer *http.Server
	socketPath string

	Debug bool // flag to expose internal error messages in this mode
}

// New initializes the API server with the given manager and socket path.
func New(mgr *manager.Manager, socketPath string) *Server {
	s := &Server{
		mgr:        mgr,
		socketPath: socketPath,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sandboxes", s.handleSandboxes)
	mux.HandleFunc("/sandboxes/", s.handleSandboxByID)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	s.httpServer = &http.Server{
		Handler: mux,
	}
	return s
}

// Start initializes the Unix socket and begins serving HTTP requests.
func (s *Server) Start() error {
	// Stale socket
	if err := s.cleanupSocket(); err != nil {
		return fmt.Errorf("cleanup socket: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket: %w", err)
	}

	// restrict socket permissions
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		return fmt.Errorf("set socket permissions: %w", err)
	}

	if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// cleanupSocket removes stale socket file if needed.
func (s *Server) cleanupSocket() error {
	_, err := os.Stat(s.socketPath)
	if err == nil {
		// Socket exists
		conn, dialErr := net.DialTimeout("unix", s.socketPath, 1*time.Second)
		if dialErr == nil {
			conn.Close()
			return errors.New("daemon already running")
		}
		// Stale socket
		return os.Remove(s.socketPath)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
