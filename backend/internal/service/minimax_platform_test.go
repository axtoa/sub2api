package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxUsesOpenAICompatibleAPIKeyRouting(t *testing.T) {
	account := &Account{
		Platform:    PlatformMiniMax,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "mm-key"},
	}

	require.True(t, account.IsOpenAICompatible())
	require.Equal(t, DefaultMiniMaxBaseURL, account.GetOpenAIBaseURL())
	require.Equal(t, "mm-key", account.GetOpenAIProtocolAPIKey())
	require.True(t, account.IsHeaderOverrideEligible())
	require.True(t, shouldForwardOpenAIResponsesViaRawChatCompletions(account))
}

func TestMiniMaxCompositeModelOwnership(t *testing.T) {
	platform, ok := DetectModelPlatform("MiniMax-M2.5")
	require.True(t, ok)
	require.Equal(t, PlatformMiniMax, platform)
	require.True(t, isConcreteRequestPlatform(PlatformMiniMax))
	require.Equal(t, PlatformMiniMax, NormalizeOpenAICompatiblePlatform(PlatformMiniMax))
}

func TestMiniMaxCountTokensUsesLocalEstimation(t *testing.T) {
	account := &Account{Platform: PlatformMiniMax, Type: AccountTypeAPIKey}
	require.True(t, shouldEstimateOpenAIInputTokensLocally(account))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	err := (&OpenAIGatewayService{}).ForwardCountTokensAsAnthropic(
		context.Background(),
		c,
		account,
		[]byte(`{"model":"MiniMax-M2.5","messages":[{"role":"user","content":"hello"}]}`),
		"",
	)

	require.NoError(t, err)
	require.Equal(t, 200, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"input_tokens"`)
}
