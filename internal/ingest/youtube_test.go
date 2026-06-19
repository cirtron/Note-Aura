package ingest

import (
	"strings"
	"testing"
)

// A caption payload is only accepted if it is a real WebVTT track. Google's
// anti-bot "Sorry / automated queries" page (returned when the caption endpoint
// rate-limits the server IP) must be rejected, not stored as a transcript.
func TestParseCaptionRejectsAntiBot(t *testing.T) {
	antiBot := "Transcript\nG o o g l e\nSorry...\nWe're sorry...\n" +
		"... but your computer or network may be sending automated queries. " +
		"To protect our users, we can't process your request right now.\nGoogle Home"
	if got, ok := parseCaption(antiBot); ok {
		t.Errorf("anti-bot page accepted as caption: ok=%v text=%q", ok, got)
	}
}

func TestParseCaptionAcceptsRealVTT(t *testing.T) {
	vtt := "WEBVTT\nKind: captions\nLanguage: en\n\n" +
		"00:00:01.000 --> 00:00:02.000\nhello there\n\n" +
		"00:00:02.000 --> 00:00:03.000\ngeneral kenobi\n"
	got, ok := parseCaption(vtt)
	if !ok {
		t.Fatal("real WebVTT rejected")
	}
	if !strings.Contains(got, "hello there") || !strings.Contains(got, "general kenobi") {
		t.Errorf("cleaned transcript missing content: %q", got)
	}
	if strings.Contains(got, "WEBVTT") || strings.Contains(got, "-->") {
		t.Errorf("cleanVTT did not strip headers/timestamps: %q", got)
	}
}
