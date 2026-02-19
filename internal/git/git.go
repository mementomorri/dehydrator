package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexkarsten/dehydrate/pkg/models"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Manager struct {
	path string
	repo *git.Repository
}

func NewManager(path string) *Manager {
	return &Manager{path: path}
}

func (m *Manager) open() error {
	if m.repo != nil {
		return nil
	}

	repo, err := git.PlainOpen(m.path)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	m.repo = repo
	return nil
}

func (m *Manager) IsClean() (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	return status.IsClean(), nil
}

func (m *Manager) IsRepo() bool {
	gitDir := filepath.Join(m.path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (m *Manager) CurrentBranch() (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}

	ref, err := m.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Name().Short(), nil
}

func (m *Manager) CurrentCommit() (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}

	ref, err := m.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return ref.Hash().String()[:8], nil
}

func (m *Manager) CreateCheckpoint(message string) error {
	if err := m.open(); err != nil {
		return err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	for file := range status {
		_, err := wt.Add(file)
		if err != nil {
			return fmt.Errorf("failed to stage %s: %w", file, err)
		}
	}

	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "DeHydrator",
			Email: "dehydrator@local",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

func (m *Manager) Commit(message string, changes []models.FileChange) error {
	if err := m.open(); err != nil {
		return err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	for _, change := range changes {
		_, err := wt.Add(change.Path)
		if err != nil {
			return fmt.Errorf("failed to stage %s: %w", change.Path, err)
		}
	}

	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "DeHydrator",
			Email: "dehydrator@local",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

func (m *Manager) Rollback() error {
	if err := m.open(); err != nil {
		return err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	ref, err := m.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := m.repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	if len(commit.ParentHashes) == 0 {
		return fmt.Errorf("no parent commit to rollback to")
	}

	parentHash := commit.ParentHashes[0]
	err = wt.Reset(&git.ResetOptions{
		Commit: parentHash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	return nil
}

func (m *Manager) Stash() error {
	if err := m.open(); err != nil {
		return err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() {
		return nil
	}

	for file := range status {
		_, err := wt.Add(file)
		if err != nil {
			return fmt.Errorf("failed to stage %s: %w", file, err)
		}
	}

	_, err = wt.Commit("WIP: stash before dehydrate", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "DeHydrator",
			Email: "dehydrator@local",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to stash commit: %w", err)
	}

	return nil
}

func (m *Manager) Diff(file string) (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}

	ref, err := m.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := m.repo.CommitObject(ref.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit: %w", err)
	}

	prevCommit := commit
	if len(commit.ParentHashes) > 0 {
		prevCommit, err = m.repo.CommitObject(commit.ParentHashes[0])
		if err != nil {
			return "", fmt.Errorf("failed to get parent commit: %w", err)
		}
	}

	patch, err := prevCommit.Patch(commit)
	if err != nil {
		return "", fmt.Errorf("failed to generate patch: %w", err)
	}

	if file != "" {
		var filtered strings.Builder
		for _, filePatch := range patch.FilePatches() {
			from, to := filePatch.Files()
			if (from != nil && from.Path() == file) || (to != nil && to.Path() == file) {
				for _, chunk := range filePatch.Chunks() {
					filtered.WriteString(chunk.Content())
				}
			}
		}
		return filtered.String(), nil
	}

	return patch.String(), nil
}

func (m *Manager) ChangedFiles() ([]string, error) {
	if err := m.open(); err != nil {
		return nil, err
	}

	wt, err := m.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var files []string
	for file := range status {
		files = append(files, file)
	}

	return files, nil
}

func (m *Manager) GetFileAtCommit(file string, hash plumbing.Hash) (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}

	commit, err := m.repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get tree: %w", err)
	}

	entry, err := tree.FindEntry(file)
	if err != nil {
		return "", fmt.Errorf("file not found in commit: %w", err)
	}

	blob, err := m.repo.BlobObject(entry.Hash)
	if err != nil {
		return "", fmt.Errorf("failed to get blob: %w", err)
	}

	reader, err := blob.Reader()
	if err != nil {
		return "", fmt.Errorf("failed to get reader: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read blob: %w", err)
	}

	return string(content), nil
}
