package qainsights

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	DB      *gorm.DB
	models  interfaces.ModelService
	tenants interfaces.TenantRepository
}

func NewService(db *gorm.DB, models interfaces.ModelService, tenants interfaces.TenantRepository) *Service {
	return &Service{DB: db, models: models, tenants: tenants}
}

func asJSON(v interface{}) types.JSON { b, _ := json.Marshal(v); return types.JSON(b) }
func hash(s string) string            { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

// Observe archives only new traffic. It never scans historical sessions.
func (s *Service) Observe(ctx context.Context, m *types.Message) {
	if s == nil || s.DB == nil || m == nil {
		return
	}
	tenant, ok := types.TenantIDFromContext(ctx)
	if owner, valid := ctx.Value(types.SessionTenantIDContextKey).(uint64); valid && owner > 0 {
		tenant = owner
	}
	p, present := types.PrincipalFromContext(ctx)
	if !ok || tenant == 0 || !present {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	i, _ := ctx.Value(types.QAIdentityContextKey).(types.QAIdentity)
	if m.Role == "user" {
		q := types.QAQuestion{ID: uuid.NewString(), TenantID: tenant, MessageID: m.ID, SessionID: m.SessionID,
			RequestID: m.RequestID, SubjectID: p.StorageID(), UserID: p.ID, Source: "web", Question: m.Content,
			ChannelID: i.ChannelID, ChatID: i.ChatID, ChatType: i.ChatType, UserName: i.SenderName,
			Classification: "pending", CreatedAt: m.CreatedAt, UpdatedAt: time.Now()}
		if p.Type != types.PrincipalWebUser {
			q.Source = "api"
		}
		if p.Type == types.PrincipalIMUser {
			q.Source = i.Platform
			parts := strings.SplitN(p.ID, ":", 4)
			if len(parts) == 4 {
				q.UserID = parts[3]
			}
			if i.OriginalQuestion != "" {
				q.Question = i.OriginalQuestion
			}
		}
		q.SourceKey = hash(fmt.Sprintf("%d:%s:%s", tenant, q.Source, m.ID))
		if i.SourceMessageID != "" {
			q.SourceKey = hash(fmt.Sprintf("%d:%s:%s:%s", tenant, i.Platform, i.ChannelID, i.SourceMessageID))
		}
		if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&q).Error; err != nil {
			logger.Warnf(ctx, "qa insights: persist question failed: %v", err)
		}
		return
	}
	if m.Role != "assistant" {
		return
	}
	var q types.QAQuestion
	query := s.DB.WithContext(ctx).Where("tenant_id = ? AND session_id = ?", tenant, m.SessionID)
	if m.RequestID != "" {
		query = query.Where("request_id = ?", m.RequestID)
	} else {
		return
	}
	if query.Order("created_at DESC").First(&q).Error != nil {
		if i.SourceMessageID == "" {
			return
		}
		key := hash(fmt.Sprintf("%d:%s:%s:%s", tenant, i.Platform, i.ChannelID, i.SourceMessageID))
		if s.DB.WithContext(ctx).Where("tenant_id = ? AND source_key = ?", tenant, key).First(&q).Error != nil {
			return
		}
	}
	status := m.GenerationStatus
	if status == "" {
		status = "generating"
		if m.IsCompleted {
			status = "completed"
		}
	}
	a := types.QAAnswer{ID: m.ID, TenantID: tenant, QuestionID: q.ID, SessionID: m.SessionID, Answer: m.Content,
		GenerationStatus: status, DeliveryStatus: "unknown", AgentID: m.AgentID, ModelID: m.ModelID,
		References: asJSON(m.KnowledgeReferences), MemoryCount: len(m.UsedMemories), DurationMS: time.Since(q.CreatedAt).Milliseconds(),
		MemoryDiagnostic: m.MemoryDiagnostic,
		Usage:            asJSON(m.Usage), CreatedAt: m.CreatedAt, UpdatedAt: time.Now()}
	if a.AgentID == "" {
		a.AgentID = i.AgentID
	}
	if a.ModelID == "" {
		a.ModelID = i.ModelID
	}
	if a.ModelID != "" {
		_ = s.DB.WithContext(ctx).Table("models").Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", a.ModelID, tenant).Select("name").Scan(&a.ModelName).Error
	}
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{
		"answer", "generation_status", "agent_id", "model_id", "model_name", "references", "memory_count", "memory_diagnostic", "duration_ms", "usage", "updated_at",
	})}).Create(&a).Error
	if err != nil {
		logger.Warnf(ctx, "qa insights: persist answer failed: %v", err)
	}
}

func (s *Service) Delivery(ctx context.Context, sessionID, requestID string, err error) {
	if s == nil || s.DB == nil || requestID == "" {
		return
	}
	tenant, _ := types.TenantIDFromContext(ctx)
	status := "submitted_unconfirmed"
	if err != nil {
		status = "failed"
	}
	query := s.DB.Model(&types.QAQuestion{}).Select("id").Where("tenant_id = ? AND session_id = ? AND request_id = ?", tenant, sessionID, requestID)
	if e := s.DB.Model(&types.QAAnswer{}).Where("tenant_id = ? AND question_id IN (?)", tenant, query).Update("delivery_status", status).Error; e != nil {
		logger.Warnf(ctx, "qa insights: delivery status failed: %v", e)
	}
}

// Normalize removes presentation noise, never version/error-code discriminants.
func Normalize(question string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(question)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var guardPattern = regexp.MustCompile(`(?i)[a-z][a-z0-9_.-]*|[0-9][0-9.]*`)

func Guard(q string) string {
	tokens := guardPattern.FindAllString(strings.ToLower(q), -1)
	sort.Strings(tokens)
	return strings.Join(tokens, "|")
}

func ambiguous(q string) bool {
	n := Normalize(q)
	if len([]rune(n)) < 5 || n == "这个怎么处理" || n == "这个怎么办" || n == "怎么处理" || n == "继续" {
		return true
	}
	return (strings.Contains(n, "图片") || strings.Contains(n, "截图")) && Guard(q) == "" && (strings.Contains(n, "怎么") || strings.Contains(n, "处理") || strings.Contains(n, "分析"))
}

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aa, bb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		aa += x * x
		bb += y * y
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	return dot / math.Sqrt(aa*bb)
}

// ClassifyBatch is bounded and restartable; failures leave durable pending rows.
func (s *Service) ClassifyBatch(ctx context.Context) error {
	var questions []types.QAQuestion
	if err := s.DB.WithContext(ctx).Where("classification = ? AND manual_topic = ?", "pending", false).Order("created_at ASC").Limit(20).Find(&questions).Error; err != nil {
		return err
	}
	for _, q := range questions {
		if err := s.classify(ctx, q); err != nil {
			logger.Warnf(ctx, "qa insights: classification failed: %v", err)
		}
	}
	// A vanished worker cannot leave an answer looking live forever.
	return s.DB.WithContext(ctx).Model(&types.QAAnswer{}).Where("generation_status = ? AND updated_at < ?", "generating", time.Now().Add(-2*time.Hour)).Update("generation_status", "interrupted").Error
}

func (s *Service) classify(ctx context.Context, q types.QAQuestion) error {
	if ambiguous(q.Question) {
		return s.DB.WithContext(ctx).Model(&types.QAQuestion{}).Where("id = ? AND manual_topic = ?", q.ID, false).Update("classification", "unclassified").Error
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, q.TenantID)
	key, guard := hash(Normalize(q.Question)), Guard(q.Question)
	var topic types.QATopic
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND normalized_key = ?", q.TenantID, key).First(&topic).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	for n := 0; topic.MergedInto != "" && n < 20; n++ {
		nextID := topic.MergedInto
		topic = types.QATopic{}
		if e := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", q.TenantID, nextID).First(&topic).Error; e != nil {
			return e
		}
	}
	if topic.ID == "" {
		var vector []float32
		modelID := ""
		if s.tenants != nil && s.models != nil {
			if tenant, e := s.tenants.GetTenantByID(ctx, q.TenantID); e == nil && tenant.MemoryConfig != nil {
				modelID = tenant.MemoryConfig.EmbeddingModelID
				if modelID != "" {
					if embedder, e := s.models.GetEmbeddingModel(ctx, modelID); e == nil {
						vector, _ = embedder.Embed(ctx, q.Question)
					}
				}
			}
		}
		if len(vector) > 0 {
			var candidates []types.QATopic
			if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND guard = ? AND embedding_model_id = ? AND manual = ? AND merged_into = ''", q.TenantID, guard, modelID, false).Order("updated_at DESC").Limit(500).Find(&candidates).Error; err != nil {
				return err
			}
			best := 0.94
			for _, t := range candidates {
				var v []float32
				_ = json.Unmarshal(t.Embedding, &v)
				if score := cosine(vector, v); score > best {
					best, topic = score, t
				}
			}
		}
		if topic.ID == "" {
			topic = types.QATopic{ID: uuid.NewString(), TenantID: q.TenantID, NormalizedKey: key, Title: q.Question, Guard: guard, EmbeddingModelID: modelID, Embedding: asJSON(vector)}
			if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&topic).Error; err != nil {
				return err
			}
			topic = types.QATopic{}
			if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND normalized_key = ?", q.TenantID, key).First(&topic).Error; err != nil {
				return err
			}
		}
	}
	return s.DB.WithContext(ctx).Model(&types.QAQuestion{}).Where("id = ? AND manual_topic = ?", q.ID, false).Updates(map[string]interface{}{"topic_id": topic.ID, "classification": "classified"}).Error
}

func (s *Service) Start() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if err := s.ClassifyBatch(ctx); err != nil {
				logger.Warnf(ctx, "qa insights worker: %v", err)
			}
			cancel()
		}
	}()
}
