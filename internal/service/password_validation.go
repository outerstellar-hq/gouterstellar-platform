package service

import (
	"unicode"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
)

func validatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return &model.WeakPasswordError{Message: "Password must be at least 8 characters"}
	}
	if len(password) > MaxPasswordLength {
		return &model.WeakPasswordError{Message: "Password must be at most 72 UTF-8 bytes"}
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, character := range password {
		hasUpper = hasUpper || unicode.IsUpper(character)
		hasLower = hasLower || unicode.IsLower(character)
		hasDigit = hasDigit || unicode.IsDigit(character)
		hasSpecial = hasSpecial || (!unicode.IsLetter(character) && !unicode.IsDigit(character))
	}

	switch {
	case !hasUpper:
		return &model.WeakPasswordError{Message: "Password must contain at least one uppercase letter"}
	case !hasLower:
		return &model.WeakPasswordError{Message: "Password must contain at least one lowercase letter"}
	case !hasDigit:
		return &model.WeakPasswordError{Message: "Password must contain at least one digit"}
	case !hasSpecial:
		return &model.WeakPasswordError{Message: "Password must contain at least one special character"}
	default:
		return nil
	}
}
