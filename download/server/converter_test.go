package server

import (
	"testing"
	"time"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/proto"
)

func TestCheckVersionCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		clientVersion string
		serverVersion string
		wantErr       bool
		errMsg        string
	}{
		{
			name:          "matching versions",
			clientVersion: "v1.2.3",
			serverVersion: "v1.2.3",
			wantErr:       false,
		},
		{
			name:          "mismatched versions",
			clientVersion: "v1.2.3",
			serverVersion: "v1.2.4",
			wantErr:       true,
			errMsg:        "version mismatch",
		},
		{
			name:          "empty client version",
			clientVersion: "",
			serverVersion: "v1.2.3",
			wantErr:       true,
			errMsg:        "version cannot be empty",
		},
		{
			name:          "empty server version",
			clientVersion: "v1.2.3",
			serverVersion: "",
			wantErr:       true,
			errMsg:        "version cannot be empty",
		},
		{
			name:          "dev versions match",
			clientVersion: "dev",
			serverVersion: "dev",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkVersionCompatibility(tt.clientVersion, tt.serverVersion)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConvertPlanItemTypeToProto(t *testing.T) {
	tests := []struct {
		name     string
		input    plan.PlanItemType
		expected proto.PlanItemType
	}{
		{"track", plan.PlanItemTypeTrack, proto.PlanItemType_PLAN_ITEM_TYPE_TRACK},
		{"album", plan.PlanItemTypeAlbum, proto.PlanItemType_PLAN_ITEM_TYPE_ALBUM},
		{"artist", plan.PlanItemTypeArtist, proto.PlanItemType_PLAN_ITEM_TYPE_ARTIST},
		{"playlist", plan.PlanItemTypePlaylist, proto.PlanItemType_PLAN_ITEM_TYPE_PLAYLIST},
		{"m3u", plan.PlanItemTypeM3U, proto.PlanItemType_PLAN_ITEM_TYPE_M3U},
		{"unknown", plan.PlanItemType("unknown"), proto.PlanItemType_PLAN_ITEM_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPlanItemTypeToProto(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertPlanItemStatusToProto(t *testing.T) {
	tests := []struct {
		name     string
		input    plan.PlanItemStatus
		expected proto.PlanItemStatus
	}{
		{"pending", plan.PlanItemStatusPending, proto.PlanItemStatus_PLAN_ITEM_STATUS_PENDING},
		{"in_progress", plan.PlanItemStatusInProgress, proto.PlanItemStatus_PLAN_ITEM_STATUS_IN_PROGRESS},
		{"completed", plan.PlanItemStatusCompleted, proto.PlanItemStatus_PLAN_ITEM_STATUS_COMPLETED},
		{"failed", plan.PlanItemStatusFailed, proto.PlanItemStatus_PLAN_ITEM_STATUS_FAILED},
		{"skipped", plan.PlanItemStatusSkipped, proto.PlanItemStatus_PLAN_ITEM_STATUS_SKIPPED},
		{"unknown", plan.PlanItemStatus("unknown"), proto.PlanItemStatus_PLAN_ITEM_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPlanItemStatusToProto(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertServiceStateToProto(t *testing.T) {
	tests := []struct {
		name     string
		input    download.ServiceState
		expected proto.ServiceState
	}{
		{"idle", download.ServiceStateIdle, proto.ServiceState_SERVICE_STATE_IDLE},
		{"running", download.ServiceStateRunning, proto.ServiceState_SERVICE_STATE_RUNNING},
		{"stopping", download.ServiceStateStopping, proto.ServiceState_SERVICE_STATE_STOPPING},
		{"error", download.ServiceStateError, proto.ServiceState_SERVICE_STATE_ERROR},
		{"unknown", download.ServiceState("unknown"), proto.ServiceState_SERVICE_STATE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertServiceStateToProto(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertServicePhaseToProto(t *testing.T) {
	tests := []struct {
		name     string
		input    download.ServicePhase
		expected proto.ServicePhase
	}{
		{"idle", download.ServicePhaseIdle, proto.ServicePhase_SERVICE_PHASE_IDLE},
		{"generating", download.ServicePhaseGenerating, proto.ServicePhase_SERVICE_PHASE_GENERATING},
		{"optimizing", download.ServicePhaseOptimizing, proto.ServicePhase_SERVICE_PHASE_OPTIMIZING},
		{"executing", download.ServicePhaseExecuting, proto.ServicePhase_SERVICE_PHASE_EXECUTING},
		{"completed", download.ServicePhaseCompleted, proto.ServicePhase_SERVICE_PHASE_COMPLETED},
		{"error", download.ServicePhaseError, proto.ServicePhase_SERVICE_PHASE_ERROR},
		{"unknown", download.ServicePhase("unknown"), proto.ServicePhase_SERVICE_PHASE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertServicePhaseToProto(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertPlanItemToProto(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-5 * time.Minute)
	completedAt := now

	item := &plan.PlanItem{
		ItemID:     "test:1",
		ItemType:   plan.PlanItemTypeTrack,
		SpotifyID:  "spotify:track:123",
		SpotifyURL: "https://open.spotify.com/track/123",
		YouTubeURL: "https://youtube.com/watch?v=abc",
		ParentID:   "album:1",
		ChildIDs:   []string{"child:1"},
		Name:       "Test Track",
		Status:     plan.PlanItemStatusCompleted,
		Error:      "",
		FilePath:   "/path/to/file.mp3",
		CreatedAt:  now,
		StartedAt:  &startedAt,
		CompletedAt: &completedAt,
		Progress:   1.0,
	}
	item.Metadata = map[string]interface{}{
		"artist": "Test Artist",
		"album":  "Test Album",
	}

	protoItem := convertPlanItemToProto(item)

	if protoItem == nil {
		t.Fatal("convertPlanItemToProto returned nil")
	}

	if protoItem.ItemId != item.ItemID {
		t.Errorf("expected ItemId %s, got %s", item.ItemID, protoItem.ItemId)
	}

	if protoItem.ItemType != proto.PlanItemType_PLAN_ITEM_TYPE_TRACK {
		t.Errorf("expected ItemType TRACK, got %v", protoItem.ItemType)
	}

	if protoItem.Status != proto.PlanItemStatus_PLAN_ITEM_STATUS_COMPLETED {
		t.Errorf("expected Status COMPLETED, got %v", protoItem.Status)
	}

	if protoItem.Progress != 1.0 {
		t.Errorf("expected Progress 1.0, got %f", protoItem.Progress)
	}

	if protoItem.StartedAt == nil || *protoItem.StartedAt != startedAt.Unix() {
		t.Errorf("expected StartedAt %d, got %v", startedAt.Unix(), protoItem.StartedAt)
	}

	if protoItem.CompletedAt == nil || *protoItem.CompletedAt != completedAt.Unix() {
		t.Errorf("expected CompletedAt %d, got %v", completedAt.Unix(), protoItem.CompletedAt)
	}

	if len(protoItem.Metadata) != 2 {
		t.Errorf("expected 2 metadata entries, got %d", len(protoItem.Metadata))
	}
}

func TestConvertPlanItemToProto_Nil(t *testing.T) {
	result := convertPlanItemToProto(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"contains", "hello world", "world", true},
		{"case insensitive", "Hello World", "hello", true},
		{"not contains", "hello", "world", false},
		{"empty substr", "hello", "", true},
		{"empty string", "", "hello", false},
		{"empty both", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}
