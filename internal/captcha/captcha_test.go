package captcha

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// solve parses "a + b" and returns the integer answer as a string.
func solve(t *testing.T, prompt string) string {
	t.Helper()
	var a, b int
	if _, err := fmt.Sscanf(prompt, "%d + %d", &a, &b); err != nil {
		t.Fatalf("unparseable prompt %q: %v", prompt, err)
	}
	return strconv.Itoa(a + b)
}

func TestVerifyCorrectAnswer(t *testing.T) {
	ch, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !Verify(ch.Token, solve(t, ch.Prompt)) {
		t.Fatal("correct answer rejected")
	}
}

func TestVerifyAcceptsSurroundingSpaces(t *testing.T) {
	ch, _ := New()
	if !Verify(ch.Token, "  "+solve(t, ch.Prompt)+"  ") {
		t.Fatal("answer with spaces rejected")
	}
}

func TestVerifyWrongAnswer(t *testing.T) {
	ch, _ := New()
	wrong := solve(t, ch.Prompt) + "9" // never equal to the real answer
	if Verify(ch.Token, wrong) {
		t.Fatal("wrong answer accepted")
	}
}

func TestVerifyNonNumeric(t *testing.T) {
	ch, _ := New()
	if Verify(ch.Token, "abc") {
		t.Fatal("non-numeric answer accepted")
	}
}

func TestVerifyExpired(t *testing.T) {
	// Craft a token that expired one second ago.
	token := signToken(time.Now().Add(-time.Second).Unix(), 5)
	if Verify(token, "5") {
		t.Fatal("expired token accepted")
	}
}

func TestVerifyTampered(t *testing.T) {
	ch, _ := New()
	tampered := ch.Token + "00"
	if Verify(tampered, solve(t, ch.Prompt)) {
		t.Fatal("tampered token accepted")
	}
}

func TestVerifyMalformed(t *testing.T) {
	if Verify("not-a-token", "5") {
		t.Fatal("malformed token accepted")
	}
	if Verify("", "5") {
		t.Fatal("empty token accepted")
	}
}

func TestTokenHasNoClearTextAnswer(t *testing.T) {
	// Force a known prompt by signing directly, then ensure the answer digits
	// are not trivially the last path segment (it must be an HMAC, not the answer).
	token := signToken(time.Now().Add(time.Minute).Unix(), 7)
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("bad token shape %q", token)
	}
	if parts[1] == "7" {
		t.Fatal("answer leaked in clear text")
	}
}
