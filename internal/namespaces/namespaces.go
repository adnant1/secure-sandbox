package namespaces

import (
	"errors"
	"os/exec"
	"sandbox-runtime/internal/sandbox"
	"syscall"
)

// NamespaceManager is responsible for configuring Linux namespaces for a
// sandbox process before execution
type NamespaceManager struct {
}

func New() *NamespaceManager {
	return &NamespaceManager{}
}

// Configure applies namespace settings to the given exec.Cmd based
// on the sandbox configuration
func (nm *NamespaceManager) Configure(cmd *exec.Cmd, sb *sandbox.Sandbox) error {
	if cmd == nil {
		return errors.New("namespace manager: nil exec.Cmd")
	}
	if sb == nil {
		return errors.New("namespace manager: nil sandbox")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS,
	}
	return nil
}
