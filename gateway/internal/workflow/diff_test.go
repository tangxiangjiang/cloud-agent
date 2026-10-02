// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow_test

import (
	"strings"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
)

func TestDiffIsSubstantive(t *testing.T) {
	ok, reason := workflow.DiffIsSubstantive(nil)
	if ok || !strings.Contains(reason, "empty") {
		t.Fatalf("nil: ok=%v reason=%s", ok, reason)
	}

	ok, reason = workflow.DiffIsSubstantive([]workflow.NodeDiffFile{
		{Path: "ai/progress.md"},
	})
	if ok || !strings.Contains(reason, "progress.md") {
		t.Fatalf("progress-only: ok=%v reason=%s", ok, reason)
	}

	ok, reason = workflow.DiffIsSubstantive([]workflow.NodeDiffFile{
		{Path: "ai/progress.md"},
		{Path: "src/a.go"},
	})
	if !ok {
		t.Fatalf("code+progress: want ok, reason=%s", reason)
	}

	ok, reason = workflow.DiffIsSubstantive([]workflow.NodeDiffFile{
		{Path: "docs/design.md"},
	})
	if !ok {
		t.Fatalf("docs-only: want ok (gateway gate is empty/progress-only), reason=%s", reason)
	}
}
