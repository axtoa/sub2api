package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTokenUsageMultiplier(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{1, 1.5, 1.2345, 100} {
		require.NoError(t, ValidateTokenUsageMultiplier(value))
	}
	for _, value := range []float64{0, 0.9999, 1.00001, 1.23456, 100.0001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.Error(t, ValidateTokenUsageMultiplier(value))
	}
}

func TestGroupEffectiveTokenUsageMultiplierFallsBackToOne(t *testing.T) {
	t.Parallel()

	var nilGroup *Group
	require.Equal(t, 1.0, nilGroup.EffectiveTokenUsageMultiplier())
	require.Equal(t, 1.0, (&Group{}).EffectiveTokenUsageMultiplier())
	require.Equal(t, 1.0, (&Group{TokenUsageMultiplier: 101}).EffectiveTokenUsageMultiplier())
	require.Equal(t, 2.5, (&Group{TokenUsageMultiplier: 2.5}).EffectiveTokenUsageMultiplier())
}

func TestUsageLogApplyTokenUsageMultiplier(t *testing.T) {
	t.Parallel()

	log := &UsageLog{
		InputTokens:           3,
		OutputTokens:          2,
		CacheCreationTokens:   1,
		CacheReadTokens:       4,
		CacheCreation5mTokens: 1,
		CacheCreation1hTokens: 2,
		ImageInputTokens:      3,
		ImageOutputTokens:     1,
	}
	log.ApplyTokenUsageMultiplier(1.5)

	require.Equal(t, 1.5, log.TokenUsageMultiplier)
	require.Equal(t, 5, log.InputTokens)
	require.Equal(t, 3, log.OutputTokens)
	require.Equal(t, 2, log.CacheCreationTokens)
	require.Equal(t, 6, log.CacheReadTokens)
	require.Equal(t, 2, log.CacheCreation5mTokens)
	require.Equal(t, 3, log.CacheCreation1hTokens)
	require.Equal(t, 5, log.ImageInputTokens)
	require.Equal(t, 2, log.ImageOutputTokens)
	require.Equal(t, 3, log.ActualInputTokens)
	require.Equal(t, 2, log.ActualOutputTokens)
	require.Equal(t, 1, log.ActualCacheCreationTokens)
	require.Equal(t, 4, log.ActualCacheReadTokens)
	require.Equal(t, 1, log.ActualCacheCreation5mTokens)
	require.Equal(t, 2, log.ActualCacheCreation1hTokens)
	require.Equal(t, 3, log.ActualImageInputTokens)
	require.Equal(t, 1, log.ActualImageOutputTokens)
}

func TestUsageLogApplyTokenUsageMultiplierHandlesInvalidAndOverflow(t *testing.T) {
	t.Parallel()
	require.Equal(t, 110, scaledTokenCount(100, 1.1))
	require.Equal(t, 7, scaledTokenCount(5, 1.3))

	invalid := &UsageLog{InputTokens: 7}
	invalid.ApplyTokenUsageMultiplier(101)
	require.Equal(t, 1.0, invalid.TokenUsageMultiplier)
	require.Equal(t, 7, invalid.InputTokens)
	require.Equal(t, 7, invalid.ActualInputTokens)

	overflow := &UsageLog{InputTokens: math.MaxInt32}
	overflow.ApplyTokenUsageMultiplier(100)
	require.Equal(t, math.MaxInt32, overflow.InputTokens)
	require.Equal(t, math.MaxInt32, overflow.ActualInputTokens)
}

func TestEnsureTokenUsageSnapshotBackfillsLegacyRows(t *testing.T) {
	t.Parallel()

	log := &UsageLog{InputTokens: 11, CacheCreation5mTokens: 3}
	log.EnsureTokenUsageSnapshot()

	require.Equal(t, 1.0, log.TokenUsageMultiplier)
	require.Equal(t, 11, log.ActualInputTokens)
	require.Equal(t, 3, log.ActualCacheCreation5mTokens)
}

func TestApplyTokenUsageBillingMultiplierPreservesUpstreamCost(t *testing.T) {
	t.Parallel()

	cost := &CostBreakdown{TotalCost: 0.25, ActualCost: 0.5}
	apiKey := &APIKey{Group: &Group{TokenUsageMultiplier: 3}}

	multiplier := applyTokenUsageBillingMultiplier(cost, apiKey)

	require.Equal(t, 3.0, multiplier)
	require.InDelta(t, 0.25, cost.TotalCost, 1e-12)
	require.InDelta(t, 1.5, cost.ActualCost, 1e-12)
}
