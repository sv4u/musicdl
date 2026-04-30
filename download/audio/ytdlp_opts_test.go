package audio

import (
	"reflect"
	"testing"
)

func TestAppendYtDlpYouTubeOpts_MergesFlags(t *testing.T) {
	p := &Provider{
		config: &Config{
			Cookies:            "/tmp/cookies.txt",
			JSRuntimes:         "node",
			RemoteComponents:   "ejs:github",
			CookiesFromBrowser: "",
		},
	}
	got := p.appendYtDlpYouTubeOpts([]string{"--quiet"})
	want := []string{
		"--quiet",
		"--cookies", "/tmp/cookies.txt",
		"--js-runtimes", "node",
		"--remote-components", "ejs:github",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendYtDlpYouTubeOpts: got %#v want %#v", got, want)
	}
}

func TestAppendYtDlpYouTubeOpts_CookiesFromBrowser(t *testing.T) {
	p := &Provider{
		config: &Config{CookiesFromBrowser: "firefox"},
	}
	got := p.appendYtDlpYouTubeOpts([]string{})
	want := []string{"--cookies-from-browser", "firefox"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
