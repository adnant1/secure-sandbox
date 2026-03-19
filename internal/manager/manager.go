package manager

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sandbox-runtime/internal/config"
	"sandbox-runtime/internal/sandbox"
	"sandbox-runtime/internal/state"
	"time"
)

const maxIDSamplingAttempts = 10

var ErrInvalidStateTransition = errors.New("invalid sandbox state transition")

type CreateSandboxRequest struct {
	BundlePath string
	Command    string
	Args       []string
}

// Manager coordinates sandbox lifecycle operations and enforces runtime invariants.
type Manager struct {
	store *state.StateStore
	cfg   config.Config
}

// New initializes and returns a new Manager with the given StateStore
func New(store *state.StateStore, cfg config.Config) *Manager {
	if store == nil {
		panic("manager: nil state store")
	}
	if cfg.RootDir == "" {
		panic("manager: root dir cannot be empty")
	}
	if err := os.MkdirAll(cfg.RootDir, 0o755); err != nil {
		panic(fmt.Sprintf("manager: failed to create root dir %q: %v", cfg.RootDir, err))
	}

	return &Manager{
		store: store,
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

	id, err := m.generateSandboxID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sandboxDir := filepath.Join(m.cfg.RootDir, "sandboxes", id)
	rootFSPath := filepath.Join(sandboxDir, "rootfs")
	logPath := filepath.Join(sandboxDir, "log.txt")
	if err := os.MkdirAll(rootFSPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directories: %w", err)
	}
	sb := &sandbox.Sandbox{
		ID:         id,
		State:      sandbox.CREATED,
		Command:    req.Command,
		Args:       req.Args,
		RootFSPath: rootFSPath,
		LogPath:    logPath,
		BundlePath: req.BundlePath,
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

	// Wait() goroutine will handle EXITED transition
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
		sb.State = sandbox.FAILED
		sb.Err = "sandbox command is empty"
		if updateErr := m.store.Update(sb); updateErr != nil {
			return nil, fmt.Errorf("sandbox command is empty; additionally failed to persist FAILED state: %w", updateErr)
		}
		return nil, fmt.Errorf("sandbox %q command is empty", id)
	}

	// Prepare host-side resources before process creation
	logFile, err := os.OpenFile(sb.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		sb.State = sandbox.FAILED
		sb.Err = fmt.Sprintf("failed to open log file: %v", err)
		if updateErr := m.store.Update(sb); updateErr != nil {
			return nil, fmt.Errorf("failed to open log file: %w; additionally failed to persist FAILED state: %v", err, updateErr)
		}
		return nil, fmt.Errorf("open log file for sandbox %q: %w", id, err)
	}

	// Build exec.Cmd from the sandbox spec and start the process
	cmd := exec.Command(sb.Command, sb.Args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = sb.BundlePath
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		sb.State = sandbox.FAILED
		sb.Err = fmt.Sprintf("failed to start process: %v", err)
		if updateErr := m.store.Update(sb); updateErr != nil {
			return nil, fmt.Errorf("failed to start process: %w; additionally failed to perist FAILED state: %v", err, updateErr)
		}
		return nil, fmt.Errorf("start sandbox %q: %w", id, err)
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
	go func(s *sandbox.Sandbox, c *exec.Cmd, f *os.File) {
		defer f.Close()
		waitErr := c.Wait()

		// Re-read the sandbox from the store before updating terminal state.
		// This avoids mutating a stale pointer just in case other manager operations
		// modify the sandbox in the meantime.
		curr, getErr := m.store.Get(s.ID)
		if getErr != nil {
			// Fail silently here because the process has already exited and been reaped.
			return
		}

		curr.State = sandbox.EXITED
		curr.FinishedAt = time.Now()
		curr.ExitCode = exitCodeFromWaitErr(waitErr)
		curr.Err = ""

		// Rare event: Wait failed with a non ExitError
		if waitErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(waitErr, &exitErr) {
				curr.Err = fmt.Sprintf("wait error: %v", waitErr)
			}
		}

		// Best effort update the sandbox since the process is already gone
		_ = m.store.Update(curr)
	}(sb, cmd, logFile)

	return sb, nil
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
