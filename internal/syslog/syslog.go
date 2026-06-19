// Package syslog records notable system events (mail, server errors, inbound
// email, worker failures, reminders) to a persistent sink an admin can read,
// while also echoing them to the standard logger for console visibility.
package syslog

import (
	"fmt"
	"log"
)

// Sink persists a log entry. *db.DB satisfies this (AddLog).
type Sink interface {
	AddLog(level, category, message string) error
}

type entry struct{ level, category, message string }

var ch chan entry

// Init wires the persistent sink and starts a background writer. Without it,
// syslog still logs to the console but doesn't persist.
func Init(s Sink) {
	ch = make(chan entry, 512)
	go func() {
		for e := range ch {
			_ = s.AddLog(e.level, e.category, e.message)
		}
	}()
}

// Log records an event at the given level/category. It never blocks the caller:
// the persistent write is queued (dropped if the queue is full).
func Log(level, category, message string) {
	log.Printf("[%s] %s: %s", level, category, message)
	if ch != nil {
		select {
		case ch <- entry{level, category, message}:
		default: // queue full — drop rather than block
		}
	}
}

func Infof(category, format string, a ...any)  { Log("info", category, fmt.Sprintf(format, a...)) }
func Warnf(category, format string, a ...any)  { Log("warn", category, fmt.Sprintf(format, a...)) }
func Errorf(category, format string, a ...any) { Log("error", category, fmt.Sprintf(format, a...)) }
