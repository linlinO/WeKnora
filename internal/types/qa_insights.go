package types

import "time"

// QAIdentity is supplied by the authenticated transport, never by model output.
type QAIdentity struct {
	ChannelID, Platform, ChatID, ChatType, SourceMessageID, OriginalQuestion, SenderName, AgentID, ModelID string
}

const QAIdentityContextKey ContextKey = "QAIdentity"
const MemoryPreferencesOnlyContextKey ContextKey = "MemoryPreferencesOnly"
const MemorySourceTimeContextKey ContextKey = "MemorySourceTime"

type QAQuestion struct {
	ID             string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID       uint64    `json:"tenant_id" gorm:"index"`
	MessageID      string    `json:"message_id" gorm:"uniqueIndex;type:varchar(36)"`
	SessionID      string    `json:"session_id" gorm:"index"`
	RequestID      string    `json:"-" gorm:"index"`
	SubjectID      string    `json:"subject_id"`
	UserID         string    `json:"user_id"`
	UserName       string    `json:"user_name"`
	Source         string    `json:"source"`
	ChannelID      string    `json:"channel_id"`
	ChatID         string    `json:"chat_id"`
	ChatType       string    `json:"chat_type"`
	SourceKey      string    `json:"-" gorm:"uniqueIndex"`
	Question       string    `json:"question" gorm:"type:text"`
	TopicID        string    `json:"topic_id" gorm:"index"`
	Classification string    `json:"classification"`
	ManualTopic    bool      `json:"manual_topic"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type QATopic struct {
	ID               string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID         uint64    `json:"tenant_id" gorm:"uniqueIndex:idx_qa_topic_key,priority:1"`
	NormalizedKey    string    `json:"-" gorm:"uniqueIndex:idx_qa_topic_key,priority:2"`
	Title            string    `json:"title" gorm:"type:text"`
	Guard            string    `json:"-"`
	EmbeddingModelID string    `json:"-"`
	Embedding        JSON      `json:"-" gorm:"type:jsonb"`
	Manual           bool      `json:"manual"`
	MergedInto       string    `json:"merged_into"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type QAAnswer struct {
	ID               string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID         uint64    `json:"tenant_id" gorm:"index"`
	QuestionID       string    `json:"question_id" gorm:"index"`
	SessionID        string    `json:"session_id"`
	Answer           string    `json:"answer" gorm:"type:text"`
	GenerationStatus string    `json:"generation_status"`
	DeliveryStatus   string    `json:"delivery_status"`
	AgentID          string    `json:"agent_id"`
	ModelID          string    `json:"model_id"`
	ModelName        string    `json:"model_name"`
	References       JSON      `json:"references" gorm:"type:jsonb"`
	MemoryCount      int       `json:"memory_count"`
	MemoryDiagnostic JSON      `json:"memory_diagnostic" gorm:"type:jsonb"`
	DurationMS       int64     `json:"duration_ms"`
	Usage            JSON      `json:"usage" gorm:"type:jsonb"`
	FeedbackID       string    `json:"feedback_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
