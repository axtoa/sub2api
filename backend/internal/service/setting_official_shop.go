package service

import (
	"context"
	"strconv"
)

func officialShopEnabledFromSettings(settings map[string]string) bool {
	return settings[SettingKeyOfficialShopEnabled] == "true"
}

func shopHomepageEnabledFromSettings(settings map[string]string) bool {
	return settings[SettingKeyShopHomepageEnabled] == "true"
}

func (s *SettingService) GetShopHomepageEnabled(ctx context.Context) (bool, error) {
	settings, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyShopHomepageEnabled})
	if err != nil {
		return false, err
	}
	return shopHomepageEnabledFromSettings(settings), nil
}

func (s *SettingService) SetShopHomepageEnabled(ctx context.Context, enabled bool) error {
	if err := s.settingRepo.Set(ctx, SettingKeyShopHomepageEnabled, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}

// A dedicated update avoids overwriting unrelated system settings.
func (s *SettingService) SetOfficialShopEnabled(ctx context.Context, enabled bool) error {
	if err := s.settingRepo.Set(ctx, SettingKeyOfficialShopEnabled, strconv.FormatBool(enabled)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
