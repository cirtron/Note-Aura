package server

import (
	"bufio"
	"context"
	"encoding/csv"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/db"
	"note-aura/internal/holidays"
)

// countryNames maps ISO-3166 alpha-2 codes to display names for the holiday
// selectors. Unknown codes fall back to the raw code.
var countryNames = map[string]string{
	"HK": "Hong Kong", "TW": "Taiwan", "CN": "China", "MO": "Macau",
	"US": "United States", "GB": "United Kingdom", "CA": "Canada", "AU": "Australia",
	"NZ": "New Zealand", "IE": "Ireland", "SG": "Singapore", "MY": "Malaysia",
	"JP": "Japan", "KR": "South Korea", "IN": "India", "PH": "Philippines",
	"TH": "Thailand", "VN": "Vietnam", "ID": "Indonesia",
	"DE": "Germany", "FR": "France", "ES": "Spain", "IT": "Italy", "PT": "Portugal",
	"NL": "Netherlands", "BE": "Belgium", "CH": "Switzerland", "AT": "Austria",
	"SE": "Sweden", "NO": "Norway", "DK": "Denmark", "FI": "Finland", "PL": "Poland",
	"RU": "Russia", "BR": "Brazil", "MX": "Mexico", "AR": "Argentina", "ZA": "South Africa",
	"AE": "United Arab Emirates", "SA": "Saudi Arabia", "IL": "Israel", "TR": "Türkiye",
}

func countryName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if n, ok := countryNames[code]; ok {
		return n
	}
	return code
}

// splitCSVList splits a comma-separated, upper-cased list of country codes.
func splitCSVList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.ToUpper(strings.TrimSpace(p)); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ----- user: choose which countries' holidays to show -----

func (s *Server) setHolidayCountries(c *fiber.Ctx) error {
	u := currentUser(c)
	var sel []string
	seen := map[string]bool{}
	c.Context().PostArgs().VisitAll(func(k, v []byte) {
		if string(k) != "holiday_countries" {
			return
		}
		code := strings.ToUpper(strings.TrimSpace(string(v)))
		if code != "" && !seen[code] {
			seen[code] = true
			sel = append(sel, code)
		}
	})
	_ = s.db.SetUserSetting(u.ID, "holiday_countries", strings.Join(sel, ","))
	return c.Redirect("/settings", fiber.StatusFound)
}

// ----- admin: load holiday data -----

// importHolidays fetches holidays for one or more country codes (current and
// next year) from the online source.
func (s *Server) importHolidays(c *fiber.Ctx) error {
	codes := splitCSVList(c.FormValue("country"))
	year := time.Now().Year()
	ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
	defer cancel()
	for _, code := range codes {
		for _, y := range []int{year, year + 1} {
			items, err := holidays.FetchNager(ctx, code, y)
			if err != nil {
				continue // unsupported country/year; skip (CSV upload is the fallback)
			}
			rows := make([]db.Holiday, 0, len(items))
			for _, it := range items {
				rows = append(rows, db.Holiday{CountryCode: code, Date: it.Date, Name: it.Name})
			}
			_, _ = s.db.InsertHolidays(rows)
		}
	}
	return c.Redirect("/admin", fiber.StatusFound)
}

// uploadHolidays imports a CSV (date,name) for a single country.
func (s *Server) uploadHolidays(c *fiber.Ctx) error {
	code := strings.ToUpper(strings.TrimSpace(c.FormValue("country")))
	if code == "" {
		return fiber.NewError(fiber.StatusBadRequest, "country code is required")
	}
	fh, err := c.FormFile("csv")
	if err != nil || fh == nil {
		return fiber.NewError(fiber.StatusBadRequest, "a CSV file is required")
	}
	f, err := fh.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	defer f.Close()

	rows, err := parseHolidayCSV(code, f)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "could not parse CSV: "+err.Error())
	}
	if _, err := s.db.InsertHolidays(rows); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/admin", fiber.StatusFound)
}

func (s *Server) deleteHolidays(c *fiber.Ctx) error {
	_ = s.db.DeleteHolidaysByCountry(c.FormValue("country"))
	return c.Redirect("/admin", fiber.StatusFound)
}

// parseHolidayCSV reads "date,name" rows (header optional, BOM tolerated). Date
// must be YYYY-MM-DD.
func parseHolidayCSV(code string, r io.Reader) ([]db.Holiday, error) {
	br := bufio.NewReader(r)
	// Strip a UTF-8 BOM if present.
	if b, err := br.Peek(3); err == nil && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	var out []db.Holiday
	first := true
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 2 {
			continue
		}
		date := strings.TrimSpace(rec[0])
		name := strings.TrimSpace(rec[1])
		if first {
			first = false
			if _, perr := time.Parse("2006-01-02", date); perr != nil {
				continue // header row
			}
		}
		if _, perr := time.Parse("2006-01-02", date); perr != nil {
			continue
		}
		if name != "" {
			out = append(out, db.Holiday{CountryCode: code, Date: date, Name: name})
		}
	}
	return out, nil
}
