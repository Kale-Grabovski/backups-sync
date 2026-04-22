package config

import "time"

type Config struct {
	BackBlaze Backblaze `mapstructure:"backblaze"`
	Backups   Backups   `mapstructure:"backups"`
	DB        DB        `mapstructure:"db"`
}

type Backups struct {
	Inputs        []string      `mapstructure:"inputs"`
	IgnoreChanges []string      `mapstructure:"ignore_changes"`
	Path7z        string        `mapstructure:"path_7z"`
	Pwd           string        `mapstructure:"pwd"`
	Output        string        `mapstructure:"output"`
	Prefix        string        `mapstructure:"prefix"`
	Interval      time.Duration `mapstructure:"interval"`
	RetentionDays int           `mapstructure:"retention_days"`
}

type Backblaze struct {
	ID            string        `mapstructure:"id"`
	Key           string        `mapstructure:"key"`
	Bucket        string        `mapstructure:"bucket"`
	Path          string        `mapstructure:"path"`
	RetentionDays int           `mapstructure:"retention_days"`
	Interval      time.Duration `mapstructure:"interval"`
}

type DB struct {
	ContainerName string        `mapstructure:"container_name"`
	User          string        `mapstructure:"user"`
	Database      string        `mapstructure:"database"`
	Path          string        `mapstructure:"path"`
	DockerPath    string        `mapstructure:"docker_path"`
	Prefix        string        `mapstructure:"prefix"`
	Interval      time.Duration `mapstructure:"interval"`
	RetentionDays int           `mapstructure:"retention_days"`
	Sudo          bool          `mapstructure:"sudo"`
	Immediate     bool          `mapstructure:"immediate"`
}
