package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	CreativeWorkbenchRetentionDaysDefault = 3
	CreativeWorkbenchMaxRecordsDefault    = 50
	CreativeWorkbenchImageRunningDefault  = 10
	CreativeWorkbenchVideoRunningDefault  = 5

	creativeWorkbenchRetentionDaysMin     = 1
	creativeWorkbenchRetentionDaysMax     = 30
	creativeWorkbenchMaxRecordsMin        = 1
	creativeWorkbenchMaxRecordsMax        = 500
	creativeWorkbenchMaxRunningPerUserMin = 1
	creativeWorkbenchMaxRunningPerUserMax = 50
)

// CreativeWorkbenchSettings controls user-facing AI creation task limits.
// Storage credentials stay in image storage settings so the workbench does not
// grow a second object-storage configuration surface.
type CreativeWorkbenchSettings struct {
	Enabled                bool `json:"enabled"`
	ImageEnabled           bool `json:"image_enabled"`
	VideoEnabled           bool `json:"video_enabled"`
	AutoCleanupEnabled     bool `json:"auto_cleanup_enabled"`
	RetentionDays          int  `json:"retention_days"`
	MaxRecordsPerUser      int  `json:"max_records_per_user"`
	ImageMaxRunningPerUser int  `json:"image_max_running_per_user"`
	VideoMaxRunningPerUser int  `json:"video_max_running_per_user"`
}

func DefaultCreativeWorkbenchSettings() *CreativeWorkbenchSettings {
	return &CreativeWorkbenchSettings{
		Enabled:                true,
		ImageEnabled:           true,
		VideoEnabled:           false,
		AutoCleanupEnabled:     true,
		RetentionDays:          CreativeWorkbenchRetentionDaysDefault,
		MaxRecordsPerUser:      CreativeWorkbenchMaxRecordsDefault,
		ImageMaxRunningPerUser: CreativeWorkbenchImageRunningDefault,
		VideoMaxRunningPerUser: CreativeWorkbenchVideoRunningDefault,
	}
}

func creativeWorkbenchEnabledFromValue(value string) bool {
	settings := DefaultCreativeWorkbenchSettings()
	if strings.TrimSpace(value) == "" {
		return settings.Enabled
	}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		return DefaultCreativeWorkbenchSettings().Enabled
	}
	return settings.Enabled
}

func normalizeCreativeWorkbenchSettings(settings *CreativeWorkbenchSettings) {
	if settings == nil {
		return
	}
	if settings.RetentionDays < creativeWorkbenchRetentionDaysMin {
		settings.RetentionDays = CreativeWorkbenchRetentionDaysDefault
	}
	if settings.RetentionDays > creativeWorkbenchRetentionDaysMax {
		settings.RetentionDays = creativeWorkbenchRetentionDaysMax
	}
	if settings.MaxRecordsPerUser < creativeWorkbenchMaxRecordsMin {
		settings.MaxRecordsPerUser = CreativeWorkbenchMaxRecordsDefault
	}
	if settings.MaxRecordsPerUser > creativeWorkbenchMaxRecordsMax {
		settings.MaxRecordsPerUser = creativeWorkbenchMaxRecordsMax
	}
	if settings.ImageMaxRunningPerUser < creativeWorkbenchMaxRunningPerUserMin {
		settings.ImageMaxRunningPerUser = CreativeWorkbenchImageRunningDefault
	}
	if settings.ImageMaxRunningPerUser > creativeWorkbenchMaxRunningPerUserMax {
		settings.ImageMaxRunningPerUser = creativeWorkbenchMaxRunningPerUserMax
	}
	if settings.VideoMaxRunningPerUser < creativeWorkbenchMaxRunningPerUserMin {
		settings.VideoMaxRunningPerUser = CreativeWorkbenchVideoRunningDefault
	}
	if settings.VideoMaxRunningPerUser > creativeWorkbenchMaxRunningPerUserMax {
		settings.VideoMaxRunningPerUser = creativeWorkbenchMaxRunningPerUserMax
	}
}

func validateCreativeWorkbenchSettings(settings *CreativeWorkbenchSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if settings.RetentionDays < creativeWorkbenchRetentionDaysMin || settings.RetentionDays > creativeWorkbenchRetentionDaysMax {
		return fmt.Errorf("retention_days must be between %d-%d", creativeWorkbenchRetentionDaysMin, creativeWorkbenchRetentionDaysMax)
	}
	if settings.MaxRecordsPerUser < creativeWorkbenchMaxRecordsMin || settings.MaxRecordsPerUser > creativeWorkbenchMaxRecordsMax {
		return fmt.Errorf("max_records_per_user must be between %d-%d", creativeWorkbenchMaxRecordsMin, creativeWorkbenchMaxRecordsMax)
	}
	if settings.ImageMaxRunningPerUser < creativeWorkbenchMaxRunningPerUserMin || settings.ImageMaxRunningPerUser > creativeWorkbenchMaxRunningPerUserMax {
		return fmt.Errorf("image_max_running_per_user must be between %d-%d", creativeWorkbenchMaxRunningPerUserMin, creativeWorkbenchMaxRunningPerUserMax)
	}
	if settings.VideoMaxRunningPerUser < creativeWorkbenchMaxRunningPerUserMin || settings.VideoMaxRunningPerUser > creativeWorkbenchMaxRunningPerUserMax {
		return fmt.Errorf("video_max_running_per_user must be between %d-%d", creativeWorkbenchMaxRunningPerUserMin, creativeWorkbenchMaxRunningPerUserMax)
	}
	return nil
}

func (s *SettingService) GetCreativeWorkbenchSettings(ctx context.Context) (*CreativeWorkbenchSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCreativeWorkbenchSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultCreativeWorkbenchSettings(), nil
		}
		return nil, fmt.Errorf("get creative workbench settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultCreativeWorkbenchSettings(), nil
	}
	settings := DefaultCreativeWorkbenchSettings()
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		return DefaultCreativeWorkbenchSettings(), nil
	}
	normalizeCreativeWorkbenchSettings(settings)
	return settings, nil
}

func (s *SettingService) SetCreativeWorkbenchSettings(ctx context.Context, settings *CreativeWorkbenchSettings) error {
	if err := validateCreativeWorkbenchSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal creative workbench settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyCreativeWorkbenchSettings, string(data))
}
