// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Repo is the flat whitelist entry (legacy field name; same as a project without milestones).
type Repo struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
	Cwd  string `json:"cwd" yaml:"cwd"`
}

// MilestonePhase is one DAG node under a milestone.
type MilestonePhase struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	PhaseRef  string         `json:"phaseRef"`
	DependsOn []string       `json:"dependsOn"`
	Model     string         `json:"model,omitempty"`
	OnFailure string         `json:"onFailure,omitempty"`
	Prompt    map[string]any `json:"prompt,omitempty"`
}

// Milestone groups phases for App navigation.
type Milestone struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	ProgressDoc string           `json:"progressDoc,omitempty"`
	Phases      []MilestonePhase `json:"phases"`
}

// Project is a whitelist cwd plus optional milestone index payload.
type Project struct {
	ID         string      `json:"id" yaml:"id"`
	Name       string      `json:"name" yaml:"name"`
	Cwd        string      `json:"cwd" yaml:"cwd"`
	Index      *string     `json:"index" yaml:"index"`
	Milestones []Milestone `json:"milestones,omitempty"`
}

type Slave struct {
	ID       string    `json:"id" yaml:"id"`
	Name     string    `json:"name" yaml:"name"`
	Online   bool      `json:"online" yaml:"online"`
	Repos    []Repo    `json:"repos" yaml:"repos"`
	Projects []Project `json:"projects,omitempty" yaml:"projects,omitempty"`
}

type Config struct {
	Slaves []Slave `json:"slaves" yaml:"slaves"`
}

type Registry struct {
	mu     sync.RWMutex
	slaves map[string]*Slave

	grace time.Duration
	// pending offline timers by slave id
	offlineTimers map[string]*time.Timer
}

func NewRegistry(initial []Slave) *Registry {
	r := &Registry{
		slaves:        make(map[string]*Slave),
		grace:         3 * time.Second,
		offlineTimers: make(map[string]*time.Timer),
	}
	for _, s := range initial {
		cp := s
		cp.Online = false // presence only via outbound register (M03)
		if cp.Repos == nil {
			cp.Repos = []Repo{}
		}
		if cp.Projects == nil {
			cp.Projects = projectsFromRepos(cp.Repos)
		}
		r.slaves[cp.ID] = &cp
	}
	return r
}

func projectsFromRepos(repos []Repo) []Project {
	out := make([]Project, 0, len(repos))
	for _, r := range repos {
		out = append(out, Project{
			ID:         r.ID,
			Name:       r.Name,
			Cwd:        r.Cwd,
			Index:      nil,
			Milestones: []Milestone{},
		})
	}
	return out
}

func reposFromProjects(projects []Project) []Repo {
	out := make([]Repo, 0, len(projects))
	for _, p := range projects {
		out = append(out, Repo{ID: p.ID, Name: p.Name, Cwd: p.Cwd})
	}
	return out
}

func (r *Registry) SetGrace(d time.Duration) {
	r.mu.Lock()
	r.grace = d
	r.mu.Unlock()
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
	list := r.List()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"slaves": list})
}

func (r *Registry) List() []Slave {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Slave, 0, len(r.slaves))
	for _, s := range r.slaves {
		cp := *s
		if cp.Repos == nil {
			cp.Repos = []Repo{}
		}
		if cp.Projects == nil {
			cp.Projects = []Project{}
		}
		// deep-ish copy slices
		cp.Repos = append([]Repo(nil), cp.Repos...)
		cp.Projects = append([]Project(nil), cp.Projects...)
		out = append(out, cp)
	}
	return out
}

// UpsertOnline records a registered slave and marks it online.
// Prefer projects when non-empty; otherwise derive projects from repos.
func (r *Registry) UpsertOnline(id, name string, repos []Repo, projects []Project) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.offlineTimers[id]; ok {
		t.Stop()
		delete(r.offlineTimers, id)
	}
	if projects == nil {
		projects = []Project{}
	}
	if repos == nil {
		repos = []Repo{}
	}
	if len(projects) > 0 {
		repos = reposFromProjects(projects)
	} else if len(repos) > 0 {
		projects = projectsFromRepos(repos)
	}
	if existing, ok := r.slaves[id]; ok {
		existing.Online = true
		if name != "" {
			existing.Name = name
		}
		existing.Repos = append([]Repo(nil), repos...)
		existing.Projects = append([]Project(nil), projects...)
		return
	}
	if name == "" {
		name = id
	}
	r.slaves[id] = &Slave{
		ID:       id,
		Name:     name,
		Online:   true,
		Repos:    append([]Repo(nil), repos...),
		Projects: append([]Project(nil), projects...),
	}
}

// Touch cancels a pending offline mark (e.g. on heartbeat).
func (r *Registry) Touch(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.offlineTimers[id]; ok {
		t.Stop()
		delete(r.offlineTimers, id)
	}
	if s, ok := r.slaves[id]; ok {
		s.Online = true
	}
}

// ScheduleOffline marks the slave offline after grace unless cancelled.
func (r *Registry) ScheduleOffline(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.offlineTimers[id]; ok {
		t.Stop()
	}
	grace := r.grace
	r.offlineTimers[id] = time.AfterFunc(grace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.offlineTimers, id)
		if s, ok := r.slaves[id]; ok {
			s.Online = false
		}
	})
}

func (r *Registry) IsOnline(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.slaves[id]
	return ok && s.Online
}
