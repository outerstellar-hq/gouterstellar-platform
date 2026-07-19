// Package auth provides application-neutral authentication mechanics.
package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/alexedwards/argon2id"
)

const (
	defaultMinPasswordBytes = 12
	defaultMaxPasswordBytes = 1024
	maxArgonHashBytes       = 256
	maxArgonMemoryKiB       = 256 * 1024
	maxArgonIterations      = 6
	maxArgonParallelism     = 16
	maxArgonWorkKiB         = 512 * 1024
	maxArgonSaltBytes       = 64
	maxArgonKeyBytes        = 64
)

var (
	ErrPasswordTooShort = errors.New("password is too short")
	ErrPasswordTooLong  = errors.New("password is too long")
	ErrUnsafeHash       = errors.New("password hash parameters exceed verification limits")
)

// PasswordConfig controls password policy and Argon2id cost. Zero values use
// conservative defaults compatible with existing platform consumers.
type PasswordConfig struct {
	MinBytes int
	MaxBytes int
	Params   argon2id.Params
}

// Passwords hashes and verifies passwords while enforcing one policy and safe
// verification limits at every call site.
type Passwords struct {
	minBytes int
	maxBytes int
	params   argon2id.Params
}

// PasswordVerification is the complete result of checking a login password.
// NeedsRehash is true only when Matched is true and the stored hash uses a
// different safe Argon2id profile from the configured profile.
type PasswordVerification struct {
	Matched     bool
	NeedsRehash bool
}

// NewPasswords validates config before any expensive password work begins.
func NewPasswords(config PasswordConfig) (*Passwords, error) {
	if config.MinBytes == 0 {
		config.MinBytes = defaultMinPasswordBytes
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = defaultMaxPasswordBytes
	}
	if config.Params == (argon2id.Params{}) {
		config.Params = argon2id.Params{
			Memory:      64 * 1024,
			Iterations:  3,
			Parallelism: 2,
			SaltLength:  16,
			KeyLength:   32,
		}
	}
	if config.MinBytes < 1 || config.MaxBytes < config.MinBytes {
		return nil, errors.New("invalid password length policy")
	}
	if err := validateArgonParams(config.Params); err != nil {
		return nil, err
	}
	return &Passwords{minBytes: config.MinBytes, maxBytes: config.MaxBytes, params: config.Params}, nil
}

// Hash validates and hashes a password as an Argon2id PHC string.
func (p *Passwords) Hash(password string) (string, error) {
	if err := p.validatePassword(password); err != nil {
		return "", err
	}
	hash, err := argon2id.CreateHash(password, &p.params)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

// Verify performs a constant-time comparison. It rejects hostile cost
// parameters before invoking Argon2, preventing a malformed database value
// from turning login into an unbounded CPU or memory allocation.
func (p *Passwords) Verify(hash, password string) (bool, error) {
	verification, err := p.VerifyWithRehash(hash, password)
	return verification.Matched, err
}

// VerifyWithRehash verifies a login password and reports whether its stored
// hash should be replaced after successful authentication. It decodes and
// validates the stored parameters once for both decisions.
func (p *Passwords) VerifyWithRehash(hash, password string) (PasswordVerification, error) {
	if err := p.validateCandidatePassword(password); err != nil {
		return PasswordVerification{}, err
	}
	params, err := decodeHash(hash)
	if err != nil {
		return PasswordVerification{}, err
	}
	matched, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return PasswordVerification{}, fmt.Errorf("verify password: %w", err)
	}
	return PasswordVerification{
		Matched:     matched,
		NeedsRehash: matched && *params != p.params,
	}, nil
}

// NeedsRehash reports whether a valid hash should be upgraded to the current
// configured cost after the next successful login.
func (p *Passwords) NeedsRehash(hash string) (bool, error) {
	params, err := decodeHash(hash)
	if err != nil {
		return false, err
	}
	return *params != p.params, nil
}

func (p *Passwords) validatePassword(password string) error {
	if len(password) < p.minBytes {
		return fmt.Errorf("%w: minimum is %d bytes", ErrPasswordTooShort, p.minBytes)
	}
	if len(password) > p.maxBytes {
		return fmt.Errorf("%w: maximum is %d bytes", ErrPasswordTooLong, p.maxBytes)
	}
	return nil
}

func (p *Passwords) validateCandidatePassword(password string) error {
	if len(password) > p.maxBytes {
		return fmt.Errorf("%w: maximum is %d bytes", ErrPasswordTooLong, p.maxBytes)
	}
	return nil
}

func decodeHash(hash string) (*argon2id.Params, error) {
	if err := preflightHash(hash); err != nil {
		return nil, err
	}
	params, _, _, err := argon2id.DecodeHash(hash)
	if err != nil {
		return nil, fmt.Errorf("decode password hash: %w", err)
	}
	if err := validateArgonParams(*params); err != nil {
		return nil, err
	}
	return params, nil
}

func preflightHash(hash string) error {
	if len(hash) > maxArgonHashBytes {
		return fmt.Errorf("%w: encoded hash exceeds %d bytes", ErrUnsafeHash, maxArgonHashBytes)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return nil
	}
	if len(parts[4]) > base64.RawStdEncoding.EncodedLen(maxArgonSaltBytes) ||
		len(parts[5]) > base64.RawStdEncoding.EncodedLen(maxArgonKeyBytes) {
		return fmt.Errorf("%w: encoded salt or key exceeds verification limits", ErrUnsafeHash)
	}
	return nil
}

func validateArgonParams(params argon2id.Params) error {
	if params.Memory == 0 || params.Memory > maxArgonMemoryKiB ||
		params.Iterations == 0 || params.Iterations > maxArgonIterations ||
		params.Parallelism == 0 || params.Parallelism > maxArgonParallelism ||
		uint64(params.Memory)*uint64(params.Iterations) > maxArgonWorkKiB ||
		params.SaltLength < 16 || params.SaltLength > maxArgonSaltBytes ||
		params.KeyLength < 16 || params.KeyLength > maxArgonKeyBytes {
		return ErrUnsafeHash
	}
	return nil
}
