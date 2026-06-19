// Package reminder periodically delivers calendar reminder emails for notes
// whose event start (minus the configured lead time) has arrived.
package reminder

import (
	"fmt"
	"log"
	"strings"
	"time"

	"note-aura/internal/db"
	"note-aura/internal/mailer"
)

// Scheduler scans for due reminders on a fixed interval.
type Scheduler struct {
	db      *db.DB
	mailer  *mailer.Mailer
	baseURL string
}

func New(database *db.DB, m *mailer.Mailer, baseURL string) *Scheduler {
	return &Scheduler{db: database, mailer: m, baseURL: strings.TrimRight(baseURL, "/")}
}

// Start runs the scheduler loop in a goroutine. It is a no-op when email is not
// configured.
func (s *Scheduler) Start() {
	if !s.mailer.Enabled() {
		log.Printf("reminders: SMTP not configured — calendar reminders disabled")
		return
	}
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		s.tick() // run once at startup
		for range ticker.C {
			s.tick()
		}
	}()
	log.Printf("reminders: scheduler started")
}

func (s *Scheduler) tick() {
	pending, err := s.db.PendingReminders()
	if err != nil {
		log.Printf("reminders: query: %v", err)
		return
	}
	now := time.Now()
	for _, r := range pending {
		start, ok := eventStart(r.EventDate, r.StartTime)
		if !ok {
			s.db.MarkReminderSent(r.NoteID) // unparseable date; don't retry forever
			continue
		}
		send, stale := decide(now, start, r.Minutes)
		if stale {
			// Event well past (e.g. server was down): mark sent without emailing
			// so a backlog doesn't unleash a flood of stale reminders.
			s.db.MarkReminderSent(r.NoteID)
			continue
		}
		if !send {
			continue // not yet due
		}
		subject := "Reminder: " + r.Title
		body := s.body(r, start)
		if err := s.mailer.Send(r.OwnerEmail, subject, body); err != nil {
			log.Printf("reminders: send to %s for note %d: %v", r.OwnerEmail, r.NoteID, err)
			continue // leave unsent; retry next tick
		}
		if err := s.db.MarkReminderSent(r.NoteID); err != nil {
			log.Printf("reminders: mark sent note %d: %v", r.NoteID, err)
		}
	}
}

func (s *Scheduler) body(r db.DueReminder, start time.Time) string {
	when := start.Format("Mon 2 Jan 2006 15:04")
	if r.AllDay {
		when = start.Format("Mon 2 Jan 2006") + " (all day)"
	}
	link := fmt.Sprintf("%s/notes/%d", s.baseURL, r.NoteID)
	return fmt.Sprintf("Reminder for your note:\n\n%s\nWhen: %s\n\nOpen it: %s\n\n— Note-Aura",
		r.Title, when, link)
}

// decide reports whether a reminder should be sent now, and whether it is so
// stale (event already 24h+ past) that it should be skipped instead.
func decide(now, start time.Time, minutes int64) (send, stale bool) {
	if now.After(start.Add(24 * time.Hour)) {
		return false, true
	}
	fire := start.Add(-time.Duration(minutes) * time.Minute)
	return !now.Before(fire), false
}

// eventStart combines an event date (YYYY-MM-DD) and optional start time (HH:MM)
// into a local time. A missing time means midnight.
func eventStart(date, startTime string) (time.Time, bool) {
	layout := "2006-01-02 15:04"
	t := startTime
	if t == "" {
		t = "00:00"
	}
	parsed, err := time.ParseInLocation(layout, date+" "+t, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
