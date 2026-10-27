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

func TestExternalPurchaseLinksAreIndependentFromOnSiteShop(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyOfficialShopEnabled: "false"}}
	s := NewSettingService(repo, &config.Config{})
	public, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultCardShopURL, public.CardShopURL)
	require.False(t, public.ExternalShopEnabled)
	require.Equal(t, DefaultExternalShopURL, public.ExternalShopURL)

	require.NoError(t, s.SetExternalShopSettings(context.Background(), true, "https://example.com/shop"))
	public, err = s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ExternalShopEnabled)
	require.Equal(t, "https://example.com/shop", public.ExternalShopURL)
	require.False(t, public.OfficialShopEnabled)

	settings, err := s.GetExternalShopSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, ExternalShopSettings{Enabled: true, URL: "https://example.com/shop"}, settings)
	require.Error(t, s.SetExternalShopSettings(context.Background(), true, "javascript:alert(1)"))
	require.Equal(t, "true", repo.values[SettingKeyExternalShopEnabled])
}

func TestShopLabelsAreIndependentAndVisibleInPublicSettings(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyOfficialShopEnabled: "false",
		SettingKeyExternalShopEnabled: "true",
		SettingKeyExternalShopURL:     "https://example.com/shop",
	}}
	s := NewSettingService(repo, &config.Config{})
	updates := 0
	s.SetOnUpdateCallback(func() { updates++ })
	labels, err := s.GetShopLabels(context.Background())
	require.NoError(t, err)
	require.Equal(t, ShopLabels{}, labels)
	_, err = s.SetShopLabel(context.Background(), "official", "  站内精选  ")
	require.NoError(t, err)
	_, err = s.SetShopLabel(context.Background(), "external", "外部商城")
	require.NoError(t, err)
	require.Equal(t, 2, updates)
	public, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "站内精选", public.OfficialShopName)
	require.Equal(t, "外部商城", public.ExternalShopName)
	require.False(t, public.OfficialShopEnabled)
	require.True(t, public.ExternalShopEnabled)
	require.Equal(t, "https://example.com/shop", public.ExternalShopURL)
	payload, err := s.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	require.Equal(t, public.OfficialShopName, payload.(*PublicSettingsInjectionPayload).OfficialShopName)
	require.Equal(t, public.ExternalShopName, payload.(*PublicSettingsInjectionPayload).ExternalShopName)

	_, err = s.SetShopLabel(context.Background(), "external", "")
	require.NoError(t, err)
	public, err = s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, public.ExternalShopName)
	require.Equal(t, "https://example.com/shop", public.ExternalShopURL)
	_, err = s.SetShopLabel(context.Background(), "official", "bad\u202e")
	require.Error(t, err)
	require.Equal(t, 3, updates)
}

func TestEmptyExternalPurchaseLinksHideOnlyTheirOwnEntries(t *testing.T) {
	s := NewSettingService(&panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyCardShopURL:         "",
		SettingKeyExternalShopEnabled: "true",
		SettingKeyExternalShopURL:     "",
	}}, &config.Config{})
	public, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, public.CardShopURL)
	require.False(t, public.ExternalShopEnabled)
	require.Empty(t, public.ExternalShopURL)
	require.True(t, public.OfficialShopEnabled)
}
