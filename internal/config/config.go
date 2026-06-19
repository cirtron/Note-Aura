// Package config loads the Note-Aura YAML configuration.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Session struct {
	CookieName string `yaml:"cookie_name"`
	TTLHours   int    `yaml:"ttl_hours"`
	Secure     bool   `yaml:"secure"`
}

// AI holds the default (Ollama) inference settings. Per-user overrides live in
// the user_settings table and are resolved at job time.
type AI struct {
	OllamaURL      string `yaml:"ollama_url"`
	ChatModel      string `yaml:"chat_model"`
	EmbedModel     string `yaml:"embed_model"`
	VisionModel    string `yaml:"vision_model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type Worker struct {
	Concurrency int `yaml:"concurrency"`
}

// Fetch tunes web-link content capture. The headless-browser fallback renders
// JavaScript pages (requires Chrome/Chromium installed) when the plain HTTP fetch
// yields too little text.
type Fetch struct {
	Headless       bool `yaml:"headless"`
	HeadlessWaitMs int  `yaml:"headless_wait_ms"`
}

type InitialUser struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

// SMTP configures outbound email (reminders, verification, invitations). Email
// is disabled when Host is empty.
type SMTP struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	// STARTTLS toggles opportunistic TLS upgrade on a plain connection (port 587
	// / 25). Defaults to true when unset; ignored for implicit-TLS port 465.
	STARTTLS *bool `yaml:"starttls"`
}

// IMAP configures inbound email → note capture. Disabled when Host is empty.
// Users send mail to a per-user plus-address (Address with +<token>); the poller
// reads the mailbox, matches the token to a user, and creates a note.
type IMAP struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// TLS selects implicit TLS (port 993). When false, a plain connection with
	// STARTTLS is used. Defaults to true when unset.
	TLS     *bool  `yaml:"tls"`
	Mailbox string `yaml:"mailbox"` // default INBOX
	// InsecureSkipVerify disables TLS certificate verification for the IMAP
	// connection. Needed for self-signed servers or when a local TLS-inspecting
	// antivirus (e.g. Avast Mail Shield) re-signs the cert with an untrusted root.
	// Security downgrade — prefer fixing the cert/AV. Defaults to false.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
	// Address is the public base address users send to, e.g. notes@example.com.
	// The per-user address shown in Settings is notes+<token>@example.com.
	// Defaults to Username.
	Address         string `yaml:"address"`
	PollSeconds     int    `yaml:"poll_seconds"`     // default 60
	DeleteProcessed bool   `yaml:"delete_processed"` // delete vs mark \Seen (default mark)
}

// TLS serves the web UI over HTTPS when both CertFile and KeyFile are set.
type TLS struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// Enabled reports whether HTTPS is configured.
func (t TLS) Enabled() bool { return t.CertFile != "" && t.KeyFile != "" }

type Config struct {
	ListenAddr  string      `yaml:"listen_addr"`
	DBPath      string      `yaml:"db_path"`
	BaseURL     string      `yaml:"base_url"`
	UploadsDir  string      `yaml:"uploads_dir"`
	TLS         TLS         `yaml:"tls"`
	Session     Session     `yaml:"session"`
	AI          AI          `yaml:"ai"`
	Worker      Worker      `yaml:"worker"`
	Fetch       Fetch       `yaml:"fetch"`
	SMTP        SMTP        `yaml:"smtp"`
	IMAP        IMAP        `yaml:"imap"`
	InitialUser InitialUser `yaml:"initial_user"`
}

// Load reads and parses the YAML config at path, applying defaults.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8090"
	}
	if cfg.Session.CookieName == "" {
		cfg.Session.CookieName = "note_aura_session"
	}
	if cfg.Session.TTLHours == 0 {
		cfg.Session.TTLHours = 720
	}
	if cfg.UploadsDir == "" {
		cfg.UploadsDir = "uploads"
	}
	if cfg.AI.OllamaURL == "" {
		cfg.AI.OllamaURL = "http://localhost:11434"
	}
	if cfg.AI.ChatModel == "" {
		cfg.AI.ChatModel = "llama3.1"
	}
	if cfg.AI.EmbedModel == "" {
		cfg.AI.EmbedModel = "nomic-embed-text"
	}
	if cfg.AI.VisionModel == "" {
		cfg.AI.VisionModel = "deepseek-ocr"
	}
	if cfg.AI.TimeoutSeconds == 0 {
		cfg.AI.TimeoutSeconds = 600 // generous: vision OCR (cold load + inference) is slow
	}
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = 2
	}
	if cfg.Fetch.HeadlessWaitMs == 0 {
		cfg.Fetch.HeadlessWaitMs = 2500
	}
	if cfg.SMTP.Host != "" && cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
	}
	if cfg.SMTP.From == "" {
		cfg.SMTP.From = cfg.SMTP.Username
	}
	if cfg.SMTP.STARTTLS == nil {
		def := true
		cfg.SMTP.STARTTLS = &def
	}
	if cfg.IMAP.TLS == nil {
		def := true
		cfg.IMAP.TLS = &def
	}
	if cfg.IMAP.Host != "" {
		if cfg.IMAP.Port == 0 {
			if *cfg.IMAP.TLS {
				cfg.IMAP.Port = 993
			} else {
				cfg.IMAP.Port = 143
			}
		}
		if cfg.IMAP.Mailbox == "" {
			cfg.IMAP.Mailbox = "INBOX"
		}
		if cfg.IMAP.Address == "" {
			cfg.IMAP.Address = cfg.IMAP.Username
		}
		if cfg.IMAP.PollSeconds == 0 {
			cfg.IMAP.PollSeconds = 60
		}
	}
	return &cfg, nil
}
