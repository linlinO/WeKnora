package feedback

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Save atomically upserts one person's rating. Retries must not reopen an
// already-reviewed record; older platform events must not undo newer votes.
func Save(ctx context.Context, db *gorm.DB, row *types.AnswerFeedback) error {
	row.ID = uuid.NewString()
	if row.Status == "" {
		row.Status = types.AnswerFeedbackPending
	}
	if row.Source == "" {
		row.Source = "web"
	}
	changed := "(answer_feedback.rating <> excluded.rating OR answer_feedback.reason <> excluded.reason OR (answer_feedback.status = 'withdrawn') <> (excluded.status = 'withdrawn'))"
	condition := changed
	updates := clause.AssignmentColumns([]string{"rating", "reason", "status", "source", "channel_id", "last_event_id", "last_event_at", "updated_at", "reviewer_id", "review_note"})
	if row.Source == "wecom" {
		condition = "answer_feedback.last_event_id <> excluded.last_event_id AND answer_feedback.last_event_at <= excluded.last_event_at"
		for i := range updates {
			name := updates[i].Column.Name
			if name == "status" || name == "reviewer_id" || name == "review_note" {
				updates[i].Value = gorm.Expr("CASE WHEN " + changed + " THEN excluded." + name + " ELSE answer_feedback." + name + " END")
			}
		}
	}
	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "message_id"}},
		DoUpdates: updates,
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: condition}}},
	}).Create(row).Error
	if err != nil {
		return err
	}
	var saved types.AnswerFeedback
	if err := db.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND message_id = ?", row.TenantID, row.UserID, row.MessageID).Take(&saved).Error; err != nil {
		return err
	}
	*row = saved
	return nil
}

// Question uses the server-side turn pairing rather than accepting a client
// supplied question/answer as authoritative review material.
func Question(ctx context.Context, db *gorm.DB, message *types.Message) (string, error) {
	var user types.Message
	query := db.WithContext(ctx).Where("session_id = ? AND role = ?", message.SessionID, "user")
	if message.RequestID != "" {
		query = query.Where("request_id = ?", message.RequestID)
	} else {
		query = query.Where("created_at <= ?", message.CreatedAt).Order("created_at DESC")
	}
	if err := query.Take(&user).Error; err != nil {
		return "", err
	}
	return strings.TrimSpace(user.Content), nil
}
