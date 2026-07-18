// Package auth provides application-neutral authentication mechanics.
package auth

import (
	"errors"
	"fmt"

	"github.com/alexedwards/argon2id"
)

const (
	defaultMinPasswordBytes = 12
	defaultMaxPasswordBytes = 1024
	maxArgonMemoryKiB       = 1024 * 1024
	maxArgonIterations      = 10
	maxArgonParallelism     = 16
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
	params, _, _, err := argon2id.DecodeHash(hash)
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}
	if err := validateArgonParams(*params); err != nil {
		return false, err
	}
	matched, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("verify password: %w", err)
	}
	return matched, nil
}

// NeedsRehash reports whether a valid hash should be upgraded to the current
// configured cost after the next successful login.
func (p *Passwords) NeedsRehash(hash string) (bool, error) {
	params, _, _, err := argon2id.DecodeHash(hash)
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}
	if err := validateArgonParams(*params); err != nil {
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

func validateArgonParams(params argon2id.Params) error {
	if params.Memory == 0 || params.Memory > maxArgonMemoryKiB ||
		params.Iterations == 0 || params.Iterations > maxArgonIterations ||
		params.Parallelism == 0 || params.Parallelism > maxArgonParallelism ||
		params.SaltLength < 16 || params.SaltLength > 64 ||
		params.KeyLength < 16 || params.KeyLength > 64 {
		return ErrUnsafeHash
	}
	return nil
}
