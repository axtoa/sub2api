package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageSelectionReasonAdminOnly(t *testing.T) {
	reason := service.AccountSelectionPriority
	log := &service.UsageLog{AccountSelectionReason: &reason}
	adminJSON, err := json.Marshal(UsageLogFromServiceAdmin(log))
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"account_selection_reason":"priority"`)
	userJSON, err := json.Marshal(UsageLogFromService(log))
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "account_selection_reason")
	log.AccountSelectionReason = nil
	historyJSON, err := json.Marshal(UsageLogFromServiceAdmin(log))
	require.NoError(t, err)
	require.NotContains(t, string(historyJSON), "account_selection_reason")
}
