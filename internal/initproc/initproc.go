package initproc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Run executes the internal bootstrap ("init") path for a sandbox.
//
// This function runs inside a child process that has been re-executed
// via /proc/self/exe in "init" mode. It is responsible for preparing
// the sandbox execution environemnt and then replacing itself with
// the target workload using syscall.Exec
func Run(args []string) error {
	// Expected args:
	// [0] sandbox ID
	// [1] rootfs path
	// [2] command
	// [3...] command args
	if len(args) < 3 {
		return fmt.Errorf("init: insufficient arguments")
	}

	sandboxID := args[0]
	rootfs := args[1]
	cmd := args[2]
	cmdArgs := args[3:]
	if cmd == "" {
		return fmt.Errorf("init: empty command for sandbox %q", sandboxID)
	}

	// Filesystem isolation
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("init: failed to make mount propagation private: %w", err)
	}
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("init: failed to bind mount rootfs: %w", err)
	}
	oldRoot := filepath.Join(rootfs, ".oldroot")
	if err := os.MkdirAll(oldRoot, 0o755); err != nil {
		return fmt.Errorf("init: failed to create oldroot directory: %w", err)
	}
	if err := syscall.PivotRoot(rootfs, oldRoot); err != nil {
		return fmt.Errorf("init: pivot_root failed: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("init: failed to chdir to new root: %w", err)
	}
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("init: failed to unmount old root: %w", err)
	}
	if err := os.RemoveAll("/.oldroot"); err != nil {
		return fmt.Errorf("init: failed to remove old root directory: %w", err)
	}

	// Process isolation
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("init: failed to mount /proc: %w", err)
	}

	// Resolve the command ourselves, assuming our filesystem is correct
	binary := cmd
	if !strings.HasPrefix(cmd, "/") {
		binary = "/bin/" + cmd
	}

	return syscall.Exec(binary, append([]string{cmd}, cmdArgs...), []string{"PATH=/bin"})
}
