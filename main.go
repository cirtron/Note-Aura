package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"note-aura/internal/ai"
	"note-aura/internal/auth"
	"note-aura/internal/config"
	"note-aura/internal/db"
	"note-aura/internal/emailin"
	"note-aura/internal/ingest"
	"note-aura/internal/mailer"
	"note-aura/internal/reminder"
	"note-aura/internal/server"
	"note-aura/internal/syslog"
	"note-aura/internal/worker"
)

//go:embed all:web/templates
var templatesFS embed.FS

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("ensure db dir: %v", err)
		}
	}
	if err := os.MkdirAll(cfg.UploadsDir, 0o755); err != nil {
		log.Fatalf("ensure uploads dir: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	// Persist notable system events (mail, errors, email-in, …) for the admin log.
	syslog.Init(database)

	// One-time: convert notes from the old Quill/HTML editor to Markdown.
	if n, err := database.BackfillMarkdown(ingest.HTMLToText); err != nil {
		log.Printf("markdown backfill: %v", err)
	} else if n > 0 {
		log.Printf("converted %d note(s) from HTML to Markdown", n)
	}

	// Optional first-run admin account.
	if cfg.InitialUser.Email != "" {
		if n, _ := database.CountUsers(); n == 0 {
			hash, err := auth.HashPassword(cfg.InitialUser.Password)
			if err != nil {
				log.Fatalf("hash initial user: %v", err)
			}
			if _, err := database.CreateUser(cfg.InitialUser.Email, hash, true, true, ""); err != nil {
				log.Fatalf("create initial user: %v", err)
			}
			log.Printf("seeded initial admin %s", cfg.InitialUser.Email)
		}
	}

	// Fallback AI config from config.yaml. Admin app_settings overlay this at
	// runtime, and the worker/handlers build a provider per request.
	fallback := ai.GlobalConfig{
		OllamaURL: cfg.AI.OllamaURL,
		Models: ai.Models{
			Title: cfg.AI.ChatModel, Summary: cfg.AI.ChatModel, Tags: cfg.AI.ChatModel,
			Chat: cfg.AI.ChatModel, OCR: cfg.AI.OCRModel, Image: cfg.AI.ImageModel,
			Embed: cfg.AI.EmbedModel,
		},
		Prompts: ai.DefaultPrompts(),
		Timeout: time.Duration(cfg.AI.TimeoutSeconds) * time.Second,
	}

	// Web-link capture: enable the headless-browser fallback for JS pages.
	ingest.EnableHeadless = cfg.Fetch.Headless
	ingest.HeadlessWait = time.Duration(cfg.Fetch.HeadlessWaitMs) * time.Millisecond
	if cfg.Fetch.Headless {
		log.Printf("web-link capture: headless-browser fallback enabled (wait %dms)", cfg.Fetch.HeadlessWaitMs)
	}

	wk := worker.New(database, fallback, time.Duration(cfg.AI.TimeoutSeconds)*time.Second)
	wk.Start(cfg.Worker.Concurrency)
	log.Printf("worker pool started (concurrency=%d)", cfg.Worker.Concurrency)
	// Surface the effective AI settings so OCR/timeout issues are diagnosable from
	// the log. The per-call timeout must cover a full (cold) vision generation; an
	// admin can override the Ollama URL at runtime via the Admin page.
	log.Printf("AI config: ollama_url=%s ocr_model=%s image_model=%s per-call timeout=%s",
		cfg.AI.OllamaURL, cfg.AI.OCRModel, cfg.AI.ImageModel, time.Duration(cfg.AI.TimeoutSeconds)*time.Second)

	mail := mailer.New(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password, cfg.SMTP.From, *cfg.SMTP.STARTTLS)

	templates, err := fs.Sub(templatesFS, "web/templates")
	if err != nil {
		log.Fatalf("templates sub-fs: %v", err)
	}

	srv := server.New(cfg, database, fallback, wk, mail, templates)

	// Calendar reminder emails (no-op unless SMTP is configured); links use the
	// effective scheme (https when TLS is on).
	reminder.New(database, mail, srv.LinkBase()).Start()

	// Inbound Email → note (no-op unless IMAP is configured).
	emailin.New(cfg.IMAP, database, srv.HandleInboundEmail).Start()

	scheme := "http"
	if cfg.TLS.Enabled() {
		scheme = "https"
	}
	log.Printf("note-aura listening on %s (%s)", cfg.ListenAddr, scheme)
	if err := srv.Listen(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
