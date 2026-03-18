package state

import (
	"errors"
	"sandbox-runtime/internal/sandbox"
	"sync"
)

var (
	ErrInvalidSandbox       = errors.New("invalid sandbox")
	ErrSandboxNotFound      = errors.New("sandbox not found")
	ErrSandboxAlreadyExists = errors.New("sandbox already exists")
)

// StateStore maintains an in-memory, thread-safe registry of all sandboxes.
type StateStore struct {
	sandboxes map[string]*sandbox.Sandbox
	mu        sync.RWMutex
}

// New initializes and returns a new StateStore.
func New() *StateStore {
	return &StateStore{
		sandboxes: make(map[string]*sandbox.Sandbox),
	}
}

// Create registers a new sandbox in the store.
func (ss *StateStore) Create(sb *sandbox.Sandbox) error {
	if sb == nil || sb.ID == "" {
		return ErrInvalidSandbox
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _, exists := ss.sandboxes[sb.ID]; exists {
		return ErrSandboxAlreadyExists
	}
	ss.sandboxes[sb.ID] = sb
	return nil
}

// Get retrieves a sandbox by ID.
func (ss *StateStore) Get(id string) (*sandbox.Sandbox, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	sb, exists := ss.sandboxes[id]
	if !exists {
		return nil, ErrSandboxNotFound
	}
	return sb, nil
}

// List returns all sandboxes currently stored.
func (ss *StateStore) List() []*sandbox.Sandbox {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	res := make([]*sandbox.Sandbox, 0, len(ss.sandboxes))
	for _, sb := range ss.sandboxes {
		res = append(res, sb)
	}
	return res
}

// Update replaces an existing sandbox in the store
func (ss *StateStore) Update(sb *sandbox.Sandbox) error {
	if sb == nil || sb.ID == "" {
		return ErrInvalidSandbox 
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _, exists := ss.sandboxes[sb.ID]; !exists {
		return ErrSandboxNotFound
	}
	ss.sandboxes[sb.ID] = sb
	return nil
}
