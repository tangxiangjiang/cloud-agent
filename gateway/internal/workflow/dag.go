// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import "fmt"

// ValidateDAG checks unique node ids, dependency references, and acyclicity.
// Returns nil if the graph is a valid DAG.
func ValidateDAG(nodes []Node) error {
	if len(nodes) == 0 {
		return fmt.Errorf("nodes must be non-empty")
	}
	ids := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		if n.ID == "" {
			return fmt.Errorf("node id required")
		}
		if _, ok := ids[n.ID]; ok {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		ids[n.ID] = struct{}{}
	}
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if dep == n.ID {
				return fmt.Errorf("node %s depends on itself", n.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("node %s depends on unknown id %s", n.ID, dep)
			}
		}
	}
	if err := detectCycle(nodes); err != nil {
		return err
	}
	return nil
}

func detectCycle(nodes []Node) error {
	// Kahn topological sort; leftover nodes imply a cycle.
	indeg := make(map[string]int, len(nodes))
	children := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		indeg[n.ID] = 0
	}
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			indeg[n.ID]++
			children[dep] = append(children[dep], n.ID)
		}
	}
	var q []string
	for id, d := range indeg {
		if d == 0 {
			q = append(q, id)
		}
	}
	seen := 0
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		seen++
		for _, c := range children[id] {
			indeg[c]--
			if indeg[c] == 0 {
				q = append(q, c)
			}
		}
	}
	if seen != len(nodes) {
		return fmt.Errorf("dependency cycle detected")
	}
	return nil
}

// InitialNodeStatus returns ready when all dependsOn are empty (roots), else pending.
func InitialNodeStatus(dependsOn []string) string {
	if len(dependsOn) == 0 {
		return NodeReady
	}
	return NodePending
}
