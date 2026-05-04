package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBCryptPasswordEncoder(t *testing.T) {
	encoder := NewBCryptPasswordEncoder(4)

	encoded, err := encoder.Encode("password123")
	assert.NoError(t, err)
	assert.NotEqual(t, "password123", encoded)
	assert.True(t, encoder.Matches("password123", encoded))
	assert.False(t, encoder.Matches("wrongpassword", encoded))

	encoded2, err := encoder.Encode("password123")
	assert.NoError(t, err)
	assert.NotEqual(t, encoded, encoded2, "each encoding should produce a different hash")
	assert.True(t, encoder.Matches("password123", encoded2))
}
