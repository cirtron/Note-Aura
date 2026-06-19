// Package emailin polls an IMAP mailbox and turns inbound email into notes. Each
// user has a secret token; mail addressed to <base>+<token>@domain is routed to
// that user (the token can't be spoofed via the From header). Disabled unless an
// IMAP host is configured.
package emailin

import (
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	_ "github.com/emersion/go-message/charset" // register non-UTF-8 charset decoders

	"note-aura/internal/config"
	"note-aura/internal/db"
	"note-aura/internal/ingest"
	"note-aura/internal/syslog"
)

// Attachment is one decoded email attachment.
type Attachment struct {
	Filename string
	Mime     string
	Data     []byte
}

// Message is a parsed inbound email ready to become a note.
type Message struct {
	Subject     string
	Text        string
	Attachments []Attachment
}

// Handler creates a note for the resolved user from a parsed message.
type Handler func(userID int64, m *Message) error

// Poller periodically imports new mail.
type Poller struct {
	cfg    config.IMAP
	db     *db.DB
	handle Handler
}

func New(cfg config.IMAP, database *db.DB, handle Handler) *Poller {
	return &Poller{cfg: cfg, db: database, handle: handle}
}

// Start runs the poll loop in a goroutine. No-op when IMAP isn't configured.
func (p *Poller) Start() {
	if p.cfg.Host == "" {
		return
	}
	interval := time.Duration(p.cfg.PollSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	syslog.Infof("email-in", "polling %s every %s", p.cfg.Host, interval)
	if p.cfg.InsecureSkipVerify {
		syslog.Warnf("email-in", "insecure_skip_verify=true — IMAP TLS certificate is not verified")
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		p.poll()
		for range ticker.C {
			p.poll()
		}
	}()
}

// poll connects, imports unseen mail, and disconnects. Errors are logged, not fatal.
func (p *Poller) poll() {
	c, err := p.connect()
	if err != nil {
		syslog.Errorf("email-in", "connect: %v", err)
		return
	}
	defer c.Logout()

	if _, err := c.Select(p.cfg.Mailbox, false); err != nil {
		syslog.Errorf("email-in", "select %q: %v", p.cfg.Mailbox, err)
		return
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(criteria)
	if err != nil {
		syslog.Errorf("email-in", "search: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)
	section := &imap.BodySectionName{}
	messages := make(chan *imap.Message, 16)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	}()

	seen := new(imap.SeqSet)
	del := new(imap.SeqSet)
	for msg := range messages {
		body := msg.GetBody(section)
		if body == nil {
			seen.AddNum(msg.SeqNum)
			continue
		}
		imported := p.importMessage(body)
		if imported && p.cfg.DeleteProcessed {
			del.AddNum(msg.SeqNum)
		} else {
			seen.AddNum(msg.SeqNum)
		}
	}
	if err := <-done; err != nil {
		syslog.Errorf("email-in", "fetch: %v", err)
	}

	if !seen.Empty() {
		c.Store(seen, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.SeenFlag}, nil)
	}
	if !del.Empty() {
		c.Store(del, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
		c.Expunge(nil)
	}
}

func (p *Poller) connect() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	tlsCfg := &tls.Config{
		ServerName:         p.cfg.Host,
		InsecureSkipVerify: p.cfg.InsecureSkipVerify, //nolint:gosec // opt-in for self-signed / AV-intercepted mail servers
	}
	var (
		c   *client.Client
		err error
	)
	if p.cfg.TLS == nil || *p.cfg.TLS {
		c, err = client.DialTLS(addr, tlsCfg)
	} else {
		c, err = client.Dial(addr)
		if err == nil {
			err = c.StartTLS(tlsCfg)
		}
	}
	if err != nil {
		return nil, err
	}
	if err := c.Login(p.cfg.Username, p.cfg.Password); err != nil {
		c.Logout()
		return nil, err
	}
	return c, nil
}

// importMessage parses one raw message, resolves the user from its plus-token,
// and calls the handler. Returns true when a note was created.
func (p *Poller) importMessage(r io.Reader) bool {
	msg, tokens, addrs, err := parseMessage(r)
	if err != nil {
		syslog.Errorf("email-in", "parse: %v", err)
		return false
	}
	for _, tok := range tokens {
		u, err := p.db.GetUserByEmailToken(tok)
		if err != nil || u.Suspended {
			continue
		}
		if err := p.handle(u.ID, msg); err != nil {
			syslog.Errorf("email-in", "handle for %s: %v", u.Email, err)
			return false
		}
		syslog.Infof("email-in", "imported mail for %s (subject %q, %d attachment(s))",
			u.Email, msg.Subject, len(msg.Attachments))
		return true
	}
	syslog.Warnf("email-in", "no matching user token (subject %q) — skipped. recipients=%v tokens=%v",
		msg.Subject, addrs, tokens)
	return false
}

// recipientHeaders are scanned for plus-addressed tokens, most-specific first.
var recipientHeaders = []string{"Delivered-To", "X-Original-To", "X-Forwarded-To", "To", "Cc"}

// parseMessage extracts the subject, a text body, attachments, the plus-address
// tokens found in the recipient headers, and (for diagnostics) the recipient
// addresses scanned.
func parseMessage(r io.Reader) (*Message, []string, []string, error) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, nil, nil, err
	}
	out := &Message{}
	out.Subject, _ = mr.Header.Subject()

	tokens, addrs := collectTokens(mr.Header)

	var plain, html strings.Builder
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // tolerate a malformed part; use what we have
		}
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			data, _ := io.ReadAll(part.Body)
			if strings.EqualFold(ct, "text/html") {
				html.Write(data)
			} else {
				plain.Write(data)
			}
		case *mail.AttachmentHeader:
			name, _ := h.Filename()
			ct, _, _ := h.ContentType()
			data, _ := io.ReadAll(part.Body)
			if name == "" || len(data) == 0 {
				continue
			}
			out.Attachments = append(out.Attachments, Attachment{Filename: name, Mime: ct, Data: data})
		}
	}

	text := strings.TrimSpace(plain.String())
	if text == "" && html.Len() > 0 {
		text = strings.TrimSpace(ingest.HTMLToText(html.String()))
	}
	out.Text = text
	return out, tokens, addrs, nil
}

// collectTokens returns the plus-address tokens found across recipient headers,
// plus the recipient addresses scanned (for diagnostics).
func collectTokens(h mail.Header) (toks, addrs []string) {
	seen := map[string]bool{}
	for _, name := range recipientHeaders {
		list, err := h.AddressList(name)
		if err != nil {
			continue
		}
		for _, addr := range list {
			addrs = append(addrs, addr.Address)
			if tok := plusToken(addr.Address); tok != "" && !seen[tok] {
				seen[tok] = true
				toks = append(toks, tok)
			}
		}
	}
	return toks, addrs
}

// plusToken returns the tag from a plus-address local part (foo+TOKEN@host → TOKEN),
// or "" when there's no tag.
func plusToken(address string) string {
	at := strings.LastIndex(address, "@")
	if at <= 0 {
		return ""
	}
	local := address[:at]
	plus := strings.Index(local, "+")
	if plus < 0 || plus == len(local)-1 {
		return ""
	}
	return strings.TrimSpace(local[plus+1:])
}
