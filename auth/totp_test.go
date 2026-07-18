package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp"
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

func TestTOTPSupportedCustomProfiles(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	for _, algorithm := range []otp.Algorithm{otp.AlgorithmSHA1, otp.AlgorithmSHA256, otp.AlgorithmSHA512} {
		t.Run(algorithm.String(), func(t *testing.T) {
			config := TOTPConfig{
				Issuer: "Example", Period: 60, Skew: 1, Digits: otp.DigitsEight, Algorithm: algorithm,
			}
			mfa, err := NewTOTP(config)
			if err != nil {
				t.Fatal(err)
			}
			enrollment, err := mfa.Enroll("user@example.test")
			if err != nil {
				t.Fatal(err)
			}
			code, err := totp.GenerateCodeCustom(enrollment.Secret, now.Add(-time.Minute), totp.ValidateOpts{
				Period: config.Period, Digits: config.Digits, Algorithm: config.Algorithm,
			})
			if err != nil {
				t.Fatal(err)
			}
			valid, err := mfa.Validate(enrollment.Secret, code, now)
			if err != nil || !valid {
				t.Fatalf("valid=%v err=%v", valid, err)
			}
		})
	}
}

func TestTOTPRejectsUnsafeProfiles(t *testing.T) {
	tests := map[string]TOTPConfig{
		"period below minimum": {Issuer: "Example", Period: minTOTPPeriodSeconds - 1},
		"period above maximum": {Issuer: "Example", Period: maxTOTPPeriodSeconds + 1},
		"excessive skew":       {Issuer: "Example", Skew: maxTOTPSkew + 1},
		"MD5":                  {Issuer: "Example", Algorithm: otp.AlgorithmMD5},
		"unknown algorithm":    {Issuer: "Example", Algorithm: otp.Algorithm(99)},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewTOTP(config); err == nil {
				t.Fatal("expected profile validation error")
			}
		})
	}
}
