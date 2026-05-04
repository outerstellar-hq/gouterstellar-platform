package security

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordEncoder interface {
	Encode(password string) (string, error)
	Matches(password, hash string) bool
}

type bcryptEncoder struct {
	cost int
}

func NewBCryptPasswordEncoder(cost int) PasswordEncoder {
	return &bcryptEncoder{cost: cost}
}

func (e *bcryptEncoder) Encode(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), e.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (e *bcryptEncoder) Matches(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
