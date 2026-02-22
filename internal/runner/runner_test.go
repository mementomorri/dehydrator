package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	r := New("/tmp")
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.path != "/tmp" {
		t.Errorf("expected path /tmp, got %s", r.path)
	}
	if r.timeout != 5*time.Minute {
		t.Errorf("expected default timeout 5m, got %v", r.timeout)
	}
}

func TestSetTimeout(t *testing.T) {
	r := New("/tmp")
	r.SetTimeout(10 * time.Second)
	if r.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", r.timeout)
	}
}

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected projectType
	}{
		{
			name:     "go project",
			files:    map[string]string{"go.mod": "module test"},
			expected: projectGo,
		},
		{
			name:     "python project with pyproject.toml",
			files:    map[string]string{"pyproject.toml": "[project]"},
			expected: projectPython,
		},
		{
			name:     "python project with setup.py",
			files:    map[string]string{"setup.py": "# setup"},
			expected: projectPython,
		},
		{
			name:     "python project with requirements.txt",
			files:    map[string]string{"requirements.txt": "pytest"},
			expected: projectPython,
		},
		{
			name:     "javascript project",
			files:    map[string]string{"package.json": `{"name": "test"}`},
			expected: projectJavaScript,
		},
		{
			name:     "typescript project",
			files:    map[string]string{"package.json": `{"devDependencies": {"typescript": "^4.0.0"}}`},
			expected: projectTypeScript,
		},
		{
			name:     "unknown project",
			files:    map[string]string{},
			expected: projectUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to create %s: %v", name, err)
				}
			}

			r := New(tmpDir)
			result := r.detectProjectType()

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "exists.txt"), []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	r := New(tmpDir)

	if !r.fileExists("exists.txt") {
		t.Error("expected exists.txt to exist")
	}

	if r.fileExists("nonexistent.txt") {
		t.Error("expected nonexistent.txt to not exist")
	}
}

func TestReadPackageJSON(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `{"name": "test-project", "version": "1.0.0"}`

		err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create package.json: %v", err)
		}

		r := New(tmpDir)
		result := r.readPackageJSON()

		if result != content {
			t.Errorf("expected %s, got %s", content, result)
		}
	})

	t.Run("file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := New(tmpDir)
		result := r.readPackageJSON()

		if result != "" {
			t.Errorf("expected empty string, got %s", result)
		}
	})
}

func TestGetTestCommand(t *testing.T) {
	tests := []struct {
		name     string
		pt       projectType
		expected []string
	}{
		{
			name:     "go",
			pt:       projectGo,
			expected: []string{"go", "test", "./..."},
		},
		{
			name:     "javascript",
			pt:       projectJavaScript,
			expected: []string{"npm", "test"},
		},
		{
			name:     "typescript",
			pt:       projectTypeScript,
			expected: []string{"npm", "test"},
		},
		{
			name:     "unknown",
			pt:       projectUnknown,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			r := New(tmpDir)

			result := r.getTestCommand(tt.pt)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}

			for i, cmd := range result {
				if cmd != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestGetTestCommandPython(t *testing.T) {
	t.Run("with pytest.ini", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "pytest.ini"), []byte("[pytest]"), 0644)
		if err != nil {
			t.Fatalf("failed to create pytest.ini: %v", err)
		}

		r := New(tmpDir)
		result := r.getTestCommand(projectPython)

		expected := []string{"python", "-m", "pytest", "-x", "-q"}
		if len(result) != len(expected) {
			t.Errorf("expected %v, got %v", expected, result)
			return
		}

		for i, cmd := range result {
			if cmd != expected[i] {
				t.Errorf("expected %v, got %v", expected, result)
				return
			}
		}
	})

	t.Run("without pytest.ini", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := New(tmpDir)
		result := r.getTestCommand(projectPython)

		expected := []string{"python", "-m", "unittest", "discover", "-v"}
		if len(result) != len(expected) {
			t.Errorf("expected %v, got %v", expected, result)
			return
		}

		for i, cmd := range result {
			if cmd != expected[i] {
				t.Errorf("expected %v, got %v", expected, result)
				return
			}
		}
	})
}

func TestGetLintCommand(t *testing.T) {
	tests := []struct {
		name     string
		pt       projectType
		expected []string
	}{
		{
			name:     "javascript",
			pt:       projectJavaScript,
			expected: []string{"npm", "run", "lint"},
		},
		{
			name:     "typescript",
			pt:       projectTypeScript,
			expected: []string{"npm", "run", "lint"},
		},
		{
			name:     "unknown",
			pt:       projectUnknown,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			r := New(tmpDir)

			result := r.getLintCommand(tt.pt)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}

			for i, cmd := range result {
				if cmd != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestParsePythonLintLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected LintIssue
	}{
		{
			name: "standard format",
			line: "file.py:10: E501 line too long",
			expected: LintIssue{
				File:     "file.py",
				Line:     10,
				Message:  " E501 line too long",
				Severity: "warning",
			},
		},
		{
			name: "with column",
			line: "test.py:5:3: invalid syntax",
			expected: LintIssue{
				File:     "test.py",
				Line:     5,
				Message:  "3: invalid syntax",
				Severity: "warning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("/tmp")
			result := r.parsePythonLintLine(tt.line)

			if len(result) == 0 {
				t.Fatal("expected at least one issue")
			}

			if result[0].File != tt.expected.File {
				t.Errorf("expected file %s, got %s", tt.expected.File, result[0].File)
			}
			if result[0].Line != tt.expected.Line {
				t.Errorf("expected line %d, got %d", tt.expected.Line, result[0].Line)
			}
		})
	}
}

func TestParseGoLintLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected LintIssue
	}{
		{
			name: "standard format",
			line: "file.go:10: unused variable",
			expected: LintIssue{
				File:     "file.go",
				Line:     10,
				Message:  " unused variable",
				Severity: "warning",
			},
		},
		{
			name: "with column",
			line: "test.go:5:3: syntax error",
			expected: LintIssue{
				File:     "test.go",
				Line:     5,
				Message:  "3: syntax error",
				Severity: "warning",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New("/tmp")
			result := r.parseGoLintLine(tt.line)

			if len(result) == 0 {
				t.Fatal("expected at least one issue")
			}

			if result[0].File != tt.expected.File {
				t.Errorf("expected file %s, got %s", tt.expected.File, result[0].File)
			}
			if result[0].Line != tt.expected.Line {
				t.Errorf("expected line %d, got %d", tt.expected.Line, result[0].Line)
			}
		})
	}
}

func TestRunTests(t *testing.T) {
	t.Run("no test command", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := New(tmpDir)

		result, err := r.RunTests()
		if err != nil {
			t.Fatalf("RunTests returned error: %v", err)
		}

		if !result.Success {
			t.Error("expected success when no test command")
		}
	})
}

func TestRunLint(t *testing.T) {
	t.Run("no lint command", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := New(tmpDir)

		result, err := r.RunLint()
		if err != nil {
			t.Fatalf("RunLint returned error: %v", err)
		}

		if !result.Success {
			t.Error("expected success when no lint command")
		}
	})
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		wantSuccess bool
	}{
		{
			name:        "unknown project",
			files:       map[string]string{},
			wantSuccess: true,
		},
		{
			name:        "python project",
			files:       map[string]string{"pyproject.toml": "[project]"},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to create %s: %v", name, err)
				}
			}

			r := New(tmpDir)
			result, err := r.Build()
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("expected success %v, got %v", tt.wantSuccess, result.Success)
			}
		})
	}
}
