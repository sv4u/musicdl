package audio

import (
	"testing"
)

func TestResolveProbeURL(t *testing.T) {
	t.Setenv("YOUTUBE_PROBE_URL", "")
	if got := ResolveProbeURL(); got != DefaultYouTubeCookieProbeURL {
		t.Fatalf("empty env: got %q want default", got)
	}
	t.Setenv("YOUTUBE_PROBE_URL", "https://www.youtube.com/watch?v=testid")
	if got := ResolveProbeURL(); got != "https://www.youtube.com/watch?v=testid" {
		t.Fatalf("override: got %q", got)
	}
}
