package chatpipeline

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestKnowledgePreviewIsIsolatedFromLiveRetrieval(t *testing.T) {
	old := &types.SearchResult{KnowledgeID: "old", Content: "旧知识"}
	other := &types.SearchResult{KnowledgeID: "other", Content: "其他资料"}
	original := []*types.SearchResult{old, other}
	cm := &types.ChatManage{}
	cm.MergeResult = original
	applyKnowledgePreview(cm)
	if len(cm.MergeResult) != 2 {
		t.Fatal("ordinary retrieval modified")
	}
	cm.KnowledgePreview = &types.KnowledgePreview{Candidate: &types.SearchResult{KnowledgeID: "preview:p", Content: "待审核"}, ExcludeKnowledgeID: "old"}
	applyKnowledgePreview(cm)
	if len(cm.MergeResult) != 2 || cm.MergeResult[0].KnowledgeID != "preview:p" || cm.MergeResult[1] != other {
		t.Fatal("preview replacement incorrect")
	}
	if original[0] != old || old.Content != "旧知识" {
		t.Fatal("shared retrieval mutated")
	}
}

func TestKnowledgePreviewGeneratesWithNoExistingSearchMatch(t *testing.T) {
	p := &PluginSearchParallel{searchPlugin: &PluginSearch{}}
	cm := &types.ChatManage{}
	cm.KnowledgeOnly = true
	called := false
	next := func() *PluginError { called = true; return nil }
	if err := p.OnEvent(context.Background(), types.CHUNK_SEARCH_PARALLEL, cm, next); err != ErrSearchNothing || called {
		t.Fatal("live no-match behavior changed")
	}
	cm.KnowledgePreview = &types.KnowledgePreview{Candidate: &types.SearchResult{KnowledgeID: "preview:new", Content: "新知识"}}
	filter := &PluginFilterTopK{}
	if err := p.OnEvent(context.Background(), types.CHUNK_SEARCH_PARALLEL, cm, func() *PluginError { return filter.OnEvent(context.Background(), types.FILTER_TOP_K, cm, next) }); err != nil {
		t.Fatal(err)
	}
	if !called || len(cm.MergeResult) != 1 || cm.MergeResult[0].KnowledgeID != "preview:new" {
		t.Fatal("preview with new knowledge did not reach generation")
	}
}
