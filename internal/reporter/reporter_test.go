package reporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexkarsten/reducto/pkg/models"
)

func TestNew(t *testing.T) {
	cfg := &models.Config{}
	r := New(cfg)

	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.cfg != cfg {
		t.Error("config not set correctly")
	}
	if r.outputDir != ".reducto" {
		t.Errorf("expected outputDir .reducto, got %s", r.outputDir)
	}
}

func TestGenerate(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &models.Config{}
	r := New(cfg)
	r.outputDir = filepath.Join(tmpDir, ".reducto")

	result := &models.RefactorResult{
		SessionID: "test-session-123",
		Changes: []models.FileChange{
			{
				Path:        "test.py",
				Description: "Refactored function",
				Original:    "def old():\n    pass\n",
				Modified:    "def new():\n    pass\n",
			},
		},
		MetricsBefore: models.ComplexityMetrics{
			LinesOfCode:          100,
			CyclomaticComplexity: 10,
			CognitiveComplexity:  15,
			MaintainabilityIndex: 70.0,
		},
		MetricsAfter: models.ComplexityMetrics{
			LinesOfCode:          80,
			CyclomaticComplexity: 8,
			CognitiveComplexity:  12,
			MaintainabilityIndex: 75.0,
		},
	}

	err := r.Generate(result)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".reducto", "reducto-report-test-session-123.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected report file at %s", expectedPath)
	}

	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read report: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test-session-123") {
		t.Error("report should contain session ID")
	}
	if !strings.Contains(contentStr, "100") || !strings.Contains(contentStr, "80") {
		t.Error("report should contain LOC before and after")
	}
}

func TestGenerateBaseline(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &models.Config{}
	r := New(cfg)
	r.outputDir = filepath.Join(tmpDir, ".reducto")

	result := &BaselineResult{
		TotalFiles:   10,
		TotalSymbols: 50,
		Hotspots: []ComplexityHotspot{
			{
				File:                 "complex.py",
				Line:                 10,
				Symbol:               "process_data",
				CyclomaticComplexity: 15,
				CognitiveComplexity:  20,
			},
		},
	}

	err := r.GenerateBaseline(result)
	if err != nil {
		t.Fatalf("GenerateBaseline returned error: %v", err)
	}

	entries, err := os.ReadDir(r.outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "reducto-baseline-") {
			found = true
			content, err := os.ReadFile(filepath.Join(r.outputDir, entry.Name()))
			if err != nil {
				t.Fatalf("failed to read baseline: %v", err)
			}

			contentStr := string(content)
			if !strings.Contains(contentStr, "10") {
				t.Error("baseline should contain total files")
			}
			if !strings.Contains(contentStr, "complex.py") {
				t.Error("baseline should contain hotspot file")
			}
			break
		}
	}

	if !found {
		t.Error("expected baseline report file to be created")
	}
}

func TestFormatBaselineMarkdown(t *testing.T) {
	cfg := &models.Config{}
	r := New(cfg)

	result := &BaselineResult{
		TotalFiles:   5,
		TotalSymbols: 25,
		Hotspots: []ComplexityHotspot{
			{
				File:                 "test.py",
				Line:                 10,
				Symbol:               "test_func",
				CyclomaticComplexity: 8,
				CognitiveComplexity:  10,
			},
		},
	}

	content := r.formatBaselineMarkdown("test-session", result)

	if !strings.Contains(content, "# reducto Baseline Analysis Report") {
		t.Error("should contain title")
	}
	if !strings.Contains(content, "| Total Files | 5 |") {
		t.Error("should contain total files")
	}
	if !strings.Contains(content, "| Total Symbols | 25 |") {
		t.Error("should contain total symbols")
	}
	if !strings.Contains(content, "test.py") {
		t.Error("should contain hotspot file")
	}
	if !strings.Contains(content, "test_func") {
		t.Error("should contain hotspot symbol")
	}
}

func TestFormatMarkdown(t *testing.T) {
	cfg := &models.Config{}
	r := New(cfg)

	report := &models.Report{
		SessionID:     "test-123",
		GeneratedAt:   time.Now(),
		LOCBefore:     100,
		LOCAfter:      80,
		LOCReduced:    20,
		FilesModified: []string{"test.py", "main.py"},
		MetricsDelta: models.MetricsDelta{
			CyclomaticComplexityDelta: 2,
			CognitiveComplexityDelta:  3,
			MaintainabilityIndexDelta: 5.0,
		},
	}

	result := &models.RefactorResult{
		SessionID: "test-123",
		Changes: []models.FileChange{
			{
				Path:        "test.py",
				Description: "Simplified function",
				Original:    "def old():\n    return 1\n",
				Modified:    "def new():\n    return 1\n",
			},
		},
		MetricsBefore: models.ComplexityMetrics{
			LinesOfCode:          100,
			CyclomaticComplexity: 10,
			CognitiveComplexity:  15,
			MaintainabilityIndex: 70.0,
		},
		MetricsAfter: models.ComplexityMetrics{
			LinesOfCode:          80,
			CyclomaticComplexity: 8,
			CognitiveComplexity:  12,
			MaintainabilityIndex: 75.0,
		},
	}

	content := r.formatMarkdown(report, result)

	if !strings.Contains(content, "# reducto Compression Report") {
		t.Error("should contain title")
	}
	if !strings.Contains(content, "test-123") {
		t.Error("should contain session ID")
	}
	if !strings.Contains(content, "test.py") {
		t.Error("should contain modified file")
	}
	if !strings.Contains(content, "Simplified function") {
		t.Error("should contain change description")
	}
}

func TestExtractModifiedFiles(t *testing.T) {
	cfg := &models.Config{}
	r := New(cfg)

	changes := []models.FileChange{
		{Path: "test.py"},
		{Path: "main.py"},
		{Path: "test.py"},
		{Path: "utils.py"},
	}

	files := r.extractModifiedFiles(changes)

	if len(files) != 3 {
		t.Errorf("expected 3 unique files, got %d", len(files))
	}

	seen := make(map[string]bool)
	for _, f := range files {
		if seen[f] {
			t.Errorf("duplicate file %s in result", f)
		}
		seen[f] = true
	}
}

func TestGenerateDiff(t *testing.T) {
	cfg := &models.Config{}
	r := New(cfg)

	tests := []struct {
		name     string
		original string
		modified string
		wantDiff bool
	}{
		{
			name:     "identical content",
			original: "line1\nline2\n",
			modified: "line1\nline2\n",
			wantDiff: false,
		},
		{
			name:     "modified line",
			original: "line1\nline2\n",
			modified: "line1\nmodified\n",
			wantDiff: true,
		},
		{
			name:     "added line",
			original: "line1\n",
			modified: "line1\nline2\n",
			wantDiff: true,
		},
		{
			name:     "removed line",
			original: "line1\nline2\n",
			modified: "line1\n",
			wantDiff: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := r.generateDiff(tt.original, tt.modified)

			if tt.wantDiff {
				if !strings.Contains(diff, "-") && !strings.Contains(diff, "+") {
					t.Error("expected diff to contain changes")
				}
			} else {
				if strings.Contains(diff, "-") || strings.Contains(diff, "+") {
					t.Error("expected no diff for identical content")
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("no reports", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &models.Config{}
		r := New(cfg)
		r.outputDir = tmpDir

		err := r.Load("")
		if err == nil {
			t.Error("expected error when no reports exist")
		}
	})

	t.Run("with report", func(t *testing.T) {
		tmpDir := t.TempDir()

		reportContent := "# reducto Compression Report\n\nSession: test-123\n"
		reportPath := filepath.Join(tmpDir, "reducto-report-test-123.md")
		err := os.WriteFile(reportPath, []byte(reportContent), 0644)
		if err != nil {
			t.Fatalf("failed to create test report: %v", err)
		}

		cfg := &models.Config{}
		r := New(cfg)
		r.outputDir = tmpDir

		err = r.Load("test-123")
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
	})

	t.Run("latest report", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldReport := "# reducto Compression Report\n\nSession: old\n"
		oldPath := filepath.Join(tmpDir, "reducto-report-old.md")
		err := os.WriteFile(oldPath, []byte(oldReport), 0644)
		if err != nil {
			t.Fatalf("failed to create old report: %v", err)
		}

		time.Sleep(10 * time.Millisecond)

		newReport := "# reducto Compression Report\n\nSession: new\n"
		newPath := filepath.Join(tmpDir, "reducto-report-new.md")
		err = os.WriteFile(newPath, []byte(newReport), 0644)
		if err != nil {
			t.Fatalf("failed to create new report: %v", err)
		}

		cfg := &models.Config{}
		r := New(cfg)
		r.outputDir = tmpDir

		err = r.Load("")
		if err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
	})
}
