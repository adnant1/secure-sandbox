package sandbox

import "time"

type SandboxState int

const (
	CREATED SandboxState = iota
	STARTING
	RUNNING
	EXITED
	FAILED
	CLEANED
)

// ResourceSpec represents the hardware resources allocated for its
// associated Sandbox
type ResourceSpec struct {
	CPU        int
	MemoryMB   int
	TimeoutSec int
}

// Sandbox represents a unit of isolated workload execution along with its
// lifecycle state and associated metadata.
type Sandbox struct {
	ID         string
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
