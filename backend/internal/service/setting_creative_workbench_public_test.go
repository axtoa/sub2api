package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreativeWorkbenchEnabledFromValue(t *testing.T) {
	require.True(t, creativeWorkbenchEnabledFromValue(""))
	require.False(t, creativeWorkbenchEnabledFromValue(`{"enabled":false}`))
	require.True(t, creativeWorkbenchEnabledFromValue(`{"video_enabled":true}`))
	require.True(t, creativeWorkbenchEnabledFromValue(`not-json`))
}
