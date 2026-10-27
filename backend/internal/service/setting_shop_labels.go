package service

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ShopLabels struct {
	OfficialName string `json:"official_name"`
	ExternalName string `json:"external_name"`
}

func normalizeShopLabel(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 24 {
		return "", false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", false
		}
	}
	return value, true
}

func IsValidShopLabel(value string) bool {
	_, valid := normalizeShopLabel(value)
	return valid
}

func shopLabelsFromValues(values map[string]string) ShopLabels {
	official, validOfficial := normalizeShopLabel(values[SettingKeyOfficialShopName])
	external, validExternal := normalizeShopLabel(values[SettingKeyExternalShopName])
	if !validOfficial {
		official = ""
	}
	if !validExternal {
		external = ""
	}
	return ShopLabels{OfficialName: official, ExternalName: external}
}

func (s *SettingService) GetShopLabels(ctx context.Context) (ShopLabels, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyOfficialShopName, SettingKeyExternalShopName})
	if err != nil {
		return ShopLabels{}, err
	}
	return shopLabelsFromValues(values), nil
}

func (s *SettingService) SetShopLabel(ctx context.Context, channel, name string) (string, error) {
	var key string
	switch channel {
	case "official":
		key = SettingKeyOfficialShopName
	case "external":
		key = SettingKeyExternalShopName
	default:
		return "", errors.New("unknown shop channel")
	}
	name, valid := normalizeShopLabel(name)
	if !valid {
		return "", errors.New("shop label must be at most 24 characters without control characters")
	}
	if err := s.settingRepo.Set(ctx, key, name); err != nil {
		return "", err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return name, nil
}
