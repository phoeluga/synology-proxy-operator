package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
)

// Server provides HTTP endpoints for health checks
type Server struct {
	checker HealthChecker
	logger  logging.Logger
	server  *http.Server
}

// NewServer creates a new health check server
func NewServer(addr string, checker HealthChecker, logger logging.Logger) *Server {
	s := &Server{
		checker: checker,
		logger:  logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleLiveness)
	mux.HandleFunc("/readyz", s.handleReadiness)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

// Start starts the health check server
func (s *Server) Start() error {
	s.logger.Info("Starting health check server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the health check server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Stopping health check server")
	return s.server.Shutdown(ctx)
}

// handleLiveness handles liveness probe requests
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if s.checker.CheckLiveness(ctx) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not OK"))
	}
}

// handleReadiness handles readiness probe requests
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := s.checker.CheckReadiness(ctx)

	w.Header().Set("Content-Type", "application/json")
	if status.Ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}
