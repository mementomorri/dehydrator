package models

import "time"

type FileInfo struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash,omitempty"`
}

type Language string

const (
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageGo         Language = "go"
	LanguageUnknown    Language = "unknown"
)

type Symbol struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	File       string   `json:"file"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Signature  string   `json:"signature,omitempty"`
	References []string `json:"references,omitempty"`
}

type ComplexityMetrics struct {
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	CognitiveComplexity  int     `json:"cognitive_complexity"`
	LinesOfCode          int     `json:"lines_of_code"`
	MaintainabilityIndex float64 `json:"maintainability_index"`
	HalsteadDifficulty   float64 `json:"halstead_difficulty"`
}

type CodeBlock struct {
	ID         string            `json:"id"`
	File       string            `json:"file"`
	StartLine  int               `json:"start_line"`
	EndLine    int               `json:"end_line"`
	Content    string            `json:"content"`
	Language   Language          `json:"language"`
	SymbolType string            `json:"symbol_type"`
	SymbolName string            `json:"symbol_name"`
	Metrics    ComplexityMetrics `json:"metrics"`
	Embedding  []float32         `json:"embedding,omitempty"`
}

type DuplicateGroup struct {
	ID           string      `json:"id"`
	Blocks       []CodeBlock `json:"blocks"`
	Similarity   float64     `json:"similarity"`
	SuggestedFix string      `json:"suggested_fix,omitempty"`
}

type RefactorPlan struct {
	SessionID   string       `json:"session_id"`
	Changes     []FileChange `json:"changes"`
	Description string       `json:"description"`
	Pattern     string       `json:"pattern,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

type FileChange struct {
	Path        string `json:"path"`
	Original    string `json:"original"`
	Modified    string `json:"modified"`
	Description string `json:"description"`
}

type RefactorResult struct {
	SessionID     string            `json:"session_id"`
	Success       bool              `json:"success"`
	Changes       []FileChange      `json:"changes"`
	TestsPassed   bool              `json:"tests_passed"`
	Error         string            `json:"error,omitempty"`
	MetricsBefore ComplexityMetrics `json:"metrics_before"`
	MetricsAfter  ComplexityMetrics `json:"metrics_after"`
}

type Report struct {
	SessionID       string           `json:"session_id"`
	GeneratedAt     time.Time        `json:"generated_at"`
	LOCBefore       int              `json:"loc_before"`
	LOCAfter        int              `json:"loc_after"`
	LOCReduced      int              `json:"loc_reduced"`
	DuplicatesFound int              `json:"duplicates_found"`
	PatternsApplied []PatternApplied `json:"patterns_applied"`
	FilesModified   []string         `json:"files_modified"`
	MetricsDelta    MetricsDelta     `json:"metrics_delta"`
}

type PatternApplied struct {
	Pattern     string   `json:"pattern"`
	Files       []string `json:"files"`
	Description string   `json:"description"`
}

type MetricsDelta struct {
	CyclomaticComplexityDelta int     `json:"cyclomatic_complexity_delta"`
	CognitiveComplexityDelta  int     `json:"cognitive_complexity_delta"`
	MaintainabilityIndexDelta float64 `json:"maintainability_index_delta"`
}
