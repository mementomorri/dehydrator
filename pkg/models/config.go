package models

type ModelTier string

const (
	ModelTierLight  ModelTier = "light"
	ModelTierMedium ModelTier = "medium"
	ModelTierHeavy  ModelTier = "heavy"
)

type ModelConfig struct {
	LocalModel  string `mapstructure:"local_model" yaml:"local_model"`
	RemoteModel string `mapstructure:"remote_model" yaml:"remote_model"`
	Provider    string `mapstructure:"provider" yaml:"provider"`
	APIKey      string `mapstructure:"api_key" yaml:"api_key"`
	BaseURL     string `mapstructure:"base_url" yaml:"base_url"`
}

type ModelsConfig struct {
	Light  ModelConfig `mapstructure:"light" yaml:"light"`
	Medium ModelConfig `mapstructure:"medium" yaml:"medium"`
	Heavy  ModelConfig `mapstructure:"heavy" yaml:"heavy"`
}

type SidecarConfig struct {
	Port            int  `mapstructure:"port" yaml:"port"`
	StartupTimeout  int  `mapstructure:"startup_timeout" yaml:"startup_timeout"`
	ShutdownTimeout int  `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	AutoInstall     bool `mapstructure:"auto_install" yaml:"auto_install"`
}

type ComplexityThresholds struct {
	CyclomaticComplexity int `mapstructure:"cyclomatic_complexity" yaml:"cyclomatic_complexity"`
	CognitiveComplexity  int `mapstructure:"cognitive_complexity" yaml:"cognitive_complexity"`
	LinesOfCode          int `mapstructure:"lines_of_code" yaml:"lines_of_code"`
}

type Config struct {
	Models               ModelsConfig         `mapstructure:"models" yaml:"models"`
	Sidecar              SidecarConfig        `mapstructure:"sidecar" yaml:"sidecar"`
	ComplexityThresholds ComplexityThresholds `mapstructure:"complexity_thresholds" yaml:"complexity_thresholds"`
	PreApprove           bool                 `mapstructure:"pre_approve" yaml:"pre_approve"`
	CommitChanges        bool                 `mapstructure:"commit_changes" yaml:"commit_changes"`
	Report               bool                 `mapstructure:"report" yaml:"report"`
	OutputFormat         string               `mapstructure:"output_format" yaml:"output_format"`
	ExcludePatterns      []string             `mapstructure:"exclude_patterns" yaml:"exclude_patterns"`
	IncludePatterns      []string             `mapstructure:"include_patterns" yaml:"include_patterns"`
}
