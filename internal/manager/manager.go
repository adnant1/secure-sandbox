package manager

import (
	"crypto/rand"
	"errors"
	"fmt"
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

	id, err := m.generateSandboxID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rootFSPath := filepath.Join(m.cfg.RootDir, "sandboxes", id, "rootfs")
	logPath := filepath.Join(m.cfg.RootDir, "sandboxes", id, "logs")
	bundlePath := req.BundlePath
	sb := &sandbox.Sandbox{
		ID:         id,
		State:      sandbox.CREATED,
		RootFSPath: rootFSPath,
		LogPath:    logPath,
		BundlePath: bundlePath,
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

// StopSandbox transitions a sandbox to the EXITED state.
func (m *Manager) StopSandbox(id string) (*sandbox.Sandbox, error) {
	if id == "" {
		return nil, state.ErrInvalidSandbox
	}

	sb, err := m.store.Get(id)
	if err != nil {
		return nil, err
	}
	if sb.State == sandbox.CLEANED || sb.State == sandbox.EXITED {
		return nil, ErrInvalidStateTransition
	}

	sb.State = sandbox.EXITED
	sb.FinishedAt = time.Now()
	sb.ExitCode = 0
	if err := m.store.Update(sb); err != nil {
		return nil, err
	}
	return sb, nil
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
