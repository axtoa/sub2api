package service

import (
	"context"
	"strconv"
)

func officialShopEnabledFromSettings(settings map[string]string) bool {
	value, exists := settings[SettingKeyOfficialShopEnabled]
	return !exists || value == "true"
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
