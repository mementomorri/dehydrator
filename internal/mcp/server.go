package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alexkarsten/reducto/internal/git"
	"github.com/alexkarsten/reducto/internal/lsp"
	"github.com/alexkarsten/reducto/internal/runner"
	"github.com/alexkarsten/reducto/internal/walker"
	"github.com/alexkarsten/reducto/pkg/models"
)

type Server struct {
	rootDir string
	walker  *walker.Walker
	runner  *runner.Runner
	gitMgr  *git.Manager
	lspMgr  *lsp.Manager

	mu       sync.RWMutex
	sessions map[string]*Session
}

type Session struct {
	ID         string
	Checkpoint string
	Files      []models.FileInfo
	Symbols    map[string][]models.Symbol
}

func NewServer(rootDir string) *Server {
	return &Server{
		rootDir:  rootDir,
		walker:   walker.New(nil, nil),
		runner:   runner.New(rootDir),
		gitMgr:   git.NewManager(rootDir),
		lspMgr:   lsp.NewManager(),
		sessions: make(map[string]*Session),
	}
}

func (s *Server) InitLSP(ctx context.Context) error {
	languages := []string{}
	hasGo := false
	hasPython := false
	hasTypeScript := false

	files, _ := s.walker.Walk(s.rootDir)
	for _, f := range files {
		lang := s.walker.DetectLanguage(f.Path)
		switch lang {
		case models.LanguageGo:
			if !hasGo {
				hasGo = true
				languages = append(languages, "go")
			}
		case models.LanguagePython:
			if !hasPython {
				hasPython = true
				languages = append(languages, "python")
			}
		case models.LanguageTypeScript, models.LanguageJavaScript:
			if !hasTypeScript {
				hasTypeScript = true
				languages = append(languages, "typescript")
			}
		}
	}

	if hasGo {
		client, err := lsp.NewGoClient(s.rootDir)
		if err == nil {
			s.lspMgr.Register("go", client)
		}
	}

	if hasPython {
		client, err := lsp.NewPythonClient(s.rootDir)
		if err == nil {
			s.lspMgr.Register("python", client)
		}
	}

	if hasTypeScript {
		client, err := lsp.NewTypeScriptClient(s.rootDir)
		if err == nil {
			s.lspMgr.Register("typescript", client)
		}
	}

	absPath, _ := filepath.Abs(s.rootDir)
	rootURI := "file://" + absPath
	return s.lspMgr.Initialize(ctx, rootURI, languages)
}

func (s *Server) Start(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	encoder := json.NewEncoder(stdout)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		req, err := ParseRequest(line)
		if err != nil {
			encoder.Encode(ErrorResponse(nil, ParseError, err.Error(), nil))
			continue
		}

		resp := s.handleRequest(ctx, req)
		if resp != nil {
			encoder.Encode(resp)
		}
	}

	return scanner.Err()
}

func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
	handler, ok := s.getHandler(req.Method)
	if !ok {
		return ErrorResponse(req.ID, MethodNotFound, "Method not found", req.Method)
	}

	result, err := handler(ctx, req.Params)
	if err != nil {
		if rpcErr, ok := err.(*ErrorObject); ok {
			return ErrorResponse(req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		}
		return ErrorResponse(req.ID, InternalError, err.Error(), nil)
	}

	return SuccessResponse(req.ID, result)
}

type HandlerFunc func(ctx context.Context, params json.RawMessage) (interface{}, error)

func (s *Server) getHandler(method string) (HandlerFunc, bool) {
	handlers := map[string]HandlerFunc{
		"initialize":      s.handleInitialize,
		"shutdown":        s.handleShutdown,
		"read_file":       s.handleReadFile,
		"get_symbols":     s.handleGetSymbols,
		"get_ast":         s.handleGetAST,
		"find_references": s.handleFindReferences,
		"apply_diff":      s.handleApplyDiff,
		"apply_diff_safe": s.handleApplyDiffSafe,
		"run_tests":       s.handleRunTests,
		"git_checkpoint":  s.handleGitCheckpoint,
		"git_rollback":    s.handleGitRollback,
		"list_files":      s.handleListFiles,
		"get_complexity":  s.handleGetComplexity,
	}
	h, ok := handlers[method]
	return h, ok
}

func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"status":  "initialized",
		"version": "0.1.0",
		"tools": []string{
			"read_file", "get_symbols", "get_ast", "find_references",
			"apply_diff", "apply_diff_safe", "run_tests", "git_checkpoint", "git_rollback",
			"list_files", "get_complexity",
		},
	}, nil
}

func (s *Server) handleShutdown(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]string{"status": "shutdown"}, nil
}

func (s *Server) handleReadFile(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	fullPath := filepath.Join(s.rootDir, input.Path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, NewError(FileNotFound, "Failed to read file", err.Error())
	}

	hash := sha256.Sum256(content)
	lang := s.walker.DetectLanguage(input.Path)

	return map[string]interface{}{
		"path":     input.Path,
		"content":  string(content),
		"hash":     hex.EncodeToString(hash[:]),
		"language": lang,
	}, nil
}

func (s *Server) handleGetSymbols(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content,omitempty"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	var content string
	if input.Content != "" {
		content = input.Content
	} else {
		fullPath := filepath.Join(s.rootDir, input.Path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, NewError(FileNotFound, "Failed to read file", err.Error())
		}
		content = string(data)
	}

	lang := s.walker.DetectLanguage(input.Path)

	symbols := s.extractSymbols(content, input.Path, lang)

	return map[string]interface{}{
		"path":    input.Path,
		"symbols": symbols,
	}, nil
}

func (s *Server) extractSymbols(content, path string, lang models.Language) []models.Symbol {
	var symbols []models.Symbol
	lines := strings.Split(content, "\n")

	switch lang {
	case models.LanguagePython:
		symbols = s.extractPythonSymbols(lines, path)
	case models.LanguageJavaScript, models.LanguageTypeScript:
		symbols = s.extractJSSymbols(lines, path)
	case models.LanguageGo:
		symbols = s.extractGoSymbols(lines, path)
	}

	return symbols
}

func (s *Server) extractPythonSymbols(lines []string, path string) []models.Symbol {
	var symbols []models.Symbol
	var currentClass string

	for i, line := range lines {
		stripped := strings.TrimSpace(line)

		if strings.HasPrefix(stripped, "def ") || strings.HasPrefix(stripped, "async def ") {
			name := s.extractFunctionName(stripped)
			symbolType := "function"
			if currentClass != "" {
				symbolType = "method"
			}
			symbols = append(symbols, models.Symbol{
				Name:      name,
				Type:      symbolType,
				File:      path,
				StartLine: i + 1,
				EndLine:   s.findPythonBlockEnd(lines, i),
			})
		} else if strings.HasPrefix(stripped, "class ") {
			name := s.extractClassName(stripped)
			currentClass = name
			symbols = append(symbols, models.Symbol{
				Name:      name,
				Type:      "class",
				File:      path,
				StartLine: i + 1,
				EndLine:   s.findPythonBlockEnd(lines, i),
			})
		} else if stripped != "" && !strings.HasPrefix(stripped, "#") && !strings.HasPrefix(stripped, "@") {
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
			} else {
				currentClass = ""
			}
		}
	}

	return symbols
}

func (s *Server) extractJSSymbols(lines []string, path string) []models.Symbol {
	var symbols []models.Symbol

	for i, line := range lines {
		stripped := strings.TrimSpace(line)

		if strings.Contains(stripped, "function ") || strings.Contains(stripped, "=>") {
			name := s.extractJSFunctionName(stripped)
			if name != "" {
				symbols = append(symbols, models.Symbol{
					Name:      name,
					Type:      "function",
					File:      path,
					StartLine: i + 1,
					EndLine:   s.findBraceBlockEnd(lines, i),
				})
			}
		} else if strings.HasPrefix(stripped, "class ") {
			name := s.extractClassName(stripped)
			symbols = append(symbols, models.Symbol{
				Name:      name,
				Type:      "class",
				File:      path,
				StartLine: i + 1,
				EndLine:   s.findBraceBlockEnd(lines, i),
			})
		}
	}

	return symbols
}

func (s *Server) extractGoSymbols(lines []string, path string) []models.Symbol {
	var symbols []models.Symbol

	for i, line := range lines {
		stripped := strings.TrimSpace(line)

		if strings.HasPrefix(stripped, "func ") {
			name := s.extractGoFunctionName(stripped)
			symbols = append(symbols, models.Symbol{
				Name:      name,
				Type:      "function",
				File:      path,
				StartLine: i + 1,
				EndLine:   s.findBraceBlockEnd(lines, i),
			})
		} else if strings.HasPrefix(stripped, "type ") && strings.Contains(stripped, " struct") {
			name := s.extractGoTypeName(stripped)
			symbols = append(symbols, models.Symbol{
				Name:      name,
				Type:      "struct",
				File:      path,
				StartLine: i + 1,
				EndLine:   s.findBraceBlockEnd(lines, i),
			})
		}
	}

	return symbols
}

func (s *Server) extractFunctionName(line string) string {
	line = strings.TrimPrefix(line, "async ")
	line = strings.TrimPrefix(line, "def ")

	idx := strings.Index(line, "(")
	if idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.Fields(line)[0]
}

func (s *Server) extractClassName(line string) string {
	line = strings.TrimPrefix(line, "class ")

	for _, delim := range []string{"(", ":", "[", "{"} {
		if idx := strings.Index(line, delim); idx > 0 {
			return strings.TrimSpace(line[:idx])
		}
	}
	return strings.TrimSpace(line)
}

func (s *Server) extractJSFunctionName(line string) string {
	patterns := []string{"function ", "const ", "let ", "var ", "async "}
	for _, p := range patterns {
		if idx := strings.Index(line, p); idx >= 0 {
			rest := line[idx+len(p):]
			if nameIdx := strings.Index(rest, "("); nameIdx > 0 {
				name := strings.TrimSpace(rest[:nameIdx])
				name = strings.TrimSuffix(name, "=")
				name = strings.TrimSpace(name)
				return name
			}
		}
	}
	return ""
}

func (s *Server) extractGoFunctionName(line string) string {
	line = strings.TrimPrefix(line, "func ")

	if strings.HasPrefix(line, "(") {
		closeIdx := strings.Index(line, ")")
		if closeIdx > 0 {
			line = line[closeIdx+1:]
		}
	}

	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return strings.TrimSpace(line)
}

func (s *Server) extractGoTypeName(line string) string {
	line = strings.TrimPrefix(line, "type ")
	if idx := strings.Index(line, " struct"); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

func (s *Server) findPythonBlockEnd(lines []string, start int) int {
	if start >= len(lines) {
		return len(lines)
	}

	startIndent := len(lines[start]) - len(strings.TrimLeft(lines[start], " \t"))

	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if currentIndent <= startIndent {
			return i
		}
	}

	return len(lines)
}

func (s *Server) findBraceBlockEnd(lines []string, start int) int {
	braceCount := 0
	started := false

	for i := start; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				braceCount++
				started = true
			} else if ch == '}' {
				braceCount--
				if started && braceCount == 0 {
					return i + 1
				}
			}
		}
	}

	return len(lines)
}

func (s *Server) handleGetAST(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return nil, NewError(InternalError, "AST extraction not yet implemented with Tree-sitter", nil)
}

func (s *Server) handleFindReferences(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path   string `json:"path"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	if input.Line == 0 {
		input.Line = 1
	}
	if input.Column == 0 {
		input.Column = 0
	}

	lang := s.walker.DetectLanguage(input.Path)
	var language string
	switch lang {
	case models.LanguageGo:
		language = "go"
	case models.LanguagePython:
		language = "python"
	case models.LanguageTypeScript, models.LanguageJavaScript:
		language = "typescript"
	default:
		return map[string]interface{}{
			"references": []interface{}{},
		}, nil
	}

	absPath, err := filepath.Abs(filepath.Join(s.rootDir, input.Path))
	if err != nil {
		return nil, NewError(InternalError, "Failed to resolve path", err.Error())
	}
	uri := "file://" + absPath

	refs, err := s.lspMgr.FindReferences(ctx, language, uri, input.Line, input.Column)
	if err != nil {
		return map[string]interface{}{
			"references": []interface{}{},
			"error":      err.Error(),
		}, nil
	}

	references := make([]map[string]interface{}, len(refs))
	for i, ref := range refs {
		relPath := ref.URI
		if strings.HasPrefix(relPath, "file://") {
			relPath = strings.TrimPrefix(relPath, "file://")
			if rel, err := filepath.Rel(s.rootDir, relPath); err == nil {
				relPath = rel
			}
		}
		references[i] = map[string]interface{}{
			"file":   relPath,
			"line":   ref.Line,
			"column": ref.Column,
		}
	}

	return map[string]interface{}{
		"references": references,
	}, nil
}

func (s *Server) handleApplyDiff(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path      string `json:"path"`
		Diff      string `json:"diff"`
		SessionID string `json:"session_id,omitempty"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	fullPath := filepath.Join(s.rootDir, input.Path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, NewError(FileNotFound, "Failed to read file", err.Error())
	}

	newContent, err := ApplyUnifiedDiff(string(content), input.Diff)
	if err != nil {
		return nil, NewError(ParseFailure, "Failed to apply diff", err.Error())
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return nil, NewError(InternalError, "Failed to write file", err.Error())
	}

	return map[string]interface{}{
		"success": true,
		"path":    input.Path,
	}, nil
}

func (s *Server) handleApplyDiffSafe(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path        string `json:"path"`
		Diff        string `json:"diff"`
		SessionID   string `json:"session_id,omitempty"`
		RunTests    bool   `json:"run_tests,omitempty"`
		TestCommand string `json:"test_command,omitempty"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	checkpointResult, err := s.handleGitCheckpoint(ctx, json.RawMessage(`{"message":"checkpoint before safe diff"}`))
	if err != nil {
		return nil, NewError(GitConflict, "Failed to create checkpoint before diff", err.Error())
	}
	checkpointData := checkpointResult.(map[string]interface{})
	checkpointHash := checkpointData["commit_hash"].(string)

	_, err = s.handleApplyDiff(ctx, params)
	if err != nil {
		_, rollbackErr := s.handleGitRollback(ctx, nil)
		if rollbackErr != nil {
			return nil, NewError(InternalError, "Failed to apply diff and rollback",
				fmt.Sprintf("apply error: %v, rollback error: %v", err, rollbackErr))
		}
		return nil, err
	}

	if !input.RunTests {
		return map[string]interface{}{
			"success":      true,
			"path":         input.Path,
			"checkpoint":   checkpointHash,
			"tests_run":    false,
			"tests_passed": true,
			"rolled_back":  false,
		}, nil
	}

	testResult, err := s.handleRunTests(ctx, nil)
	if err != nil {
		_, rollbackErr := s.handleGitRollback(ctx, nil)
		if rollbackErr != nil {
			return nil, NewError(InternalError, "Tests failed and rollback failed",
				fmt.Sprintf("test error: %v, rollback error: %v", err, rollbackErr))
		}
		return map[string]interface{}{
			"success":      false,
			"path":         input.Path,
			"checkpoint":   checkpointHash,
			"tests_run":    true,
			"tests_passed": false,
			"rolled_back":  true,
			"error":        fmt.Sprintf("Tests failed after diff, rolled back: %v", err),
		}, nil
	}

	testData := testResult.(map[string]interface{})
	testsPassed := testData["success"].(bool)

	if !testsPassed {
		_, rollbackErr := s.handleGitRollback(ctx, nil)
		if rollbackErr != nil {
			return map[string]interface{}{
				"success":      false,
				"path":         input.Path,
				"checkpoint":   checkpointHash,
				"tests_run":    true,
				"tests_passed": false,
				"rolled_back":  false,
				"error":        fmt.Sprintf("Tests failed and rollback failed: %v", rollbackErr),
				"test_output":  testData["output"],
			}, nil
		}
		return map[string]interface{}{
			"success":      false,
			"path":         input.Path,
			"checkpoint":   checkpointHash,
			"tests_run":    true,
			"tests_passed": false,
			"rolled_back":  true,
			"test_output":  testData["output"],
			"error":        "Tests failed after diff, automatically rolled back",
		}, nil
	}

	return map[string]interface{}{
		"success":      true,
		"path":         input.Path,
		"checkpoint":   checkpointHash,
		"tests_run":    true,
		"tests_passed": true,
		"rolled_back":  false,
		"test_output":  testData["output"],
	}, nil
}

func (s *Server) handleRunTests(ctx context.Context, params json.RawMessage) (interface{}, error) {
	result, err := s.runner.RunTests()
	if err != nil {
		return nil, NewError(InternalError, "Failed to run tests", err.Error())
	}

	return map[string]interface{}{
		"success":   result.Success,
		"output":    result.Output,
		"duration":  result.Duration.Milliseconds(),
		"command":   result.Command,
		"exit_code": result.ExitCode,
	}, nil
}

func (s *Server) handleGitCheckpoint(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Message string `json:"message"`
	}
	json.Unmarshal(params, &input)
	if input.Message == "" {
		input.Message = "checkpoint before refactoring"
	}

	if err := s.gitMgr.CreateCheckpoint(input.Message); err != nil {
		return nil, NewError(GitConflict, "Failed to create checkpoint", err.Error())
	}

	commit, err := s.gitMgr.CurrentCommit()
	if err != nil {
		return nil, NewError(InternalError, "Failed to get commit hash", err.Error())
	}

	return map[string]interface{}{
		"success":     true,
		"commit_hash": commit,
	}, nil
}

func (s *Server) handleGitRollback(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if err := s.gitMgr.Rollback(); err != nil {
		return nil, NewError(GitConflict, "Failed to rollback", err.Error())
	}

	return map[string]interface{}{
		"success": true,
	}, nil
}

func (s *Server) handleListFiles(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		IncludePatterns []string `json:"include_patterns,omitempty"`
		ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	}
	json.Unmarshal(params, &input)

	w := walker.New(input.ExcludePatterns, input.IncludePatterns)
	files, err := w.Walk(s.rootDir)
	if err != nil {
		return nil, NewError(InternalError, "Failed to list files", err.Error())
	}

	fileList := make([]map[string]interface{}, len(files))
	for i, f := range files {
		fileList[i] = map[string]interface{}{
			"path":     f.Path,
			"hash":     f.Hash,
			"language": s.walker.DetectLanguage(f.Path),
			"size":     len(f.Content),
		}
	}

	return map[string]interface{}{
		"files":    fileList,
		"total":    len(files),
		"root_dir": s.rootDir,
	}, nil
}

func (s *Server) handleGetComplexity(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content,omitempty"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, NewError(InvalidParams, "Invalid params", err.Error())
	}

	var content string
	if input.Content != "" {
		content = input.Content
	} else {
		fullPath := filepath.Join(s.rootDir, input.Path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, NewError(FileNotFound, "Failed to read file", err.Error())
		}
		content = string(data)
	}

	metrics := s.calculateComplexity(content)

	return map[string]interface{}{
		"path":    input.Path,
		"metrics": metrics,
	}, nil
}

func (s *Server) calculateComplexity(content string) models.ComplexityMetrics {
	lines := strings.Split(content, "\n")
	metrics := models.ComplexityMetrics{
		LinesOfCode: len(lines),
	}

	complexityKeywords := []string{
		"if ", "else if", "elif ", "else:",
		"for ", "while ", "case ",
		"switch ", "try:", "except ",
		"catch ", "finally:",
	}

	nesting := 0
	for _, line := range lines {
		stripped := strings.TrimSpace(line)

		for _, kw := range complexityKeywords {
			if strings.HasPrefix(stripped, kw) {
				metrics.CyclomaticComplexity++
			}
		}

		if strings.HasPrefix(stripped, "if ") || strings.HasPrefix(stripped, "elif ") ||
			strings.HasPrefix(stripped, "for ") || strings.HasPrefix(stripped, "while ") {
			metrics.CognitiveComplexity += 1 + nesting
			nesting++
		}

		if strings.Contains(stripped, " and ") || strings.Contains(stripped, " or ") ||
			strings.Contains(stripped, "&&") || strings.Contains(stripped, "||") {
			metrics.CyclomaticComplexity++
		}
	}

	return metrics
}

func (s *Server) Shutdown() {
	if s.lspMgr != nil {
		s.lspMgr.Shutdown()
	}
}
