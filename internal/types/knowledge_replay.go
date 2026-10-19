package types

// KnowledgePreview is internal-only. HTTP QA requests cannot supply it. It
// overlays one reviewed candidate after normal retrieval for a dry run, without
// writing to the knowledge index. Published verification never sets this field.
type KnowledgePreview struct {
	Candidate          *SearchResult
	ExcludeKnowledgeID string
}
