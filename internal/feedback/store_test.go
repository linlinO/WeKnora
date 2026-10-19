package feedback

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feedback.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, file := range []string{"000028_answer_feedback.up.sql", "000029_im_answer_feedback.up.sql"} {
		sql, err := os.ReadFile(filepath.Join("../../migrations/sqlite", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestFeedbackSaveIdempotenceIsolationAndWithdrawal(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	vote := types.AnswerFeedback{TenantID: 1, UserID: "wecom:u", MessageID: "m", SessionID: "s", Rating: "not_useful", Source: "wecom", LastEventID: "e1", LastEventAt: 100, Reason: "内容有错误"}
	save := func(v types.AnswerFeedback) types.AnswerFeedback {
		t.Helper()
		if err := Save(ctx, db, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	first := save(vote)
	if err := db.Model(&first).Updates(map[string]any{"status": "resolved", "reviewer_id": "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if duplicate := save(vote); duplicate.Status != "resolved" || duplicate.ID != first.ID {
		t.Fatalf("retry reset review: %+v", duplicate)
	}
	// Same vote at a newer time must advance the watermark without reopening review.
	vote.LastEventID, vote.LastEventAt = "e2", 200
	if same := save(vote); same.Status != "resolved" {
		t.Fatal("same vote reset review")
	}
	stale := vote
	stale.LastEventID, stale.LastEventAt, stale.Rating = "old", 150, "useful"
	if got := save(stale); got.Rating != "not_useful" {
		t.Fatal("stale event changed vote")
	}
	vote.LastEventID, vote.LastEventAt, vote.Status, vote.Rating = "cancel", 300, "withdrawn", ""
	if got := save(vote); got.Status != "withdrawn" || got.Rating != "" {
		t.Fatal("cancel failed")
	}
	vote.LastEventID, vote.LastEventAt, vote.Status, vote.Rating = "new", 400, "pending", "useful"
	if got := save(vote); got.Status != "pending" {
		t.Fatal("new vote did not reopen")
	}
	vote.UserID = "wecom:other"
	if got := save(vote); got.ID == first.ID {
		t.Fatal("different users merged")
	}
	vote.TenantID = 2
	if got := save(vote); got.ID == first.ID {
		t.Fatal("different tenants merged")
	}
	var count int64
	db.Model(&types.AnswerFeedback{}).Count(&count)
	if count != 3 {
		t.Fatalf("rows=%d", count)
	}
}

func TestFeedbackWebRetryPreservesReview(t *testing.T) {
	db := testDB(t)
	v := types.AnswerFeedback{TenantID: 1, UserID: "u", MessageID: "m", SessionID: "s", Rating: "useful"}
	if err := Save(context.Background(), db, &v); err != nil {
		t.Fatal(err)
	}
	db.Model(&v).Update("status", "ignored")
	v.Status = "pending"
	if err := Save(context.Background(), db, &v); err != nil {
		t.Fatal(err)
	}
	if v.Status != "ignored" {
		t.Fatal("duplicate vote reopened review")
	}
}
