// Package mailer sends plain-text email over SMTP for calendar reminders. It is
// a no-op (Enabled() == false) when no SMTP host is configured.
package mailer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/smtp"
	"strings"
	"time"

	"note-aura/internal/syslog"
)

type Mailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS bool // upgrade a plain connection via STARTTLS (ignored for port 465)
}

func New(host string, port int, username, password, from string, startTLS bool) *Mailer {
	return &Mailer{Host: host, Port: port, Username: username, Password: password, From: from, StartTLS: startTLS}
}

// Enabled reports whether sending is configured.
func (m *Mailer) Enabled() bool { return m != nil && m.Host != "" }

// Send delivers a plain-text message. Port 465 uses implicit TLS. Other ports
// use a plain connection, upgraded with STARTTLS when StartTLS is enabled and
// the server supports it.
func (m *Mailer) Send(to, subject, body string) (err error) {
	if !m.Enabled() {
		return fmt.Errorf("smtp not configured")
	}
	// Record the send outcome to the admin system log.
	defer func() {
		if err != nil {
			syslog.Errorf("mail", "send to %s (%q) failed: %v", to, subject, err)
		} else {
			syslog.Infof("mail", "sent to %s: %q", to, subject)
		}
	}()
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	msg := m.build(to, subject, body)

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	c, err := m.dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	// Opportunistic STARTTLS on plain connections when enabled.
	if m.Port != 465 && m.StartTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
				return err
			}
		}
	}
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(m.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// dial opens an SMTP client: implicit TLS for port 465, otherwise plain.
func (m *Mailer) dial(addr string) (*smtp.Client, error) {
	if m.Port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.Host})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, m.Host)
	}
	return smtp.Dial(addr)
}

func (m *Mailer) build(to, subject, body string) []byte {
	// Encode body as quoted-printable so non-ASCII content is safe over any
	// SMTP relay without requiring the SMTPUTF8 extension (RFC 6531).
	var qpBuf bytes.Buffer
	qpw := quotedprintable.NewWriter(&qpBuf)
	_, _ = qpw.Write([]byte(body))
	_ = qpw.Close()

	var b strings.Builder
	b.WriteString("From: " + m.From + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	// RFC 2047 encoded-word keeps the Subject header ASCII-safe.
	b.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("\r\n")
	b.WriteString(qpBuf.String())
	return []byte(b.String())
}
