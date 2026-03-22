package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSandboxes handles:
//
// POST /sandboxes -> create + start
// GET /sandboxes -> list
func (s *Server) handleSandboxes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreate(w, r)
	case http.MethodGet:
		s.handleList(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSandboxByID handles:
//
// GET /sandboxes/{id} -> inspect
// GET /sandboxes/{id}/logs -> logs
// POST /sandboxes/{id}/stop -> stop
func (s *Server) handleSandboxByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/sandboxes/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "invalid sandbox id")
		return
	}

	id := parts[0]

	// /sandboxes/{id}
	if len(parts) == 1 && r.Method == http.MethodGet {
		s.handleInspect(w, r, id)
		return
	}
	// /sandboxes/{id}/logs
	if len(parts) == 2 && parts[1] == "logs" && r.Method == http.MethodGet {
		s.handleLogs(w, r, id)
		return
	}
	// /sandboxes/{id}/stop
	if len(parts) == 2 && parts[1] == "stop" && r.Method == http.MethodPost {
		s.handleStop(w, r, id)
		return
	}

	writeError(w, http.StatusNotFound, "endpoint not found")
}

// handleCreate handles POST /sandboxes
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateSandboxRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Convert API request → internal request
	internalReq := toManagerCreateRequest(req)
	sb, err := s.mgr.CreateSandbox(internalReq)
	if err != nil {
		s.mapError(w, err)
		return
	}
	sb, err = s.mgr.StartSandbox(sb.ID)
	if err != nil {
		s.mapError(w, err)
		return
	}
	resp := toSandboxResponse(sb)
	writeJSON(w, http.StatusOK, resp)
}

// handleList handles GET /sandboxes
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	sandboxes, err := s.mgr.ListSandboxes()
	if err != nil {
		s.mapError(w, err)
		return
	}

	var resp []SandboxResponse
	for _, sb := range sandboxes {
		resp = append(resp, toSandboxResponse(sb))
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleInspect handles GET /sandboxes/{id}
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request, id string) {
	sb, err := s.mgr.GetSandbox(id)
	if err != nil {
		s.mapError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSandboxResponse(sb))
}

// handleLogs handles GET /sandboxes/{id}/logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, id string) {
	logs, err := s.mgr.GetSandboxLogs(id)
	if err != nil {
		s.mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(logs))
}

// handleStop handles POST /sandboxes/{id}/stop
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request, id string) {
	sb, err := s.mgr.StopSandbox(id)
	if err != nil {
		s.mapError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSandboxResponse(sb))
}

// handleShutdown handles POST /shutdown
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	// Respond immediately
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "shutting down",
	})

	// OS Signal shutdown
	go func() {
		close(s.ShutdownCh)
	}()
}
