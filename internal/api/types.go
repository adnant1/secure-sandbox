package api

// ResourceOverrides represents optional resoruce limits provided
// via the HTTP API.
//
// This is intentionally decoupled from the internal sandbox.ResourceSpec
// to avoid leaking runtime details.
type ResourceOverrides struct {
	MemoryMB   int `json:"memoryMB,omitempty"`
	CPU        int `json:"cpu,omitempty"`
	Pids       int `json:"pids,omitempty"`
	TimeoutSec int `json:"timeoutSec,omitempty"`
}

// CreateSandboxRequest represents the request body for: POST /sandboxes
// This defines everything needed to create + start a sandbox.
//
// This is not the same as the internal manager.CreateSandboxRequest.
// This struct exists purely for API boundary isolation.
type CreateSandboxRequest struct {
	BundlePath string `json:"bundlePath"`

	// Optional overrides (if not provided, bundle config is used)
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Resources ResourceOverrides `json:"resources,omitempty"`
}

// SandboxResponse is the public representation of a sandbox returned by:
// - POST /sandboxes
// - GET /sandboxes
// - GET /sandboxes/{id}
//
// This is a sanitized, stable API response - not the actual internal sandbox.Sandbox.
type SandboxResponse struct {
	ID    string `json:"id"`
	PID   int    `json:"pid"`
	State string `json:"state"`

	// Optional overrides
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Resources ResourceOverrides `json:"resources,omitempty"`

	// Timestamps are strings to avoid exposing Go time internals
	CreatedAt  string `json:"createdAt,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`

	ExitReason string `json:"exitReason,omitempty"`
	ExitCode   *int   `json:"exitCode,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ErrorResponse represents a structured API error.
type ErrorResponse struct {
	Error string `json:"error"`
}
