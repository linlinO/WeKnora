package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func qaHandlerHarness(t *testing.T) (*QAInsightsHandler, *gorm.DB) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, e)
	require.NoError(t, db.AutoMigrate(&types.QAQuestion{}, &types.QAAnswer{}, &types.QATopic{}, &types.AnswerFeedback{}, &types.IMFeedbackUserName{}))
	require.NoError(t, db.Exec("CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT)").Error)
	return &QAInsightsHandler{db: db}, db
}
func qaRequest(t *testing.T, call gin.HandlerFunc, id, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest("POST", url, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request = c.Request.WithContext(types.WithCaller(context.Background(), types.Caller{TenantID: 1, UserID: "admin", Role: types.TenantRoleAdmin}))
	call(c)
	return w
}
func TestQAInsightsIsolationNamesAndCorrectionDoesNotPublish(t *testing.T) {
	h, db := qaHandlerHarness(t)
	q := types.QAQuestion{ID: "q1", TenantID: 1, MessageID: "m1", SourceKey: "k1", UserID: "alice", ChannelID: "bot", Source: "wecom", Question: "数据库不同步怎么办", CreatedAt: time.Now()}
	require.NoError(t, db.Create(&q).Error)
	foreign := types.QAQuestion{ID: "q2", TenantID: 2, MessageID: "m2", SourceKey: "k2", Question: "不能读取的空间", CreatedAt: time.Now()}
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&types.IMFeedbackUserName{TenantID: 1, ChannelID: "bot", UserID: "wecom:alice", Name: "方绍羽"}).Error)
	a := types.QAAnswer{ID: "a", TenantID: 1, QuestionID: q.ID, SessionID: "s", Answer: "先检查同步状态", GenerationStatus: "completed"}
	require.NoError(t, db.Create(&a).Error)
	w := qaRequest(t, h.List, "", "/?reporter=方绍羽", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "方绍羽")
	require.NotContains(t, w.Body.String(), "不能读取")
	w = qaRequest(t, h.Detail, "q2", "/", "")
	require.Equal(t, 404, w.Code)
	w = qaRequest(t, h.Review, "a", "/", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var first map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	w = qaRequest(t, h.Review, "a", "/", "")
	require.Equal(t, 200, w.Code)
	var second map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	require.Equal(t, first["feedback_id"], second["feedback_id"])
	var f types.AnswerFeedback
	require.NoError(t, db.First(&f).Error)
	require.Equal(t, "correcting", f.Status)
	require.Empty(t, f.CorrectionKnowledgeID)
	require.Equal(t, "admin_review", f.Source)
}
func TestQAInsightsTopicCountsDoNotMultiplyByAttemptsOrVotes(t *testing.T) {
	h, db := qaHandlerHarness(t)
	require.NoError(t, db.Create(&types.QATopic{ID: "topic", TenantID: 1, NormalizedKey: "key", Title: "数据库不同步"}).Error)
	for i, user := range []string{"alice", "alice", "bob"} {
		q := types.QAQuestion{ID: fmt.Sprint(i), TenantID: 1, MessageID: fmt.Sprint(i), SourceKey: fmt.Sprint(i), SubjectID: user, Source: "wecom", TopicID: "topic", Question: "数据库不同步怎么办", CreatedAt: time.Now()}
		require.NoError(t, db.Create(&q).Error)
	}
	for _, id := range []string{"a1", "a2"} {
		require.NoError(t, db.Create(&types.QAAnswer{ID: id, TenantID: 1, QuestionID: "0", Answer: "答复", GenerationStatus: "completed"}).Error)
	}
	for i, id := range []string{"a1", "a2"} {
		require.NoError(t, db.Create(&types.AnswerFeedback{ID: fmt.Sprintf("f%d", i), TenantID: 1, UserID: "voter", MessageID: id, Rating: "not_useful", Status: "pending", Source: "wecom"}).Error)
	}
	w := qaRequest(t, h.Topics, "", "/?frequent=true", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var result struct {
		Data  []qaTopicStats `json:"data"`
		Total int64          `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Data, 1)
	require.Equal(t, int64(3), result.Data[0].Questions)
	require.Equal(t, int64(2), result.Data[0].People)
	require.Equal(t, int64(1), result.Data[0].NotUseful)
	w = qaRequest(t, h.Topics, "", "/?frequent=true&min_people=3", "")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"total":0`)
}
func TestQAInsightsMergeRejectsForeignTopicAndSplitPreservesOriginal(t *testing.T) {
	h, db := qaHandlerHarness(t)
	for _, topic := range []types.QATopic{{ID: "one", TenantID: 1, NormalizedKey: "one", Title: "主题一"}, {ID: "foreign", TenantID: 2, NormalizedKey: "other", Title: "其他空间"}} {
		require.NoError(t, db.Create(&topic).Error)
	}
	q := types.QAQuestion{ID: "q", TenantID: 1, MessageID: "m", SourceKey: "source", TopicID: "one", Question: "原始问题"}
	require.NoError(t, db.Create(&q).Error)
	w := qaRequest(t, h.Reclassify, "one", "/", `{"action":"merge","target_id":"foreign"}`)
	require.Equal(t, 400, w.Code)
	w = qaRequest(t, h.Reclassify, "one", "/", `{"action":"split","title":"细分主题","question_ids":["q"]}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NoError(t, db.First(&q).Error)
	require.NotEqual(t, "one", q.TopicID)
	require.Equal(t, "原始问题", q.Question)
	require.True(t, q.ManualTopic)
}
