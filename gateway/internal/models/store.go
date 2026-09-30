// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package models

import (
	"strings"
	"sync"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
)

// Store backs App model pickers + manage UI.
type Store struct {
	mu    sync.Mutex
	persist persist.ModelCatalogStore
	seeded bool
}

func NewStore(p persist.ModelCatalogStore) *Store {
	if p == nil {
		p = persist.NewMemoryModelCatalog()
	}
	return &Store{persist: p}
}

func (s *Store) ensureSeeded() error {
	if s.seeded {
		return nil
	}
	sel, err := s.persist.ListSelected()
	if err != nil {
		return err
	}
	if len(sel) == 0 {
		c := FromEnv()
		entries := make([]persist.ModelEntry, 0, len(c.Models))
		for _, m := range c.Models {
			entries = append(entries, persist.ModelEntry{ID: m.ID, Label: m.Label})
		}
		if err := s.persist.ReplaceSelected(entries, c.Default); err != nil {
			return err
		}
	} else {
		// Always keep Auto in the picker.
		hasAuto := false
		for _, e := range sel {
			if e.ID == "auto" {
				hasAuto = true
				break
			}
		}
		if !hasAuto {
			_ = s.persist.AddSelected(persist.ModelEntry{ID: "auto", Label: "Auto"})
		}
	}
	s.seeded = true
	return nil
}

// Catalog returns picker catalog (GET /v1/models).
func (s *Store) Catalog() (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSeeded(); err != nil {
		return Catalog{}, err
	}
	sel, err := s.persist.ListSelected()
	if err != nil {
		return Catalog{}, err
	}
	def, err := s.persist.GetDefault()
	if err != nil {
		return Catalog{}, err
	}
	models := make([]Entry, 0, len(sel))
	hasDef := false
	for _, e := range sel {
		models = append(models, Entry{ID: e.ID, Label: e.Label})
		if e.ID == def {
			hasDef = true
		}
	}
	if !hasDef {
		def = "auto"
	}
	return Catalog{Default: def, Models: models}, nil
}

// ManageView is GET /v1/models/manage.
type ManageView struct {
	Default   string  `json:"default"`
	Selected  []Entry `json:"selected"`
	Available []Entry `json:"available"`
}

func (s *Store) Manage() (ManageView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSeeded(); err != nil {
		return ManageView{}, err
	}
	sel, err := s.persist.ListSelected()
	if err != nil {
		return ManageView{}, err
	}
	avail, err := s.persist.ListAvailable()
	if err != nil {
		return ManageView{}, err
	}
	def, err := s.persist.GetDefault()
	if err != nil {
		return ManageView{}, err
	}
	return ManageView{
		Default:   def,
		Selected:  toEntries(sel),
		Available: toEntries(avail),
	}, nil
}

func toEntries(in []persist.ModelEntry) []Entry {
	out := make([]Entry, 0, len(in))
	for _, e := range in {
		out = append(out, Entry{ID: e.ID, Label: e.Label})
	}
	return out
}

func normalizeEntry(id, label string) (persist.ModelEntry, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return persist.ModelEntry{}, false
	}
	label = strings.TrimSpace(label)
	if label == "" {
		if id == "auto" {
			label = "Auto"
		} else if id == "default" {
			label = "Default"
		} else {
			label = id
		}
	}
	return persist.ModelEntry{ID: id, Label: label}, true
}

// AddSelected adds id to the App picker list.
func (s *Store) AddSelected(id, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSeeded(); err != nil {
		return err
	}
	e, ok := normalizeEntry(id, label)
	if !ok {
		return errBadID
	}
	return s.persist.AddSelected(e)
}

// RemoveSelected removes id from picker (auto cannot be removed).
func (s *Store) RemoveSelected(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSeeded(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errBadID
	}
	if id == "auto" {
		return errKeepAuto
	}
	return s.persist.RemoveSelected(id)
}

// SetDefault updates catalog default id (must be in selected).
func (s *Store) SetDefault(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureSeeded(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errBadID
	}
	sel, err := s.persist.ListSelected()
	if err != nil {
		return err
	}
	found := false
	for _, e := range sel {
		if e.ID == id {
			found = true
			break
		}
	}
	if !found {
		return errNotSelected
	}
	return s.persist.SetDefault(id)
}

// SetAvailable replaces Slave-reported Cursor catalog.
func (s *Store) SetAvailable(entries []Entry, slaveID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]persist.ModelEntry, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		pe, ok := normalizeEntry(e.ID, e.Label)
		if !ok || seen[pe.ID] {
			continue
		}
		seen[pe.ID] = true
		out = append(out, pe)
	}
	return s.persist.ReplaceAvailable(out, slaveID)
}

type storeError string

func (e storeError) Error() string { return string(e) }

const (
	errBadID       storeError = "id required"
	errKeepAuto    storeError = "cannot remove auto"
	errNotSelected storeError = "model not in selected list"
)
