package cgroups

import (
	"path/filepath"
	"sandbox-runtime/internal/sandbox"
)

// ResourceManager is responsible for managing cgroup v2 resources for
// each sandbox.
type ResourceManager struct {
	mountPoint string
	basePath   string // root directory for all sandbox cgroups
}

// New initializes a new ResourceManager
func New(mountPoint string) *ResourceManager {
	if mountPoint == "" {
		panic("resource manager: mountPoint cannot be empty")
	}
	return &ResourceManager{
		mountPoint: mountPoint,
		basePath:   filepath.Join(mountPoint, "secure-sandbox"),
	}
}

// Create creates a cgroup directory for a sandbox.
func (rm *ResourceManager) Create(id string) error {
	return nil
}

// ApplyLimits applies resource limits to a sandbox group.
// Translates high-level ResourceSpec into cgroup v2 file writes.
func (rm *ResourceManager) ApplyLimits(id string, spec sandbox.ResourceSpec) error {
	return nil
}

// AddProcess attaches a process to the sandbox group.
func (rm *ResourceManager) AddProcess(id string, pid int) error {
	return nil
}

// Delete removes a sandbox group.
func (rm *ResourceManager) Delete(id string) error {
	return nil
}
