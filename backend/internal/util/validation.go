package util

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

var englishUsernamePattern = regexp.MustCompile(`^[A-Za-z]{4,}$`)

// IsValidUsername accepts either:
// 1) at least 4 English letters; or
// 2) at least 2 Chinese (Han) characters.
func IsValidUsername(raw string) bool {
	name := strings.TrimSpace(raw)
	if name == "" {
		return false
	}
	if englishUsernamePattern.MatchString(name) {
		return true
	}

	count := 0
	for _, r := range name {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
		count++
	}
	return count >= 2
}

func IsValidEmail(raw string) bool {
	email := strings.TrimSpace(raw)
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
