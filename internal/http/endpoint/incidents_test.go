package endpoint

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIncidentFilters(t *testing.T) {
	t.Parallel()

	t.Run("allows optional filters", func(t *testing.T) {
		t.Parallel()

		assert.True(t, validIncidentRuleFilter(""))
		assert.True(t, validIncidentKindFilter(""))
	})

	t.Run("allows named rules and both event categories", func(t *testing.T) {
		t.Parallel()

		assert.True(t, validIncidentRuleFilter("admin-audit"))
		assert.True(t, validIncidentKindFilter("user"))
		assert.True(t, validIncidentKindFilter("admin"))
	})

	t.Run("rejects oversized rule names", func(t *testing.T) {
		t.Parallel()

		assert.False(t, validIncidentRuleFilter(strings.Repeat("a", 65)))
	})

	t.Run("rejects unicode whitespace", func(t *testing.T) {
		t.Parallel()

		assert.False(t, validIncidentRuleFilter("admin\u00a0audit"))
	})

	t.Run("rejects control characters", func(t *testing.T) {
		t.Parallel()

		assert.False(t, validIncidentRuleFilter("admin\x00audit"))
	})

	t.Run("rejects unknown event categories", func(t *testing.T) {
		t.Parallel()

		assert.False(t, validIncidentKindFilter("future"))
	})
}
