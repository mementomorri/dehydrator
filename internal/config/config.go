package config

import (
	"os"
	"path/filepath"

	"github.com/alexkarsten/dehydrate/pkg/models"
	"github.com/spf13/viper"
)

const (
	DefaultConfigName = ".dehydrate"
	DefaultConfigType = "yaml"
)

func DefaultConfig() *models.Config {
	return &models.Config{
		Models: models.ModelsConfig{
			Light: models.ModelConfig{
				LocalModel:  "llama3.2:3b",
				RemoteModel: "gpt-4o-mini",
				Provider:    "ollama",
			},
			Medium: models.ModelConfig{
				LocalModel:  "qwen2.5:32b",
				RemoteModel: "claude-3-5-sonnet-20241022",
				Provider:    "ollama",
			},
			Heavy: models.ModelConfig{
				LocalModel:  "deepseek-coder-v2",
				RemoteModel: "claude-3-5-sonnet-20241022",
				Provider:    "ollama",
			},
		},
		Sidecar: models.SidecarConfig{
			Port:            9876,
			StartupTimeout:  30,
			ShutdownTimeout: 5,
			AutoInstall:     true,
		},
		ComplexityThresholds: models.ComplexityThresholds{
			CyclomaticComplexity: 10,
			CognitiveComplexity:  15,
			LinesOfCode:          50,
		},
		PreApprove:      false,
		CommitChanges:   false,
		Report:          false,
		OutputFormat:    "markdown",
		ExcludePatterns: []string{".git", "node_modules", "venv", "__pycache__", "vendor", "dist", "build"},
		IncludePatterns: []string{"*.py", "*.js", "*.ts", "*.go", "*.java"},
	}
}

func Load(configPath string) (*models.Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigType(DefaultConfigType)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName(DefaultConfigName)
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME")
	}

	setDefaults(v, cfg)

	v.SetEnvPrefix("DEHYDRATE")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper, cfg *models.Config) {
	v.SetDefault("models.light.local_model", cfg.Models.Light.LocalModel)
	v.SetDefault("models.light.remote_model", cfg.Models.Light.RemoteModel)
	v.SetDefault("models.light.provider", cfg.Models.Light.Provider)

	v.SetDefault("models.medium.local_model", cfg.Models.Medium.LocalModel)
	v.SetDefault("models.medium.remote_model", cfg.Models.Medium.RemoteModel)
	v.SetDefault("models.medium.provider", cfg.Models.Medium.Provider)

	v.SetDefault("models.heavy.local_model", cfg.Models.Heavy.LocalModel)
	v.SetDefault("models.heavy.remote_model", cfg.Models.Heavy.RemoteModel)
	v.SetDefault("models.heavy.provider", cfg.Models.Heavy.Provider)

	v.SetDefault("sidecar.port", cfg.Sidecar.Port)
	v.SetDefault("sidecar.startup_timeout", cfg.Sidecar.StartupTimeout)
	v.SetDefault("sidecar.shutdown_timeout", cfg.Sidecar.ShutdownTimeout)
	v.SetDefault("sidecar.auto_install", cfg.Sidecar.AutoInstall)

	v.SetDefault("complexity_thresholds.cyclomatic_complexity", cfg.ComplexityThresholds.CyclomaticComplexity)
	v.SetDefault("complexity_thresholds.cognitive_complexity", cfg.ComplexityThresholds.CognitiveComplexity)
	v.SetDefault("complexity_thresholds.lines_of_code", cfg.ComplexityThresholds.LinesOfCode)

	v.SetDefault("pre_approve", cfg.PreApprove)
	v.SetDefault("commit_changes", cfg.CommitChanges)
	v.SetDefault("report", cfg.Report)
	v.SetDefault("output_format", cfg.OutputFormat)
	v.SetDefault("exclude_patterns", cfg.ExcludePatterns)
	v.SetDefault("include_patterns", cfg.IncludePatterns)
}

func Save(cfg *models.Config, path string) error {
	v := viper.New()
	v.SetConfigType(DefaultConfigType)

	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, DefaultConfigName+".yaml")
	}

	v.SetConfigFile(path)

	v.Set("models", cfg.Models)
	v.Set("sidecar", cfg.Sidecar)
	v.Set("complexity_thresholds", cfg.ComplexityThresholds)
	v.Set("pre_approve", cfg.PreApprove)
	v.Set("commit_changes", cfg.CommitChanges)
	v.Set("report", cfg.Report)
	v.Set("output_format", cfg.OutputFormat)
	v.Set("exclude_patterns", cfg.ExcludePatterns)
	v.Set("include_patterns", cfg.IncludePatterns)

	return v.WriteConfig()
}
