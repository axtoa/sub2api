package service

import (
	"context"
	"errors"
	"strings"
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
		{"missing defaults closed", nil, false},
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

func TestShopHomepageSwitchDefaultsOffAndDoesNotChangeChannels(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyOfficialShopEnabled: "false",
		SettingKeyExternalShopEnabled: "true",
		SettingKeyExternalShopURL:     "https://example.com/shop",
	}}
	s := NewSettingService(repo, &config.Config{})
	public, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.ShopHomepageEnabled)
	require.False(t, public.OfficialShopEnabled)
	require.True(t, public.ExternalShopEnabled)
	require.NoError(t, s.SetShopHomepageEnabled(context.Background(), true))
	public, err = s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ShopHomepageEnabled)
	require.False(t, public.OfficialShopEnabled)
	require.True(t, public.ExternalShopEnabled)
	require.NoError(t, s.SetShopHomepageEnabled(context.Background(), false))
	public, err = s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.ShopHomepageEnabled)
	require.False(t, public.OfficialShopEnabled)
	require.True(t, public.ExternalShopEnabled)
	value, err := s.GetShopHomepageEnabled(context.Background())
	require.NoError(t, err)
	require.False(t, value)
}

func TestShopHomepageContentIsIndependentAndPublic(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyOfficialShopEnabled: "false"}}
	s := NewSettingService(repo, &config.Config{})
	content := ShopHomepageContent{
		Title: "官方订阅", Lead: "官方订阅/续费，也省下一笔。\n登录完成代充，赠送站点余额。", Cards: []ShopHomepageCard{{Title: "Plus", Summary: "适合日常使用", PriceHint: "约 ¥200/月", BonusCents: 500}},
	}
	require.NoError(t, s.SetShopHomepageContent(context.Background(), content))
	saved, err := s.GetShopHomepageContent(context.Background())
	require.NoError(t, err)
	require.Equal(t, &content, saved)
	public, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, &content, public.ShopHomepageContent)
	require.False(t, public.OfficialShopEnabled)
	payload, err := s.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	require.Equal(t, &content, payload.(*PublicSettingsInjectionPayload).ShopHomepageContent)
	require.Error(t, s.SetShopHomepageContent(context.Background(), ShopHomepageContent{Title: "", Lead: "x"}))
	require.Equal(t, &content, public.ShopHomepageContent)
}

func TestShopHomepageContentOnlyAllowsLineBreaksInLead(t *testing.T) {
	valid := ShopHomepageContent{
		Title: "ChatGPT", Lead: "Line one\nLine two",
		Cards: []ShopHomepageCard{{Title: "Plus", Summary: "Monthly plan", PriceHint: "$20", BonusCents: 100}},
	}
	require.True(t, IsValidShopHomepageContent(valid))
	for _, test := range []struct {
		name string
		edit func(*ShopHomepageContent)
	}{
		{"title newline", func(c *ShopHomepageContent) { c.Title = "Chat\nGPT" }},
		{"card title newline", func(c *ShopHomepageContent) { c.Cards[0].Title = "Plus\nPlan" }},
		{"summary newline", func(c *ShopHomepageContent) { c.Cards[0].Summary = "Monthly\nplan" }},
		{"price newline", func(c *ShopHomepageContent) { c.Cards[0].PriceHint = "$20\nmonth" }},
		{"lead tab", func(c *ShopHomepageContent) { c.Lead = "Line\tone" }},
		{"lead null", func(c *ShopHomepageContent) { c.Lead = "Line\x00one" }},
		{"lead delete", func(c *ShopHomepageContent) { c.Lead = "Line\x7fone" }},
		{"lead too long", func(c *ShopHomepageContent) { c.Lead = strings.Repeat("x", 180) + "\nx" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := valid
			content.Cards = append([]ShopHomepageCard(nil), valid.Cards...)
			test.edit(&content)
			require.False(t, IsValidShopHomepageContent(content))
		})
	}
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
	require.False(t, public.OfficialShopEnabled)
}
