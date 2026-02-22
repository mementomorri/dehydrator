package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp")
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.path != "/tmp" {
		t.Errorf("expected path /tmp, got %s", mgr.path)
	}
}

func TestIsRepo(t *testing.T) {
	t.Run("not a repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)

		if mgr.IsRepo() {
			t.Error("expected IsRepo to return false for non-git directory")
		}
	})

	t.Run("is a repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		mgr := NewManager(tmpDir)
		if !mgr.IsRepo() {
			t.Error("expected IsRepo to return true for git directory")
		}
	})
}

func TestIsClean(t *testing.T) {
	t.Run("clean repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("test.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mgr := NewManager(tmpDir)
		clean, err := mgr.IsClean()
		if err != nil {
			t.Fatalf("IsClean returned error: %v", err)
		}
		if !clean {
			t.Error("expected repo to be clean after commit")
		}
	})

	t.Run("dirty repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "untracked.txt"), []byte("untracked"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		mgr := NewManager(tmpDir)
		clean, err := mgr.IsClean()
		if err != nil {
			t.Fatalf("IsClean returned error: %v", err)
		}
		if clean {
			t.Error("expected repo to be dirty with untracked file")
		}
	})
}

func TestCurrentBranch(t *testing.T) {
	t.Run("no commits", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		mgr := NewManager(tmpDir)
		_, err = mgr.CurrentBranch()
		if err == nil {
			t.Error("expected error for repo with no commits")
		}
	})

	t.Run("with commits", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("test.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mgr := NewManager(tmpDir)
		branch, err := mgr.CurrentBranch()
		if err != nil {
			t.Fatalf("CurrentBranch returned error: %v", err)
		}
		if branch != "master" && branch != "main" {
			t.Errorf("expected branch master or main, got %s", branch)
		}
	})
}

func TestCurrentCommit(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = wt.Add("test.txt")
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@test.com",
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	mgr := NewManager(tmpDir)
	commit, err := mgr.CurrentCommit()
	if err != nil {
		t.Fatalf("CurrentCommit returned error: %v", err)
	}
	if len(commit) != 8 {
		t.Errorf("expected 8 character commit hash, got %d", len(commit))
	}
}

func TestCreateCheckpoint(t *testing.T) {
	t.Run("with changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "initial.txt"), []byte("initial"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("initial.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644)
		if err != nil {
			t.Fatalf("failed to write new file: %v", err)
		}

		mgr := NewManager(tmpDir)
		err = mgr.CreateCheckpoint("test checkpoint")
		if err != nil {
			t.Fatalf("CreateCheckpoint returned error: %v", err)
		}

		clean, err := mgr.IsClean()
		if err != nil {
			t.Fatalf("IsClean returned error: %v", err)
		}
		if !clean {
			t.Error("expected repo to be clean after checkpoint")
		}
	})
}

func TestRollback(t *testing.T) {
	t.Run("single commit", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "initial.txt"), []byte("initial"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("initial.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mgr := NewManager(tmpDir)
		err = mgr.Rollback()
		if err == nil {
			t.Error("expected error when rolling back single commit")
		}
	})

	t.Run("multiple commits", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "initial.txt"), []byte("initial"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("initial.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "second.txt"), []byte("second"), 0644)
		if err != nil {
			t.Fatalf("failed to write second file: %v", err)
		}

		_, err = wt.Add("second.txt")
		if err != nil {
			t.Fatalf("failed to add second file: %v", err)
		}

		_, err = wt.Commit("second", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mgr := NewManager(tmpDir)
		err = mgr.Rollback()
		if err != nil {
			t.Fatalf("Rollback returned error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(tmpDir, "second.txt")); !os.IsNotExist(err) {
			t.Error("expected second.txt to be removed after rollback")
		}
	})
}

func TestChangedFiles(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		wt, err := repo.Worktree()
		if err != nil {
			t.Fatalf("failed to get worktree: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "initial.txt"), []byte("initial"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		_, err = wt.Add("initial.txt")
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}

		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
			},
		})
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mgr := NewManager(tmpDir)
		files, err := mgr.ChangedFiles()
		if err != nil {
			t.Fatalf("ChangedFiles returned error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected 0 changed files, got %d", len(files))
		}
	})

	t.Run("with changes", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := git.PlainInit(tmpDir, false)
		if err != nil {
			t.Fatalf("failed to init repo: %v", err)
		}

		err = os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		mgr := NewManager(tmpDir)
		files, err := mgr.ChangedFiles()
		if err != nil {
			t.Fatalf("ChangedFiles returned error: %v", err)
		}
		if len(files) == 0 {
			t.Error("expected changed files to include new.txt")
		}
	})
}
