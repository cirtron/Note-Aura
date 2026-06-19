package server

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
)

// calCell is one day in the month grid (Day == 0 means a padding cell).
type calCell struct {
	Day      int
	Date     string
	IsToday  bool
	Notes    []*db.Note
	Holidays []db.Holiday
}

func (s *Server) getCalendar(c *fiber.Ctx) error {
	u := currentUser(c)
	now := time.Now()

	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	if mp := c.Query("month"); mp != "" {
		if t, err := time.ParseInLocation("2006-01", mp, time.Local); err == nil {
			first = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
		}
	}
	last := first.AddDate(0, 1, -1)

	notes, err := s.db.NotesByDateRange(u.ID, first.Format("2006-01-02"), last.Format("2006-01-02"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	byDate := map[string][]*db.Note{}
	for _, n := range notes {
		byDate[n.EventDate] = append(byDate[n.EventDate], n)
	}

	// Overlay public holidays for the user's selected countries.
	settings, _ := s.db.GetUserSettings(u.ID)
	holidayCountries := splitCSVList(settings["holiday_countries"])
	holsByDate := map[string][]db.Holiday{}
	if len(holidayCountries) > 0 {
		hs, _ := s.db.HolidaysByCountriesRange(holidayCountries, first.Format("2006-01-02"), last.Format("2006-01-02"))
		for _, h := range hs {
			holsByDate[h.Date] = append(holsByDate[h.Date], h)
		}
	}

	todayStr := now.Format("2006-01-02")
	var cells []calCell
	for i := 0; i < int(first.Weekday()); i++ { // leading blanks (week starts Sunday)
		cells = append(cells, calCell{})
	}
	for d := 1; d <= last.Day(); d++ {
		date := first.AddDate(0, 0, d-1).Format("2006-01-02")
		cells = append(cells, calCell{Day: d, Date: date, IsToday: date == todayStr, Notes: byDate[date], Holidays: holsByDate[date]})
	}
	for len(cells)%7 != 0 { // trailing blanks
		cells = append(cells, calCell{})
	}

	m := baseMap(c, "Calendar")
	m["Nav"] = "calendar"
	m["Cells"] = cells
	m["Weekdays"] = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	m["MonthTitle"] = first.Format("January 2006")
	m["MonthParam"] = first.Format("2006-01")
	m["PrevMonth"] = first.AddDate(0, -1, 0).Format("2006-01")
	m["NextMonth"] = first.AddDate(0, 1, 0).Format("2006-01")
	m["ThisMonth"] = now.Format("2006-01")

	if sel := c.Query("date"); sel != "" {
		m["SelectedDate"] = sel
		agenda, ok := byDate[sel]
		if !ok {
			agenda, _ = s.db.NotesByDateRange(u.ID, sel, sel)
		}
		m["Agenda"] = agenda
		m["AgendaHolidays"] = holsByDate[sel]
	}
	return c.Render("calendar", m, "layout")
}

// saveSchedule reads the calendar fields from a note form and persists them.
func (s *Server) saveSchedule(c *fiber.Ctx, noteID int64) {
	eventDate := strings.TrimSpace(c.FormValue("event_date"))
	startTime := strings.TrimSpace(c.FormValue("start_time"))
	endTime := strings.TrimSpace(c.FormValue("end_time"))
	allDay := c.FormValue("all_day") == "on"
	if allDay {
		startTime, endTime = "", ""
	}
	var reminder *int
	if rs := strings.TrimSpace(c.FormValue("reminder")); rs != "" && eventDate != "" {
		if v, err := strconv.Atoi(rs); err == nil {
			reminder = &v
		}
	}
	_ = s.db.SetNoteSchedule(noteID, eventDate, startTime, endTime, allDay, reminder)
}
