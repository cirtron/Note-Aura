package ingest

import "testing"

func TestIsFacebook(t *testing.T) {
	yes := []string{
		"https://www.facebook.com/watch/?v=123",
		"https://facebook.com/someone/posts/456",
		"https://m.facebook.com/story.php?story_fbid=1",
		"https://fb.watch/abcdEFG/",
		"https://www.facebook.com/reel/789",
	}
	no := []string{
		"https://www.youtube.com/watch?v=abc",
		"https://example.com/facebook-clone",
		"https://notfacebook.example.com/x",
	}
	for _, u := range yes {
		if !IsFacebook(u) {
			t.Errorf("IsFacebook(%q) = false, want true", u)
		}
	}
	for _, u := range no {
		if IsFacebook(u) {
			t.Errorf("IsFacebook(%q) = true, want false", u)
		}
	}
}

func TestValidVTT(t *testing.T) {
	if !validVTT("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nhi") {
		t.Error("real VTT rejected")
	}
	if validVTT("Transcript\nG o o g l e\nSorry...\nWe're sorry... automated queries") {
		t.Error("anti-bot HTML accepted as VTT")
	}
}

func TestParseNetscapeCookies(t *testing.T) {
	txt := "# Netscape HTTP Cookie File\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\tc_user\t123456\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\txs\tabcDEF\n"
	cs := parseNetscapeCookies(txt)
	if len(cs) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cs))
	}
	if cs[0].Name != "c_user" || cs[0].Value != "123456" || cs[0].Domain != ".facebook.com" {
		t.Errorf("cookie[0] = %+v", cs[0])
	}
}
