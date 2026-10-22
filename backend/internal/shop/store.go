package shop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type Store struct{ DB *sql.DB }

func (s *Store) Products(ctx context.Context, admin bool) ([]Product, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT payload, version FROM shop_products WHERE $1 OR (payload->>'active')::boolean ORDER BY (payload->>'sort')::integer, id`, admin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Product, 0)
	for rows.Next() {
		var raw []byte
		var p Product
		var version int
		if err := rows.Scan(&raw, &version); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		p.Version = version
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Store) SaveProduct(ctx context.Context, p Product) (*Product, error) {
	if err := validateProduct(&p); err != nil {
		return nil, err
	}
	previous := p.Version
	if p.ID == "" {
		p.ID, p.Version = uuid.NewString(), 1
	} else {
		if _, err := uuid.Parse(p.ID); err != nil || previous < 1 {
			return nil, invalid("商品编号或版本无效")
		}
		p.Version++
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	if previous == 0 {
		_, err = s.DB.ExecContext(ctx, `INSERT INTO shop_products(id,payload,version) VALUES($1,$2,1)`, p.ID, string(raw))
	} else {
		var result sql.Result
		result, err = s.DB.ExecContext(ctx, `UPDATE shop_products SET payload=$1,version=$2,updated_at=NOW() WHERE id=$3 AND version=$4`, string(raw), p.Version, p.ID, previous)
		if err == nil {
			n, countErr := result.RowsAffected()
			if countErr != nil {
				return nil, countErr
			}
			if n != 1 {
				return nil, conflict("商品已被修改，请刷新后重试")
			}
		}
	}
	return &p, err
}

func (s *Store) Create(ctx context.Context, userID int64, req CreateOrder) (*Order, error) {
	if _, err := uuid.Parse(req.ProductID); err != nil {
		return nil, invalid("请选择有效商品")
	}
	if _, err := uuid.Parse(req.RequestKey); err != nil {
		return nil, invalid("请求编号无效，请重新提交")
	}
	if !req.Accepted || req.ProductVersion < 1 || len(req.Note) > 3000 {
		return nil, invalid("请核对订单并同意购买须知")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serialize only this user's shop submissions, without locking gateway billing rows.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("subscription-shop:%d", userID)); err != nil {
		return nil, err
	}
	var applicant string
	if err = tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, userID).Scan(&applicant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.Forbidden("SHOP_USER_INACTIVE", "账号不可用")
		}
		return nil, err
	}
	if strings.TrimSpace(req.TargetEmail) == "" {
		req.TargetEmail = applicant
	}
	if strings.TrimSpace(req.ContactEmail) == "" {
		req.ContactEmail = applicant
	}
	if req.TargetEmail, err = normalizedEmail(req.TargetEmail); err != nil {
		return nil, err
	}
	if req.ContactEmail, err = normalizedEmail(req.ContactEmail); err != nil {
		return nil, err
	}
	req.Note = strings.TrimSpace(req.Note)
	hash := requestHash(req)
	var raw []byte
	var oldHash string
	var version int
	err = tx.QueryRowContext(ctx, `SELECT payload,request_hash,version FROM shop_orders WHERE user_id=$1 AND request_key=$2`, userID, req.RequestKey).Scan(&raw, &oldHash, &version)
	if err == nil {
		if hash != oldHash {
			return nil, conflict("同一请求编号不能用于不同订单")
		}
		var order Order
		if err = json.Unmarshal(raw, &order); err != nil {
			return nil, err
		}
		order.Version = version
		return &order, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// Hold the switch row until commit, so closing waits for in-flight new orders.
	var enabled bool
	if err = tx.QueryRowContext(ctx, enabledForShareSQL, service.SettingKeyOfficialShopEnabled).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, shopDisabled()
	}
	var p Product
	if err = tx.QueryRowContext(ctx, `SELECT payload,version FROM shop_products WHERE id=$1 FOR SHARE`, req.ProductID).Scan(&raw, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, appErrors.NotFound("SHOP_PRODUCT_NOT_FOUND", "商品不存在")
		}
		return nil, err
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	p.Version = version
	if version != req.ProductVersion {
		return nil, conflict("商品信息已调整，请刷新后重新核对")
	}
	if !p.Active || p.PriceCents == nil || *p.PriceCents <= 0 || !validLink(p.PaymentURL) {
		return nil, conflict("该商品暂未开售")
	}
	var duplicate string
	err = tx.QueryRowContext(ctx, `SELECT id FROM shop_orders WHERE user_id=$1 AND product_id=$2 AND payload->>'target_email'=$3 AND payload->>'payment_status' IN ('PENDING','PAID') AND payload->>'fulfillment_status' NOT IN ('COMPLETED','CLOSED') LIMIT 1`, userID, p.ID, req.TargetEmail).Scan(&duplicate)
	if err == nil {
		return nil, conflict("该账号已有未完成的同款订单，请在我的代购订单中继续处理")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	now := time.Now().UTC()
	order := Order{
		ID: "SO" + strings.ReplaceAll(uuid.NewString(), "-", ""), UserID: userID,
		ApplicantEmail: applicant, TargetEmail: req.TargetEmail, ContactEmail: req.ContactEmail,
		Product: p, AmountCents: *p.PriceCents, PaymentStatus: "PENDING", FulfillmentStatus: "WAITING",
		Note: req.Note, CreatedAt: now, Version: 1,
		Events: []Event{{ActorID: userID, Action: "created", At: now}},
	}
	raw, err = json.Marshal(order)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shop_orders(id,user_id,product_id,request_key,request_hash,payload,version,created_at) VALUES($1,$2,$3,$4,$5,$6,1,$7)`, order.ID, userID, p.ID, req.RequestKey, hash, string(raw), now)
	if err != nil {
		return nil, err
	}
	return &order, tx.Commit()
}

type OrderFilter struct {
	UserID                                         int64
	Admin                                          bool
	Query, Payment, Fulfillment, Product, From, To string
	Page, PageSize                                 int
}

func (s *Store) Orders(ctx context.Context, f OrderFilter) ([]Order, int64, error) {
	conditions := []string{"TRUE"}
	args := make([]any, 0)
	add := func(condition string, arg any) {
		args = append(args, arg)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if !f.Admin {
		add("user_id=$%d", f.UserID)
	}
	if f.Query != "" {
		add("(id || ' ' || (payload->>'applicant_email') || ' ' || (payload->>'target_email') || ' ' || COALESCE(payload->>'external_reference','')) ILIKE ('%%' || $%d || '%%')", f.Query)
	}
	if f.Payment != "" {
		add("payload->>'payment_status'=$%d", f.Payment)
	}
	if f.Fulfillment != "" {
		add("payload->>'fulfillment_status'=$%d", f.Fulfillment)
	}
	if f.Product != "" {
		if _, err := uuid.Parse(f.Product); err != nil {
			return nil, 0, invalid("商品筛选无效")
		}
		add("product_id=$%d", f.Product)
	}
	for _, date := range []struct{ value, condition string }{{f.From, "created_at >= $%d::timestamptz"}, {f.To, "created_at < ($%d::timestamptz + INTERVAL '1 day')"}} {
		if date.value == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, date.value); err != nil {
			return nil, 0, invalid("日期筛选无效")
		}
		add(date.condition, date.value+"T00:00:00+08:00")
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM shop_orders WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	query := fmt.Sprintf("SELECT payload,version FROM shop_orders WHERE %s ORDER BY created_at DESC,id DESC LIMIT $%d OFFSET $%d", where, len(args)-1, len(args))
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Order, 0)
	for rows.Next() {
		var raw []byte
		var order Order
		var version int
		if err = rows.Scan(&raw, &version); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(raw, &order); err != nil {
			return nil, 0, err
		}
		order.Version = version
		items = append(items, order)
	}
	return items, total, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string, userID int64, admin bool) (*Order, error) {
	var raw []byte
	var version int
	err := s.DB.QueryRowContext(ctx, `SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3)`, id, admin, userID).Scan(&raw, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appErrors.NotFound("SHOP_ORDER_NOT_FOUND", "订单不存在")
	}
	if err != nil {
		return nil, err
	}
	var order Order
	if err = json.Unmarshal(raw, &order); err != nil {
		return nil, err
	}
	order.Version = version
	return &order, nil
}

func (s *Store) Act(ctx context.Context, id string, userID int64, admin bool, action OrderAction) (*Order, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var raw []byte
	var version int
	err = tx.QueryRowContext(ctx, `SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3) FOR UPDATE`, id, admin, userID).Scan(&raw, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appErrors.NotFound("SHOP_ORDER_NOT_FOUND", "订单不存在")
	}
	if err != nil {
		return nil, err
	}
	var order Order
	if err = json.Unmarshal(raw, &order); err != nil {
		return nil, err
	}
	order.Version = version
	if err = applyAction(&order, action, admin, userID, time.Now().UTC()); err != nil {
		return nil, err
	}
	raw, err = json.Marshal(order)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE shop_orders SET payload=$1,version=$2 WHERE id=$3`, string(raw), order.Version, id); err != nil {
		return nil, err
	}
	return &order, tx.Commit()
}
