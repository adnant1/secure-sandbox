package cgroups

import "path/filepath"

// sandboxPath returns the full cgroup directory path for a sandbox.
func (rm *ResourceManager) sandboxPath(id string) string {
	return filepath.Join(rm.basePath, id)
}

// memoryFile returns the path to memory.max
func (rm *ResourceManager) memoryFile(id string) string {
	return filepath.Join(rm.sandboxPath(id), "memory.max")
}

// cpuFile retruns the path to cpu.max
func (rm *ResourceManager) cpuFile(id string) string {
	return filepath.Join(rm.sandboxPath(id), "cpu.max")
}

// pidsFile returns the path to pids.max
func (rm *ResourceManager) pidsFile(id string) string {
	return filepath.Join(rm.sandboxPath(id), "pids.max")
}

// procsFile returns the path to cgroup.procs
func (rm *ResourceManager) procsFile(id string) string {
	return filepath.Join(rm.sandboxPath(id), "cgroup.procs")
}
