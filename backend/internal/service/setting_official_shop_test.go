package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type officialShopSettingRepo struct {
	SettingRepository
	setErr error
}

func (r *officialShopSettingRepo) Set(ctx context.Context, key, value string) error {
	if r.setErr != nil {
		return r.setErr
	}
	return r.SettingRepository.Set(ctx, key, value)
}

func TestOfficialShopPublicSettingsAndInjection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		want   bool
	}{
		{"missing preserves enabled state", nil, true},
		{"enabled", map[string]string{SettingKeyOfficialShopEnabled: "true"}, true},
		{"disabled", map[string]string{SettingKeyOfficialShopEnabled: "false"}, false},
		{"invalid fails closed", map[string]string{SettingKeyOfficialShopEnabled: "invalid"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSettingService(&panelRateLimitSettingRepo{values: tc.values}, &config.Config{})
			settings, err := s.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.want, settings.OfficialShopEnabled)
			payload, err := s.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			require.Equal(t, tc.want, payload.(*PublicSettingsInjectionPayload).OfficialShopEnabled)
		})
	}
}

func TestOfficialShopSwitchPersistsOnlyItsOwnSettingAndInvalidatesCache(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingPaymentEnabled: "true", SettingKeySiteName: "Best AI"}}
	s := NewSettingService(repo, &config.Config{})
	updates := 0
	s.SetOnUpdateCallback(func() { updates++ })
	for _, enabled := range []bool{false, true, false} {
		require.NoError(t, s.SetOfficialShopEnabled(context.Background(), enabled))
		// A fresh service instance reads the persisted value.
		settings, err := NewSettingService(repo, &config.Config{}).GetPublicSettings(context.Background())
		require.NoError(t, err)
		require.Equal(t, enabled, settings.OfficialShopEnabled)
		require.True(t, settings.PaymentEnabled)
		require.Equal(t, "Best AI", settings.SiteName)
	}
	require.Equal(t, 3, updates)
}

func TestOfficialShopFailedSaveDoesNotInvalidateCacheOrChangeState(t *testing.T) {
	repo := &officialShopSettingRepo{
		SettingRepository: &panelRateLimitSettingRepo{values: map[string]string{SettingKeyOfficialShopEnabled: "true"}},
		setErr:            errors.New("database unavailable"),
	}
	s := NewSettingService(repo, &config.Config{})
	updates := 0
	s.SetOnUpdateCallback(func() { updates++ })
	require.ErrorIs(t, s.SetOfficialShopEnabled(context.Background(), false), repo.setErr)
	require.Zero(t, updates)
	value, err := repo.GetValue(context.Background(), SettingKeyOfficialShopEnabled)
	require.NoError(t, err)
	require.Equal(t, "true", value)
}
