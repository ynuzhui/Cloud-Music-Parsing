package util

import (
	"net/mail"
	"regexp"
	"strings"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z\p{Han}][A-Za-z0-9_\-\p{Han}]{1,31}$`)

// IsValidUsername requires username:
// 1) starts with Chinese or English letter;
// 2) contains only Chinese/English/numbers/_/-;
// 3) total length 2-32.
func IsValidUsername(raw string) bool {
	name := strings.TrimSpace(raw)
	if name == "" {
		return false
	}
	return usernamePattern.MatchString(name)
}

func IsValidEmail(raw string) bool {
	email := strings.TrimSpace(raw)
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
