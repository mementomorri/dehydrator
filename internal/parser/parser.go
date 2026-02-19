package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexkarsten/dehydrate/pkg/models"
)

type Parser struct{}

func New() *Parser {
	return &Parser{}
}

type ParseResult struct {
	Symbols []models.Symbol
	Imports []string
	Exports []string
}

func (p *Parser) Parse(content string, language models.Language) (*ParseResult, error) {
	switch language {
	case models.LanguagePython:
		return p.parsePython(content), nil
	case models.LanguageJavaScript, models.LanguageTypeScript:
		return p.parseJS(content), nil
	case models.LanguageGo:
		return p.parseGo(content), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func (p *Parser) parsePython(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	symbols := []models.Symbol{}
	imports := []string{}
	exports := []string{}

	funcRegex := regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)`)
	classRegex := regexp.MustCompile(`^class\s+(\w+)`)
	importRegex := regexp.MustCompile(`^(?:from\s+\S+\s+)?import\s+.+`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			endLine := p.findPythonBlockEnd(lines, i)
			symbols = append(symbols, models.Symbol{
				Name:      matches[1],
				Type:      "function",
				StartLine: i + 1,
				EndLine:   endLine,
				Signature: "(" + matches[2] + ")",
			})
		}

		if matches := classRegex.FindStringSubmatch(trimmed); matches != nil {
			endLine := p.findPythonBlockEnd(lines, i)
			symbols = append(symbols, models.Symbol{
				Name:      matches[1],
				Type:      "class",
				StartLine: i + 1,
				EndLine:   endLine,
			})
		}

		if importRegex.MatchString(trimmed) {
			imports = append(imports, trimmed)
		}
	}

	return &ParseResult{
		Symbols: symbols,
		Imports: imports,
		Exports: exports,
	}
}

func (p *Parser) parseJS(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	symbols := []models.Symbol{}
	imports := []string{}
	exports := []string{}

	funcRegex := regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?(?:function|\())`)
	classRegex := regexp.MustCompile(`^class\s+(\w+)`)
	importRegex := regexp.MustCompile(`^import\s+.+`)
	exportRegex := regexp.MustCompile(`^export\s+(?:default\s+)?(?:class|function|const|let|var)?`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			name := matches[1]
			if name == "" {
				name = matches[2]
			}
			if name != "" {
				endLine := p.findJSBlockEnd(lines, i)
				symbols = append(symbols, models.Symbol{
					Name:      name,
					Type:      "function",
					StartLine: i + 1,
					EndLine:   endLine,
				})
			}
		}

		if matches := classRegex.FindStringSubmatch(trimmed); matches != nil {
			endLine := p.findJSBlockEnd(lines, i)
			symbols = append(symbols, models.Symbol{
				Name:      matches[1],
				Type:      "class",
				StartLine: i + 1,
				EndLine:   endLine,
			})
		}

		if importRegex.MatchString(trimmed) {
			imports = append(imports, trimmed)
		}

		if exportRegex.MatchString(trimmed) {
			exports = append(exports, trimmed)
		}
	}

	return &ParseResult{
		Symbols: symbols,
		Imports: imports,
		Exports: exports,
	}
}

func (p *Parser) parseGo(content string) *ParseResult {
	lines := strings.Split(content, "\n")
	symbols := []models.Symbol{}
	imports := []string{}
	exports := []string{}

	funcRegex := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s*)?(\w+)\s*\(`)
	structRegex := regexp.MustCompile(`^type\s+(\w+)\s+struct`)
	importRegex := regexp.MustCompile(`^import\s+(?:\(|"?[^"]+"?)`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if matches := funcRegex.FindStringSubmatch(trimmed); matches != nil {
			endLine := p.findGoBlockEnd(lines, i)
			symbols = append(symbols, models.Symbol{
				Name:      matches[1],
				Type:      "function",
				StartLine: i + 1,
				EndLine:   endLine,
			})
		}

		if matches := structRegex.FindStringSubmatch(trimmed); matches != nil {
			endLine := p.findGoBlockEnd(lines, i)
			symbols = append(symbols, models.Symbol{
				Name:      matches[1],
				Type:      "struct",
				StartLine: i + 1,
				EndLine:   endLine,
			})
		}

		if importRegex.MatchString(trimmed) {
			imports = append(imports, trimmed)
		}
	}

	return &ParseResult{
		Symbols: symbols,
		Imports: imports,
		Exports: exports,
	}
}

func (p *Parser) findPythonBlockEnd(lines []string, start int) int {
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

func (p *Parser) findJSBlockEnd(lines []string, start int) int {
	braceCount := 0
	started := false

	for i := start; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
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

func (p *Parser) findGoBlockEnd(lines []string, start int) int {
	return p.findJSBlockEnd(lines, start)
}

func (p *Parser) CalculateComplexity(content string) models.ComplexityMetrics {
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

	cognitiveKeywords := []string{
		"if ", "elif ", "else if", "for ", "while ",
	}

	nesting := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		for _, kw := range complexityKeywords {
			if strings.HasPrefix(trimmed, kw) {
				metrics.CyclomaticComplexity++
			}
		}

		for _, kw := range cognitiveKeywords {
			if strings.HasPrefix(trimmed, kw) {
				metrics.CognitiveComplexity += 1 + nesting
				nesting++
			}
		}

		if strings.Contains(trimmed, " and ") || strings.Contains(trimmed, " or ") ||
			strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||") {
			metrics.CyclomaticComplexity++
		}
	}

	return metrics
}

func (p *Parser) FindBlocks(content string, language models.Language) []models.CodeBlock {
	result, _ := p.Parse(content, language)
	blocks := []models.CodeBlock{}

	lines := strings.Split(content, "\n")

	for _, sym := range result.Symbols {
		start := sym.StartLine - 1
		if start < 0 {
			start = 0
		}
		end := sym.EndLine
		if end > len(lines) {
			end = len(lines)
		}

		blockContent := strings.Join(lines[start:end], "\n")

		blocks = append(blocks, models.CodeBlock{
			File:       "",
			StartLine:  sym.StartLine,
			EndLine:    sym.EndLine,
			Content:    blockContent,
			Language:   language,
			SymbolType: sym.Type,
			SymbolName: sym.Name,
			Metrics:    p.CalculateComplexity(blockContent),
		})
	}

	return blocks
}
