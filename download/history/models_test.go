package history

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRunHistory_ToJSON_FromJSON(t *testing.T) {
	now := time.Now()
	run := &RunHistory{
		RunID:      "test-run-123",
		StartedAt:  now,
		CompletedAt: &now,
		State:      "completed",
		Phase:      "completed",
		Statistics: map[string]interface{}{
			"completed": 10,
			"failed":    2,
		},
		Snapshots: []RunSnapshot{
			{
				Timestamp:  now,
				Progress:   50.0,
				Statistics: map[string]interface{}{"completed": 5},
				State:      "running",
				Phase:      "executing",
			},
		},
	}

	// Convert to JSON
	data, err := run.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	// Parse JSON to verify it's valid
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON is invalid: %v", err)
	}

	// Convert back from JSON
	var restored RunHistory
	if err := restored.FromJSON(data); err != nil {
		t.Fatalf("FromJSON() failed: %v", err)
	}

	if restored.RunID != run.RunID {
		t.Errorf("Expected RunID %s, got %s", run.RunID, restored.RunID)
	}
	if restored.State != run.State {
		t.Errorf("Expected State %s, got %s", run.State, restored.State)
	}
	if len(restored.Snapshots) != len(run.Snapshots) {
		t.Errorf("Expected %d snapshots, got %d", len(run.Snapshots), len(restored.Snapshots))
	}
}

func TestActivityHistory_ToJSON_FromJSON(t *testing.T) {
	history := &ActivityHistory{
		Entries: []ActivityEntry{
			{
				ID:        "1",
				Timestamp: time.Now(),
				Type:      "test_event",
				Message:   "Test message",
				Details: map[string]interface{}{
					"key": "value",
				},
			},
		},
	}

	// Convert to JSON
	data, err := history.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	// Parse JSON to verify it's valid
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON is invalid: %v", err)
	}

	// Convert back from JSON
	var restored ActivityHistory
	if err := restored.FromJSON(data); err != nil {
		t.Fatalf("FromJSON() failed: %v", err)
	}

	if len(restored.Entries) != len(history.Entries) {
		t.Errorf("Expected %d entries, got %d", len(history.Entries), len(restored.Entries))
	}
	if restored.Entries[0].Type != history.Entries[0].Type {
		t.Errorf("Expected type %s, got %s", history.Entries[0].Type, restored.Entries[0].Type)
	}
}
