package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	minTOTPPeriodSeconds = 15
	maxTOTPPeriodSeconds = 60
	maxTOTPSkew          = 2
)

// TOTPConfig defines one RFC 6238 profile shared by enrollment and validation.
type TOTPConfig struct {
	Issuer    string
	Period    uint
	Skew      uint
	Digits    otp.Digits
	Algorithm otp.Algorithm
}

// TOTP owns enrollment and verification policy while pquerna/otp implements
// the cryptographic protocol and otpauth URI generation.
type TOTP struct {
	config TOTPConfig
}

// Enrollment is the secret and standards-compatible otpauth URL shown once to
// the enrolling user.
type Enrollment struct {
	Secret string
	URL    string
}

// NewTOTP constructs a TOTP profile with broadly compatible RFC defaults.
func NewTOTP(config TOTPConfig) (*TOTP, error) {
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("TOTP issuer is required")
	}
	if config.Period == 0 {
		config.Period = 30
	}
	if config.Digits == 0 {
		config.Digits = otp.DigitsSix
	}
	if config.Algorithm == 0 {
		config.Algorithm = otp.AlgorithmSHA1
	}
	if config.Digits != otp.DigitsSix && config.Digits != otp.DigitsEight {
		return nil, errors.New("TOTP digits must be six or eight")
	}
	if config.Period < minTOTPPeriodSeconds || config.Period > maxTOTPPeriodSeconds {
		return nil, fmt.Errorf("TOTP period must be between %d and %d seconds", minTOTPPeriodSeconds, maxTOTPPeriodSeconds)
	}
	if config.Skew > maxTOTPSkew {
		return nil, fmt.Errorf("TOTP skew must not exceed %d periods", maxTOTPSkew)
	}
	if !supportedTOTPAlgorithm(config.Algorithm) {
		return nil, errors.New("TOTP algorithm must be SHA1, SHA256, or SHA512")
	}
	return &TOTP{config: config}, nil
}

func supportedTOTPAlgorithm(algorithm otp.Algorithm) bool {
	switch algorithm {
	case otp.AlgorithmSHA1, otp.AlgorithmSHA256, otp.AlgorithmSHA512:
		return true
	default:
		return false
	}
}

// Enroll generates a new secret and otpauth URL for an account label.
func (t *TOTP) Enroll(account string) (Enrollment, error) {
	if strings.TrimSpace(account) == "" {
		return Enrollment{}, errors.New("TOTP account name is required")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      t.config.Issuer,
		AccountName: account,
		Period:      t.config.Period,
		SecretSize:  20,
		Secret:      nil,
		Digits:      t.config.Digits,
		Algorithm:   t.config.Algorithm,
	})
	if err != nil {
		return Enrollment{}, fmt.Errorf("generate TOTP enrollment: %w", err)
	}
	return Enrollment{Secret: key.Secret(), URL: key.URL()}, nil
}

// Validate verifies one numeric code at now using the enrollment profile.
func (t *TOTP) Validate(secret, code string, now time.Time) (bool, error) {
	if len(code) != t.config.Digits.Length() {
		return false, nil
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false, nil
		}
	}
	valid, err := totp.ValidateCustom(code, secret, now, totp.ValidateOpts{
		Period: t.config.Period, Skew: t.config.Skew,
		Digits: t.config.Digits, Algorithm: t.config.Algorithm,
	})
	if err != nil {
		return false, fmt.Errorf("validate TOTP code: %w", err)
	}
	return valid, nil
}
