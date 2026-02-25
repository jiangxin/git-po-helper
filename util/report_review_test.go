package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReportReviewWithTotalEntries(t *testing.T) {
	// Create a review JSON with total_entries
	review := &ReviewJSONResult{
		TotalEntries: 100,
		Issues: []ReviewIssue{
			{MsgID: "commit", MsgStr: "承诺", Score: 0, Description: "term error", Suggestion: "提交"},
			{MsgID: "file", MsgStr: "文件", Score: 2, Description: "minor", Suggestion: "档案"},
		},
	}
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "review.json")
	if err := os.WriteFile(jsonFile, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify CalculateReviewScore works
	score, err := CalculateReviewScore(review)
	if err != nil {
		t.Fatalf("CalculateReviewScore failed: %v", err)
	}
	// 100 entries * 3 = 300 max. Issues: 0 deducts 3, 2 deducts 1. Total deduction = 4. Score = (300-4)*100/300 = 98
	if score < 95 || score > 100 {
		t.Errorf("expected score ~98, got %d", score)
	}
}
