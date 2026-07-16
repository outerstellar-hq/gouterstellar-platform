package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
)

func TestRegistrationErrorMessageDoesNotExposeInternalFailures(t *testing.T) {
	message := registrationErrorMessage(errors.New("database detail"))

	assert.Equal(t, "Registration failed. Please try again.", message)
}

func TestRegistrationPolicyErrorsMapToPublicHTTPResponses(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "disabled", err: &model.RegistrationDisabledError{}, status: http.StatusForbidden},
		{name: "weak password", err: &model.WeakPasswordError{Message: "weak"}, status: http.StatusBadRequest},
		{name: "current password", err: &model.InvalidPasswordError{}, status: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handleServiceError(recorder, test.err)
			assert.Equal(t, test.status, recorder.Code)
		})
	}
}
