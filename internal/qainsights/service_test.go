package qainsights

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func insightsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.QAQuestion{}, &types.QAAnswer{}, &types.QATopic{}))
	return db
}
func insightsContext(user, source string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalIMUser, ID: "1:bot:wecom:" + user})
	return context.WithValue(ctx, types.QAIdentityContextKey, types.QAIdentity{Platform: "wecom", ChannelID: "bot", ChatID: "group", ChatType: "group", SourceMessageID: source, OriginalQuestion: "数据库不同步怎么办？"})
}
func TestArchiveDeduplicatesTransportButKeepsPeopleAndAttempts(t *testing.T) {
	db := insightsDB(t)
	s := &Service{DB: db}
	ctx := insightsContext("alice", "incoming1")
	u := &types.Message{ID: "u1", SessionID: "session", RequestID: "request1", Role: "user", Content: "数据库不同步怎么办？", CreatedAt: time.Now()}
	s.Observe(ctx, u)
	s.Observe(ctx, u)
	duplicate := *u
	duplicate.ID = "replayed"
	s.Observe(ctx, &duplicate)
	var n int64
	require.NoError(t, db.Model(&types.QAQuestion{}).Count(&n).Error)
	require.Equal(t, int64(1), n)
	a := &types.Message{ID: "a1", SessionID: u.SessionID, RequestID: u.RequestID, Role: "assistant", CreatedAt: time.Now()}
	s.Observe(ctx, a)
	a.Content = "先检查同步链路"
	a.IsCompleted = true
	s.Observe(ctx, a)
	second := *a
	second.ID = "a2"
	second.GenerationStatus = "failed"
	second.Content = "上游模型错误"
	s.Observe(ctx, &second)
	var answers []types.QAAnswer
	require.NoError(t, db.Order("id").Find(&answers).Error)
	require.Len(t, answers, 2)
	require.Equal(t, answers[0].QuestionID, answers[1].QuestionID)
	require.Equal(t, "failed", answers[1].GenerationStatus)
	s.Delivery(ctx, u.SessionID, u.RequestID, nil)
	require.NoError(t, db.First(&answers[0], "id = ?", "a1").Error)
	require.Equal(t, "submitted_unconfirmed", answers[0].DeliveryStatus)
	bob := *u
	bob.ID = "u2"
	bob.RequestID = "r2"
	s.Observe(insightsContext("bob", "incoming2"), &bob)
	var questions []types.QAQuestion
	require.NoError(t, db.Find(&questions).Error)
	require.Len(t, questions, 2)
	require.NotEqual(t, questions[0].SubjectID, questions[1].SubjectID)
}
func TestClassificationPreservesVersionsAndDoesNotMergeAmbiguousQuestions(t *testing.T) {
	db := insightsDB(t)
	s := &Service{DB: db}
	texts := []string{"V3R4 安装失败怎么办？", "V3R5 安装失败怎么办？", "这个怎么处理？", "V3R4 安装失败怎么办?"}
	for i, text := range texts {
		q := types.QAQuestion{ID: fmt.Sprint(i), TenantID: 1, MessageID: fmt.Sprint(i), SourceKey: fmt.Sprint(i), Question: text, Classification: "pending", CreatedAt: time.Now()}
		require.NoError(t, db.Create(&q).Error)
	}
	require.NoError(t, s.ClassifyBatch(context.Background()))
	var rows []types.QAQuestion
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.NotEqual(t, rows[0].TopicID, rows[1].TopicID)
	require.Equal(t, rows[0].TopicID, rows[3].TopicID)
	require.Empty(t, rows[2].TopicID)
	require.Equal(t, "unclassified", rows[2].Classification)
	require.NotEqual(t, Guard("error 500"), Guard("error 401"))
}
func TestManualAssignmentSurvivesReclassification(t *testing.T) {
	db := insightsDB(t)
	s := &Service{DB: db}
	q := types.QAQuestion{ID: "q", TenantID: 1, MessageID: "m", SourceKey: "s", Question: "数据库不同步怎么办", TopicID: "manual", Classification: "pending", ManualTopic: true, CreatedAt: time.Now()}
	require.NoError(t, db.Create(&q).Error)
	require.NoError(t, s.ClassifyBatch(context.Background()))
	require.NoError(t, db.First(&q).Error)
	require.Equal(t, "manual", q.TopicID)
}

func TestMergedTopicAcceptsNewExactQuestion(t *testing.T) {
	db := insightsDB(t)
	s := &Service{DB: db}
	require.NoError(t, db.Create(&types.QATopic{ID: "old", TenantID: 1, NormalizedKey: hash(Normalize("数据库不同步怎么办")), Title: "旧主题", MergedInto: "new"}).Error)
	require.NoError(t, db.Create(&types.QATopic{ID: "new", TenantID: 1, NormalizedKey: "other", Title: "已合并主题"}).Error)
	q := types.QAQuestion{ID: "q", TenantID: 1, MessageID: "m", SourceKey: "key", Question: "数据库不同步怎么办", Classification: "pending"}
	require.NoError(t, db.Create(&q).Error)
	require.NoError(t, s.ClassifyBatch(context.Background()))
	require.NoError(t, db.First(&q).Error)
	require.Equal(t, "new", q.TopicID)
}
