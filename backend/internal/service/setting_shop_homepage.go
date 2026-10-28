package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

type ShopHomepageCard struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	PriceHint  string `json:"price_hint"`
	BonusCents int64  `json:"bonus_cents"`
}

type ShopHomepageContent struct {
	Title string             `json:"title"`
	Lead  string             `json:"lead"`
	Cards []ShopHomepageCard `json:"cards"`
}

func IsValidShopHomepageContent(value ShopHomepageContent) bool {
	if len(value.Cards) > 6 || !validShopHomepageText(value.Title, 60, true) || !validShopHomepageText(value.Lead, 180, true) {
		return false
	}
	for _, card := range value.Cards {
		if !validShopHomepageText(card.Title, 60, true) || !validShopHomepageText(card.Summary, 140, true) ||
			!validShopHomepageText(card.PriceHint, 40, false) || card.BonusCents < 0 || card.BonusCents > 1000000 {
			return false
		}
	}
	return true
}

func validShopHomepageText(value string, limit int, required bool) bool {
	value = strings.TrimSpace(value)
	if (required && value == "") || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func normalizeShopHomepageContent(value ShopHomepageContent) ShopHomepageContent {
	value.Title = strings.TrimSpace(value.Title)
	value.Lead = strings.TrimSpace(value.Lead)
	if value.Cards == nil {
		value.Cards = []ShopHomepageCard{}
	}
	for i := range value.Cards {
		value.Cards[i].Title = strings.TrimSpace(value.Cards[i].Title)
		value.Cards[i].Summary = strings.TrimSpace(value.Cards[i].Summary)
		value.Cards[i].PriceHint = strings.TrimSpace(value.Cards[i].PriceHint)
	}
	return value
}

func (s *SettingService) GetShopHomepageContent(ctx context.Context) (*ShopHomepageContent, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyShopHomepageContent})
	if err != nil {
		return nil, err
	}
	raw := values[SettingKeyShopHomepageContent]
	if raw == "" {
		return nil, nil
	}
	var content ShopHomepageContent
	if json.Unmarshal([]byte(raw), &content) != nil || !IsValidShopHomepageContent(content) {
		return nil, nil
	}
	content = normalizeShopHomepageContent(content)
	return &content, nil
}

func (s *SettingService) SetShopHomepageContent(ctx context.Context, content ShopHomepageContent) error {
	if !IsValidShopHomepageContent(content) {
		return errors.New("invalid shop homepage content")
	}
	content = normalizeShopHomepageContent(content)
	raw, err := json.Marshal(content)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyShopHomepageContent, string(raw)); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
