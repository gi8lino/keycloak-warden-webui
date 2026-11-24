package auth

import (
	"errors"
	"testing"

	"github.com/gi8lino/warden-webui/internal/db"
	"github.com/stretchr/testify/require"
)

func TestAccountOperationError(t *testing.T) {
	t.Parallel()

	t.Run("translates duplicate identity", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, accountOperationError(db.ErrIdentityLinked), ErrIdentityLinked)
		require.NotErrorIs(t, accountOperationError(db.ErrIdentityLinked), db.ErrIdentityLinked)
	})

	t.Run("translates duplicate username", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, accountOperationError(db.ErrUsernameTaken), ErrUsernameTaken)
	})

	t.Run("translates missing administrator", func(t *testing.T) {
		t.Parallel()
		require.ErrorIs(t, accountOperationError(db.ErrNotFound), ErrAccountUnavailable)
	})

	t.Run("preserves unrelated failures", func(t *testing.T) {
		t.Parallel()
		other := errors.New("storage unavailable")
		require.ErrorIs(t, accountOperationError(other), other)
	})
}
