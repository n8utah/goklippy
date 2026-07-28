package klipperconfig

import (
	"fmt"
	"path/filepath"
	"sync"

	kcfg "goklippy/backend/klipperconfig"
)

// Workspace holds the active in-memory Klipper config being edited.
type Workspace struct {
	mu     sync.RWMutex
	root   string
	path   string
	file   *kcfg.File
	merged *kcfg.File
	dirty  bool
}

// Service manages Klipper config load/save within a config root directory.
type Service struct {
	root string
	mu   sync.RWMutex
	ws   *Workspace
}

// NewService creates a config service rooted at configRoot.
func NewService(configRoot string) (*Service, error) {
	if configRoot == "" {
		return &Service{}, nil
	}
	abs, err := filepath.Abs(configRoot)
	if err != nil {
		return nil, err
	}
	return &Service{root: abs}, nil
}

// ListFiles returns .cfg filenames under the config root.
func (s *Service) ListFiles() ([]string, error) {
	if s.root == "" {
		return nil, fmt.Errorf("klipper config_root is not configured")
	}
	return kcfg.ListConfigFiles(s.root)
}

// Load opens a config file relative to config root (or absolute within root).
func (s *Service) Load(relPath string) (*Workspace, error) {
	if s.root == "" {
		return nil, fmt.Errorf("klipper config_root is not configured")
	}
	path := relPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.root, relPath)
	}
	path = filepath.Clean(path)
	if err := kcfg.ValidatePath(s.root, path); err != nil {
		return nil, err
	}

	f, err := kcfg.LoadFile(path)
	if err != nil {
		return nil, err
	}
	merged, err := kcfg.ResolveIncludes(f, func(p string) (*kcfg.File, error) {
		if err := kcfg.ValidatePath(s.root, p); err != nil {
			return nil, err
		}
		return kcfg.LoadFile(p)
	})
	if err != nil {
		return nil, err
	}

	ws := &Workspace{
		root:   s.root,
		path:   path,
		file:   f,
		merged: merged,
		dirty:  false,
	}

	s.mu.Lock()
	s.ws = ws
	s.mu.Unlock()
	return ws, nil
}

// Current returns the active workspace, if any.
func (s *Service) Current() (*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ws == nil {
		return nil, fmt.Errorf("no config loaded")
	}
	return s.ws, nil
}

// Save writes the active workspace file to disk.
func (s *Service) Save() error {
	ws, err := s.Current()
	if err != nil {
		return err
	}
	return ws.Save()
}

// Path returns the loaded file path.
func (w *Workspace) Path() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.path
}

// Dirty reports unsaved changes.
func (w *Workspace) Dirty() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.dirty
}

// File returns the editable primary file (not merged includes).
func (w *Workspace) File() *kcfg.File {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.file
}

// Merged returns the merged view including includes.
func (w *Workspace) Merged() *kcfg.File {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.merged
}

// UpdateSection replaces all options in a section on the primary file.
func (w *Workspace) UpdateSection(section string, options map[string]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	sec := kcfg.Section{Name: section, Options: map[string]string{}, Order: []string{}}
	for k, v := range options {
		sec.Order = append(sec.Order, k)
		sec.Options[k] = v
	}
	w.file.Sections[section] = sec
	if !contains(w.file.Order, section) {
		w.file.Order = append(w.file.Order, section)
	}
	w.dirty = true
	w.remergeLocked()
}

// DeleteSection removes a section from the primary file.
func (w *Workspace) DeleteSection(section string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.file.Sections, section)
	newOrder := w.file.Order[:0]
	for _, name := range w.file.Order {
		if name != section {
			newOrder = append(newOrder, name)
		}
	}
	w.file.Order = newOrder
	w.dirty = true
	w.remergeLocked()
}

// SetRaw replaces entire primary file content by parsing new text.
func (w *Workspace) SetRaw(content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := kcfg.Parse(content)
	if err != nil {
		return err
	}
	f.Path = w.path
	w.file = f
	w.dirty = true
	w.remergeLocked()
	return nil
}

// Save persists the primary file.
func (w *Workspace) Save() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := kcfg.SaveFile(w.file, w.path); err != nil {
		return err
	}
	w.dirty = false
	return nil
}

func (w *Workspace) remergeLocked() {
	merged, err := kcfg.ResolveIncludes(w.file, func(p string) (*kcfg.File, error) {
		if err := kcfg.ValidatePath(w.root, p); err != nil {
			return nil, err
		}
		return kcfg.LoadFile(p)
	})
	if err == nil {
		w.merged = merged
	}
}

func contains(items []string, target string) bool {
	for _, v := range items {
		if v == target {
			return true
		}
	}
	return false
}
