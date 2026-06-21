package server

import (
	"html/template"
	"time"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/captcha"
)

// captchaCookie holds the signed captcha token between GET (issue) and POST
// (verify) of an auth form.
const captchaCookie = "na_captcha"

// issueCaptcha generates a challenge, stores its token in a short-lived cookie,
// and returns the image as a data-URI (empty string on the rare RNG error).
func (s *Server) issueCaptcha(c *fiber.Ctx) string {
	ch, err := captcha.New()
	if err != nil {
		return ""
	}
	c.Cookie(&fiber.Cookie{
		Name:     captchaCookie,
		Value:    ch.Token,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   s.cfg.Session.Secure,
		Path:     "/",
	})
	return ch.Image
}

// checkCaptcha verifies the submitted "captcha" form value against the cookie.
func (s *Server) checkCaptcha(c *fiber.Ctx) bool {
	return captcha.Verify(c.Cookies(captchaCookie), c.FormValue("captcha"))
}

// renderAuth renders an auth template (login/register/forgot) with a freshly
// issued captcha, so the page's form always has a working, single-use challenge.
// The image is passed as template.URL so html/template allows the data: URI in
// an <img src> (a plain string would be replaced with "#ZgotmplZ").
func (s *Server) renderAuth(c *fiber.Ctx, tmpl string, m fiber.Map) error {
	if img := s.issueCaptcha(c); img != "" {
		m["CaptchaImage"] = template.URL(img)
	}
	return c.Render(tmpl, withLang(c, m), "layout")
}
