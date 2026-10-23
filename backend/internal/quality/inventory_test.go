package quality

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestQualityLocalModelDiscovery(t *testing.T) {
	a := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"custom-text": "gpt-5.4", "gpt-*": "gpt-5.4", "image-test": "gpt-image-1", "another-text": "gpt-5.4"}}}
	require.Equal(t, []string{"another-text", "custom-text"}, monitorModels(a))
	a.ParentAccountID = new(int64)
	require.Empty(t, monitorModels(a))
	a.ParentAccountID = nil
	a.Platform = service.PlatformGemini
	require.Empty(t, monitorModels(a))
	a.Platform = service.PlatformAnthropic
	a.Credentials = nil
	require.NotEmpty(t, monitorModels(a))
	a.Extra = map[string]any{"synthetic_ui_test": true}
	require.Empty(t, monitorModels(a))
}
