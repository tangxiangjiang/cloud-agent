// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Repo struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
	Cwd  string `json:"cwd" yaml:"cwd"`
}

type Slave struct {
	ID     string `json:"id" yaml:"id"`
	Name   string `json:"name" yaml:"name"`
	Online bool   `json:"online" yaml:"online"`
	Repos  []Repo `json:"repos" yaml:"repos"`
}

type Config struct {
	Slaves []Slave `json:"slaves" yaml:"slaves"`
}

type Registry struct {
	mu     sync.RWMutex
	slaves []Slave
}

func NewRegistry(slaves []Slave) *Registry {
	if slaves == nil {
		slaves = []Slave{}
	}
	// defensive copy
	out := make([]Slave, len(slaves))
	copy(out, slaves)
	return &Registry{slaves: out}
}

func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Slaves == nil {
		cfg.Slaves = []Slave{}
	}
	return &cfg, nil
}

func (r *Registry) HandleList(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Slave, len(r.slaves))
	copy(list, r.slaves)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"slaves": list})
}
