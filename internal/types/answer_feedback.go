package types

import "time"

const (
	AnswerFeedbackUseful     = "useful"
	AnswerFeedbackNotUseful  = "not_useful"
	AnswerFeedbackPending    = "pending"
	AnswerFeedbackResolved   = "resolved"
	AnswerFeedbackIgnored    = "ignored"
	AnswerFeedbackWithdrawn  = "withdrawn"
	AnswerFeedbackReviewed   = "reviewed"
	AnswerFeedbackCorrecting = "correcting"
)

// AnswerFeedback records a user's judgement of one completed assistant answer.
// The question and answer are kept as a snapshot so reviewers can work even
// when the original session is later archived or changed.
type AnswerFeedback struct {
	ID                    string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID              uint64    `json:"tenant_id" gorm:"index"`
	UserID                string    `json:"user_id" gorm:"type:varchar(128);index"`
	UserName              string    `json:"user_name,omitempty" gorm:"-"`
	Source                string    `json:"source" gorm:"type:varchar(20);default:web"`
	ChannelID             string    `json:"channel_id,omitempty" gorm:"type:varchar(36)"`
	LastEventID           string    `json:"-" gorm:"type:varchar(128)"`
	LastEventAt           int64     `json:"-"`
	SessionID             string    `json:"session_id" gorm:"type:varchar(36);index"`
	MessageID             string    `json:"message_id" gorm:"type:varchar(36);index"`
	AgentID               string    `json:"agent_id,omitempty" gorm:"type:varchar(36)"`
	KnowledgeBaseID       string    `json:"knowledge_base_id,omitempty" gorm:"type:varchar(36)"`
	Rating                string    `json:"rating" gorm:"type:varchar(20);index"`
	Question              string    `json:"question" gorm:"type:text"`
	Answer                string    `json:"answer" gorm:"type:text"`
	Reason                string    `json:"reason,omitempty" gorm:"type:text"`
	Status                string    `json:"status" gorm:"type:varchar(20);index"`
	ReviewerID            string    `json:"reviewer_id,omitempty" gorm:"type:varchar(128)"`
	ReviewNote            string    `json:"review_note,omitempty" gorm:"type:text"`
	CorrectionKnowledgeID string    `json:"correction_knowledge_id,omitempty" gorm:"type:varchar(36)"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (AnswerFeedback) TableName() string { return "answer_feedback" }

// IMFeedbackTarget persists the opaque ID sent to WeCom before the first
// stream frame, so feedback remains resolvable after a restart or /clear.
type IMFeedbackTarget struct {
	ID        string `gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64
	ChannelID string `gorm:"type:varchar(36);index"`
	SessionID string `gorm:"type:varchar(36)"`
	AgentID   string `gorm:"type:varchar(36)"`
	UserID    string `gorm:"type:varchar(128)"`
	ChatID    string `gorm:"type:varchar(128)"`
	CreatedAt time.Time
}

func (IMFeedbackTarget) TableName() string { return "im_feedback_targets" }

// IMFeedbackUserName is an administrator-maintained display label, not an
// account binding or authorization identity. It is scoped to one IM channel.
type IMFeedbackUserName struct {
	TenantID  uint64 `gorm:"primaryKey"`
	ChannelID string `gorm:"type:varchar(36);primaryKey"`
	UserID    string `gorm:"type:varchar(128);primaryKey"`
	Name      string `gorm:"type:varchar(100);not null"`
	UpdatedBy string `gorm:"type:varchar(36)"`
	UpdatedAt time.Time
}

func (IMFeedbackUserName) TableName() string { return "im_feedback_user_names" }
