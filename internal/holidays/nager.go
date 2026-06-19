// Package holidays fetches public-holiday data from the free Nager.Date API.
// Coverage is good for many countries but incomplete for some (e.g. Hong Kong,
// Taiwan, mainland China); use CSV upload for those.
package holidays

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Item is one fetched holiday (date YYYY-MM-DD, display name).
type Item struct {
	Date string
	Name string
}

type nagerHoliday struct {
	Date      string `json:"date"`
	LocalName string `json:"localName"`
	Name      string `json:"name"`
}

// FetchNager returns the public holidays for a country (ISO-3166 alpha-2) and
// year from date.nager.at. Returns an error when the country/year is unsupported.
func FetchNager(ctx context.Context, country string, year int) ([]Item, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	url := fmt.Sprintf("https://date.nager.at/api/v3/PublicHolidays/%d/%s", year, country)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch holidays: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no holiday data for %s (country not supported by the online source)", country)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("holiday source returned status %d for %s", resp.StatusCode, country)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var raw []nagerHoliday
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode holiday data: %w", err)
	}
	out := make([]Item, 0, len(raw))
	for _, h := range raw {
		name := h.LocalName
		if name == "" {
			name = h.Name
		}
		if h.Date != "" && name != "" {
			out = append(out, Item{Date: h.Date, Name: name})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no holidays returned for %s %d", country, year)
	}
	return out, nil
}
