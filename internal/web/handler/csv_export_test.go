package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeCSVRowNeutralizesSpreadsheetFormulas(t *testing.T) {
	row := []string{"plain", "=cmd()", " +SUM(A1:A2)", "@import", "-2", "user@example.com"}
	assert.Equal(t, []string{"plain", "'=cmd()", "' +SUM(A1:A2)", "'@import", "'-2", "user@example.com"}, safeCSVRow(row))
}
