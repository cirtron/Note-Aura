package server

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// usageRow is a per-user usage summary for the dashboard.
type usageRow struct {
	ID        int64
	Email     string
	Role      string
	IsAdmin   bool
	Suspended bool
	Notes     int
	UsedMB    string
	LimitMB   int64
	Unlimited bool
}

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

// dashPageSize is the rows-per-page for the dashboard's paginated tables.
const dashPageSize = 20

func dashPage(c *fiber.Ctx, key string) int {
	p, _ := strconv.Atoi(c.Query(key, "1"))
	if p < 1 {
		p = 1
	}
	return p
}

func totalPages(total, size int) int {
	if total <= 0 {
		return 1
	}
	return (total + size - 1) / size
}

func (s *Server) getDashboard(c *fiber.Ctx) error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// User usage (paginated).
	allUsers, _ := s.db.ListUsers()
	totalUsers := len(allUsers)
	uPages := totalPages(totalUsers, dashPageSize)
	upage := dashPage(c, "upage")
	if upage > uPages {
		upage = uPages
	}
	uStart := (upage - 1) * dashPageSize
	uEnd := uStart + dashPageSize
	if uStart > totalUsers {
		uStart = totalUsers
	}
	if uEnd > totalUsers {
		uEnd = totalUsers
	}
	usage := make([]usageRow, 0, uEnd-uStart)
	for _, u := range allUsers[uStart:uEnd] {
		n, _ := s.db.CountNotesByOwner(u.ID)
		used := s.storageUsed(u)
		limit := s.capacityLimitBytes(u)
		usage = append(usage, usageRow{
			ID: u.ID, Email: u.Email, Role: u.RoleSlug, IsAdmin: u.IsAdmin, Suspended: u.Suspended,
			Notes: n, UsedMB: fmt.Sprintf("%.1f", float64(used)/bytesPerMB),
			LimitMB: limit / bytesPerMB, Unlimited: limit <= 0,
		})
	}

	totalNotes, _ := s.db.CountNotes()
	totalGroups, _ := s.db.CountGroups()

	// Recent notes (paginated).
	rPages := totalPages(totalNotes, dashPageSize)
	rpage := dashPage(c, "rpage")
	if rpage > rPages {
		rpage = rPages
	}
	recent, _ := s.db.RecentNotesWithOwnerPage(dashPageSize, (rpage-1)*dashPageSize)

	link := func(up, rp int) string { return fmt.Sprintf("/dashboard?upage=%d&rpage=%d", up, rp) }

	m := baseMap(c, "Dashboard")
	m["Nav"] = "dashboard"
	// Pagination state/links.
	m["UPage"], m["UPages"] = upage, uPages
	m["RPage"], m["RPages"] = rpage, rPages
	if upage > 1 {
		m["UPrev"] = link(upage-1, rpage)
	}
	if upage < uPages {
		m["UNext"] = link(upage+1, rpage)
	}
	if rpage > 1 {
		m["RPrev"] = link(upage, rpage-1)
	}
	if rpage < rPages {
		m["RNext"] = link(upage, rpage+1)
	}
	// Server monitor.
	m["Uptime"] = time.Since(s.startTime).Round(time.Second).String()
	m["GoVersion"] = runtime.Version()
	m["CPUs"] = runtime.NumCPU()
	m["Goroutines"] = runtime.NumGoroutine()
	m["MemAllocMB"] = ms.Alloc / (1 << 20)
	m["MemSysMB"] = ms.Sys / (1 << 20)
	m["DBSizeMB"] = fmt.Sprintf("%.1f", float64(fileSize(s.cfg.DBPath))/bytesPerMB)
	m["UploadsSizeMB"] = fmt.Sprintf("%.1f", float64(dirSize(s.uploadsDir))/bytesPerMB)
	m["Jobs"] = s.db.JobCounts()
	// Totals.
	m["TotalUsers"] = totalUsers
	m["TotalNotes"] = totalNotes
	m["TotalGroups"] = totalGroups
	// Tables.
	m["Usage"] = usage
	m["Recent"] = recent
	return c.Render("dashboard", m, "layout")
}

// suspendUser suspends or reactivates a user (can't suspend yourself or the last
// admin).
func (s *Server) suspendUser(c *fiber.Ctx) error {
	u := currentUser(c)
	uid, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	suspend := c.FormValue("suspend") == "on" || c.FormValue("suspend") == "true"
	if uid == 0 || uid == u.ID {
		return c.Redirect("/admin/users", fiber.StatusFound)
	}
	if suspend {
		if t, err := s.db.GetUser(uid); err == nil && t.IsAdmin {
			if n, _ := s.db.CountAdmins(); n <= 1 {
				return c.Redirect("/admin/users", fiber.StatusFound) // never lock out the last admin
			}
		}
	}
	var untilTime *time.Time
	if suspend {
		if h, err := strconv.Atoi(c.FormValue("suspend_hours")); err == nil && h > 0 {
			t := time.Now().Add(time.Duration(h) * time.Hour)
			untilTime = &t
		}
	}
	_ = s.db.SetUserSuspended(uid, suspend, untilTime)
	return c.Redirect("/admin/users", fiber.StatusFound)
}

// deleteUser removes a user and all their data (can't delete yourself or the
// last admin).
func (s *Server) deleteUser(c *fiber.Ctx) error {
	u := currentUser(c)
	uid, _ := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if uid == 0 || uid == u.ID {
		return c.Redirect("/admin/users", fiber.StatusFound)
	}
	if t, err := s.db.GetUser(uid); err == nil && t.IsAdmin {
		if n, _ := s.db.CountAdmins(); n <= 1 {
			return c.Redirect("/admin/users", fiber.StatusFound)
		}
	}
	_ = s.db.DeleteUser(uid)
	return c.Redirect("/admin/users", fiber.StatusFound)
}
