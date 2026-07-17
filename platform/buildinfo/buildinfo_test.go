package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveUnstampedBuild(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Info{
		Date:     "local",
		Number:   "dev",
		Identity: "local/dev",
	}, resolve("", "", "unpublished-commit"))
}

func TestResolveStampedBuild(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Info{
		Date:     "2026-07-16",
		Number:   "418",
		Commit:   "0123456789ab",
		Identity: "2026-07-16 #418 (0123456789ab)",
	}, resolve(" 2026-07-16 ", " 418 ", "0123456789abcdef"))
}

func TestResolvePartiallyStampedBuildIsHonest(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "local/dev", resolve("2026-07-16", "", "abc1234").Identity)
	assert.Equal(t, "local/dev", resolve("local", "418", "abc1234").Identity)
}
