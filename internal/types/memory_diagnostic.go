package types

// MemoryDiagnostic contains no remembered text. It is safe for the workspace
// operations view; reading personal content uses a separate audited endpoint.
type MemoryDiagnostic struct {
	Reason         string `json:"reason"`
	Mode           string `json:"mode"`
	Used           int    `json:"used"`
	Candidates     int    `json:"candidates"`
	VectorFallback string `json:"vector_fallback,omitempty"`
}

type MemoryAdminGrant struct {
	TenantID  uint64 `json:"tenant_id" gorm:"primaryKey;autoIncrement:false"`
	UserID    string `json:"user_id" gorm:"primaryKey;type:varchar(36)"`
	GrantedBy string `json:"granted_by"`
}
