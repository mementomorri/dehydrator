package sidecar

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/alexkarsten/dehydrate/pkg/models"
)

type Manager struct {
	cfg     *models.Config
	client  *Client
	process *os.Process
}

func NewManager(cfg *models.Config) *Manager {
	return &Manager{
		cfg:    cfg,
		client: NewClient(cfg.Sidecar.Port),
	}
}

func (m *Manager) Start() error {
	if err := m.client.Health(); err == nil {
		return nil
	}

	if err := m.checkPythonInstalled(); err != nil {
		return err
	}

	sidecarPath := m.findSidecarPath()
	if sidecarPath == "" {
		return fmt.Errorf("could not find ai_sidecar module")
	}

	cmd := exec.Command(
		"python3",
		"-m", "ai_sidecar",
		"--port", fmt.Sprintf("%d", m.cfg.Sidecar.Port),
	)
	cmd.Dir = sidecarPath
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sidecar: %w", err)
	}

	m.process = cmd.Process

	timeout := time.Duration(m.cfg.Sidecar.StartupTimeout) * time.Second
	if err := m.waitForReady(timeout); err != nil {
		m.Stop()
		return fmt.Errorf("sidecar failed to start: %w", err)
	}

	return nil
}

func (m *Manager) Stop() {
	if m.process != nil {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(m.cfg.Sidecar.ShutdownTimeout)*time.Second)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- m.client.Shutdown()
		}()

		select {
		case <-ctx.Done():
			if runtime.GOOS == "windows" {
				m.process.Kill()
			} else {
				syscall.Kill(-m.process.Pid, syscall.SIGTERM)
			}
		case <-done:
		}

		m.process.Wait()
		m.process = nil
	}
}

func (m *Manager) Client() *Client {
	return m.client
}

func (m *Manager) checkPythonInstalled() error {
	cmd := exec.Command("python3", "--version")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("python", "--version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("python3 is not installed or not in PATH")
		}
	}
	return nil
}

func (m *Manager) findSidecarPath() string {
	candidates := []string{
		"python",
		"../python",
		"../../python",
	}

	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, "python"),
			filepath.Join(execDir, "../python"),
			filepath.Join(execDir, "../../python"),
		)
	}

	for _, candidate := range candidates {
		absPath, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}

		if m.isValidSidecarPath(absPath) {
			return absPath
		}
	}

	return ""
}

func (m *Manager) isValidSidecarPath(path string) bool {
	initPath := filepath.Join(path, "ai_sidecar", "__init__.py")
	if _, err := os.Stat(initPath); err == nil {
		return true
	}

	mainPath := filepath.Join(path, "ai_sidecar", "main.py")
	if _, err := os.Stat(mainPath); err == nil {
		return true
	}

	return false
}

func (m *Manager) waitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for sidecar to be ready")
		case <-ticker.C:
			if err := m.client.Health(); err == nil {
				return nil
			}
		}
	}
}

func (m *Manager) IsRunning() bool {
	if m.process == nil {
		return false
	}

	if runtime.GOOS == "windows" {
		return m.process.Signal(syscall.Signal(0)) == nil
	}

	return m.process.Signal(syscall.Signal(0)) == nil
}

func (m *Manager) GetPythonVersion() (string, error) {
	cmd := exec.Command("python3", "--version")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("python", "--version")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(output)), nil
}
