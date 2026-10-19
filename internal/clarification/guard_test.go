package clarification

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFactsRequireUserEvidence(t *testing.T) {
	current := "升级失败怎么办？"
	_, facts, missing, err := ValidateAssessment(Assessment{Mode: "incident", Relation: "new"}, current, "KE报错端口占用")
	require.NoError(t, err)
	require.Empty(t, facts)
	require.Equal(t, []string{"component", "symptom"}, missing)
	_, facts, missing, err = ValidateAssessment(Assessment{Mode: "incident", Relation: "new", Facts: map[string]string{"component": "KE"}}, current, "KE报错")
	require.NoError(t, err)
	require.Empty(t, facts, "new issue must not inherit old facts")
	require.Contains(t, missing, "component")
	input, facts, missing, err := ValidateAssessment(Assessment{Mode: "incident", Relation: "continue", Facts: map[string]string{"component": "KE", "symptom": "端口占用"}}, "报错端口占用", "KE升级失败")
	require.NoError(t, err)
	require.Contains(t, input, "KE升级失败")
	require.Empty(t, missing)
	require.Equal(t, "KE", facts["component"])
	_, _, missing, err = ValidateAssessment(Assessment{Mode: "general", Relation: "new"}, "xtwin是什么", "")
	require.NoError(t, err)
	require.Empty(t, missing)
}
func TestApplicabilityMustBeGroundedAndNotRepeatKnownFacts(t *testing.T) {
	sources := []string{"PostgreSQL数据库适用：执行步骤A", "MongoDB数据库适用：执行步骤B"}
	_, missing, err := ValidateEvidence(Evidence{Decision: "clarify", Conditions: []Condition{{Field: "database", Source: 1, Quote: "PostgreSQL数据库适用"}}}, sources, map[string]string{})
	require.NoError(t, err)
	require.Equal(t, []string{"database"}, missing)
	for _, condition := range []Condition{{Field: "database", Source: 3, Quote: "不存在"}, {Field: "version", Source: 1, Quote: "版本必须5.0"}, {Field: "database", Source: 1, Quote: "PostgreSQL数据库适用"}} {
		_, _, err = ValidateEvidence(Evidence{Decision: "clarify", Conditions: []Condition{condition}}, sources, map[string]string{"database": "PostgreSQL"})
		require.Error(t, err)
	}
	ids, _, err := ValidateEvidence(Evidence{Decision: "answer", Matches: []int{2}}, sources, nil)
	require.NoError(t, err)
	require.Equal(t, []int{1}, ids)
	_, _, err = ValidateEvidence(Evidence{Decision: "answer"}, sources, nil)
	require.Error(t, err)
}

func TestConfirmedFactsSurviveFollowupExtraction(t *testing.T) {
	prior := map[string]string{"component": "KE", "version": "未经用户提供"}
	_, facts, missing, err := ValidateAssessment(Assessment{Mode: "incident", Relation: "continue", Facts: map[string]string{"symptom": "端口占用"}}, "报错端口占用", "是KE平台升级", prior)
	require.NoError(t, err)
	require.Equal(t, "KE", facts["component"])
	require.Empty(t, facts["version"])
	require.Empty(t, missing)
	_, facts, missing, err = ValidateAssessment(Assessment{Mode: "incident", Relation: "new", Facts: map[string]string{"symptom": "端口占用"}}, "端口占用", "是KE平台升级", prior)
	require.NoError(t, err)
	require.Empty(t, facts["component"])
	require.Contains(t, missing, "component")
	_, _, missing, err = ValidateAssessment(Assessment{Mode: "incident", Relation: "continue", Facts: map[string]string{"symptom": "升级失败"}}, "升级失败", "是KE平台升级", prior)
	require.NoError(t, err)
	require.Equal(t, []string{"symptom"}, missing)
}

func TestStoredFactsRoundtripAndLegacyInput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "state.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&State{}))
	store := Store{DB: db}
	s := State{Key: "roundtrip", Input: "KE升级失败", Facts: map[string]string{"component": "KE"}, Missing: "symptom"}
	require.NoError(t, store.Save(context.Background(), s))
	got, err := store.Load(context.Background(), s.Key)
	require.NoError(t, err)
	require.Equal(t, s.Input, got.Input)
	require.Equal(t, s.Facts, got.Facts)
	legacy := State{Key: "legacy", Input: "旧版原话", ExpiresAt: time.Now().Add(time.Minute)}
	require.NoError(t, db.Create(&legacy).Error)
	got, err = store.Load(context.Background(), legacy.Key)
	require.NoError(t, err)
	require.Equal(t, legacy.Input, got.Input)
}
func TestScopeTTLAndTopicReset(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "state.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&State{}))
	store := Store{DB: db}
	ctx := context.Background()
	key := Scope(1, "group-session", "im_user:alice", "agent")
	require.NoError(t, store.Save(ctx, State{Key: key, Input: "KE升级失败", Rounds: 1}))
	for _, other := range []string{Scope(1, "group-session", "im_user:bob", "agent"), Scope(2, "group-session", "im_user:alice", "agent"), Scope(1, "private-session", "im_user:alice", "agent"), Scope(1, "group-session", "im_user:alice", "other")} {
		got, e := store.Load(ctx, other)
		require.NoError(t, e)
		require.Empty(t, got.Input)
	}
	got, err := store.Load(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "KE升级失败", got.Input)
	require.NoError(t, db.Model(&State{}).Where("scope_key=?", key).Update("expires_at", time.Now().Add(-time.Second)).Error)
	got, err = store.Load(ctx, key)
	require.NoError(t, err)
	require.Empty(t, got.Input)
	require.NoError(t, store.Save(ctx, State{Key: key, Input: "new"}))
	require.NoError(t, store.Clear(ctx, key))
	got, err = store.Load(ctx, key)
	require.NoError(t, err)
	require.Empty(t, got.Input)
	require.Empty(t, Scope(1, "s", "", "a"))
}
func TestBoundedQuestionsAndHandoff(t *testing.T) {
	text := Reply([]string{"component", "symptom", "version"}, 1, false)
	require.Contains(t, text, "1.")
	require.Contains(t, text, "2.")
	require.NotContains(t, text, "3.")
	require.False(t, strings.Contains(text, "版本"))
	require.Contains(t, Reply([]string{"component"}, 3, false), "运维人员")
}
