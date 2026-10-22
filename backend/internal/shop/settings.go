package shop

import (
	"context"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const enabledSQL = `SELECT COALESCE((SELECT value = 'true' FROM settings WHERE key = $1), true)`
const enabledForShareSQL = `SELECT COALESCE((SELECT value = 'true' FROM settings WHERE key = $1 FOR SHARE), true)`

func (s *Store) Enabled(ctx context.Context) (bool, error) {
	var enabled bool
	err := s.DB.QueryRowContext(ctx, enabledSQL, service.SettingKeyOfficialShopEnabled).Scan(&enabled)
	return enabled, err
}

func shopDisabled() error {
	return appErrors.Forbidden("SHOP_DISABLED", "官方代购暂未开放，已有订单仍可查看和处理")
}
