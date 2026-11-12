package app

import (
	"bytes"
	"context"
	"github.com/gi8lino/warden-webui/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRunHelp(t *testing.T) {
	t.Parallel()
	var out, errs bytes.Buffer
	require.NoError(t, Run(context.Background(), []string{"--help"}, web.Assets, "v1.0.0", &out, &errs))
	assert.Contains(t, out.String(), "warden-webui")
}
