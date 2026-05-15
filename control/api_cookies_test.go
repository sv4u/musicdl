package main

import "testing"

func TestNetscapeJarHasYouTubeOrGoogle(t *testing.T) {
	t.Parallel()
	jar := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t0\tSID\tabc\n"
	if !netscapeJarHasYouTubeOrGoogle([]byte(jar)) {
		t.Fatal("expected true for .youtube.com row")
	}
	jar2 := ".google.com\tTRUE\t/\tFALSE\t0\tx\ty\n"
	if !netscapeJarHasYouTubeOrGoogle([]byte(jar2)) {
		t.Fatal("expected true for .google.com row")
	}
	jar3 := "# empty\nfoo.com\tTRUE\t/\tFALSE\t0\ta\tb\n"
	if netscapeJarHasYouTubeOrGoogle([]byte(jar3)) {
		t.Fatal("expected false for unrelated domain")
	}
}
