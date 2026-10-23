package quality

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualityManualConfigurationChanges(t *testing.T) {
	cfg := DefaultSettings()
	original := Target{Enabled: true, AccountEnabled: true, Version: 1}
	changed := original
	changed.Enabled, changed.AccountEnabled, changed.Version = false, false, 2
	settings := cfg
	settings.AutoEnabled = false
	settings.Version++
	settings.AvailabilityEnabled, settings.QualityEnabled, settings.CacheEnabled = false, false, false
	settings.AvailabilityIntervalMinutes++
	settings.QualityIntervalMinutes++
	settings.CacheIntervalMinutes++
	settings.RetryCount++
	settings.RetryIntervalMinutes++
	settings.DailyRequests++
	settings.DailyTokens++
	manual := Run{Manual: true, Kind: "quality"}
	require.False(t, configurationChanged(changed, settings, original, cfg, cfg.Profile, manual))
	require.True(t, configurationChanged(changed, settings, original, cfg, cfg.Profile, Run{Kind: "quality"}))
	for _, tc := range []struct {
		name string
		edit func(*Profile)
	}{
		{"questions", func(p *Profile) { p.Questions++ }},
		{"grading", func(p *Profile) { p.PassPercent++ }},
		{"output", func(p *Profile) { p.MaxOutputTokens++ }},
		{"reasoning", func(p *Profile) { p.ReasoningEffort = "high" }},
		{"cache_prefix", func(p *Profile) { p.CachePrefixKB++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated := settings
			tc.edit(&updated.Profile)
			require.True(t, configurationChanged(changed, updated, original, cfg, cfg.Profile, manual))
		})
	}
	changed.Overrides = &cfg.Profile
	require.False(t, configurationChanged(changed, settings, original, cfg, cfg.Profile, manual))
}

func TestQualityAutomaticConfigurationChanges(t *testing.T) {
	cfg := DefaultSettings()
	original := Target{Enabled: true, AccountEnabled: true, Version: 1}
	job := Run{Kind: "quality"}
	require.False(t, configurationChanged(original, cfg, original, cfg, cfg.Profile, job))
	for _, tc := range []struct {
		name string
		edit func(*Target, *Settings)
	}{
		{"target_off", func(t *Target, _ *Settings) { t.Enabled = false }},
		{"account_off", func(t *Target, _ *Settings) { t.AccountEnabled = false }},
		{"global_off", func(_ *Target, s *Settings) { s.AutoEnabled = false }},
		{"kind_off", func(_ *Target, s *Settings) { s.QualityEnabled = false }},
		{"target_version", func(t *Target, _ *Settings) { t.Version++ }},
		{"settings_version", func(_ *Target, s *Settings) { s.Version++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, settings := original, cfg
			tc.edit(&target, &settings)
			require.True(t, configurationChanged(target, settings, original, cfg, cfg.Profile, job))
		})
	}
}
