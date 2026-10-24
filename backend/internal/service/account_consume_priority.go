package service

import (
	"context"
	"log/slog"
)

// ConsumePriorityExtraKey is independent of the existing numeric priority.
// Missing, malformed and false values all preserve ordinary scheduling.
const ConsumePriorityExtraKey = "consume_priority_enabled"

const (
	AccountSelectionNormal   = "normal"
	AccountSelectionPriority = "priority"
	AccountSelectionFallback = "fallback"
)

func (a *Account) SupportsConsumePriority() bool {
	return a != nil && (a.Platform == PlatformOpenAI || a.Platform == PlatformAnthropic)
}

func (a *Account) ConsumePriorityEnabled() bool {
	if !a.SupportsConsumePriority() {
		return false
	}
	enabled, _ := a.Extra[ConsumePriorityExtraKey].(bool)
	return enabled
}

// AccountSelectionReasonAfterRetry preserves actual failover attribution. It
// never influences selection and never inspects the account's current flag.
func AccountSelectionReasonAfterRetry(previous, current string) string {
	if current == AccountSelectionNormal && (previous == AccountSelectionPriority || previous == AccountSelectionFallback) {
		return AccountSelectionFallback
	}
	return current
}

func normalAccountSelectionReason(result *AccountSelectionResult) {
	if result != nil && result.Account != nil && result.AccountSelectionReason == "" {
		result.AccountSelectionReason = AccountSelectionNormal
	}
}

func accountSelectionReasonPointer(reason string) *string {
	switch reason {
	case AccountSelectionNormal, AccountSelectionPriority, AccountSelectionFallback:
		return &reason
	default:
		return nil
	}
}

// Subsequent WebSocket turns reuse the connection's account; they are not new
// preferred selections, even when the first turn chose a preferred account.
func AccountSelectionReasonForTurn(reason string, turn int) string {
	if turn > 1 && reason != "" {
		return AccountSelectionNormal
	}
	return reason
}

func preferConsumptionInOrder(accounts []accountWithLoad) []accountWithLoad {
	preferred := preferredAvailableAccounts(accounts)
	if len(preferred) == len(accounts) {
		return accounts
	}
	ordered := make([]accountWithLoad, 0, len(accounts))
	ordered = append(ordered, preferred...)
	for _, item := range accounts {
		if !item.account.ConsumePriorityEnabled() {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

// preferredAvailableAccounts returns the original slice when disabled. It is
// called only after eligibility and load checks, inside the ordinary acquire
// loop; the full candidate list remains available for fallback.
func preferredAvailableAccounts(accounts []accountWithLoad) []accountWithLoad {
	count := 0
	for _, candidate := range accounts {
		if candidate.account.ConsumePriorityEnabled() {
			count++
		}
	}
	if count == 0 || count == len(accounts) {
		return accounts
	}
	preferred := make([]accountWithLoad, 0, count)
	for _, candidate := range accounts {
		if candidate.account.ConsumePriorityEnabled() {
			preferred = append(preferred, candidate)
		}
	}
	return preferred
}

// consumePriorityOrder adds at most one immediate preferred attempt, using the
// existing score/weighted ordering among preferred accounts. Ordinary Top-K
// candidates and the original waiting plan are retained. Special capabilities
// and weighted sticky sessions keep their existing selection policy.
func consumePriorityOrder(req OpenAIAccountScheduleRequest, plan openAIAccountLoadPlan) ([]openAIAccountCandidateScore, int64) {
	if req.Platform != "" && req.Platform != PlatformOpenAI || req.RequireCompact || req.RequiredImageCapability != "" ||
		(req.RequiredCapability != "" && req.RequiredCapability != OpenAIEndpointCapabilityChatCompletions) ||
		(req.StickyWeighted && (req.StickyPreviousAccountID > 0 || req.StickyAccountID > 0)) {
		return plan.selectionOrder, 0
	}
	var preferred []openAIAccountCandidateScore
	for _, candidate := range plan.candidates {
		if candidate.account.ConsumePriorityEnabled() {
			preferred = append(preferred, candidate)
		}
	}
	if len(preferred) == 0 || plan.topK <= 0 {
		return plan.selectionOrder, 0
	}
	k := min(plan.topK, len(preferred))
	preferred = buildOpenAIWeightedSelectionOrder(selectTopKOpenAICandidates(preferred, k), req)
	first := preferred[0]
	order := make([]openAIAccountCandidateScore, 0, len(plan.selectionOrder)+1)
	order = append(order, first)
	for _, candidate := range plan.selectionOrder {
		if candidate.account.ID != first.account.ID {
			order = append(order, candidate)
		}
	}
	return order, first.account.ID
}

func (s *defaultOpenAIAccountScheduler) tryAcquireWithConsumePriority(ctx context.Context, req OpenAIAccountScheduleRequest, plan openAIAccountLoadPlan, budget *openAISelectionProbeBudget) (*AccountSelectionResult, bool, bool, error) {
	order, preferredID := consumePriorityOrder(req, plan)
	if preferredID == 0 {
		result, blocked, err := s.tryAcquireOpenAISelectionOrderWithBudget(ctx, req, plan.selectionOrder, budget)
		return result, blocked, false, err
	}
	// The optional attempt cannot exhaust the ordinary fallback's budget.
	// Errors here do not fail the request. No new upstream requests are issued.
	preferredBudget := newOpenAISelectionProbeBudget()
	preferredBudget.enableLimit()
	result, _, err := s.tryAcquireOpenAISelectionOrderWithBudget(ctx, req, order[:1], preferredBudget)
	if err != nil {
		slog.Warn("consume_priority_attempt_failed", "account_id", preferredID, "error", err)
	}
	if result != nil && result.Account != nil {
		result.AccountSelectionReason = AccountSelectionNormal
		if result.Account.ConsumePriorityEnabled() {
			result.AccountSelectionReason = AccountSelectionPriority
		}
		return result, false, true, nil
	}
	result, blocked, err := s.tryAcquireOpenAISelectionOrderWithBudget(ctx, req, plan.selectionOrder, budget)
	if result != nil {
		result.AccountSelectionReason = AccountSelectionFallback
	}
	return result, blocked, true, err
}
