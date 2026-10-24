//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestConsumePriorityPersistence(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUsageLogRepositoryWithSQL(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@example.com"})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-" + uuid.NewString(), Name: "priority-test"})
	account := mustCreateAccount(t, client, &service.Account{Name: "priority-test-" + uuid.NewString(), Extra: map[string]any{"retained_field": "keep"}})

	for _, path := range []string{"single", "batch", "best_effort"} {
		for _, reason := range []string{"", service.AccountSelectionNormal, service.AccountSelectionPriority, service.AccountSelectionFallback} {
			t.Run(path+"/"+reason, func(t *testing.T) {
				log := &service.UsageLog{UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, RequestID: uuid.NewString(), Model: "gpt-5", InputTokens: 10, OutputTokens: 5, ActualCost: 0.125, TotalCost: 0.25, CreatedAt: time.Now().UTC()}
				if reason != "" {
					log.AccountSelectionReason = &reason
				}
				prepared := prepareUsageLogInsert(log)
				switch path {
				case "single":
					_, err := repo.createSingle(ctx, integrationDB, log)
					require.NoError(t, err)
				case "batch":
					key := usageLogBatchKey(log.RequestID, log.APIKeyID)
					query, args := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
					rows, err := integrationDB.QueryContext(ctx, query, args...)
					require.NoError(t, err)
					for rows.Next() {
					}
					require.NoError(t, rows.Err())
					require.NoError(t, rows.Close())
				case "best_effort":
					query, args := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
					_, err := integrationDB.ExecContext(ctx, query, args...)
					require.NoError(t, err)
				}
				var id int64
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", log.RequestID, apiKey.ID).Scan(&id))
				got, err := repo.GetByID(ctx, id)
				require.NoError(t, err)
				require.Equal(t, log.AccountSelectionReason, got.AccountSelectionReason)
				require.Equal(t, log.InputTokens, got.InputTokens)
				require.Equal(t, log.OutputTokens, got.OutputTokens)
				require.Equal(t, log.ActualCost, got.ActualCost)
				require.Equal(t, log.TotalCost, got.TotalCost)
			})
		}
	}

	accounts := NewAccountRepository(client, integrationDB, nil)
	for _, enabled := range []bool{true, false} {
		require.NoError(t, accounts.UpdateExtra(ctx, account.ID, map[string]any{service.ConsumePriorityExtraKey: enabled}))
		got, err := accounts.GetByID(ctx, account.ID)
		require.NoError(t, err)
		require.Equal(t, enabled, got.Extra[service.ConsumePriorityExtraKey])
		require.Equal(t, "keep", got.Extra["retained_field"])
	}
}
