package walker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexkarsten/dehydrate/pkg/models"
)

type Walker struct {
	excludePatterns []string
	includePatterns []string
}

func New(excludePatterns, includePatterns []string) *Walker {
	return &Walker{
		excludePatterns: excludePatterns,
		includePatterns: includePatterns,
	}
}

func (w *Walker) Walk(root string) ([]models.FileInfo, error) {
	var files []models.FileInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if w.shouldExcludeDir(path) {
				return fs.SkipDir
			}
			return nil
		}

		if w.shouldExcludeFile(path) {
			return nil
		}

		if !w.shouldIncludeFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}

		hash := sha256.Sum256(content)

		files = append(files, models.FileInfo{
			Path:    relPath,
			Content: string(content),
			Hash:    hex.EncodeToString(hash[:]),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return files, nil
}

func (w *Walker) shouldExcludeDir(path string) bool {
	name := filepath.Base(path)
	for _, pattern := range w.excludePatterns {
		if name == pattern {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (w *Walker) shouldExcludeFile(path string) bool {
	name := filepath.Base(path)

	if strings.HasPrefix(name, ".") && name != ".gitignore" && name != ".env.example" {
		return true
	}

	excludeExts := []string{".min.js", ".min.css", ".lock", ".sum"}
	for _, ext := range excludeExts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}

	binaryExts := []string{".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2", ".ttf", ".eot", ".pdf", ".zip", ".tar", ".gz", ".so", ".dll", ".dylib", ".exe", ".bin"}
	ext := strings.ToLower(filepath.Ext(name))
	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	return false
}

func (w *Walker) shouldIncludeFile(path string) bool {
	if len(w.includePatterns) == 0 {
		return true
	}

	ext := filepath.Ext(path)
	for _, pattern := range w.includePatterns {
		if strings.HasPrefix(pattern, "*") {
			if ext == strings.TrimPrefix(pattern, "*") {
				return true
			}
		} else if strings.HasSuffix(path, pattern) {
			return true
		}
	}

	return false
}

func (w *Walker) DetectLanguage(path string) models.Language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".py":
		return models.LanguagePython
	case ".js":
		return models.LanguageJavaScript
	case ".ts", ".tsx":
		return models.LanguageTypeScript
	case ".go":
		return models.LanguageGo
	default:
		return models.LanguageUnknown
	}
}

func (w *Walker) CountLines(content string) int {
	return strings.Count(content, "\n") + 1
}

func (w *Walker) GetProjectStats(root string) (*ProjectStats, error) {
	files, err := w.Walk(root)
	if err != nil {
		return nil, err
	}

	stats := &ProjectStats{
		TotalFiles: len(files),
		ByLanguage: make(map[models.Language]int),
	}

	for _, f := range files {
		lang := w.DetectLanguage(f.Path)
		stats.ByLanguage[lang]++
		stats.TotalLines += w.CountLines(f.Content)
	}

	return stats, nil
}

type ProjectStats struct {
	TotalFiles int
	TotalLines int
	ByLanguage map[models.Language]int
}
