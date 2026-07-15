package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		message  string
	}{
		{name: "valid", password: "Password1!"},
		{name: "too short", password: "Pass1!", message: "at least 8"},
		{name: "too long", password: "Aa1!" + strings.Repeat("x", MaxPasswordLength), message: "at most 72"},
		{name: "uppercase", password: "password1!", message: "uppercase"},
		{name: "lowercase", password: "PASSWORD1!", message: "lowercase"},
		{name: "digit", password: "Password!", message: "digit"},
		{name: "special", password: "Password1", message: "special"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePassword(test.password)
			if test.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, test.message)
		})
	}
}
