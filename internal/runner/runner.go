package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner struct {
	path    string
	timeout time.Duration
}

func New(path string) *Runner {
	return &Runner{
		path:    path,
		timeout: 5 * time.Minute,
	}
}

func (r *Runner) SetTimeout(timeout time.Duration) {
	r.timeout = timeout
}

type TestResult struct {
	Success  bool
	Output   string
	Duration time.Duration
	Command  string
	ExitCode int
}

type LintResult struct {
	Success  bool
	Output   string
	Issues   []LintIssue
	Duration time.Duration
}

type LintIssue struct {
	File     string
	Line     int
	Column   int
	Message  string
	Severity string
}

func (r *Runner) RunTests() (*TestResult, error) {
	detector := r.detectProjectType()
	testCmd := r.getTestCommand(detector)

	if testCmd == nil {
		return &TestResult{
			Success: true,
			Output:  "No test command detected for this project type",
		}, nil
	}

	return r.execute(testCmd)
}

func (r *Runner) RunLint() (*LintResult, error) {
	detector := r.detectProjectType()
	lintCmd := r.getLintCommand(detector)

	if lintCmd == nil {
		return &LintResult{
			Success: true,
			Output:  "No lint command detected for this project type",
		}, nil
	}

	result, err := r.execute(lintCmd)
	if err != nil {
		return nil, err
	}

	lintResult := &LintResult{
		Success:  result.Success,
		Output:   result.Output,
		Duration: result.Duration,
	}

	lintResult.Issues = r.parseLintOutput(result.Output, detector)

	return lintResult, nil
}

func (r *Runner) execute(cmd []string) (*TestResult, error) {
	start := time.Now()

	ctx := exec.Command(cmd[0], cmd[1:]...)
	ctx.Dir = r.path

	var stdout, stderr bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &stderr

	err := ctx.Run()
	duration := time.Since(start)

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run command: %w", err)
		}
	}

	return &TestResult{
		Success:  exitCode == 0,
		Output:   output,
		Duration: duration,
		Command:  strings.Join(cmd, " "),
		ExitCode: exitCode,
	}, nil
}

type projectType string

const (
	projectPython     projectType = "python"
	projectJavaScript projectType = "javascript"
	projectTypeScript projectType = "typescript"
	projectGo         projectType = "go"
	projectUnknown    projectType = "unknown"
)

func (r *Runner) detectProjectType() projectType {
	if r.fileExists("go.mod") {
		return projectGo
	}

	if r.fileExists("pyproject.toml") || r.fileExists("setup.py") || r.fileExists("requirements.txt") {
		return projectPython
	}

	if r.fileExists("package.json") {
		pkg := r.readPackageJSON()
		if strings.Contains(pkg, "typescript") {
			return projectTypeScript
		}
		return projectJavaScript
	}

	return projectUnknown
}

func (r *Runner) fileExists(name string) bool {
	_, err := os.Stat(filepath.Join(r.path, name))
	return err == nil
}

func (r *Runner) readPackageJSON() string {
	content, err := os.ReadFile(filepath.Join(r.path, "package.json"))
	if err != nil {
		return ""
	}
	return string(content)
}

func (r *Runner) getTestCommand(pt projectType) []string {
	switch pt {
	case projectPython:
		if r.fileExists("pytest.ini") || r.fileExists("pyproject.toml") {
			return []string{"python", "-m", "pytest", "-x", "-q"}
		}
		return []string{"python", "-m", "unittest", "discover", "-v"}
	case projectJavaScript, projectTypeScript:
		return []string{"npm", "test"}
	case projectGo:
		return []string{"go", "test", "./..."}
	default:
		return nil
	}
}

func (r *Runner) getLintCommand(pt projectType) []string {
	switch pt {
	case projectPython:
		if _, err := exec.LookPath("ruff"); err == nil {
			return []string{"ruff", "check", "."}
		}
		if _, err := exec.LookPath("flake8"); err == nil {
			return []string{"flake8", "."}
		}
		return nil
	case projectJavaScript, projectTypeScript:
		return []string{"npm", "run", "lint"}
	case projectGo:
		if _, err := exec.LookPath("golangci-lint"); err == nil {
			return []string{"golangci-lint", "run"}
		}
		return []string{"go", "vet", "./..."}
	default:
		return nil
	}
}

func (r *Runner) parseLintOutput(output string, pt projectType) []LintIssue {
	var issues []LintIssue

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		switch pt {
		case projectPython:
			issues = append(issues, r.parsePythonLintLine(line)...)
		case projectGo:
			issues = append(issues, r.parseGoLintLine(line)...)
		case projectJavaScript, projectTypeScript:
			issues = append(issues, r.parseJSLintLine(line)...)
		}
	}

	return issues
}

func (r *Runner) parsePythonLintLine(line string) []LintIssue {
	parts := strings.Split(line, ":")
	if len(parts) < 3 {
		return nil
	}

	file := strings.TrimSpace(parts[0])
	lineNum := 0
	fmt.Sscanf(parts[1], "%d", &lineNum)

	message := ""
	if len(parts) > 3 {
		message = strings.TrimSpace(strings.Join(parts[2:], ":"))
	} else {
		message = strings.TrimSpace(parts[2])
	}

	return []LintIssue{{
		File:     file,
		Line:     lineNum,
		Message:  message,
		Severity: "warning",
	}}
}

func (r *Runner) parseGoLintLine(line string) []LintIssue {
	parts := strings.Split(line, ":")
	if len(parts) < 3 {
		return nil
	}

	file := strings.TrimSpace(parts[0])
	lineNum := 0
	fmt.Sscanf(parts[1], "%d", &lineNum)

	message := ""
	if len(parts) > 3 {
		message = strings.TrimSpace(strings.Join(parts[2:], ":"))
	} else {
		message = strings.TrimSpace(parts[2])
	}

	return []LintIssue{{
		File:     file,
		Line:     lineNum,
		Message:  message,
		Severity: "warning",
	}}
}

func (r *Runner) parseJSLintLine(line string) []LintIssue {
	return []LintIssue{{
		Message:  line,
		Severity: "warning",
	}}
}

func (r *Runner) Build() (*TestResult, error) {
	pt := r.detectProjectType()

	switch pt {
	case projectGo:
		return r.execute([]string{"go", "build", "./..."})
	case projectJavaScript, projectTypeScript:
		return r.execute([]string{"npm", "run", "build"})
	case projectPython:
		return &TestResult{Success: true, Output: "Python does not require build step"}, nil
	default:
		return &TestResult{Success: true, Output: "No build step required"}, nil
	}
}
