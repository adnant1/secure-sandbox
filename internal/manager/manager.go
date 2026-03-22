package manager

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sandbox-runtime/internal/cgroups"
	"sandbox-runtime/internal/config"
	"sandbox-runtime/internal/namespaces"
	"sandbox-runtime/internal/sandbox"
	"sandbox-runtime/internal/state"
	"syscall"
	"time"
)

const maxIDSamplingAttempts = 10
const defaultTimeoutSec = 120

var ErrInvalidStateTransition = errors.New("invalid sandbox state transition")

type CreateSandboxRequest struct {
	BundlePath string

	// Optional overrides
	Command   string
	Args      []string
	Resources *sandbox.ResourceSpec
}

type BundleConfig struct {
	Command   string               `json:"command"`
	Args      []string             `json:"args"`
	Resources sandbox.ResourceSpec `json:"resources"`
}

// Manager coordinates sandbox lifecycle operations and enforces runtime invariants.
type Manager struct {
	store *state.StateStore
	ns    *namespaces.NamespaceManager
	cg    *cgroups.ResourceManager
	cfg   config.Config
}

// New initializes and returns a new Manager with the given StateStore
func New(store *state.StateStore, cg *cgroups.ResourceManager, cfg config.Config) *Manager {
	if store == nil {
		panic("manager: nil state store")
	}
	if cg == nil {
		panic("manager: nil resource manager")
	}
	if cfg.RootDir == "" {
		panic("manager: root dir cannot be empty")
	}
	if cfg.InitBinaryPath == "" {
		panic("manager: init binary path cannot be empty")
	}
	if err := os.MkdirAll(cfg.RootDir, 0o755); err != nil {
		panic(fmt.Sprintf("manager: failed to create root dir %q: %v", cfg.RootDir, err))
	}

	return &Manager{
		store: store,
		ns:    namespaces.New(),
		cg:    cg,
		cfg:   cfg,
	}
}

// CreateSandbox creates a new sandbox, initializes its metadata,
// and registers it in the state store.
func (m *Manager) CreateSandbox(req CreateSandboxRequest) (*sandbox.Sandbox, error) {
	if req.BundlePath == "" {
		return nil, errors.New("invalid bundle path")
	}
	if _, err := os.Stat(req.BundlePath); err != nil {
		return nil, fmt.Errorf("bundle path does not exist: %w", err)
	}
	absBundlePath, err := filepath.Abs(req.BundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute bundle path: %w", err)
	}

	rootFSPath := filepath.Join(absBundlePath, "rootfs")
	info, err := os.Stat(rootFSPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("rootfs does not exist at path: %s", rootFSPath)
		}
		return nil, fmt.Errorf("failed to validate rootfs path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("rootfs path exists but is not a directory: %s", rootFSPath)
	}

	// Load bundle config
	cfgPath := filepath.Join(absBundlePath, "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle config: %w", err)
	}
	var cfg BundleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse bundle config: %w", err)
	}

	// Merge bundle config + request overrides
	finalCmd := cfg.Command
	finalArgs := cfg.Args
	finalRes := cfg.Resources
	if req.Command != "" {
		finalCmd = req.Command
		finalArgs = req.Args // Both command + args must be overridden together
	} else if len(req.Args) > 0 {
		finalArgs = req.Args
	}
	if req.Resources != nil {
		if req.Resources.CPU > 0 {
			finalRes.CPU = req.Resources.CPU
		}
		if req.Resources.MemoryMB > 0 {
			finalRes.MemoryMB = req.Resources.MemoryMB
		}
		if req.Resources.Pids > 0 {
			finalRes.Pids = req.Resources.Pids
		}
		if req.Resources.TimeoutSec > 0 {
			finalRes.TimeoutSec = req.Resources.TimeoutSec
		}
	}
	if finalCmd == "" {
		return nil, errors.New("command cannot be empty after merge")
	}
	if finalRes.MemoryMB <= 0 {
		return nil, fmt.Errorf("invalid memory limit: must be > 0")
	}
	if finalRes.Pids <= 0 {
		return nil, fmt.Errorf("invalid pids limit: must be > 0")
	}
	if finalRes.CPU <= 0 || finalRes.CPU > 100 {
		return nil, fmt.Errorf("invalid cpu limit: must be between 1–100")
	}
	if finalRes.TimeoutSec <= 0 {
		finalRes.TimeoutSec = defaultTimeoutSec // Default to 2 minutes if not set or invalid
	}

	id, err := m.generateSandboxID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sandboxDir := filepath.Join(m.cfg.RootDir, "sandboxes", id)
	logPath := filepath.Join(sandboxDir, "log.txt")
	if err := os.MkdirAll(sandboxDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	sb := &sandbox.Sandbox{
		ID:         id,
		State:      sandbox.CREATED,
		Command:    finalCmd,
		Args:       finalArgs,
		Resources:  finalRes,
		RootFSPath: rootFSPath,
		LogPath:    logPath,
		BundlePath: absBundlePath,
		CreatedAt:  now,
		ExitCode:   -1,
	}
	if err := m.store.Create(sb); err != nil {
		return nil, err
	}
	return sb, nil
}

// ListSandboxes returns all sandboxes currently tracked by the runtime.
func (m *Manager) ListSandboxes() ([]*sandbox.Sandbox, error) {
	return m.store.List(), nil
}

// GetSandbox retrieves a sandbox by ID.
func (m *Manager) GetSandbox(id string) (*sandbox.Sandbox, error) {
	if id == "" {
		return nil, state.ErrInvalidSandbox
	}
	return m.store.Get(id)
}

// GetSandboxLogs retrieves the logs from the given sandbox ID.
func (m *Manager) GetSandboxLogs(id string) (string, error) {
	if id == "" {
		return "", state.ErrInvalidSandbox
	}
	if _, err := m.store.Get(id); err != nil {
		return "", err
	}

	logPath := filepath.Join(
		m.cfg.RootDir,
		"sandboxes",
		id,
		"log.txt",
	)
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no logs available for sandbox %q", id)
		}
		return "", fmt.Errorf("read logs for sandbox %q: %w", id, err)
	}
	return string(data), nil
}

// StopSandbox sends a termination signal to a running sandbox process.
func (m *Manager) StopSandbox(id string) (*sandbox.Sandbox, error) {
	if id == "" {
		return nil, state.ErrInvalidSandbox
	}

	sb, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	if sb.State != sandbox.RUNNING {
		return nil, ErrInvalidStateTransition
	}

	sb.ExitReason = sandbox.ExitReasonStopped
	if err := m.store.Update(sb); err != nil {
		return nil, err
	}

	// Find process by PID and send a kill signal
	proc, err := os.FindProcess(sb.PID)
	if err != nil {
		return nil, fmt.Errorf("failed to find process for sandbox %q: %w", id, err)
	}
	err = proc.Kill()
	if err != nil {
		// If the process is already finished, treat as sucess because the desired
		// outcome is already true.
		if errors.Is(err, os.ErrProcessDone) {
			return sb, nil
		}
		return nil, fmt.Errorf("failed to kill process for sandbox %q: %w", id, err)
	}

	// superviseExecution will handle EXITED transition
	return sb, nil
}

// StartSandbox starts a sandbox process and transitions the sandbox from CREATED -> RUNNING
func (m *Manager) StartSandbox(id string) (*sandbox.Sandbox, error) {
	if id == "" {
		return nil, errors.New("sandbox id is required")
	}

	sb, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}

	if sb.State != sandbox.CREATED {
		return nil, fmt.Errorf("sandbox %q is not in CREATED state", id)
	}
	if sb.Command == "" {
		return m.failSandbox(sb, "sandbox command is empty", nil, "sandbox %q command is empty", nil)
	}

	// Ensure rootfs is valid and ready, then setup the sandbox log file for stdout/stderr
	if sb.RootFSPath == "" {
		return m.failSandbox(sb, "rootfs path is empty", nil, "rootfs path for sandbox %q is empty", nil)
	}
	info, err := os.Stat(sb.RootFSPath)
	if err != nil {
		sandboxErr := ""
		if os.IsNotExist(err) {
			sandboxErr = fmt.Sprintf("rootfs path does not exist: %v", err)
		} else {
			sandboxErr = fmt.Sprintf("failed to validate rootfs path: %v", err)
		}
		return m.failSandbox(sb, sandboxErr, err, "failed to validate rootfs path for sandbox %q: %w", nil)
	}
	if !info.IsDir() {
		return m.failSandbox(sb, "rootfs path exists but is not a directory", nil, "rootfs path for sandbox %q is not a directory", nil)
	}

	logFile, err := os.OpenFile(sb.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return m.failSandbox(sb, fmt.Sprintf("failed to open log file: %v", err), err, "open log file for sandbox %q: %w", nil)
	}

	// Build and configure the cgroup for the sandbox
	if err := m.cg.Create(id); err != nil {
		return m.failSandbox(sb, fmt.Sprintf("failed to create cgroup: %v", err), err, "create cgroup for sandbox %q: %w", nil)
	}
	if err := m.cg.ApplyLimits(id, sb.Resources); err != nil {
		return m.failSandbox(sb, fmt.Sprintf("failed to apply cgroup limits: %v", err), err, "apply cgroup limits for sandbox %q: %w", func() { _ = m.cg.Delete(id) })
	}

	// Build and configure exec.Cmd from the sandbox spec and launch the bootstrap process
	// so the child can perform namespace + filesystem setup before execing the workload.
	cmd := exec.Command(
		m.cfg.InitBinaryPath,
		append([]string{
			"init",
			sb.ID,
			sb.RootFSPath,
			sb.Command,
		}, sb.Args...)...,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = sb.BundlePath
	if err := m.ns.Configure(cmd, sb); err != nil {
		return m.failSandbox(sb, fmt.Sprintf("failed to configure namespaces: %v", err), err, "configure namespaces for sandbox %q: %w", nil)
	}
	if err := cmd.Start(); err != nil {
		return m.failSandbox(
			sb,
			fmt.Sprintf("failed to start process: %v", err),
			err,
			"start sandbox %q: %w",
			func() { _ = logFile.Close() },
			func() { _ = m.cg.Delete(id) },
		)
	}

	// If attaching the process to the cgroup fails this results in a ungoverned process
	if err := m.cg.AddProcess(id, cmd.Process.Pid); err != nil {
		return m.failSandbox(
			sb,
			fmt.Sprintf("failed to attach process to cgroup: %v", err),
			err,
			"attach process to cgroup for sandbox %q: %w",
			func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			},
			func() { _ = logFile.Close() },
			func() { _ = m.cg.Delete(id) },
		)
	}

	sb.PID = cmd.Process.Pid
	sb.State = sandbox.RUNNING
	sb.StartedAt = time.Now()
	sb.Err = ""
	if err := m.store.Update(sb); err != nil {
		// Process exists but failed to persist RUNNING -> kill the process
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = logFile.Close()
		return nil, fmt.Errorf("persist RUNNING state for sandbox %q after successful process start: %w", id, err)
	}

	// Every child must be waited on, otherwise there's a risk of a zombie process
	go m.superviseExecution(sb.ID, cmd, logFile)
	return sb, nil
}

// superviseExecution is the execution control loop for running a sandbox.
//
// It is responsible for supervising the lifecycle of a sandbox process after
// it has been sucessfully started and transitioned to RUNNING.
func (m *Manager) superviseExecution(id string, cmd *exec.Cmd, logFile *os.File) {
	defer logFile.Close()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	sb, err := m.store.Get(id)
	if err != nil {
		// If we can't load state, we still must reap the process
		<-done
		return
	}

	timeout := time.Duration(sb.Resources.TimeoutSec) * time.Second
	timer := time.NewTimer(timeout)

	// Helper function to finalize the process exit (enforce only a single path succeeds)
	finalize := func(reason string, waitErr error) {
		// Re-read the sandbox from the store before updating terminal state.
		// This avoids mutating a stale pointer just in case other manager operations
		// modify the sandbox in the meantime.
		curr, getErr := m.store.Get(id)
		if getErr != nil {
			// Fail silently here because the process has already exited and been reaped.
			return
		}

		if curr.State != sandbox.RUNNING {
			return
		}
		curr.State = sandbox.EXITED
		curr.FinishedAt = time.Now()
		curr.ExitCode = exitCodeFromWaitErr(waitErr)

		if curr.ExitReason == sandbox.ExitReasonStopped {
			curr.Err = "stopped by user"
			_ = m.store.Update(curr)
			return
		}
		switch reason {
		case "timeout":
			curr.ExitReason = sandbox.ExitReasonTimeout
			curr.Err = "timeout exceeded"
		case "wait-error":
			curr.ExitReason = sandbox.ExitReasonError
			curr.Err = fmt.Sprintf("wait error: %v", waitErr)
		case "completed":
			curr.ExitReason = sandbox.ExitReasonCompleted
			curr.Err = ""
		default:
			curr.Err = ""
		}

		_ = m.store.Update(curr)
	}

	select {
	// Process exits normally
	case waitErr := <-done:
		timer.Stop()

		var exitErr *exec.ExitError
		if waitErr != nil && !errors.As(waitErr, &exitErr) {
			finalize("wait-error", waitErr)
		}
		finalize("completed", waitErr)

	// Process timeout fires
	case <-timer.C:
		// Send a sigterm and let the process finish off cleanly
		// If the process is still alive after a given grace period -> force kill
		_ = cmd.Process.Signal(syscall.SIGTERM)
		graceTimer := time.NewTimer(2 * time.Second)

		select {
		case waitErr := <-done:
			finalize("terminated", waitErr)

		// Grace period expired
		case <-graceTimer.C:
			_ = cmd.Process.Kill()
			waitErr := <-done
			finalize("timeout", waitErr)
		}
	}
}

// exitCodeFromWaitErr extracts a process exit code from cmd.Wait()
func exitCodeFromWaitErr(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// failSandbox marks a sandbox as FAILED, persists it, and returns a consistent error.
func (m *Manager) failSandbox(
	sb *sandbox.Sandbox,
	sandboxErr string,
	cause error,
	userFmt string,
	cleanup ...func(),
) (*sandbox.Sandbox, error) {
	for _, fn := range cleanup {
		if fn != nil {
			fn()
		}
	}

	sb.State = sandbox.FAILED
	sb.Err = sandboxErr

	if updateErr := m.store.Update(sb); updateErr != nil {
		if cause != nil {
			return nil, fmt.Errorf("%s: %w; additionally failed to persist FAILED state: %v", sandboxErr, cause, updateErr)
		}
		return nil, fmt.Errorf("%s; additionally failed to persist FAILED state: %v", sandboxErr, updateErr)
	}

	if cause != nil {
		return nil, fmt.Errorf(userFmt, sb.ID, cause)
	}

	return nil, fmt.Errorf(userFmt, sb.ID)
}

func (m *Manager) generateSandboxID() (string, error) {
	for range maxIDSamplingAttempts {
		id, err := randomUint32Hex()
		if err != nil {
			return "", err
		}

		if _, err := m.store.Get(id); errors.Is(err, state.ErrSandboxNotFound) {
			return id, nil
		}
	}
	return "", errors.New("failed to generate unique sandbox id")
}

func randomUint32Hex() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("sbx_%02x%02x%02x%02x", b[0], b[1], b[2], b[3]), nil
}
