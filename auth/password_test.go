package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
)

func TestPasswordsRoundTripAndRehash(t *testing.T) {
	passwords, err := NewPasswords(PasswordConfig{})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := passwords.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	matched, err := passwords.Verify(hash, "correct horse battery staple")
	if err != nil || !matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	matched, err = passwords.Verify(hash, "incorrect password")
	if err != nil || matched {
		t.Fatalf("matched=%v err=%v", matched, err)
	}
	needsRehash, err := passwords.NeedsRehash(hash)
	if err != nil || needsRehash {
		t.Fatalf("needsRehash=%v err=%v", needsRehash, err)
	}
	verification, err := passwords.VerifyWithRehash(hash, "correct horse battery staple")
	if err != nil || !verification.Matched || verification.NeedsRehash {
		t.Fatalf("verification=%+v err=%v", verification, err)
	}
}

func TestPasswordsPolicyAndVerificationLimits(t *testing.T) {
	passwords, err := NewPasswords(PasswordConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := passwords.Hash("too short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short password error = %v", err)
	}
	if _, err := passwords.Hash(strings.Repeat("x", 1025)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("long password error = %v", err)
	}
	malicious := "$argon2id$v=19$m=1048577,t=1,p=1$c29tZXNhbHQxMjM0NTY$MDEyMzQ1Njc4OWFiY2RlZg"
	if _, err := passwords.Verify(malicious, "password"); !errors.Is(err, ErrUnsafeHash) {
		t.Fatalf("unsafe hash error = %v", err)
	}
	if _, err := passwords.Verify(malicious, strings.Repeat("x", 1025)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("long candidate error = %v", err)
	}
}

func TestPasswordsRejectsOversizedPHCFieldsBeforeDecode(t *testing.T) {
	passwords, err := NewPasswords(PasswordConfig{})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"hash": strings.Repeat("x", maxArgonHashBytes+1),
		"salt": "$argon2id$v=19$m=65536,t=3,p=2$" + strings.Repeat("A", 87) + "$MDEyMzQ1Njc4OWFiY2RlZg",
		"key":  "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQxMjM0NTY$" + strings.Repeat("A", 87),
	}
	for name, hash := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := passwords.Verify(hash, "password"); !errors.Is(err, ErrUnsafeHash) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestArgonVerificationWorkCeiling(t *testing.T) {
	atLimit := argon2id.Params{
		Memory: 128 * 1024, Iterations: 4, Parallelism: 2, SaltLength: 16, KeyLength: 32,
	}
	if err := validateArgonParams(atLimit); err != nil {
		t.Fatalf("exact work ceiling rejected: %v", err)
	}

	aboveLimit := atLimit
	aboveLimit.Memory++
	if err := validateArgonParams(aboveLimit); !errors.Is(err, ErrUnsafeHash) {
		t.Fatalf("above work ceiling error = %v", err)
	}
}

func TestPasswordsDetectsOldCost(t *testing.T) {
	passwords, err := NewPasswords(PasswordConfig{})
	if err != nil {
		t.Fatal(err)
	}
	old := argon2id.Params{Memory: 64 * 1024, Iterations: 2, Parallelism: 2, SaltLength: 16, KeyLength: 32}
	hash, err := argon2id.CreateHash("correct horse battery staple", &old)
	if err != nil {
		t.Fatal(err)
	}
	needsRehash, err := passwords.NeedsRehash(hash)
	if err != nil || !needsRehash {
		t.Fatalf("needsRehash=%v err=%v", needsRehash, err)
	}

	verification, err := passwords.VerifyWithRehash(hash, "correct horse battery staple")
	if err != nil || !verification.Matched || !verification.NeedsRehash {
		t.Fatalf("verification=%+v err=%v", verification, err)
	}
	verification, err = passwords.VerifyWithRehash(hash, "incorrect password")
	if err != nil || verification.Matched || verification.NeedsRehash {
		t.Fatalf("verification=%+v err=%v", verification, err)
	}
}
