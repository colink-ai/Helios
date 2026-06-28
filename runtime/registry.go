package runtime

import (
	"fmt"
	"sort"
	"sync"
)

// AdapterFactory creates an adapter instance for a runtime-ready agent spec.
type AdapterFactory func(spec AgentSpec) (Adapter, error)

// AdapterMeta describes a registered adapter.
type AdapterMeta struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	DefaultPath string         `json:"defaultPath,omitempty"`
	Factory     AdapterFactory `json:"-"`
}

// Registry stores adapter factories by type.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]AdapterMeta
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{adapters: map[string]AdapterMeta{}}
}

// Register registers an adapter type.
func (r *Registry) Register(meta AdapterMeta) error {
	if meta.Type == "" {
		return fmt.Errorf("adapter type is required")
	}
	if meta.Factory == nil {
		return fmt.Errorf("adapter %s factory is required", meta.Type)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[meta.Type]; exists {
		return fmt.Errorf("adapter %s already registered", meta.Type)
	}
	r.adapters[meta.Type] = meta
	return nil
}

// Create creates an adapter for the provided agent spec.
func (r *Registry) Create(spec AgentSpec) (Adapter, error) {
	r.mu.RLock()
	meta, ok := r.adapters[spec.Type]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter %s is not registered", spec.Type)
	}
	return meta.Factory(spec)
}

// Types lists registered adapter metadata in stable order.
func (r *Registry) Types() []AdapterMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AdapterMeta, 0, len(r.adapters))
	for _, meta := range r.adapters {
		meta.Factory = nil
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
