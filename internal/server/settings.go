package server

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/ai"
	"note-aura/internal/auth"
)

// emailInAddress builds a user's plus-addressed inbound address from the base
// address (local@domain) and their token: local+token@domain. Returns "" when
// the base isn't a valid address.
func emailInAddress(base, token string) string {
	at := strings.LastIndex(base, "@")
	if at <= 0 || token == "" {
		return ""
	}
	return base[:at] + "+" + token + "@" + base[at+1:]
}

func (s *Server) getSettings(c *fiber.Ctx) error {
	u := currentUser(c)
	settings, _ := s.db.GetUserSettings(u.ID)
	m := baseMap(c, "Settings")
	m["Nav"] = "settings"
	m["BaseURL"] = settings[ai.KeyBaseURL]
	m["HasKey"] = settings[ai.KeyAPIKey] != ""
	m["ChatModel"] = settings[ai.KeyChatModel]
	m["EmbedModel"] = settings[ai.KeyEmbedModel]
	m["VisionModel"] = settings[ai.KeyVisionModel]

	// Holiday country selection.
	avail, _ := s.db.DistinctHolidayCountries()
	m["HolidayCountries"] = avail
	selSet := map[string]bool{}
	for _, code := range splitCSVList(settings["holiday_countries"]) {
		selSet[code] = true
	}
	m["HolidaySelected"] = selSet

	// Invite new users (shown when the role/override grants any allowance).
	if limit, remaining := s.inviteAllowance(u); limit != 0 {
		m["ShowInvite"] = true
		m["InviteUnlimited"] = limit < 0
		m["InviteRemaining"] = remaining
		m["Invitations"], _ = s.db.ListInvitationsBy(u.ID)
		switch c.Query("ierror") {
		case "limit":
			m["InviteError"] = "You've reached your invitation limit."
		case "none":
			m["InviteError"] = "Your role doesn't allow sending invitations."
		case "invalid":
			m["InviteError"] = "Please enter a valid email address."
		case "exists":
			m["InviteError"] = "That email already has an account."
		}
		if c.Query("isent") == "1" {
			m["InviteSent"] = true
		}
		if c.Query("iresent") == "1" {
			m["InviteResent"] = true
		}
	}

	// Blocked users.
	m["Blocked"], _ = s.db.ListBlocked(u.ID)
	switch c.Query("berror") {
	case "admin":
		m["BlockError"] = "You can't block an admin."
	case "notfound":
		m["BlockError"] = "No user with that email."
	}

	// Storage usage.
	limit := s.capacityLimitBytes(u)
	used := s.storageUsed(u)
	m["StorageUsed"] = fmt.Sprintf("%.1f", float64(used)/bytesPerMB)
	if limit <= 0 {
		m["StorageUnlimited"] = true
	} else {
		m["StorageLimit"] = limit / bytesPerMB
		pct := used * 100 / limit
		if pct > 100 {
			pct = 100
		}
		m["StoragePercent"] = pct
	}

	// Daily built-in-AI (Ollama) usage, when limited.
	if _, usingOllama := s.db.UserAICapability(u.ID); usingOllama {
		if limit := s.db.OllamaDailyLimit(u.ID); limit > 0 {
			m["OllamaLimit"] = limit
			m["OllamaUsedToday"] = s.db.OllamaUsedToday(u.ID)
		}
	}

	// Import/export flash messages.
	if n := c.Query("imported"); n != "" && n != "0" {
		m["Imported"] = n
	}
	switch c.Query("imperr") {
	case "file":
		m["ImportError"] = "Couldn't read the uploaded file."
	case "format":
		m["ImportError"] = "That file isn't a valid Note-Aura export (JSON)."
	case "capacity":
		m["ImportError"] = "Storage limit reached — some notes were not imported."
	}

	// Email → note inbound address (shown only when IMAP is configured).
	if s.cfg.IMAP.Host != "" {
		m["EmailInEnabled"] = true
		m["EmailInAddress"] = emailInAddress(s.cfg.IMAP.Address, u.EmailToken)
	}

	// Per-user prompt customization: available to users running on their own
	// external AI server. The admin's prompts are deliberately not exposed.
	if s.db.UserHasCloudAI(u.ID) {
		m["CanEditPrompts"] = true
		m["PromptTitle"] = settings[ai.KeyPromptTitle]
		m["PromptSummary"] = settings[ai.KeyPromptSummary]
		m["PromptTags"] = settings[ai.KeyPromptTags]
		m["PromptOCR"] = settings[ai.KeyPromptOCR]
		m["PromptImage"] = settings[ai.KeyPromptImage]
	}
	return c.Render("settings", m, "layout")
}

// regenerateEmailToken issues (or rotates) the user's inbound-email token, which
// changes their Email→note address.
func (s *Server) regenerateEmailToken(c *fiber.Ctx) error {
	u := currentUser(c)
	tok, err := auth.NewToken()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "token error")
	}
	if err := s.db.SetUserEmailToken(u.ID, tok[:16]); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.Redirect("/settings", fiber.StatusFound)
}

// postSettings stores per-user AI overrides. Leaving base URL or API key blank
// reverts that user to the default Ollama backend.
func (s *Server) postSettings(c *fiber.Ctx) error {
	u := currentUser(c)
	set := func(k, v string) { _ = s.db.SetUserSetting(u.ID, k, strings.TrimSpace(v)) }

	set(ai.KeyBaseURL, c.FormValue("base_url"))
	// Only overwrite the key when a new value is supplied, so re-saving the form
	// doesn't wipe an existing key the UI never shows.
	if k := strings.TrimSpace(c.FormValue("api_key")); k != "" {
		set(ai.KeyAPIKey, k)
	}
	if c.FormValue("clear_key") == "on" {
		set(ai.KeyAPIKey, "")
	}
	set(ai.KeyChatModel, c.FormValue("chat_model"))
	set(ai.KeyEmbedModel, c.FormValue("embed_model"))
	set(ai.KeyVisionModel, c.FormValue("vision_model"))

	// Per-user prompt overrides — only for users on their own external AI server.
	if s.db.UserHasCloudAI(u.ID) {
		set(ai.KeyPromptTitle, c.FormValue("prompt_title"))
		set(ai.KeyPromptSummary, c.FormValue("prompt_summary"))
		set(ai.KeyPromptTags, c.FormValue("prompt_tags"))
		set(ai.KeyPromptOCR, c.FormValue("prompt_ocr"))
		set(ai.KeyPromptImage, c.FormValue("prompt_image"))
	}

	return c.Redirect("/settings", fiber.StatusFound)
}
