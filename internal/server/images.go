package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"note-aura/internal/auth"
)

// inlineImageExts are the image types accepted for inline note images. SVG is
// deliberately excluded: a stored SVG can carry script that would run if opened
// directly, which matters for images embedded in shared notes.
var inlineImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

const maxInlineImageBytes = 10 << 20 // 10 MB

// uploadImage receives an image from the editor (EasyMDE upload / drag-drop /
// paste), stores it under uploads/inline/<userID>/, and returns the JSON shape
// EasyMDE expects: {"data":{"filePath":"<url>"}} (or {"error":"<code>"}).
func (s *Server) uploadImage(c *fiber.Ctx) error {
	u := currentUser(c)
	fh, err := c.FormFile("image")
	if err != nil || fh == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "noFileGiven"})
	}
	if fh.Size > maxInlineImageBytes {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "fileTooLarge"})
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !inlineImageExts[ext] || !s.userCanUploadExt(u, ext) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "typeNotAllowed"})
	}
	if s.overCapacity(u, fh.Size) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Storage limit reached"})
	}

	dir := filepath.Join(s.uploadsDir, "inline", strconv.FormatInt(u.ID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "importError"})
	}
	token, err := auth.NewToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "importError"})
	}
	name := token[:24] + ext
	if err := c.SaveFile(fh, filepath.Join(dir, name)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "importError"})
	}

	url := "/uploads/inline/" + strconv.FormatInt(u.ID, 10) + "/" + name
	return c.JSON(fiber.Map{"data": fiber.Map{"filePath": url}})
}
