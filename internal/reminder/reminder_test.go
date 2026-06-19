package reminder

import (
	"testing"
	"time"
)

func TestDecide(t *testing.T) {
	start := time.Date(2026, 6, 8, 14, 30, 0, 0, time.Local) // event at 14:30

	cases := []struct {
		name           string
		now            time.Time
		minutes        int64
		wantSend, stale bool
	}{
		{"before fire window", time.Date(2026, 6, 8, 14, 0, 0, 0, time.Local), 15, false, false}, // fire 14:15, now 14:00
		{"exactly at fire", time.Date(2026, 6, 8, 14, 15, 0, 0, time.Local), 15, true, false},
		{"after fire, before event", time.Date(2026, 6, 8, 14, 20, 0, 0, time.Local), 15, true, false},
		{"at time of event (0 min)", time.Date(2026, 6, 8, 14, 30, 0, 0, time.Local), 0, true, false},
		{"1 day before", time.Date(2026, 6, 7, 14, 30, 0, 0, time.Local), 1440, true, false},
		{"stale (event 2 days past)", time.Date(2026, 6, 10, 14, 30, 0, 0, time.Local), 15, false, true},
	}
	for _, tc := range cases {
		send, stale := decide(tc.now, start, tc.minutes)
		if send != tc.wantSend || stale != tc.stale {
			t.Errorf("%s: decide=(%v,%v), want (%v,%v)", tc.name, send, stale, tc.wantSend, tc.stale)
		}
	}
}

func TestEventStart(t *testing.T) {
	if _, ok := eventStart("2026-06-08", "14:30"); !ok {
		t.Error("expected valid parse for timed event")
	}
	got, ok := eventStart("2026-06-08", "") // all-day → midnight
	if !ok || got.Hour() != 0 || got.Minute() != 0 {
		t.Errorf("all-day start = %v (ok=%v), want midnight", got, ok)
	}
	if _, ok := eventStart("not-a-date", "14:30"); ok {
		t.Error("expected parse failure for bad date")
	}
}
