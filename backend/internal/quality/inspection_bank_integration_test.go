//go:build integration

package quality

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInspectionBankLegacyImportCRUDAndConflicts(t *testing.T) {
	db := qualityTestDB(t)
	store := &Store{DB: db}
	ctx := context.Background()
	cfg, err := store.Settings(ctx)
	require.NoError(t, err)
	question := InspectionQuestion{Name: "Original task", Prompt: "Explain a payment failure", Match: "manual", Expected: "Be clear"}
	cfg.InspectionQuestions = []InspectionQuestion{question}
	cfg, err = store.SaveSettings(ctx, cfg)
	require.NoError(t, err)
	target, err := store.SaveTarget(ctx, Target{AccountID: 1, Model: "test-model", Enabled: true}, cfg)
	require.NoError(t, err)
	target, err = store.Target(ctx, target.ID)
	require.NoError(t, err)
	_, err = db.Exec(`DELETE FROM quality_inspection_question_bank`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("253_inspection_question_bank.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	bank, err := store.InspectionQuestions(ctx)
	require.NoError(t, err)
	require.Len(t, bank.Items, 1)
	require.Equal(t, question, bank.Items[0].InspectionQuestion)
	require.NotEmpty(t, bank.Items[0].ID)

	queued, err := store.QueueInspection(ctx, target.ID, InspectionOptions{Mode: "quick", Checks: []string{"custom"}, Questions: []InspectionQuestion{question}})
	require.NoError(t, err)
	stale := bank
	bank.Items = append(bank.Items, InspectionQuestionTemplate{ID: uuid.NewString(), InspectionQuestion: InspectionQuestion{Name: "Second", Prompt: "2+2?", Match: "exact", Expected: "4"}})
	bank, err = store.SaveInspectionQuestions(ctx, bank)
	require.NoError(t, err)
	require.Equal(t, 2, bank.Version)
	_, err = store.SaveInspectionQuestions(ctx, stale)
	require.True(t, isConflict(err))
	bank.Items[0].Name = "Edited"
	bank, err = store.SaveInspectionQuestions(ctx, bank)
	require.NoError(t, err)
	_, err = db.Exec(string(migration)) // Must not reimport legacy items on subsequent deployment.
	require.NoError(t, err)
	persisted, err := store.InspectionQuestions(ctx)
	require.NoError(t, err)
	require.Equal(t, bank, persisted)
	afterCfg, err := store.Settings(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.Version, afterCfg.Version)
	afterTarget, err := store.Target(ctx, target.ID)
	require.NoError(t, err)
	require.True(t, target.NextQualityAt.Equal(afterTarget.NextQualityAt))

	bank.Items = nil
	_, err = store.SaveInspectionQuestions(ctx, bank)
	require.NoError(t, err)
	empty, err := store.InspectionQuestions(ctx)
	require.NoError(t, err)
	raw, err := json.Marshal(empty)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"items":[]`)
	snapshot, err := scanRun(db.QueryRow(`SELECT `+runColumns+` FROM quality_monitor_runs WHERE id=$1`, queued.ID))
	require.NoError(t, err)
	require.Equal(t, question, snapshot.Inspection.Questions[0])
}
