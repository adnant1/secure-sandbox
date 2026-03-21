package sandbox

import "time"

type SandboxState int

const (
	CREATED SandboxState = iota
	STARTING
	RUNNING
	EXITED // process ran and finished (regardless of success/failure)
	FAILED // runtime could not execute the process properly
	CLEANED
)

// ResourceSpec represents the hardware resources allocated for its
// associated Sandbox
type ResourceSpec struct {
	CPU        int // Represents % of a single core (0-100)
	MemoryMB   int
	Pids       int
	TimeoutSec int
}

// Sandbox represents a unit of isolated workload execution along with its lifecycle state and associated metadata.
type Sandbox struct {
	ID         string
	PID        int
	State      SandboxState
	Command    string
	Args       []string
	Resources  ResourceSpec
	RootFSPath string
	LogPath    string
	BundlePath string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	Err        string
}
