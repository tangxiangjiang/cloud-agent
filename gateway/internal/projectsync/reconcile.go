// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package projectsync

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
)

var (
	phaseIDExact = regexp.MustCompile(`^M\d{2}-P\d{2}$`)
	phaseIDAny   = regexp.MustCompile(`M\d{2}-P\d{2}`)
)

// Warning is a drift hint for App UI (never auto-patches nodes).
type Warning struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	PhaseID    string `json:"phaseId,omitempty"`
	WorkflowID string `json:"workflowId,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// PhaseRow is progress vs Gateway node side-by-side.
type PhaseRow struct {
	ID              string  `json:"id"`
	Title           string  `json:"title,omitempty"`
	ProgressStatus  string  `json:"progressStatus"`
	NodeStatus      *string `json:"nodeStatus"`
	WorkflowID      string  `json:"workflowId,omitempty"`
}

// Report is the UI-oriented reconcile view.
type Report struct {
	WorkflowID     string     `json:"workflowId,omitempty"`
	BundleID       string     `json:"bundleId,omitempty"`
	WorkflowStatus string     `json:"workflowStatus,omitempty"`
	Active         bool       `json:"active"`
	Branch         *string    `json:"branch"`
	Dirty          *bool      `json:"dirty"`
	Phases         []PhaseRow `json:"phases"`
}

type syncPayloadView struct {
	Branch *string `json:"branch"`
	Dirty  *bool   `json:"dirty"`
	Phases []struct {
		ID             string `json:"id"`
		ProgressStatus string `json:"progressStatus"`
		Title          string `json:"title"`
	} `json:"phases"`
}

type nodeHit struct {
	status     string
	title      string
	workflowID string
	bundleID   string
	wfStatus   string
}

// PhaseKeyFromNode extracts Mxx-Pxx from node id or phaseRef basename.
func PhaseKeyFromNode(n workflow.Node) string {
	id := strings.TrimSpace(n.ID)
	if phaseIDExact.MatchString(id) {
		return id
	}
	if n.UnitID != nil {
		u := strings.TrimSpace(*n.UnitID)
		if phaseIDExact.MatchString(u) {
			return u
		}
	}
	if n.PhaseRef != nil {
		base := path.Base(strings.TrimSpace(*n.PhaseRef))
		if m := phaseIDAny.FindString(base); m != "" {
			return m
		}
	}
	if m := phaseIDAny.FindString(id); m != "" {
		return m
	}
	return ""
}

func progressDone(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "approved" || s == "done"
}

func progressPending(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "pending" || s == "" || s == "unknown"
}

func nodeBehindProgress(status string) bool {
	switch strings.TrimSpace(status) {
	case workflow.NodeReady, workflow.NodePending, workflow.NodeRunning,
		workflow.NodeAwaitingReview, workflow.NodeFailed, workflow.NodeRejected,
		workflow.NodeCancelled:
		return true
	default:
		return false
	}
}

// nodeStatusRank: higher = further along (prefer for reconcile index).
func nodeStatusRank(status string) int {
	switch strings.TrimSpace(status) {
	case workflow.NodeApproved, workflow.NodeSkipped:
		return 60
	case workflow.NodeAwaitingReview:
		return 50
	case workflow.NodeRunning:
		return 40
	case workflow.NodeReady:
		return 30
	case workflow.NodePending:
		return 20
	case workflow.NodeFailed, workflow.NodeRejected, workflow.NodeCancelled:
		return 10
	default:
		return 0
	}
}

// buildNodeIndex maps phaseId → best node across milestone runs.
// Prefer higher node status (approved beats ready) so a completed prior run
// is not masked by a newer incomplete active run (false progress_ahead).
func buildNodeIndex(runs []*workflow.Run) map[string]nodeHit {
	out := make(map[string]nodeHit)
	// runs are newest-first from ListMilestoneRuns; on equal rank keep first (newest).
	for _, run := range runs {
		if run == nil {
			continue
		}
		for _, n := range run.Nodes {
			key := PhaseKeyFromNode(n)
			if key == "" {
				continue
			}
			title := ""
			if n.Title != nil {
				title = strings.TrimSpace(*n.Title)
			}
			hit := nodeHit{
				status:     n.Status,
				title:      title,
				workflowID: run.ID,
				bundleID:   run.BundleID,
				wfStatus:   run.Status,
			}
			prev, ok := out[key]
			if !ok || nodeStatusRank(hit.status) > nodeStatusRank(prev.status) {
				out[key] = hit
			}
		}
	}
	return out
}

// Reconcile compares Slave project_sync payload with milestone workflows.
// Does not mutate workflow node status.
func Reconcile(summaryJSON json.RawMessage, runs []*workflow.Run, primary *workflow.Run, primaryActive bool) (warnings []Warning, report Report) {
	var view syncPayloadView
	_ = json.Unmarshal(summaryJSON, &view)

	report.Branch = view.Branch
	report.Dirty = view.Dirty
	report.Active = primaryActive
	if primary != nil {
		report.WorkflowID = primary.ID
		report.BundleID = primary.BundleID
		report.WorkflowStatus = primary.Status
	}

	nodes := buildNodeIndex(runs)
	phaseIDs := map[string]struct{}{}
	progressBy := map[string]struct {
		status string
		title  string
	}{}
	for _, p := range view.Phases {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		phaseIDs[id] = struct{}{}
		progressBy[id] = struct {
			status string
			title  string
		}{status: strings.TrimSpace(p.ProgressStatus), title: strings.TrimSpace(p.Title)}
	}
	for id := range nodes {
		phaseIDs[id] = struct{}{}
	}

	ids := make([]string, 0, len(phaseIDs))
	for id := range phaseIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	unfinishedProgress := 0
	for _, id := range ids {
		pr := progressBy[id]
		var nodeStatus *string
		wfID := ""
		title := pr.title
		if hit, ok := nodes[id]; ok {
			st := hit.status
			nodeStatus = &st
			wfID = hit.workflowID
			if title == "" {
				title = hit.title
			}
		}
		progStatus := pr.status
		if progStatus == "" {
			progStatus = "unknown"
		}
		report.Phases = append(report.Phases, PhaseRow{
			ID:             id,
			Title:          title,
			ProgressStatus: progStatus,
			NodeStatus:     nodeStatus,
			WorkflowID:     wfID,
		})

		if progressPending(progStatus) && !progressDone(progStatus) {
			unfinishedProgress++
		}

		if progressDone(pr.status) && nodeStatus != nil && nodeBehindProgress(*nodeStatus) {
			warnings = append(warnings, Warning{
				Code:       "progress_ahead",
				Message:    "本机 progress 已标记 " + id + " 完成，但 Gateway 节点仍为 " + *nodeStatus,
				PhaseID:    id,
				WorkflowID: wfID,
				Suggestion: "align_workflow",
			})
		}
		if nodeStatus != nil && *nodeStatus == workflow.NodeApproved && progressPending(pr.status) {
			warnings = append(warnings, Warning{
				Code:       "gateway_ahead",
				Message:    "Gateway 节点 " + id + " 已 approved，但本机 progress 仍为 pending",
				PhaseID:    id,
				WorkflowID: wfID,
				Suggestion: "align_progress",
			})
		}
	}

	if view.Dirty != nil && *view.Dirty {
		warnings = append(warnings, Warning{
			Code:       "dirty_worktree",
			Message:    "工作区有未提交变更，Diff 基线可能不稳",
			Suggestion: "commit_or_stash",
		})
	}

	if !primaryActive {
		if unfinishedProgress > 0 {
			msg := "无 active Workflow，但 progress 仍有未完成 phase；可继续或新建 Milestone"
			w := Warning{
				Code:       "no_active_workflow",
				Message:    msg,
				Suggestion: "resume_or_create",
			}
			if primary != nil {
				w.WorkflowID = primary.ID
			}
			warnings = append(warnings, w)
		} else if primary == nil && len(view.Phases) > 0 {
			warnings = append(warnings, Warning{
				Code:       "no_workflow",
				Message:    "该工程尚无 milestone Workflow；可从 Milestone 列表新建",
				Suggestion: "create_milestone",
			})
		}
	}

	return warnings, report
}

// WarningMessages flattens structured warnings to strings (compat / logs).
func WarningMessages(ws []Warning) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Message)
	}
	return out
}

// ApprovedPhaseKeys lists Mxx-Pxx ids whose Gateway node is approved (for progress catch-up).
func ApprovedPhaseKeys(runs []*workflow.Run) []string {
	seen := map[string]struct{}{}
	for _, run := range runs {
		if run == nil {
			continue
		}
		for _, n := range run.Nodes {
			if n.Status != workflow.NodeApproved {
				continue
			}
			key := PhaseKeyFromNode(n)
			if key == "" {
				continue
			}
			seen[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ProgressDocFromPayload reads progressDoc from a stored sync summary JSON.
func ProgressDocFromPayload(summaryJSON json.RawMessage) string {
	var v struct {
		ProgressDoc string `json:"progressDoc"`
	}
	_ = json.Unmarshal(summaryJSON, &v)
	return strings.TrimSpace(v.ProgressDoc)
}

// ProgressDonePhaseKeys lists phase ids marked approved/done in a sync payload.
func ProgressDonePhaseKeys(summaryJSON json.RawMessage) []string {
	var view syncPayloadView
	_ = json.Unmarshal(summaryJSON, &view)
	seen := map[string]struct{}{}
	for _, p := range view.Phases {
		id := strings.TrimSpace(p.ID)
		if id == "" || !progressDone(p.ProgressStatus) {
			continue
		}
		seen[id] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
