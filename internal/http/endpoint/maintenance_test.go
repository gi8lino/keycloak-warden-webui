package endpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaintenanceNotice(t *testing.T) {
	t.Parallel()

	t.Run("prune", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Observation cleanup completed.", maintenanceNotice("prune"))
	})

	t.Run("optimize", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Database optimization completed.", maintenanceNotice("optimize"))
	})

	t.Run("unknown operation", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, maintenanceNotice("unknown"))
	})
}
