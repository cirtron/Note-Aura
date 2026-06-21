// Package captcha provides a dependency-free, stateless math challenge for the
// public auth forms. The answer is carried only inside an HMAC-signed token
// (never in clear text), so a script must actually solve the sum.
package captcha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ttl bounds how long a challenge stays valid.
const ttl = 10 * time.Minute

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

// Challenge is a math question to show the user plus its signed token.
type Challenge struct {
	Prompt string // e.g. "7 + 4"
	Token  string // "<expUnix>.<hexHMAC>"
}

// New returns a fresh addition challenge with two operands in [1,9].
func New() (Challenge, error) {
	a, err := randDigit()
	if err != nil {
		return Challenge{}, err
	}
	b, err := randDigit()
	if err != nil {
		return Challenge{}, err
	}
	exp := time.Now().Add(ttl).Unix()
	return Challenge{
		Prompt: fmt.Sprintf("%d + %d", a, b),
		Token:  signToken(exp, a+b),
	}, nil
}

// Verify reports whether submitted is the right answer for an unexpired,
// untampered token. submitted is trimmed and parsed as a base-10 integer.
func Verify(token, submitted string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(submitted))
	if err != nil {
		return false
	}
	exp, ok := tokenExpiry(token)
	if !ok || time.Now().Unix() > exp {
		return false
	}
	expected := signToken(exp, n)
	return hmac.Equal([]byte(expected), []byte(token))
}

// signToken builds "<exp>.<hex HMAC of exp|answer>".
func signToken(exp int64, answer int) string {
	payload := fmt.Sprintf("%d|%d", exp, answer)
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

// randDigit returns a uniform-ish integer in [1,9].
func randDigit() (int, error) {
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	return int(b[0]%9) + 1, nil
}
