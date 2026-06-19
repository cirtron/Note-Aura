package emailin

import (
	"strings"
	"testing"
)

func TestPlusToken(t *testing.T) {
	cases := map[string]string{
		"notes+abc123@mail.example.com": "abc123",
		"plain@example.com":             "",
		"no-at-sign":                    "",
		"notes+@example.com":            "",
		"NOTES+Tok@Host":                "Tok",
	}
	for in, want := range cases {
		if got := plusToken(in); got != want {
			t.Errorf("plusToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseMessage_multipartWithAttachment(t *testing.T) {
	raw := "From: alice@example.com\r\n" +
		"To: notes+ab12cd34ef567890@mail.example.com\r\n" +
		"Subject: Hello from email\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=BOUND\r\n" +
		"\r\n" +
		"--BOUND\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"This is the body text.\r\n" +
		"--BOUND\r\n" +
		"Content-Type: application/octet-stream; name=\"note.txt\"\r\n" +
		"Content-Disposition: attachment; filename=\"note.txt\"\r\n" +
		"\r\n" +
		"attached content\r\n" +
		"--BOUND--\r\n"

	msg, tokens, _, err := parseMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	if msg.Subject != "Hello from email" {
		t.Errorf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.Text, "This is the body text.") {
		t.Errorf("text = %q", msg.Text)
	}
	if len(tokens) != 1 || tokens[0] != "ab12cd34ef567890" {
		t.Errorf("tokens = %v", tokens)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "note.txt" {
		t.Fatalf("attachments = %+v", msg.Attachments)
	}
	if string(msg.Attachments[0].Data) != "attached content" {
		t.Errorf("attachment data = %q", msg.Attachments[0].Data)
	}
}

func TestParseMessage_htmlOnly(t *testing.T) {
	raw := "To: a+tok9@h.com, other@h.com\r\n" +
		"Cc: notes+cc1@h.com\r\n" +
		"Subject: HTML mail\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Hello <b>world</b></p>\r\n"

	msg, tokens, _, err := parseMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}
	if !strings.Contains(msg.Text, "Hello") || !strings.Contains(msg.Text, "world") {
		t.Errorf("html→text = %q", msg.Text)
	}
	// Tokens collected from To then Cc; "other@h.com" has none.
	joined := strings.Join(tokens, ",")
	if !strings.Contains(joined, "tok9") || !strings.Contains(joined, "cc1") {
		t.Errorf("tokens = %v", tokens)
	}
}
