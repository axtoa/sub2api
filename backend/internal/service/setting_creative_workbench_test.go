package service

import (
	"context"
	"testing"
)

type creativeWorkbenchSettingRepo struct {
	data map[string]string
}

func newCreativeWorkbenchSettingRepo() *creativeWorkbenchSettingRepo {
	return &creativeWorkbenchSettingRepo{data: make(map[string]string)}
}

func (r *creativeWorkbenchSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	if value, ok := r.data[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *creativeWorkbenchSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.data[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *creativeWorkbenchSettingRepo) Set(_ context.Context, key, value string) error {
	r.data[key] = value
	return nil
}

func (r *creativeWorkbenchSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.data[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *creativeWorkbenchSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.data[key] = value
	}
	return nil
}

func (r *creativeWorkbenchSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.data))
	for key, value := range r.data {
		out[key] = value
	}
	return out, nil
}

func (r *creativeWorkbenchSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.data, key)
	return nil
}

func TestCreativeWorkbenchSettingsDefaultsAndPersistence(t *testing.T) {
	ctx := context.Background()
	svc := NewSettingService(newCreativeWorkbenchSettingRepo(), nil)

	defaults, err := svc.GetCreativeWorkbenchSettings(ctx)
	if err != nil {
		t.Fatalf("default settings: %v", err)
	}
	if !defaults.Enabled || !defaults.ImageEnabled || defaults.VideoEnabled {
		t.Fatalf("unexpected default switches: %+v", defaults)
	}
	if defaults.RetentionDays != 3 || defaults.MaxRecordsPerUser != 50 ||
		defaults.ImageMaxRunningPerUser != 10 || defaults.VideoMaxRunningPerUser != 5 {
		t.Fatalf("unexpected default limits: %+v", defaults)
	}

	input := &CreativeWorkbenchSettings{
		Enabled:                true,
		ImageEnabled:           true,
		VideoEnabled:           true,
		AutoCleanupEnabled:     true,
		RetentionDays:          7,
		MaxRecordsPerUser:      80,
		ImageMaxRunningPerUser: 12,
		VideoMaxRunningPerUser: 6,
	}
	if err := svc.SetCreativeWorkbenchSettings(ctx, input); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	saved, err := svc.GetCreativeWorkbenchSettings(ctx)
	if err != nil {
		t.Fatalf("get saved settings: %v", err)
	}
	if *saved != *input {
		t.Fatalf("settings mismatch: got %+v want %+v", saved, input)
	}
}

func TestCreativeWorkbenchSettingsRejectsOutOfRangeValues(t *testing.T) {
	svc := NewSettingService(newCreativeWorkbenchSettingRepo(), nil)
	err := svc.SetCreativeWorkbenchSettings(context.Background(), &CreativeWorkbenchSettings{
		Enabled:                true,
		ImageEnabled:           true,
		AutoCleanupEnabled:     true,
		RetentionDays:          0,
		MaxRecordsPerUser:      50,
		ImageMaxRunningPerUser: 10,
		VideoMaxRunningPerUser: 5,
	})
	if err == nil {
		t.Fatal("expected invalid retention_days to be rejected")
	}
}
