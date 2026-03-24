package audio

import (
	"math"
	"testing"
)

func TestTitleContainsLiveKeyword(t *testing.T) {
	tests := []struct {
		title    string
		expected bool
	}{
		{"The Future Is Now", false},
		{"Diary of Jane", false},
		{"toe - The Future Is Now (Live)", true},
		{"Breaking Benjamin - Diary of Jane Live at Rock am Ring", true},
		{"Unplugged Session", true},
		{"Festival Mix 2024", true},
		{"My Concert Dream", true},
		{"Acoustic Version", true},
		{"Performance Art", true},
		{"LIVE FROM TOKYO", true},
		{"ライブ版", true},
		{"Bootleg Recording", true},
		{"Tour Edition", true},
		{"Studio Album Track", false},
		{"Alive and Well", false},
		{"Oliver Tree", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := TitleContainsLiveKeyword(tt.title)
			if got != tt.expected {
				t.Errorf("TitleContainsLiveKeyword(%q) = %v, want %v", tt.title, got, tt.expected)
			}
		})
	}
}

func TestScoreResult_RejectsLiveForStudioTrack(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "The Future Is Now",
		ExpectedArtist:     "toe",
		ExpectedDurationS:  240,
		OriginalHasLiveTag: false,
	}

	liveResult := ytDlpSearchResult{
		Title:    "toe - The Future Is Now (Live at Liquidroom)",
		Duration: 310,
		URL:      "https://www.youtube.com/watch?v=live123",
	}
	score := scoreResult(liveResult, criteria)
	if score != math.MaxFloat64 {
		t.Errorf("expected live result to be rejected (MaxFloat64), got %v", score)
	}
}

func TestScoreResult_AcceptsLiveForLiveTrack(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "Away (Live)",
		ExpectedArtist:     "Breaking Benjamin",
		ExpectedDurationS:  200,
		OriginalHasLiveTag: true,
	}

	liveResult := ytDlpSearchResult{
		Title:    "Breaking Benjamin - Away (Live)",
		Duration: 210,
		URL:      "https://www.youtube.com/watch?v=live456",
	}
	score := scoreResult(liveResult, criteria)
	if score == math.MaxFloat64 {
		t.Error("live result should NOT be rejected when original track is live")
	}
}

func TestScoreResult_RejectsDurationMismatch(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "Short Song",
		ExpectedArtist:     "Artist",
		ExpectedDurationS:  180,
		OriginalHasLiveTag: false,
	}

	longResult := ytDlpSearchResult{
		Title:    "Short Song - Full Album",
		Duration: 3600,
		URL:      "https://www.youtube.com/watch?v=long789",
	}
	score := scoreResult(longResult, criteria)
	if score != math.MaxFloat64 {
		t.Errorf("expected duration-mismatched result to be rejected, got %v", score)
	}
}

func TestScoreResult_PrefersSimilarDuration(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "Test Song",
		ExpectedArtist:     "Artist",
		ExpectedDurationS:  200,
		OriginalHasLiveTag: false,
	}

	exact := ytDlpSearchResult{Title: "Test Song", Duration: 200, URL: "a"}
	close := ytDlpSearchResult{Title: "Test Song", Duration: 210, URL: "b"}

	scoreExact := scoreResult(exact, criteria)
	scoreClose := scoreResult(close, criteria)

	if scoreExact >= scoreClose {
		t.Errorf("exact duration match (%v) should score better than close match (%v)", scoreExact, scoreClose)
	}
}

func TestScoreResult_NilCriteriaReturnsZero(t *testing.T) {
	result := ytDlpSearchResult{Title: "Anything", Duration: 100, URL: "x"}
	score := scoreResult(result, nil)
	if score != 0 {
		t.Errorf("nil criteria should return 0, got %v", score)
	}
}

func TestPickBestResult_EmptySlice(t *testing.T) {
	_, ok := pickBestResult(nil, nil)
	if ok {
		t.Error("pickBestResult on empty slice should return ok=false")
	}
}

func TestPickBestResult_NilCriteriaReturnsFirst(t *testing.T) {
	results := []ytDlpSearchResult{
		{Title: "First", URL: "a"},
		{Title: "Second", URL: "b"},
	}
	best, ok := pickBestResult(results, nil)
	if !ok || best.Title != "First" {
		t.Errorf("nil criteria should return first result, got %q", best.Title)
	}
}

func TestPickBestResult_PicksStudioOverLive(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "The Future Is Now",
		ExpectedArtist:     "toe",
		ExpectedDurationS:  240,
		OriginalHasLiveTag: false,
	}

	results := []ytDlpSearchResult{
		{Title: "toe - The Future Is Now (Live at Liquidroom)", Duration: 310, URL: "live"},
		{Title: "toe - The Future Is Now Live Performance", Duration: 280, URL: "live2"},
		{Title: "toe - The Future Is Now", Duration: 238, URL: "studio"},
		{Title: "toe - The Future Is Now (Official Audio)", Duration: 245, URL: "official"},
	}

	best, ok := pickBestResult(results, criteria)
	if !ok {
		t.Fatal("expected a valid result")
	}
	if best.URL != "studio" {
		t.Errorf("expected studio version (url=studio), got url=%s title=%q", best.URL, best.Title)
	}
}

func TestPickBestResult_AllRejectedReturnsNotOk(t *testing.T) {
	criteria := &SearchCriteria{
		ExpectedTitle:      "Test",
		ExpectedArtist:     "Artist",
		ExpectedDurationS:  60,
		OriginalHasLiveTag: false,
	}

	results := []ytDlpSearchResult{
		{Title: "Test Live Concert", Duration: 60, URL: "a"},
		{Title: "Test Acoustic Session", Duration: 60, URL: "b"},
	}

	_, ok := pickBestResult(results, criteria)
	if ok {
		t.Error("all results contain live keywords for a non-live track; should return ok=false")
	}
}

func TestGenerateSearchVariants_OrderMostSpecificFirst(t *testing.T) {
	variants := GenerateSearchVariants("toe", "The Future Is Now")
	if len(variants) == 0 {
		t.Fatal("expected at least one variant")
	}
	if variants[0] != "toe - The Future Is Now" {
		t.Errorf("first variant should be most specific 'artist - title', got %q", variants[0])
	}
}

func TestGenerateSearchVariants_StripsFeat(t *testing.T) {
	variants := GenerateSearchVariants("Artist", "Song (feat. Other)")
	found := false
	for _, v := range variants {
		if v == "Artist - Song" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected feat-stripped variant 'Artist - Song', variants = %v", variants)
	}
}
