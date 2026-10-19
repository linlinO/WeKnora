package memory

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func preferencesOnly(ctx context.Context) bool {
	v, _ := ctx.Value(types.MemoryPreferencesOnlyContextKey).(bool)
	return v
}

func (s *Service) groupPreferences(ctx context.Context) interfaces.MemoryRecall {
	scope, _, ok := s.enabledScope(ctx)
	if !ok {
		return interfaces.MemoryRecall{Diagnostic: types.MemoryDiagnostic{Reason: s.scopeDisableReason(ctx), Mode: "group_preferences"}}
	}
	items, err := s.repo.ListActiveByKinds(ctx, scope, []string{types.MemoryKindPreference}, 20)
	if err != nil {
		return interfaces.MemoryRecall{Diagnostic: types.MemoryDiagnostic{Reason: "subject_load_failed", Mode: "group_preferences"}}
	}
	block := types.RenderMemoryBlock(items)
	used := residentItemsWithinBlock(items, block)
	s.touchAsync(ctx, scope, used)
	reason := "recalled"
	if len(used) == 0 {
		reason = "no_active_preference"
	}
	return interfaces.MemoryRecall{Prompt: types.WrapMemoryForPrompt(block, ""), Items: used, Diagnostic: types.MemoryDiagnostic{Reason: reason, Mode: "group_preferences", Used: len(used), Candidates: len(items)}}
}

// Legacy IM messages intentionally have no author and are not backfilled.
func extractionUserText(ctx context.Context, message *types.Message) (string, bool) {
	if message == nil || message.Role != "user" {
		return "", false
	}
	scope, err := ResolveScope(ctx)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(scope.SubjectID, "im_user:") || message.MemoryOwnerID != "" {
		if message.MemoryOwnerID != scope.SubjectID || !message.MemoryEligible {
			return "", false
		}
		return strings.TrimSpace(message.MemoryUserText), true
	}
	return strings.TrimSpace(message.Content), true
}
