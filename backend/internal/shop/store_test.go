package shop

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	appErrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); db.Close() })
	return &Store{DB: db}, mock
}

func jsonFixture(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

func createFixture() CreateOrder {
	return CreateOrder{ProductID: productFixture().ID, ProductVersion: 3, Accepted: true, RequestKey: "99f78e2c-0fd2-4b8f-895a-6239c9b0d6ad"}
}

func expectCreateAccount(mock sqlmock.Sqlmock, req CreateOrder, replay *Order, hash string) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock(hashtextextended($1,0))`).WithArgs("subscription-shop:42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT email FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("member@example.com"))
	rows := sqlmock.NewRows([]string{"payload", "request_hash", "version"})
	if replay != nil {
		raw, _ := json.Marshal(replay)
		rows.AddRow(string(raw), hash, replay.Version)
	}
	mock.ExpectQuery(`SELECT payload,request_hash,version FROM shop_orders WHERE user_id=$1 AND request_key=$2`).WithArgs(int64(42), req.RequestKey).WillReturnRows(rows)
}

func expectShopEnabled(mock sqlmock.Sqlmock, enabled bool) {
	mock.ExpectQuery(enabledForShareSQL).WithArgs(service.SettingKeyOfficialShopEnabled).WillReturnRows(sqlmock.NewRows([]string{"enabled"}).AddRow(enabled))
}

func expectProduct(mock sqlmock.Sqlmock, t *testing.T, p Product) {
	mock.ExpectQuery(`SELECT payload,version FROM shop_products WHERE id=$1 FOR SHARE`).WithArgs(p.ID).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}).AddRow(jsonFixture(t, p), p.Version))
}

const duplicateSQL = `SELECT id FROM shop_orders WHERE user_id=$1 AND product_id=$2 AND payload->>'target_email'=$3 AND payload->>'payment_status' IN ('PENDING','PAID') AND payload->>'fulfillment_status' NOT IN ('COMPLETED','CLOSED') LIMIT 1`
const insertOrderSQL = `INSERT INTO shop_orders(id,user_id,product_id,request_key,request_hash,payload,version,created_at) VALUES($1,$2,$3,$4,$5,$6,1,$7)`

type orderArgument struct {
	t      *testing.T
	target string
}

func (arg orderArgument) Match(value driver.Value) bool {
	var o Order
	if json.Unmarshal([]byte(value.(string)), &o) != nil {
		return false
	}
	return o.UserID == 42 && o.ApplicantEmail == "member@example.com" && o.TargetEmail == arg.target && o.ContactEmail == "member@example.com" && o.AmountCents == 12300 && o.Product.Version == 3 && o.Product.PaymentURL == productFixture().PaymentURL && o.PaymentStatus == "PENDING" && o.Version == 1
}

func TestCreateUsesAccountIdentityAndImmutablePriceSnapshot(t *testing.T) {
	for _, target := range []string{"", " Other@Example.com "} {
		t.Run(target, func(t *testing.T) {
			s, mock := testStore(t)
			req := createFixture()
			req.TargetEmail = target
			want := "member@example.com"
			if target != "" {
				want = "other@example.com"
			}
			expectCreateAccount(mock, req, nil, "")
			expectShopEnabled(mock, true)
			expectProduct(mock, t, productFixture())
			mock.ExpectQuery(duplicateSQL).WithArgs(int64(42), req.ProductID, want).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectExec(insertOrderSQL).WithArgs(sqlmock.AnyArg(), int64(42), req.ProductID, req.RequestKey, sqlmock.AnyArg(), orderArgument{t, want}, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			o, err := s.Create(context.Background(), 42, req)
			require.NoError(t, err)
			require.Equal(t, want, o.TargetEmail)
			require.Equal(t, int64(12300), o.AmountCents)
		})
	}
}

func TestCreateIdempotency(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "replay", true: "changed payload"}[mismatch], func(t *testing.T) {
			s, mock := testStore(t)
			req := createFixture()
			normalized := req
			normalized.TargetEmail, normalized.ContactEmail = "member@example.com", "member@example.com"
			o := orderFixture()
			hash := requestHash(normalized)
			if mismatch {
				req.Note = "changed"
			}
			expectCreateAccount(mock, req, &o, hash)
			if mismatch {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			result, err := s.Create(context.Background(), 42, req)
			if mismatch {
				require.Equal(t, 409, appErrors.Code(err))
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, o.ID, result.ID)
			}
		})
	}
}

func TestCreateRejectsStaleHiddenUnpricedOrUnsafeProducts(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Product)
	}{
		{"stale", func(p *Product) { p.Version++ }}, {"hidden", func(p *Product) { p.Active = false }},
		{"unpriced", func(p *Product) { p.PriceCents = nil }}, {"no link", func(p *Product) { p.PaymentURL = "" }},
		{"unsafe link", func(p *Product) { p.PaymentURL = "javascript:alert(1)" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, mock := testStore(t)
			req := createFixture()
			p := productFixture()
			tc.edit(&p)
			expectCreateAccount(mock, req, nil, "")
			expectShopEnabled(mock, true)
			expectProduct(mock, t, p)
			mock.ExpectRollback()
			o, err := s.Create(context.Background(), 42, req)
			require.Nil(t, o)
			require.Equal(t, 409, appErrors.Code(err))
		})
	}
}

func TestCreateRejectsDuplicateUnfinishedOrders(t *testing.T) {
	s, mock := testStore(t)
	req := createFixture()
	expectCreateAccount(mock, req, nil, "")
	expectShopEnabled(mock, true)
	expectProduct(mock, t, productFixture())
	mock.ExpectQuery(duplicateSQL).WithArgs(int64(42), req.ProductID, "member@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("existing"))
	mock.ExpectRollback()
	_, err := s.Create(context.Background(), 42, req)
	require.Equal(t, 409, appErrors.Code(err))
}

func TestGetAndActEnforceOwnershipInSQL(t *testing.T) {
	s, mock := testStore(t)
	mock.ExpectQuery(`SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3)`).WithArgs("other", false, int64(42)).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}))
	_, err := s.Get(context.Background(), "other", 42, false)
	require.Equal(t, 404, appErrors.Code(err))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3) FOR UPDATE`).WithArgs("other", false, int64(42)).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}))
	mock.ExpectRollback()
	_, err = s.Act(context.Background(), "other", 42, false, OrderAction{Action: "cancel", Version: 1})
	require.Equal(t, 404, appErrors.Code(err))
}

func TestConcurrentOrderUpdateIsRejectedBeforeWrite(t *testing.T) {
	s, mock := testStore(t)
	o := orderFixture()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT payload,version FROM shop_orders WHERE id=$1 AND ($2 OR user_id=$3) FOR UPDATE`).WithArgs(o.ID, false, int64(42)).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}).AddRow(jsonFixture(t, o), 2))
	mock.ExpectRollback()
	_, err := s.Act(context.Background(), o.ID, 42, false, OrderAction{Action: "cancel", Version: 1})
	require.Equal(t, 409, appErrors.Code(err))
}

func TestProductUpdateUsesOptimisticLock(t *testing.T) {
	s, mock := testStore(t)
	p := productFixture()
	next := p
	next.Version++
	mock.ExpectExec(`UPDATE shop_products SET payload=$1,version=$2,updated_at=NOW() WHERE id=$3 AND version=$4`).WithArgs(jsonFixture(t, next), 4, p.ID, 3).WillReturnResult(sqlmock.NewResult(0, 0))
	_, err := s.SaveProduct(context.Background(), p)
	require.Equal(t, 409, appErrors.Code(err))
}

func TestOrdersListAlwaysScopesOrdinaryUsers(t *testing.T) {
	s, mock := testStore(t)
	mock.ExpectQuery(`SELECT COUNT(*) FROM shop_orders WHERE TRUE AND user_id=$1`).WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT payload,version FROM shop_orders WHERE TRUE AND user_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`).WithArgs(int64(42), 20, 0).WillReturnRows(sqlmock.NewRows([]string{"payload", "version"}))
	items, total, err := s.Orders(context.Background(), OrderFilter{UserID: 42, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
}
