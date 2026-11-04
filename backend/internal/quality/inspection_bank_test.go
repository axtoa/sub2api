package quality

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInspectionBankValidation(t *testing.T) {
	bank := InspectionQuestionBank{Version: 1, Items: []InspectionQuestionTemplate{}}
	for i := 0; i < 100; i++ {
		bank.Items = append(bank.Items, InspectionQuestionTemplate{ID: uuid.NewString(), InspectionQuestion: InspectionQuestion{Name: "任务", Prompt: "Explain caching", Match: "manual"}})
	}
	require.NoError(t, bank.validate()) // Library size is independent of the per-run limit.
	bank.Items = append(bank.Items, bank.Items[0])
	require.Error(t, bank.validate())
	bank.Items = bank.Items[:100]
	bank.Items[1].ID = bank.Items[0].ID
	require.Error(t, bank.validate())
	bank.Items[1].ID = uuid.NewString()
	bank.Items[0].Match = "contains"
	require.Error(t, bank.validate())
	bank.Items[0].Expected = "cache\nhit"
	require.NoError(t, bank.validate())
	for i := range bank.Items {
		bank.Items[i].Prompt = strings.Repeat("中", 4000)
	}
	require.Error(t, bank.validate()) // Total byte limit applies even with valid single questions.
}
