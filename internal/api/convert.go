package api

import (
	"sandbox-runtime/internal/manager"
	"sandbox-runtime/internal/sandbox"
	"time"
)

// toManagerCreateRequest converts API request → internal request
func toManagerCreateRequest(req CreateSandboxRequest) manager.CreateSandboxRequest {
	var resources *sandbox.ResourceSpec

	// Only set resources if any override is provided
	if req.Resources.MemoryMB != 0 ||
		req.Resources.CPU != 0 ||
		req.Resources.Pids != 0 {

		resources = &sandbox.ResourceSpec{
			MemoryMB: req.Resources.MemoryMB,
			CPU:      req.Resources.CPU,
			Pids:     req.Resources.Pids,
		}
	}

	return manager.CreateSandboxRequest{
		BundlePath: req.BundlePath,
		Command:    req.Command,
		Args:       req.Args,
		Resources:  resources,
	}
}

// toSandboxResponse converts internal sandbox → API response
func toSandboxResponse(sb *sandbox.Sandbox) SandboxResponse {
	var exitCode *int
	if sb.State == sandbox.EXITED {
		exitCode = &sb.ExitCode
	}

	return SandboxResponse{
		ID:    sb.ID,
		PID:   sb.PID,
		State: sb.State.String(),

		Command: sb.Command,
		Args:    sb.Args,

		Resources: ResourceOverrides{
			MemoryMB: sb.Resources.MemoryMB,
			CPU:      sb.Resources.CPU,
			Pids:     sb.Resources.Pids,
		},

		CreatedAt:  formatTime(sb.CreatedAt),
		StartedAt:  formatTime(sb.StartedAt),
		FinishedAt: formatTime(sb.FinishedAt),

		ExitReason: string(sb.ExitReason),
		ExitCode:   exitCode,
		Error:      sb.Err,
	}
}

// formatTime safely formats time.Time → string
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
