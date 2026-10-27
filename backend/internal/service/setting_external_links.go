package service

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	DefaultCardShopURL     = "https://catfk.com/shop/DFMROLGS"
	DefaultExternalShopURL = "https://wzyp.cn/shop/best-ai"
)

func externalLinkSetting(settings map[string]string, key, fallback string) string {
	value, exists := settings[key]
	if !exists {
		return fallback
	}
	return strings.TrimSpace(value)
}

func IsValidExternalPurchaseURL(value string) bool {
	if len(value) > 2000 || value != strings.TrimSpace(value) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil
}

type ExternalShopSettings struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

func externalShopSettingsFromValues(values map[string]string) ExternalShopSettings {
	link := externalLinkSetting(values, SettingKeyExternalShopURL, DefaultExternalShopURL)
	if !IsValidExternalPurchaseURL(link) {
		return ExternalShopSettings{}
	}
	return ExternalShopSettings{Enabled: values[SettingKeyExternalShopEnabled] == "true", URL: link}
}

func (s *SettingService) GetExternalShopSettings(ctx context.Context) (ExternalShopSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyExternalShopEnabled, SettingKeyExternalShopURL})
	if err != nil {
		return ExternalShopSettings{}, err
	}
	return externalShopSettingsFromValues(values), nil
}

func (s *SettingService) SetExternalShopSettings(ctx context.Context, enabled bool, link string) error {
	link = strings.TrimSpace(link)
	if (link != "" && !IsValidExternalPurchaseURL(link)) || (enabled && link == "") {
		return errors.New("external shop URL must be an HTTPS address when enabled")
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyExternalShopEnabled: strconv.FormatBool(enabled),
		SettingKeyExternalShopURL:     link,
	}); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
