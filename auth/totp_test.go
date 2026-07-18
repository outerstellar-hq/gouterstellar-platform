package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTOTPRoundTrip(t *testing.T) {
	mfa, err := NewTOTP(TOTPConfig{Issuer: "Example"})
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := mfa.Enroll("user@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Secret == "" || enrollment.URL == "" {
		t.Fatalf("enrollment = %#v", enrollment)
	}
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	code, err := totp.GenerateCode(enrollment.Secret, now)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := mfa.Validate(enrollment.Secret, code, now)
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	valid, err = mfa.Validate(enrollment.Secret, "not-a-code", now)
	if err != nil || valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
}
