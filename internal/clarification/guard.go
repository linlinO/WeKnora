package clarification

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
	"unicode"
)

const TTL = 30 * time.Minute

type State struct {
	Key       string `gorm:"primaryKey;column:scope_key"`
	Input     string
	Facts     map[string]string `gorm:"-"`
	Missing   string
	Rounds    int
	ExpiresAt time.Time
}

func (State) TableName() string { return "qa_clarification_states" }

// The terminal caller is distinct from the IM bot's shared Web account.
func Scope(tenant uint64, session, principal, agent string) string {
	if tenant == 0 || session == "" || principal == "" {
		return ""
	}
	b, _ := json.Marshal([]any{tenant, session, principal, agent})
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

type Store struct{ DB *gorm.DB }

func (s Store) Load(ctx context.Context, key string) (State, error) {
	var state State
	if key == "" {
		return state, nil
	}
	err := s.DB.WithContext(ctx).Where("scope_key=? AND expires_at>?", key, time.Now()).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return State{}, nil
	}
	if err == nil {
		var stored struct {
			Version int               `json:"version"`
			Input   string            `json:"user_input"`
			Facts   map[string]string `json:"facts"`
		}
		if json.Unmarshal([]byte(state.Input), &stored) == nil && stored.Version == 1 {
			state.Input, state.Facts = stored.Input, stored.Facts
		}
	}
	return state, err
}
func (s Store) Save(ctx context.Context, state State) error {
	if state.Key == "" {
		return nil
	}
	state.ExpiresAt = time.Now().Add(TTL)
	// Keep verified facts alongside the raw input, without a second model
	// extraction losing a fact that the caller already supplied. Load also
	// accepts the plain-text records created before this envelope existed.
	payload, err := json.Marshal(struct {
		Version int               `json:"version"`
		Input   string            `json:"user_input"`
		Facts   map[string]string `json:"facts"`
	}{1, state.Input, state.Facts})
	if err != nil {
		return err
	}
	state.Input = string(payload)
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scope_key"}}, DoUpdates: clause.AssignmentColumns([]string{"input", "missing", "rounds", "expires_at"})}).Create(&state).Error
}
func (s Store) Clear(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	return s.DB.WithContext(ctx).Where("scope_key=? OR expires_at<?", key, time.Now()).Delete(&State{}).Error
}

type Assessment struct {
	Mode     string            `json:"mode"`
	Relation string            `json:"relation"`
	Facts    map[string]string `json:"facts"`
}

var Questions = map[string]string{
	"component":   "是哪个产品或服务在操作时出错？",
	"symptom":     "具体报错是什么、卡在哪一步？请粘贴报错原文或发送清晰截图。",
	"version":     "当前版本和准备升级到的版本分别是什么？",
	"database":    "实际使用的是哪一种数据库？",
	"deployment":  "当前是单机、双机还是集群部署？",
	"phase":       "失败发生在哪个操作步骤？",
	"environment": "失败发生在什么运行环境（操作系统或容器）？",
}

func Clip(s string, limit int) string {
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit])
	}
	return s
}

// Fact values must be excerpts of user evidence, never inferred from solutions.
func ValidateAssessment(a Assessment, current, previous string, previousFacts ...map[string]string) (string, map[string]string, []string, error) {
	if a.Mode != "general" && a.Mode != "incident" || a.Relation != "new" && a.Relation != "continue" {
		return "", nil, nil, errors.New("invalid assessment")
	}
	input := current
	if a.Relation == "continue" && previous != "" {
		input = previous + "\n补充：" + current
	}
	if len([]rune(input)) > 10000 {
		return "", nil, nil, errors.New("incident context too large")
	}
	facts := map[string]string{}
	if a.Relation == "continue" && previous != "" && len(previousFacts) > 0 {
		for field, value := range previousFacts[0] {
			if _, known := Questions[field]; known && value != "" && strings.Contains(previous, value) {
				facts[field] = value
			}
		}
	}
	for field, value := range a.Facts {
		value = strings.TrimSpace(value)
		if _, ok := Questions[field]; !ok || value == "" {
			continue
		}
		if !strings.Contains(input, value) {
			// A classifier may paraphrase an optional field. Discard that
			// ungrounded field instead of rejecting all otherwise valid facts.
			// Missing required facts will still trigger clarification.
			continue
		}
		facts[field] = value
	}
	if genericSymptom(facts["symptom"]) {
		delete(facts, "symptom")
	}
	missing := []string{}
	if a.Mode == "incident" {
		for _, f := range []string{"component", "symptom"} {
			if facts[f] == "" {
				missing = append(missing, f)
			}
		}
	}
	return input, facts, missing, nil
}

func genericSymptom(value string) bool {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, value)
	for _, prefix := range []string{"", "升级", "安装", "启动", "部署", "运行", "操作", "检查", "系统", "服务"} {
		for _, vague := range []string{"失败", "失败了", "不了", "不行", "报错", "报错了", "异常", "异常了", "没成功", "不能用"} {
			if value == prefix+vague {
				return true
			}
		}
	}
	return false
}

type Condition struct {
	Field  string `json:"field"`
	Source int    `json:"source"`
	Quote  string `json:"quote"`
}
type Evidence struct {
	Decision   string      `json:"decision"`
	Matches    []int       `json:"matches"`
	Conditions []Condition `json:"conditions"`
}

func ValidateEvidence(e Evidence, sources []string, facts map[string]string) ([]int, []string, error) {
	if e.Decision != "answer" && e.Decision != "clarify" && e.Decision != "no_match" {
		return nil, nil, errors.New("invalid evidence decision")
	}
	ids := []int{}
	seen := map[int]bool{}
	for _, id := range e.Matches {
		if id < 1 || id > len(sources) {
			return nil, nil, errors.New("invalid source")
		}
		if !seen[id] {
			ids = append(ids, id-1)
			seen[id] = true
		}
	}
	missing := []string{}
	used := map[string]bool{}
	for _, c := range e.Conditions {
		if _, ok := Questions[c.Field]; !ok || facts[c.Field] != "" || c.Source < 1 || c.Source > len(sources) || len([]rune(strings.TrimSpace(c.Quote))) < 4 || !strings.Contains(sources[c.Source-1], c.Quote) {
			return nil, nil, errors.New("unsupported applicability condition")
		}
		if !used[c.Field] {
			missing = append(missing, c.Field)
			used[c.Field] = true
		}
	}
	if e.Decision == "clarify" && len(missing) == 0 || e.Decision == "answer" && (len(ids) == 0 || len(missing) > 0) {
		return nil, nil, errors.New("inconsistent evidence decision")
	}
	return ids, missing, nil
}
func Reply(fields []string, rounds int, evidence bool) string {
	if rounds >= 3 {
		return "目前仍无法确认适用的处理方案，请将具体产品、失败步骤和报错原文整理后交给运维人员核对。取得信息后可在本会话补充，我会继续检索知识库；暂不提供可能不适用的修复命令。"
	}
	text := "目前的信息还不足以确定处理方案，请补充："
	if evidence {
		text = "知识库中的处理步骤有不同适用条件，需要先确认："
	}
	for i, f := range fields {
		if i == 2 {
			break
		}
		text += fmt.Sprintf("\n%d. %s", i+1, Questions[f])
	}
	return text + "\n\n直接在本会话回复即可，我会结合你已提供的信息继续查找。"
}
