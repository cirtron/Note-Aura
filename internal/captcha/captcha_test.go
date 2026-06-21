package captcha

import (
	"encoding/base64"
	"image/png"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestVerifyCorrectAnswer(t *testing.T) {
	token := signToken(time.Now().Add(time.Minute).Unix(), "ABC23")
	if !Verify(token, "ABC23") {
		t.Fatal("correct answer rejected")
	}
}

func TestVerifyCaseInsensitiveAndTrimmed(t *testing.T) {
	token := signToken(time.Now().Add(time.Minute).Unix(), "ABC23")
	if !Verify(token, "  abc23 ") {
		t.Fatal("lowercase/whitespace answer rejected")
	}
}

func TestVerifyWrongAnswer(t *testing.T) {
	token := signToken(time.Now().Add(time.Minute).Unix(), "ABC23")
	if Verify(token, "XYZ99") {
		t.Fatal("wrong answer accepted")
	}
}

func TestVerifyEmptySubmission(t *testing.T) {
	token := signToken(time.Now().Add(time.Minute).Unix(), "ABC23")
	if Verify(token, "   ") {
		t.Fatal("empty submission accepted")
	}
}

func TestVerifyExpired(t *testing.T) {
	token := signToken(time.Now().Add(-time.Second).Unix(), "ABC23")
	if Verify(token, "ABC23") {
		t.Fatal("expired token accepted")
	}
}

func TestVerifyTampered(t *testing.T) {
	token := signToken(time.Now().Add(time.Minute).Unix(), "ABC23")
	if Verify(token+"00", "ABC23") {
		t.Fatal("tampered token accepted")
	}
}

func TestVerifyMalformed(t *testing.T) {
	if Verify("not-a-token", "ABC23") {
		t.Fatal("malformed token accepted")
	}
	if Verify("", "ABC23") {
		t.Fatal("empty token accepted")
	}
}

func TestRandCode(t *testing.T) {
	code, err := randCode()
	if err != nil {
		t.Fatalf("randCode: %v", err)
	}
	if len(code) != codeLen {
		t.Fatalf("code length = %d, want %d", len(code), codeLen)
	}
	for _, r := range code {
		if !strings.ContainsRune(alphabet, r) {
			t.Fatalf("code char %q not in alphabet", r)
		}
	}
}

func TestNewProducesPNGDataURI(t *testing.T) {
	ch, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(ch.Image, prefix) {
		t.Fatalf("image does not start with %q", prefix)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ch.Image, prefix))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	img, err := png.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	if img.Bounds().Dx() != imgW || img.Bounds().Dy() != imgH {
		t.Fatalf("image size = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), imgW, imgH)
	}
}

func TestTokenShapeIsHMACNotCode(t *testing.T) {
	// "<digits>.<64 lowercase hex>" — proves the code is not stored in clear text.
	ch, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9a-f]{64}$`).MatchString(ch.Token) {
		t.Fatalf("unexpected token shape: %q", ch.Token)
	}
}
