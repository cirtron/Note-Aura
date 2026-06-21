// Package captcha provides a dependency-light, stateless image challenge for the
// public auth forms. A random code is rendered into a distorted PNG; the code
// itself is carried only inside an HMAC-signed token (never in clear text), so a
// script must actually read the image to answer.
package captcha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// ttl bounds how long a challenge stays valid.
const ttl = 10 * time.Minute

// codeLen is the number of characters in a challenge.
const codeLen = 5

// alphabet is the set of challenge characters, excluding visually ambiguous
// glyphs (O/0, I/1, L) so users aren't tripped up by the distortion.
const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// secret signs tokens. It is random per process: a restart simply invalidates
// in-flight challenges, which only makes a user re-answer a fresh one.
var secret = mustSecret()

func mustSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("captcha: cannot read random secret: " + err.Error())
	}
	return b
}

// Challenge is a rendered image to show the user plus its signed token.
type Challenge struct {
	Image string // "data:image/png;base64,..." PNG of the code
	Token string // "<expUnix>.<hexHMAC>"
}

// New returns a fresh challenge: a random code rendered to a distorted PNG and a
// signed token binding that code to an expiry.
func New() (Challenge, error) {
	code, err := randCode()
	if err != nil {
		return Challenge{}, err
	}
	pngBytes, err := renderPNG(code)
	if err != nil {
		return Challenge{}, err
	}
	exp := time.Now().Add(ttl).Unix()
	return Challenge{
		Image: "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes),
		Token: signToken(exp, code),
	}, nil
}

// Verify reports whether submitted matches the code for an unexpired, untampered
// token. Matching is case-insensitive and ignores surrounding whitespace.
func Verify(token, submitted string) bool {
	code := normalize(submitted)
	if code == "" {
		return false
	}
	exp, ok := tokenExpiry(token)
	if !ok || time.Now().Unix() > exp {
		return false
	}
	expected := signToken(exp, code)
	return hmac.Equal([]byte(expected), []byte(token))
}

// normalize canonicalizes a submission for comparison.
func normalize(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// signToken builds "<exp>.<hex HMAC of exp|code>".
func signToken(exp int64, code string) string {
	payload := fmt.Sprintf("%d|%s", exp, code)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return fmt.Sprintf("%d.%s", exp, hex.EncodeToString(mac.Sum(nil)))
}

// tokenExpiry pulls the exp field out of a token without trusting the MAC yet.
func tokenExpiry(token string) (int64, bool) {
	dot := strings.IndexByte(token, '.')
	if dot <= 0 {
		return 0, false
	}
	exp, err := strconv.ParseInt(token[:dot], 10, 64)
	if err != nil {
		return 0, false
	}
	return exp, true
}

// randCode returns a cryptographically random codeLen-character code.
func randCode() (string, error) {
	b := make([]byte, codeLen)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
